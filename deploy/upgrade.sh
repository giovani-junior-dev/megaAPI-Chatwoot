#!/usr/bin/env bash
# Upgrade chatwoot-megaapi-bridge stack in place. Preserves DB volume + rendered
# .env files. Steps: git pull, docker compose pull, up -d --build, run migrations.
#
# Usage: bash deploy/upgrade.sh [--profile tls|tunnel]

set -euo pipefail

PROFILE="tls"
while [[ $# -gt 0 ]]; do
    case "$1" in
        --profile)
            PROFILE="${2:?--profile requires tls or tunnel}"
            shift 2
            ;;
        *) echo "unknown arg: $1" >&2; exit 2 ;;
    esac
done

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
COMPOSE_FILE="$SCRIPT_DIR/docker-compose.yml"

[[ "$PROFILE" == "tls" || "$PROFILE" == "tunnel" ]] || {
    echo "invalid profile: $PROFILE (must be tls or tunnel)" >&2
    exit 2
}

require_files() {
    local missing=0
    for f in "$SCRIPT_DIR/.env.bridge" "$SCRIPT_DIR/.env.chatwoot"; do
        if [[ ! -f "$f" ]]; then
            echo "missing rendered env: $f (run install.sh first)" >&2
            missing=1
        fi
    done
    (( missing == 0 )) || exit 1
}

compose() {
    docker compose -f "$COMPOSE_FILE" --profile "$PROFILE" --env-file "$SCRIPT_DIR/.env.bridge" "$@"
}

backup_before_upgrade() {
    local stamp ts
    ts="$(date -u +%Y%m%dT%H%M%SZ)"
    stamp="$SCRIPT_DIR/backups/pre-upgrade-${ts}"
    mkdir -p "$stamp"
    echo "[upgrade] dumping DBs to $stamp"
    compose exec -T postgres pg_dump -U postgres -Fc chatwoot > "$stamp/chatwoot.dump" || true
    compose exec -T postgres pg_dump -U postgres -Fc bridge   > "$stamp/bridge.dump"   || true
}

main() {
    require_files
    echo "[upgrade] git pull"
    git -C "$REPO_DIR" pull --ff-only

    backup_before_upgrade

    echo "[upgrade] docker compose pull"
    compose pull

    echo "[upgrade] docker compose up -d --build"
    compose up -d --build

    echo "[upgrade] running migrations"
    bash "$SCRIPT_DIR/bootstrap-chatwoot.sh"
    bash "$SCRIPT_DIR/bootstrap-bridge.sh"

    echo "[upgrade] done. status:"
    compose ps
}

main "$@"
