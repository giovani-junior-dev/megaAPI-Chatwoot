#!/usr/bin/env bash
# setup.sh — instalador guiado do pack (Chatwoot + bridge) em Docker Swarm.
# Estilo SetupOrion: pergunta o minimo, gera os segredos, sobe tudo.
#
# Uso:
#   bash setup.sh                 # interativo
#   DOMINIO=ex.com TOKEN_DO_TUNNEL=eyJ... ADMIN_EMAIL=a@b ADMIN_PASSWORD=... \
#     NONINTERACTIVE=1 bash setup.sh
#
# Reexecucao e segura: stacks ja existentes sao atualizadas.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ENV_FILE="$SCRIPT_DIR/.env"
NET="network_swarm_public"
NONINTERACTIVE="${NONINTERACTIVE:-0}"

log()  { printf '\033[1;34m[setup]\033[0m %s\n' "$*"; }
warn() { printf '\033[1;33m[setup]\033[0m %s\n' "$*" >&2; }
die()  { printf '\033[1;31m[setup]\033[0m %s\n' "$*" >&2; exit 1; }

ask() { # ask VAR "Pergunta" -> exporta VAR (respeita valor ja existente no ambiente)
  local var="$1" prompt="$2" cur="${!1:-}"
  if [[ -n "$cur" ]]; then return; fi
  [[ "$NONINTERACTIVE" == "1" ]] && die "$var nao definido (modo nao-interativo)"
  read -r -p "$prompt: " "$var"
}

ask_secret() { local var="$1" prompt="$2" cur="${!1:-}"; if [[ -n "$cur" ]]; then return; fi
  [[ "$NONINTERACTIVE" == "1" ]] && die "$var nao definido"; read -r -s -p "$prompt: " "$var"; echo; }

# ---------- 1. infra ----------
ensure_docker() {
  if ! command -v docker >/dev/null 2>&1; then
    log "instalando docker..."
    # docker.io (repo Ubuntu) basta para Swarm; docker stack deploy nao depende
    # do compose-plugin. Fallback para o script oficial se docker.io faltar.
    apt-get update -y
    apt-get install -y docker.io || curl -fsSL https://get.docker.com | sh
    systemctl enable --now docker
  fi
}
ensure_swarm() {
  local st; st="$(docker info --format '{{.Swarm.LocalNodeState}}' 2>/dev/null || echo inactive)"
  [[ "$st" == "active" ]] || { log "iniciando swarm..."; docker swarm init >/dev/null; }
}
ensure_network() {
  docker network inspect "$NET" >/dev/null 2>&1 || {
    log "criando rede $NET..."; docker network create --driver overlay --attachable "$NET" >/dev/null; }
}

# ---------- 2. segredos / .env ----------
gen_env() {
  if [[ -f "$ENV_FILE" ]]; then log ".env existente — reaproveitando"; return; fi
  log "gerando segredos..."
  ask DOMINIO "Dominio raiz (ex: minhaempresa.com.br)"
  ask TOKEN_DO_TUNNEL "Token do Cloudflare Tunnel (eyJ...)"
  ask ADMIN_EMAIL "E-mail do admin do painel bridge"
  ask_secret ADMIN_PASSWORD "Senha do admin do painel bridge"
  local MASTER_KEY BRIDGE_ENCRYPTION_KEY SECRET_KEY_BASE PBW_ENCRYPTION_KEY SENHA_POSTGRES SENHA_BRIDGE
  MASTER_KEY="$(openssl rand -base64 32)"
  BRIDGE_ENCRYPTION_KEY="$(openssl rand -base64 32)"
  SECRET_KEY_BASE="$(openssl rand -hex 32)"
  PBW_ENCRYPTION_KEY="$(openssl rand -base64 16)"
  SENHA_POSTGRES="$(openssl rand -hex 24)"   # hex = seguro em DSN (sem / @ + :)
  SENHA_BRIDGE="$(openssl rand -hex 24)"
  umask 077
  cat > "$ENV_FILE" <<EOF
DOMINIO='$DOMINIO'
TOKEN_DO_TUNNEL='$TOKEN_DO_TUNNEL'
SENHA_POSTGRES='$SENHA_POSTGRES'
SENHA_BRIDGE='$SENHA_BRIDGE'
SECRET_KEY_BASE='$SECRET_KEY_BASE'
MASTER_KEY='$MASTER_KEY'
BRIDGE_ENCRYPTION_KEY='$BRIDGE_ENCRYPTION_KEY'
PBW_ENCRYPTION_KEY='$PBW_ENCRYPTION_KEY'
ADMIN_EMAIL='$ADMIN_EMAIL'
ADMIN_PASSWORD='$ADMIN_PASSWORD'
EOF
  chmod 600 "$ENV_FILE"
  log ".env salvo em $ENV_FILE (chmod 600) — guarde em local seguro"
}

# ---------- 3. deploy ----------
deploy() { # deploy <stack-name> <yaml>
  log "deploy stack '$1'"
  docker stack deploy -c "$SCRIPT_DIR/$2" "$1" >/dev/null
}
wait_service() { # wait_service <service> <timeout-s>
  local svc="$1" t="${2:-120}" i=0
  while (( i < t )); do
    [[ "$(docker service ls --filter "name=$svc" --format '{{.Replicas}}' 2>/dev/null)" == "1/1" ]] && return 0
    sleep 3; i=$((i+3))
  done
  warn "timeout esperando $svc ficar 1/1 (segue assim mesmo)"
}
task_cid() { docker ps --filter "name=$1" -q | head -1; }

main() {
  [[ "$(id -u)" == "0" ]] || warn "rodando sem root — instalacao de docker pode falhar"
  ensure_docker; ensure_swarm; ensure_network
  gen_env
  set -a; # shellcheck disable=SC1090
  . "$ENV_FILE"; set +a

  deploy portainer   02-portainer/portainer.yaml
  deploy postgres    03-postgres/postgres.yaml
  wait_service postgres_postgres 120
  # banco interno do pgbackweb (idempotente)
  local pg; pg="$(task_cid postgres_postgres)"
  [[ -n "$pg" ]] && docker exec "$pg" psql -U postgres -c "CREATE DATABASE pgbackweb" >/dev/null 2>&1 || true
  deploy cloudflared 04-cloudflared/cloudflared.yaml
  deploy chatwoot    05-chatwoot/chatwoot.yaml
  deploy bridge      06-bridge-admin/bridge.yaml
  deploy pgbackupweb 07-backup/pgbackupweb.yaml

  log "esperando chatwoot_admin..."; wait_service chatwoot_chatwoot_admin 240
  local cw; cw="$(task_cid chatwoot_chatwoot_admin)"
  if [[ -n "$cw" ]]; then
    log "preparando banco do Chatwoot (db:chatwoot_prepare)"
    docker exec "$cw" bundle exec rails db:chatwoot_prepare || warn "db:chatwoot_prepare falhou — rode manualmente"
  fi

  log "esperando bridge..."; wait_service bridge_bridge 120
  local br; br="$(task_cid bridge_bridge)"
  if [[ -n "$br" && -n "${ADMIN_EMAIL:-}" ]]; then
    log "criando admin do bridge ($ADMIN_EMAIL)"
    docker exec "$br" /bridge admin add --email "$ADMIN_EMAIL" --password "$ADMIN_PASSWORD" || warn "admin add falhou — rode manualmente"
  fi

  cat <<EOF

=== Instalacao concluida ===
  Chatwoot:  https://chat.${DOMINIO}
  Bridge:    https://bridge.${DOMINIO}   (login: ${ADMIN_EMAIL:-defina via 'bridge admin add'})

  FALTA criar os 2 Public Hostnames no Cloudflare Zero Trust:
    chat.${DOMINIO}   -> HTTP -> chatwoot_admin:3000
    bridge.${DOMINIO} -> HTTP -> bridge:8080

  Segredos em: $ENV_FILE  (NUNCA comite este arquivo)
EOF
}

main "$@"
