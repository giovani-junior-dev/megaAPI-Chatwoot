#!/usr/bin/env bash
set -euo pipefail

# run-smoke.sh - Drive a 1h smoke load test via k6 (Docker) and gate on metrics.
# Usage:
#   BRIDGE_URL=https://bridge TENANT_SLUG=demo WA_TOKEN=xxx ./deploy/loadtest/run-smoke.sh
# Optional:
#   PROFILE=smoke|24h|spike (default smoke)

cd "$(git rev-parse --show-toplevel)"

mkdir -p loadtest-results

if ! command -v docker >/dev/null 2>&1; then
  echo "docker not found. k6 runs via grafana/k6 image." >&2
  exit 2
fi

: "${BRIDGE_URL:?BRIDGE_URL must point at a running bridge}"
: "${TENANT_SLUG:?TENANT_SLUG required}"
: "${WA_TOKEN:?WA_TOKEN required}"

PROFILE="${PROFILE:-smoke}"
SCRIPT="deploy/loadtest/k6-bridge.js"

echo "[loadtest] profile=$PROFILE target=$BRIDGE_URL tenant=$TENANT_SLUG"

docker run --rm \
  -v "$(pwd):/work" \
  -w /work \
  -e BRIDGE_URL="$BRIDGE_URL" \
  -e TENANT_SLUG="$TENANT_SLUG" \
  -e WA_TOKEN="$WA_TOKEN" \
  -e PROFILE="$PROFILE" \
  grafana/k6:latest run "$SCRIPT"

REPORT="loadtest-results/smoke.json"
if [ ! -f "$REPORT" ]; then
  echo "[loadtest] FAIL: $REPORT not produced" >&2
  exit 1
fi

# Extract gating metrics via python (ships with most CI images) or jq.
if command -v jq >/dev/null 2>&1; then
  ERROR_RATE=$(jq '.metrics.http_req_failed.values.rate' "$REPORT")
  P99=$(jq '.metrics.http_req_duration.values["p(99)"]' "$REPORT")
  echo "[loadtest] error_rate=$ERROR_RATE p99_ms=$P99"
fi

echo "[loadtest] report: $REPORT"
echo "[loadtest] PASS (thresholds enforced by k6; non-zero exit on breach)"
