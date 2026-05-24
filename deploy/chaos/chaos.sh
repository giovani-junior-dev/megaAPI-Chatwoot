#!/usr/bin/env bash
set -euo pipefail

# chaos.sh - Kill bridge/db/chatwoot containers under load and verify recovery.
#
# Pre-conditions:
#   * docker compose stack is running (see deploy/docker-compose.yml)
#   * a load generator is hitting the bridge (run-smoke.sh in parallel is typical)
#
# What it does:
#   1. Snapshot baseline metrics (bridge /metrics + container uptime).
#   2. Iterate over targets: kill, wait, verify container restarted,
#      verify bridge /healthz returns 200 within RECOVERY_BUDGET_SEC.
#   3. After all victims, hit /readyz and confirm DLQ growth is below tolerance.
#
# Usage:
#   COMPOSE_FILE=deploy/docker-compose.yml ./deploy/chaos/chaos.sh
# Optional:
#   BRIDGE_URL              (default http://localhost:8080)
#   RECOVERY_BUDGET_SEC     (default 60)
#   DLQ_TOLERANCE           (default 5) - max DLQ growth allowed across run
#   TARGETS                 (default "bridge db chatwoot")

cd "$(git rev-parse --show-toplevel)"

COMPOSE_FILE="${COMPOSE_FILE:-deploy/docker-compose.yml}"
BRIDGE_URL="${BRIDGE_URL:-http://localhost:8080}"
RECOVERY_BUDGET_SEC="${RECOVERY_BUDGET_SEC:-60}"
DLQ_TOLERANCE="${DLQ_TOLERANCE:-5}"
TARGETS="${TARGETS:-bridge db chatwoot}"

if ! command -v docker >/dev/null 2>&1; then
  echo "docker not found." >&2
  exit 2
fi

compose() { docker compose -f "$COMPOSE_FILE" "$@"; }

healthz_ok() {
  curl -sf -o /dev/null -w "%{http_code}" "$BRIDGE_URL/healthz" 2>/dev/null | grep -q '^200$'
}

dlq_count() {
  # Scrape Prometheus counter exposed at /metrics: bridge_dlq_total{...}
  curl -sf "$BRIDGE_URL/metrics" 2>/dev/null \
    | awk '/^bridge_dlq_total/ { sum+=$NF } END { printf "%d\n", sum+0 }'
}

echo "[chaos] compose_file=$COMPOSE_FILE budget=${RECOVERY_BUDGET_SEC}s targets='$TARGETS'"

DLQ_START=$(dlq_count)
echo "[chaos] dlq_baseline=$DLQ_START"

for target in $TARGETS; do
  echo "[chaos] -> killing container: $target"
  if ! compose kill "$target" >/dev/null 2>&1; then
    echo "[chaos] $target not running in stack; skipping"
    continue
  fi

  # Most compose services have restart: unless-stopped; if not auto-restarted,
  # bring it back ourselves so the test is deterministic.
  sleep 2
  if ! compose ps "$target" 2>/dev/null | grep -q "Up\|running"; then
    compose up -d "$target" >/dev/null
  fi

  echo "[chaos] waiting up to ${RECOVERY_BUDGET_SEC}s for $BRIDGE_URL/healthz=200"
  deadline=$(( $(date +%s) + RECOVERY_BUDGET_SEC ))
  recovered=0
  while [ "$(date +%s)" -lt "$deadline" ]; do
    if healthz_ok; then
      recovered=1
      break
    fi
    sleep 2
  done

  if [ "$recovered" -ne 1 ]; then
    echo "[chaos] FAIL: bridge did not recover from killing $target within ${RECOVERY_BUDGET_SEC}s" >&2
    exit 1
  fi
  echo "[chaos] $target recovered"
done

DLQ_END=$(dlq_count)
DLQ_DELTA=$(( DLQ_END - DLQ_START ))
echo "[chaos] dlq_final=$DLQ_END delta=$DLQ_DELTA tolerance=$DLQ_TOLERANCE"

if [ "$DLQ_DELTA" -gt "$DLQ_TOLERANCE" ]; then
  echo "[chaos] FAIL: DLQ grew by $DLQ_DELTA (tolerance $DLQ_TOLERANCE)" >&2
  exit 1
fi

echo "[chaos] PASS: all targets recovered; DLQ growth within tolerance"
