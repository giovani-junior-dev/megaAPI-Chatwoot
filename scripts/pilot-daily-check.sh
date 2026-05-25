#!/usr/bin/env bash
# Pilot daily health snapshot. Run once per day.
# Usage: ./scripts/pilot-daily-check.sh

set -euo pipefail

BRIDGE_URL="${BRIDGE_URL:-http://localhost:8090}"
DAY_LABEL="$(date +%Y-%m-%d)"

printf '== Pilot Day Check — %s ==\n\n' "$DAY_LABEL"

printf '[1] Bridge health:\n'
curl -sS "$BRIDGE_URL/healthz" || true
echo
curl -sS "$BRIDGE_URL/readyz" || true
echo

printf '\n[2] Messages last 24h (by status):\n'
docker compose exec -T db psql -U bridge -d bridge -tA -F '|' -c "
  SELECT status, count(*)
  FROM messages
  WHERE created_at >= NOW() - INTERVAL '1 day'
  GROUP BY status
  ORDER BY status;" || true

printf '\n[3] Stale pending (>1h old, should be ~0 thanks to janitor):\n'
docker compose exec -T db psql -U bridge -d bridge -tA -c "
  SELECT count(*)
  FROM messages
  WHERE status='pending' AND created_at < NOW() - INTERVAL '1 hour';" || true

printf '\n[4] Failed messages last 24h (DLQ growth):\n'
docker compose exec -T db psql -U bridge -d bridge -tA -F '|' -c "
  SELECT external_id, direction, last_error
  FROM messages
  WHERE status='failed' AND created_at >= NOW() - INTERVAL '1 day'
  ORDER BY created_at DESC
  LIMIT 20;" || true

printf '\n[5] Metrics snapshot:\n'
curl -sS "$BRIDGE_URL/metrics" 2>/dev/null | grep -E "^bridge_(messages|queue|job)" | head -20 || true

printf '\n[6] DLQ growth ratio (failed/total last 24h):\n'
docker compose exec -T db psql -U bridge -d bridge -tA -c "
  SELECT
    ROUND(100.0 * SUM(CASE WHEN status='failed' THEN 1 ELSE 0 END) / NULLIF(count(*),0), 3) AS dlq_pct
  FROM messages
  WHERE created_at >= NOW() - INTERVAL '1 day';" || true
echo "(threshold: <0.5%)"

printf '\n== Check complete ==\n'
