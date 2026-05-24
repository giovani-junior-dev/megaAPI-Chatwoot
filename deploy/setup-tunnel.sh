#!/usr/bin/env bash
# Setup Cloudflare Tunnel as alternative to Caddy automatic HTTPS.
# Idempotent: detects existing tunnel by name and reuses.
#
# Usage: DOMAIN=example.com TUNNEL_NAME=bridge bash deploy/setup-tunnel.sh
# Writes CLOUDFLARE_TUNNEL_TOKEN to deploy/.env.tunnel for compose to pick up.

set -euo pipefail

: "${DOMAIN:?DOMAIN required}"
TUNNEL_NAME="${TUNNEL_NAME:-bridge-${DOMAIN//./-}}"
CFD_DIR="${CFD_DIR:-$HOME/.cloudflared}"
ENV_OUT="${ENV_OUT:-deploy/.env.tunnel}"

install_cloudflared() {
    if command -v cloudflared >/dev/null 2>&1; then
        echo "[tunnel] cloudflared present: $(cloudflared --version 2>&1 | head -1)"
        return
    fi
    echo "[tunnel] installing cloudflared"
    local arch
    arch="$(uname -m)"
    case "$arch" in
        x86_64|amd64) arch=amd64 ;;
        aarch64|arm64) arch=arm64 ;;
        *) echo "unsupported arch: $arch" >&2; exit 1 ;;
    esac
    local url="https://github.com/cloudflare/cloudflared/releases/latest/download/cloudflared-linux-${arch}"
    curl -fsSL "$url" -o /tmp/cloudflared
    chmod +x /tmp/cloudflared
    sudo mv /tmp/cloudflared /usr/local/bin/cloudflared
    cloudflared --version
}

ensure_login() {
    if [[ -f "$CFD_DIR/cert.pem" ]]; then
        echo "[tunnel] already logged in ($CFD_DIR/cert.pem)"
        return
    fi
    echo "[tunnel] launching cloudflared tunnel login (browser flow)"
    cloudflared tunnel login
}

ensure_tunnel() {
    if cloudflared tunnel list 2>/dev/null | awk '{print $2}' | grep -qx "$TUNNEL_NAME"; then
        echo "[tunnel] tunnel '$TUNNEL_NAME' exists"
    else
        echo "[tunnel] creating tunnel '$TUNNEL_NAME'"
        cloudflared tunnel create "$TUNNEL_NAME"
    fi
    cloudflared tunnel route dns "$TUNNEL_NAME" "$DOMAIN" || true
}

write_token() {
    local token
    token="$(cloudflared tunnel token "$TUNNEL_NAME")"
    if [[ -z "$token" ]]; then
        echo "failed to fetch tunnel token" >&2
        exit 1
    fi
    umask 077
    printf 'CLOUDFLARE_TUNNEL_TOKEN=%s\n' "$token" > "$ENV_OUT"
    chmod 600 "$ENV_OUT"
    echo "[tunnel] wrote $ENV_OUT (0600)"
}

install_cloudflared
ensure_login
ensure_tunnel
write_token
echo "[tunnel] done. start container via: docker compose --profile tunnel up -d cloudflared"
