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
  ask CHATWOOT_HOST "Hostname do Chatwoot (ex: chatwoot.suaempresa.com)"
  ask BRIDGE_HOST "Hostname do painel bridge (ex: bridge.suaempresa.com)"
  ask PORTAINER_HOST "Hostname do Portainer (ex: portainer.suaempresa.com)"
  ask BACKUP_HOST "Hostname do painel de backup (ex: backup.suaempresa.com)"
  ask TOKEN_DO_TUNNEL "Token do Cloudflare Tunnel (eyJ...)"
  ask ADMIN_EMAIL "E-mail do admin do painel bridge"
  ask_secret ADMIN_PASSWORD "Senha do admin do painel bridge"
  ask_secret PORTAINER_PASSWORD "Senha do admin do Portainer (min 12 caracteres)"
  [[ ${#PORTAINER_PASSWORD} -ge 12 ]] || die "PORTAINER_PASSWORD precisa de no minimo 12 caracteres"
  local MASTER_KEY BRIDGE_ENCRYPTION_KEY SECRET_KEY_BASE PBW_ENCRYPTION_KEY SENHA_POSTGRES SENHA_BRIDGE
  MASTER_KEY="$(openssl rand -base64 32)"
  BRIDGE_ENCRYPTION_KEY="$(openssl rand -base64 32)"
  SECRET_KEY_BASE="$(openssl rand -hex 32)"
  PBW_ENCRYPTION_KEY="$(openssl rand -base64 16)"
  SENHA_POSTGRES="$(openssl rand -hex 24)"   # hex = seguro em DSN (sem / @ + :)
  SENHA_BRIDGE="$(openssl rand -hex 24)"
  umask 077
  cat > "$ENV_FILE" <<EOF
CHATWOOT_HOST='$CHATWOOT_HOST'
BRIDGE_HOST='$BRIDGE_HOST'
PORTAINER_HOST='$PORTAINER_HOST'
BACKUP_HOST='$BACKUP_HOST'
PORTAINER_PASSWORD='$PORTAINER_PASSWORD'
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

# pcurl: roda curl num container efemero na overlay (host nao alcanca portainer:9000)
pcurl() { docker run --rm --network "$NET" curlimages/curl:latest -s "$@"; }
# pcurl_mnt: idem, montando o install-pack em /work (para upload de arquivo)
pcurl_mnt() { docker run --rm --network "$NET" -v "$SCRIPT_DIR:/work:ro" curlimages/curl:latest -s "$@"; }

PJWT="" ; PEID="" ; PSWARM="" ; PENV="[]" ; PORTAINER_OK=0

# portainer_ctx: faz login, garante o ambiente swarm e prepara o contexto
# (PJWT/PEID/PSWARM/PENV) usado pelo deploy via API. Sem -H no Portainer, o
# ambiente e registrado aqui de forma deterministica (evita race no boot).
portainer_ctx() {
  local i=0
  while (( i < 60 )); do
    PJWT="$(pcurl --max-time 8 -X POST http://portainer:9000/api/auth -H 'Content-Type: application/json' \
      -d "{\"username\":\"admin\",\"password\":\"$PORTAINER_PASSWORD\"}" 2>/dev/null | sed 's/.*"jwt":"//;s/".*//')"
    [[ ${#PJWT} -gt 20 ]] && break
    sleep 3; i=$((i+3))
  done
  [[ ${#PJWT} -gt 20 ]] || { warn "Portainer API indisponivel; usando deploy via CLI (stacks ficam 'Limited')"; return; }
  # Registra o ambiente com RETRY: criar um endpoint tipo-agent valida a conexao
  # com tasks.agent:9001; se o agent ainda nao subiu, a criacao falha e o
  # endpoint nao aparece. Repetimos ate o agent responder (~2min) para nao cair
  # no fallback CLI (stacks ficariam 'Limited').
  log "registrando ambiente swarm no Portainer (aguardando agent)"
  local j=0 eps="[]"
  while (( j < 120 )); do
    eps="$(pcurl --max-time 8 http://portainer:9000/api/endpoints -H "Authorization: Bearer $PJWT" 2>/dev/null)"
    [[ -n "$eps" && "$eps" != "[]" ]] && break
    pcurl --max-time 15 -X POST http://portainer:9000/api/endpoints -H "Authorization: Bearer $PJWT" \
      -F "Name=swarm-local" -F "EndpointCreationType=2" -F "URL=tcp://tasks.agent:9001" \
      -F "TLS=true" -F "TLSSkipVerify=true" -F "TLSSkipClientVerify=true" >/dev/null 2>&1 || true
    sleep 4; j=$((j+4))
  done
  PEID="$(echo "$eps" | sed 's/.*"Id"://;s/[^0-9].*//')"
  PSWARM="$(pcurl --max-time 8 "http://portainer:9000/api/endpoints/$PEID/docker/swarm" -H "Authorization: Bearer $PJWT" 2>/dev/null | sed 's/.*"ID":"//;s/".*//')"
  # monta o array de env (todas as vars do .env) para o Portainer interpolar ${VAR}
  PENV="$(python3 - "$ENV_FILE" <<'PY'
import sys,json
out=[]
for line in open(sys.argv[1]):
    line=line.strip()
    if not line or line.startswith('#') or '=' not in line: continue
    k,v=line.split('=',1)
    out.append({"name":k.strip(),"value":v.strip().strip("'").strip('"')})
print(json.dumps(out))
PY
)"
  [[ -n "$PEID" && -n "$PSWARM" && -n "$PENV" ]] && PORTAINER_OK=1 || warn "contexto Portainer incompleto; deploy via CLI"
}

# deploy_api: cria a stack VIA Portainer (controle total/editavel na GUI).
# Faz fallback para CLI (docker stack deploy) se a API nao estiver pronta.
deploy_api() { # deploy_api <name> <rel-yaml>
  if [[ "$PORTAINER_OK" != "1" ]]; then deploy "$1" "$2"; return; fi
  log "deploy (Portainer) stack '$1'"
  local code
  code="$(pcurl_mnt -o /dev/null -w '%{http_code}' --max-time 90 \
    -X POST "http://portainer:9000/api/stacks/create/swarm/file?endpointId=$PEID" \
    -H "Authorization: Bearer $PJWT" \
    -F "Name=$1" -F "SwarmID=$PSWARM" -F "Env=$PENV" -F "file=@/work/$2" 2>/dev/null)"
  if [[ "$code" =~ ^2 ]]; then return; fi
  if [[ "$code" == "409" ]]; then log "stack '$1' ja existe no Portainer (mantida)"; return; fi
  warn "deploy via API de '$1' retornou $code; tentando CLI"
  deploy "$1" "$2"
}

main() {
  [[ "$(id -u)" == "0" ]] || warn "rodando sem root — instalacao de docker pode falhar"
  ensure_docker; ensure_swarm; ensure_network
  gen_env
  set -a; # shellcheck disable=SC1090
  . "$ENV_FILE"; set +a

  # secret com a senha do admin do Portainer (texto puro; o Portainer faz o hash
  # via --admin-password-file). rm+create e idempotente no 1o run; em reexecucao
  # o secret em uso nao e removido (mantem a senha atual).
  docker secret rm portainer_admin_password >/dev/null 2>&1 || true
  printf '%s' "$PORTAINER_PASSWORD" | docker secret create portainer_admin_password - >/dev/null 2>&1 || true
  # Portainer sobe via CLI (bootstrap). As demais stacks sobem VIA Portainer
  # (API) para ficarem com controle total/editaveis na GUI.
  deploy portainer   02-portainer/portainer.yaml
  log "preparando Portainer..."; wait_service portainer_portainer 90; portainer_ctx

  deploy_api postgres    03-postgres/postgres.yaml
  wait_service postgres_postgres 120
  # banco interno do pgbackweb (idempotente)
  local pg; pg="$(task_cid postgres_postgres)"
  [[ -n "$pg" ]] && docker exec "$pg" psql -U postgres -c "CREATE DATABASE pgbackweb" >/dev/null 2>&1 || true
  deploy_api cloudflared 04-cloudflared/cloudflared.yaml
  deploy_api chatwoot    05-chatwoot/chatwoot.yaml
  deploy_api bridge      06-bridge-admin/bridge.yaml
  deploy_api pgbackupweb 07-backup/pgbackupweb.yaml

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
  Chatwoot:   https://${CHATWOOT_HOST}
  Bridge:     https://${BRIDGE_HOST}    (login: ${ADMIN_EMAIL:-defina via 'bridge admin add'})
  Portainer:  https://${PORTAINER_HOST} (login: admin / a senha que voce definiu)
  Backup:     https://${BACKUP_HOST}

  FALTA criar os 4 Public Hostnames no Cloudflare Zero Trust (Type = HTTP):
    ${CHATWOOT_HOST}  -> chatwoot_admin:3000
    ${BRIDGE_HOST}    -> bridge:8080
    ${PORTAINER_HOST} -> portainer:9000
    ${BACKUP_HOST}    -> pgbackupweb:8085

  IMPORTANTE: use subdominios de 1 nivel (ex: chatwoot.seudominio.com).
  Subdominio de 2 niveis (chat.chatwoot.seudominio.com) quebra o SSL gratis
  do Cloudflare (*.seudominio.com nao cobre 2 niveis).

  Segredos em: $ENV_FILE  (NUNCA comite este arquivo)
EOF
}

main "$@"
