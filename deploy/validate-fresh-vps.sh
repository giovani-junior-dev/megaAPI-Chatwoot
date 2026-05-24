#!/usr/bin/env bash
# validate-fresh-vps.sh — proves install.sh works on a clean Ubuntu 22.04 host.
# Spawns ubuntu:22.04 docker container with docker-in-docker, copies this repo
# in, runs install.sh with synthetic inputs, waits for /healthz, prints
# INSTALL_DURATION_SECONDS=<n>. Exits 0 iff n < MAX_SECONDS (default 900).
#
# Pre-req: docker daemon on host.
#
# Usage: bash deploy/validate-fresh-vps.sh [--max-seconds 900]

set -euo pipefail

MAX_SECONDS=900
DOMAIN="${VALIDATE_DOMAIN:-bridge.local.test}"
EMAIL="${VALIDATE_EMAIL:-ops@local.test}"

while [[ $# -gt 0 ]]; do
    case "$1" in
        --max-seconds) MAX_SECONDS="$2"; shift 2 ;;
        --domain)      DOMAIN="$2";      shift 2 ;;
        --email)       EMAIL="$2";       shift 2 ;;
        *) echo "unknown arg: $1" >&2; exit 2 ;;
    esac
done

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
CTR_NAME="validate-fresh-vps-$$"

cleanup() {
    docker rm -f "$CTR_NAME" >/dev/null 2>&1 || true
}
trap cleanup EXIT

START_TS="$(date +%s)"

echo "[validate] launching ubuntu:22.04 sandbox (container=$CTR_NAME)"
docker run -d \
    --name "$CTR_NAME" \
    --privileged \
    -v /var/run/docker.sock:/var/run/docker.sock \
    -v "$REPO_DIR":/repo:ro \
    ubuntu:22.04 \
    sleep infinity >/dev/null

echo "[validate] bootstrapping deps inside container"
docker exec "$CTR_NAME" bash -c '
    set -euo pipefail
    export DEBIAN_FRONTEND=noninteractive
    apt-get update -qq
    apt-get install -y -qq curl ca-certificates gnupg gettext-base openssl \
        docker.io docker-compose-v2 >/dev/null
    cp -r /repo /work
'

echo "[validate] running install.sh"
docker exec \
    -e NONINTERACTIVE=1 \
    -e DOMAIN="$DOMAIN" \
    -e EMAIL="$EMAIL" \
    -e TLS_MODE=tls \
    "$CTR_NAME" bash -c '
        set -euo pipefail
        cd /work
        bash deploy/install.sh --non-interactive
    '

echo "[validate] running postinstall-check.sh"
docker exec "$CTR_NAME" bash -c "
    set -euo pipefail
    cd /work
    DOMAIN='$DOMAIN' MODE=tls bash deploy/postinstall-check.sh --mode tls --domain '$DOMAIN' || true
"

# /healthz reachable from the sandbox's host network is the gating signal.
echo "[validate] verifying /healthz from inside sandbox"
docker exec "$CTR_NAME" bash -c '
    for i in $(seq 1 60); do
        if curl -fsS http://127.0.0.1:8080/healthz >/dev/null 2>&1; then
            echo "/healthz ok"
            exit 0
        fi
        sleep 5
    done
    echo "/healthz never returned 200" >&2
    exit 1
'

END_TS="$(date +%s)"
DURATION=$((END_TS - START_TS))

printf 'INSTALL_DURATION_SECONDS=%d\n' "$DURATION"

if (( DURATION < MAX_SECONDS )); then
    echo "[validate] PASS ($DURATION s < $MAX_SECONDS s)"
    exit 0
else
    echo "[validate] FAIL ($DURATION s >= $MAX_SECONDS s)" >&2
    exit 1
fi
