#!/usr/bin/env bash
set -euo pipefail

# gosec.sh - Run gosec static security analyzer on the Go codebase.
# Writes JSON + text reports to security-reports/.
# Fails (exit 1) when high or critical severity findings are detected.

cd "$(git rev-parse --show-toplevel)"

REPORT_DIR="security-reports"
mkdir -p "$REPORT_DIR"

if ! command -v gosec >/dev/null 2>&1; then
  echo "gosec not found. Install: go install github.com/securego/gosec/v2/cmd/gosec@latest" >&2
  exit 2
fi

JSON_REPORT="$REPORT_DIR/gosec.json"
TEXT_REPORT="$REPORT_DIR/gosec.txt"

echo "[gosec] scanning ./..."
# -no-fail so we always emit a report; gate on severity below.
gosec -fmt=json -out="$JSON_REPORT" -no-fail ./... >/dev/null
gosec -fmt=text -out="$TEXT_REPORT" -no-fail ./... >/dev/null || true

HIGH=$(grep -o '"severity": *"HIGH"' "$JSON_REPORT" | wc -l | tr -d ' ')
CRIT=$(grep -o '"severity": *"CRITICAL"' "$JSON_REPORT" | wc -l | tr -d ' ')
TOTAL=$(grep -o '"severity": *"' "$JSON_REPORT" | wc -l | tr -d ' ')

echo "[gosec] findings: total=$TOTAL high=$HIGH critical=$CRIT"
echo "[gosec] reports: $JSON_REPORT $TEXT_REPORT"

if [ "$HIGH" -gt 0 ] || [ "$CRIT" -gt 0 ]; then
  echo "[gosec] FAIL: high/critical findings present" >&2
  exit 1
fi

echo "[gosec] PASS: no high/critical findings"
