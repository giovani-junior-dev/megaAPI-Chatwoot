#!/usr/bin/env bash
# install.sh — one-command installer for chatwoot-megaapi-bridge.
# Idempotent: rerun safe; detects existing state and skips finished steps.
#
# Usage:
#   bash deploy/install.sh                          # interactive
#   DOMAIN=ex.com EMAIL=a@b.com TLS_MODE=tls bash deploy/install.sh
#   bash deploy/install.sh --tls
#   bash deploy/install.sh --tunnel
#
# Env overrides: DOMAIN, EMAIL, TLS_MODE (tls|tunnel), MASTER_KEY,
# POSTGRES_PASSWORD, REDIS_PASSWORD, CHATWOOT_SECRET_KEY_BASE.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
COMPOSE_FILE="$SCRIPT_DIR/docker-compose.yml"
STATE_FILE="$SCRIPT_DIR/.env.local"

TLS_MODE="${TLS_MODE:-}"
NONINTERACTIVE="${NONINTERACTIVE:-0}"

log()  { printf '\033[1;34m[install]\033[0m %s\n' "$*"; }
warn() { printf '\033[1;33m[install]\033[0m %s\n' "$*" >&2; }
die()  { printf '\033[1;31m[install]\033[0m %s\n' "$*" >&2; exit 1; }

parse_args() {
    while [[ $# -gt 0 ]]; do
        case "$1" in
            --tls)    TLS_MODE=tls;    shift ;;
            --tunnel) TLS_MODE=tunnel; shift ;;
            --non-interactive) NONINTERACTIVE=1; shift ;;
            -h|--help)
                sed -n '2,12p' "$0"
                exit 0
                ;;
            *) die "unknown arg: $1" ;;
        esac
    done
}

require_tools() {
    local missing=()
    for t in docker openssl envsubst curl; do
        command -v "$t" >/dev/null 2>&1 || missing+=("$t")
    done
    docker compose version >/dev/null 2>&1 || missing+=("docker-compose-plugin")
    (( ${#missing[@]} == 0 )) || die "missing tools: ${missing[*]}"
}

prompt() {
    local var="$1" question="$2" default="${3:-}"
    if [[ -n "${!var:-}" ]]; then return; fi
    if [[ "$NONINTERACTIVE" == "1" ]]; then
        [[ -n "$default" ]] || die "$var required (non-interactive)"
        printf -v "$var" '%s' "$default"
        return
    fi
    local answer
    if [[ -n "$default" ]]; then
        read -r -p "$question [$default]: " answer
        answer="${answer:-$default}"
    else
        read -r -p "$question: " answer
    fi
    printf -v "$var" '%s' "$answer"
}

load_state() {
    if [[ -f "$STATE_FILE" ]]; then
        log "loading existing state: $STATE_FILE"
        # shellcheck disable=SC1090
        set -a; source "$STATE_FILE"; set +a
    fi
}

save_state() {
    umask 077
    cat > "$STATE_FILE" <<EOF
DOMAIN=$DOMAIN
EMAIL=$EMAIL
TLS_MODE=$TLS_MODE
MASTER_KEY=$MASTER_KEY
POSTGRES_PASSWORD=$POSTGRES_PASSWORD
REDIS_PASSWORD=$REDIS_PASSWORD
CHATWOOT_SECRET_KEY_BASE=$CHATWOOT_SECRET_KEY_BASE
EOF
    chmod 600 "$STATE_FILE"
    log "state saved $STATE_FILE (0600)"
}

gen_secrets() {
    : "${MASTER_KEY:=$(openssl rand -base64 32)}"
    : "${POSTGRES_PASSWORD:=$(openssl rand -base64 32 | tr -d '/=+' | head -c 24)}"
    : "${REDIS_PASSWORD:=$(openssl rand -base64 32 | tr -d '/=+' | head -c 24)}"
    : "${CHATWOOT_SECRET_KEY_BASE:=$(openssl rand -hex 64)}"
}

gather_inputs() {
    prompt DOMAIN   "Domain (e.g. bridge.example.com)"
    prompt EMAIL    "Admin email (Let's Encrypt / Cloudflare)"
    if [[ -z "$TLS_MODE" ]]; then
        if [[ "$NONINTERACTIVE" == "1" ]]; then TLS_MODE=tls
        else
            read -r -p "TLS mode [tls/tunnel] (tls=Caddy automatic HTTPS, tunnel=Cloudflare Tunnel) [tls]: " TLS_MODE
            TLS_MODE="${TLS_MODE:-tls}"
        fi
    fi
    [[ "$TLS_MODE" == "tls" || "$TLS_MODE" == "tunnel" ]] || die "TLS_MODE must be tls or tunnel (got $TLS_MODE)"
}

render() {
    log "rendering templates"
    DOMAIN="$DOMAIN" EMAIL="$EMAIL" \
        MASTER_KEY="$MASTER_KEY" POSTGRES_PASSWORD="$POSTGRES_PASSWORD" \
        REDIS_PASSWORD="$REDIS_PASSWORD" CHATWOOT_SECRET_KEY_BASE="$CHATWOOT_SECRET_KEY_BASE" \
        bash "$SCRIPT_DIR/render-templates.sh"
}

setup_tunnel_if_needed() {
    if [[ "$TLS_MODE" != "tunnel" ]]; then return; fi
    if [[ -f "$SCRIPT_DIR/.env.tunnel" ]]; then
        log "tunnel already configured ($SCRIPT_DIR/.env.tunnel)"
        return
    fi
    DOMAIN="$DOMAIN" bash "$SCRIPT_DIR/setup-tunnel.sh"
}

compose_up() {
    local profile="$TLS_MODE"
    log "docker compose pull"
    docker compose -f "$COMPOSE_FILE" --profile "$profile" \
        --env-file "$SCRIPT_DIR/.env.bridge" pull
    log "docker compose up -d (profile=$profile)"
    docker compose -f "$COMPOSE_FILE" --profile "$profile" \
        --env-file "$SCRIPT_DIR/.env.bridge" up -d
}

wait_for_health() {
    log "waiting for /healthz"
    local i=0
    until curl -fsS "http://127.0.0.1:8080/healthz" >/dev/null 2>&1; do
        (( i++ < 60 )) || die "bridge /healthz did not become ready in 5min"
        sleep 5
    done
    log "bridge healthy"
}

bootstrap_dbs() {
    log "bootstrap Chatwoot DB"
    COMPOSE_FILE="$COMPOSE_FILE" RAILS_SERVICE=rails \
        bash "$SCRIPT_DIR/bootstrap-chatwoot.sh"
    log "bootstrap bridge DB"
    COMPOSE_FILE="$COMPOSE_FILE" BRIDGE_SERVICE=bridge \
        bash "$SCRIPT_DIR/bootstrap-bridge.sh"
}

main() {
    parse_args "$@"
    require_tools
    load_state
    gather_inputs
    gen_secrets
    save_state
    render
    setup_tunnel_if_needed
    compose_up
    wait_for_health
    bootstrap_dbs

    cat <<EOF

==============================================================================
chatwoot-megaapi-bridge installed.
  Domain:   https://$DOMAIN
  TLS mode: $TLS_MODE
  State:    $STATE_FILE (0600 — contains secrets)
  Next:     bash $SCRIPT_DIR/postinstall-check.sh
==============================================================================
EOF
}

main "$@"
