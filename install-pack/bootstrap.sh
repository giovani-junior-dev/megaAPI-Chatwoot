#!/usr/bin/env bash
# bootstrap.sh — ponto de entrada do one-liner. Clona o repo e chama o setup.sh.
#
#   bash <(curl -fsSL https://raw.githubusercontent.com/giovani-junior-dev/megaAPI-Chatwoot/master/install-pack/bootstrap.sh)
#
set -euo pipefail
REPO="https://github.com/giovani-junior-dev/megaAPI-Chatwoot.git"
DEST="${DEST:-/opt/megaapi-chatwoot}"

log() { printf '\033[1;34m[bootstrap]\033[0m %s\n' "$*"; }

command -v git >/dev/null 2>&1 || { log "instalando git..."; apt-get update -y && apt-get install -y git; }

if [[ -d "$DEST/.git" ]]; then
  log "repo ja existe em $DEST — atualizando"; git -C "$DEST" pull --ff-only || true
else
  log "clonando em $DEST"; git clone --depth 1 "$REPO" "$DEST"
fi

exec bash "$DEST/install-pack/setup.sh" "$@"
