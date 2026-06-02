# 01-infra — Base do servidor

Prepara o servidor (VPS Ubuntu) para receber as stacks.

## O que faz

1. **Instala o Docker** (se ainda nao tiver).
2. **Inicia o Docker Swarm** — transforma o servidor no "gerente" (manager).
3. **Cria a rede `network_swarm_public`** — a rede interna que liga todas as stacks (Postgres, Chatwoot, bridge, etc.).

## Como rodar

Conecte no servidor por SSH e rode, como root:

```bash
# 1) Instalar Docker (pule se ja tiver)
apt-get update && apt-get install -y docker.io docker-compose-plugin
systemctl enable --now docker

# 2) Subir Swarm + criar a rede
bash setup-swarm.sh
```

Se aparecer **"Swarm ativo + rede network_swarm_public criada"**, deu certo.

> Faca isto **uma vez por servidor**. Repetir nao quebra nada (o script
> detecta o que ja existe e pula).

Proximo passo: **02-portainer**.
