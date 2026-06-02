#!/bin/bash
set -euo pipefail

###################################################################
# 01-infra — Docker Swarm + rede overlay
#
# Roda ESTE script UMA vez no servidor (VPS Ubuntu), como root,
# DEPOIS de instalar o Docker. Ele:
#   1. Inicia o Docker Swarm (vira o "manager").
#   2. Cria a rede overlay `network_swarm_public` usada por TODAS
#      as stacks do pack.
#
# Pre-requisito: Docker instalado. Se ainda nao tem, rode antes:
#   apt-get update && apt-get install -y docker.io docker-compose-plugin
#   systemctl enable --now docker
###################################################################

# IP publico do servidor. Detecta sozinho; se falhar, edite a mao.
IP_PUBLICO="$(hostname -I | awk '{print $1}')"

echo ">> Iniciando Docker Swarm (advertise-addr=${IP_PUBLICO})"
if docker info 2>/dev/null | grep -q "Swarm: active"; then
  echo "   Swarm ja estava ativo. Pulando init."
else
  docker swarm init --advertise-addr="${IP_PUBLICO}"
fi

echo ">> Criando rede overlay network_swarm_public (attachable)"
if docker network ls --format '{{.Name}}' | grep -qx "network_swarm_public"; then
  echo "   Rede ja existe. Pulando."
else
  docker network create \
    --driver=overlay \
    --attachable \
    network_swarm_public
fi

echo ""
echo ">> Pronto. Swarm ativo + rede network_swarm_public criada."
echo "   Proximo passo: subir o Portainer (02-portainer)."
