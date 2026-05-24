#!/usr/bin/env bash
# postinstall-check.sh — validate stack is healthy after install.sh.
# Exits 0 if all checks pass, 1 otherwise. Prints colored checklist.
#
# Usage: bash deploy/postinstall-check.sh [--domain ex.com] [--mode tls|tunnel]

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
COMPOSE_FILE="$SCRIPT_DIR/docker-compose.yml"
STATE_FILE="$SCRIPT_DIR/.env.local"

DOMAIN=""
MODE=""

while [[ $# -gt 0 ]]; do
    case "$1" in
        --domain) DOMAIN="$2"; shift 2 ;;
        --mode)   MODE="$2";   shift 2 ;;
        *) echo "unknown arg: $1" >&2; exit 2 ;;
    esac
done

if [[ -f "$STATE_FILE" ]]; then
    # shellcheck disable=SC1090
    set -a; source "$STATE_FILE"; set +a
fi
DOMAIN="${DOMAIN:-${DOMAIN:-}}"
MODE="${MODE:-${TLS_MODE:-tls}}"

pass=0
fail=0

GREEN=$'\033[1;32m'; RED=$'\033[1;31m'; YEL=$'\033[1;33m'; NC=$'\033[0m'

ok()   { printf '%s✓%s %s\n' "$GREEN" "$NC" "$1"; pass=$((pass+1)); }
bad()  { printf '%s✗%s %s\n' "$RED"   "$NC" "$1"; fail=$((fail+1)); }
skip() { printf '%s-%s %s\n' "$YEL"   "$NC" "$1"; }

check_containers_up() {
    local need=(postgres redis rails sidekiq bridge)
    [[ "$MODE" == "tls" ]] && need+=(caddy)
    [[ "$MODE" == "tunnel" ]] && need+=(cloudflared)
    need+=(postgres-backup)
    local missing=()
    for svc in "${need[@]}"; do
        if docker compose -f "$COMPOSE_FILE" ps --status running --services 2>/dev/null | grep -qx "$svc"; then
            :
        else
            missing+=("$svc")
        fi
    done
    if (( ${#missing[@]} == 0 )); then
        ok "all containers running (${need[*]})"
    else
        bad "containers not running: ${missing[*]}"
    fi
}

check_bridge_healthz() {
    if curl -fsS "http://127.0.0.1:8080/healthz" >/dev/null 2>&1; then
        ok "bridge /healthz 200"
    else
        bad "bridge /healthz unreachable"
    fi
}

check_chatwoot_ui() {
    local code
    code="$(curl -s -o /dev/null -w '%{http_code}' "http://127.0.0.1:3000" || true)"
    if [[ "$code" =~ ^(200|301|302)$ ]]; then
        ok "Chatwoot UI HTTP $code"
    else
        bad "Chatwoot UI returned $code (expected 200/301/302)"
    fi
}

check_migrations() {
    if docker compose -f "$COMPOSE_FILE" exec -T postgres \
        psql -U postgres -d bridge -tAc 'SELECT count(*) FROM schema_migrations' >/dev/null 2>&1
    then
        ok "bridge migrations table present"
    else
        bad "bridge schema_migrations not found"
    fi
    if docker compose -f "$COMPOSE_FILE" exec -T postgres \
        psql -U postgres -d chatwoot -tAc 'SELECT count(*) FROM ar_internal_metadata' >/dev/null 2>&1
    then
        ok "Chatwoot schema bootstrapped"
    else
        bad "Chatwoot ar_internal_metadata not found"
    fi
}

check_backup() {
    if docker compose -f "$COMPOSE_FILE" ps --status running --services 2>/dev/null | grep -qx postgres-backup; then
        ok "postgres-backup sidecar running (BACKUP_KEEP_DAYS=14)"
    else
        bad "postgres-backup sidecar not running"
    fi
}

check_tls() {
    if [[ "$MODE" != "tls" ]]; then
        skip "TLS check (mode=$MODE)"
        return
    fi
    if [[ -z "$DOMAIN" ]]; then
        skip "TLS check (no DOMAIN configured)"
        return
    fi
    if curl -fsS --max-time 10 "https://$DOMAIN/healthz" -o /dev/null; then
        ok "Caddy TLS valid (https://$DOMAIN/healthz reachable)"
    else
        bad "https://$DOMAIN/healthz unreachable (DNS/TLS issue?)"
    fi
}

main() {
    echo "postinstall checks (mode=$MODE domain=${DOMAIN:-?})"
    echo "---------------------------------------------"
    check_containers_up
    check_bridge_healthz
    check_chatwoot_ui
    check_migrations
    check_backup
    check_tls
    echo "---------------------------------------------"
    printf 'passed=%d failed=%d\n' "$pass" "$fail"
    (( fail == 0 ))
}

main "$@"
