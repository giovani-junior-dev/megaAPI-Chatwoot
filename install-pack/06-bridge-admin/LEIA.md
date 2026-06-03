# 06-bridge-admin — a aplicacao bridge

Esta e a stack do **chatwoot-megaapi-bridge** (este repositorio): liga o
Chatwoot ao WhatsApp via megaAPI e serve o painel de administracao.

## Imagem (opensource, publica)

A imagem `ghcr.io/giovani-junior-dev/chatwoot-megaapi-bridge:v1.1.1` e
**publica** — o swarm baixa sem login, nada de token. A imagem contem
apenas o binario compilado; nenhum segredo vive nela (as chaves entram por
variavel de ambiente no Portainer).

> Se voce **fork**ar este projeto, publique a sua imagem e marque o
> package como **Public** no GitHub (Packages > package > Settings >
> Change visibility), senao o swarm recebe `denied` ao puxar.

## Nota sobre o banco

O bridge **nao** sobe um Postgres proprio — ele reaproveita o `postgres`
do **03-postgres**. Na primeira subida, com `POSTGRES_ADMIN_URL` setado,
o bridge cria sozinho o role e o database `bridge` (idempotente) e roda as
migrations (`RUN_MIGRATIONS=1`). Depois disso voce pode remover
`POSTGRES_ADMIN_URL` da stack.

## Antes de subir

Troque no `bridge.yaml` os placeholders MAIUSCULOS (todos no `.env.modelo`):

| Placeholder            | O que e                                            |
|------------------------|----------------------------------------------------|
| `MASTER_KEY`           | login do painel — `openssl rand -base64 32`        |
| `BRIDGE_ENCRYPTION_KEY`| chave AES dos segredos — `openssl rand -base64 32` |
| `SENHA_BRIDGE`         | senha do role `bridge` (voce escolhe)              |
| `SENHA_POSTGRES`       | senha do superusuario postgres (do 03-postgres)    |

> **Cuidado:** depois que existir dado real, **nunca** troque
> `BRIDGE_ENCRYPTION_KEY` — os segredos dos tenants ficam ilegiveis.

## Subir

```bash
docker stack deploy -c bridge.yaml bridge
```

Acesso ao painel: `https://bridge.DOMINIO` (ingress configurado no
Cloudflare Zero Trust — ver 00-LEIA-PRIMEIRO.md, passo 5).
