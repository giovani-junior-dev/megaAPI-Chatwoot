#!/usr/bin/env bash
# Render deploy/templates/*.tpl to deploy/ using DOMAIN, EMAIL, MASTER_KEY,
# POSTGRES_PASSWORD, REDIS_PASSWORD, CHATWOOT_SECRET_KEY_BASE from env.
#
# Usage: DOMAIN=example.com EMAIL=a@b.com ... bash deploy/render-templates.sh

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TPL_DIR="${SCRIPT_DIR}/templates"
OUT_DIR="${SCRIPT_DIR}"

: "${DOMAIN:?DOMAIN is required}"
: "${EMAIL:?EMAIL is required}"
: "${MASTER_KEY:?MASTER_KEY is required}"
: "${POSTGRES_PASSWORD:?POSTGRES_PASSWORD is required}"
: "${REDIS_PASSWORD:?REDIS_PASSWORD is required}"
: "${CHATWOOT_SECRET_KEY_BASE:?CHATWOOT_SECRET_KEY_BASE is required}"

export DOMAIN EMAIL MASTER_KEY POSTGRES_PASSWORD REDIS_PASSWORD CHATWOOT_SECRET_KEY_BASE

# Whitelist vars envsubst expands so $shell_vars in templates survive.
VARS='${DOMAIN} ${EMAIL} ${MASTER_KEY} ${POSTGRES_PASSWORD} ${REDIS_PASSWORD} ${CHATWOOT_SECRET_KEY_BASE}'

render() {
    local src="$1" dst="$2"
    if ! command -v envsubst >/dev/null 2>&1; then
        echo "envsubst missing (apt-get install gettext-base)" >&2
        exit 1
    fi
    envsubst "$VARS" < "$src" > "$dst.tmp"
    mv "$dst.tmp" "$dst"
    chmod 600 "$dst"
    echo "rendered $dst"
}

render "$TPL_DIR/.env.bridge.tpl"    "$OUT_DIR/.env.bridge"
render "$TPL_DIR/.env.chatwoot.tpl"  "$OUT_DIR/.env.chatwoot"

# Caddyfile less sensitive — 644 ok.
envsubst "$VARS" < "$TPL_DIR/Caddyfile.tpl" > "$OUT_DIR/Caddyfile.tmp"
mv "$OUT_DIR/Caddyfile.tmp" "$OUT_DIR/Caddyfile"
chmod 644 "$OUT_DIR/Caddyfile"
echo "rendered $OUT_DIR/Caddyfile"
