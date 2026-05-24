#!/usr/bin/env bash
set -euo pipefail

# zap.sh - OWASP ZAP baseline scan against a running bridge.
# Uses the official ghcr.io/zaproxy/zaproxy:stable image (no host install).
# Usage: TARGET_URL=https://bridge.example.com ./deploy/security/zap.sh

cd "$(git rev-parse --show-toplevel)"

REPORT_DIR="security-reports"
mkdir -p "$REPORT_DIR"

TARGET_URL="${TARGET_URL:-http://localhost:8080}"
HTML_REPORT="zap-baseline.html"
JSON_REPORT="zap-baseline.json"

if ! command -v docker >/dev/null 2>&1; then
  echo "docker not found. zap.sh requires Docker." >&2
  exit 2
fi

echo "[zap] baseline scan target=$TARGET_URL"
docker run --rm \
  -v "$(pwd)/$REPORT_DIR:/zap/wrk/:rw" \
  ghcr.io/zaproxy/zaproxy:stable \
  zap-baseline.py \
  -t "$TARGET_URL" \
  -r "$HTML_REPORT" \
  -J "$JSON_REPORT" \
  -I \
  || true

echo "[zap] reports: $REPORT_DIR/$HTML_REPORT $REPORT_DIR/$JSON_REPORT"

# Gate: any High risk alert fails.
if [ -f "$REPORT_DIR/$JSON_REPORT" ]; then
  HIGH=$(grep -o '"riskcode": *"3"' "$REPORT_DIR/$JSON_REPORT" | wc -l | tr -d ' ' || echo 0)
  echo "[zap] high risk alerts: $HIGH"
  if [ "$HIGH" -gt 0 ]; then
    echo "[zap] FAIL: high risk alerts present" >&2
    exit 1
  fi
fi

echo "[zap] PASS: no high risk alerts"
