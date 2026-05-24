#!/usr/bin/env bash
set -euo pipefail

# nuclei.sh - Run projectdiscovery/nuclei against a running bridge instance.
# Uses the official Docker image so no host install is required.
# Usage: TARGET_URL=https://bridge.example.com ./deploy/security/nuclei.sh

cd "$(git rev-parse --show-toplevel)"

REPORT_DIR="security-reports"
mkdir -p "$REPORT_DIR"

TARGET_URL="${TARGET_URL:-http://localhost:8080}"
TEMPLATES_TAG="${NUCLEI_TAGS:-cves,exposure,misconfig,tech}"
REPORT="$REPORT_DIR/nuclei.json"

if ! command -v docker >/dev/null 2>&1; then
  echo "docker not found. nuclei.sh requires Docker." >&2
  exit 2
fi

echo "[nuclei] target=$TARGET_URL tags=$TEMPLATES_TAG"
docker run --rm \
  -v "$(pwd)/$REPORT_DIR:/reports" \
  projectdiscovery/nuclei:latest \
  -u "$TARGET_URL" \
  -tags "$TEMPLATES_TAG" \
  -severity high,critical \
  -jsonl \
  -o /reports/nuclei.json \
  -stats \
  || true

HITS=$(wc -l < "$REPORT" 2>/dev/null | tr -d ' ' || echo 0)
echo "[nuclei] high/critical hits: $HITS"
echo "[nuclei] report: $REPORT"

if [ "$HITS" -gt 0 ]; then
  echo "[nuclei] FAIL: high/critical findings present" >&2
  exit 1
fi

echo "[nuclei] PASS: no high/critical findings"
