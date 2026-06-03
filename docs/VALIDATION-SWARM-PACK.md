# Validação E2E — Install-Pack Docker Swarm

Data: 2026-06-03
Validador: sessão Claude Code (swarm single-node local, Docker Desktop)
Versão: bridge `v1.1.1` (`ghcr.io/giovani-junior-dev/chatwoot-megaapi-bridge:v1.1.1`)

## Escopo

Subida ponta a ponta de um swarm single-node real para validar as stacks de
`install-pack/`: imagem pública, auto-bootstrap do banco, auto-migrate,
idempotência em restart, e o design de **Postgres compartilhado** entre
Chatwoot e bridge.

## Resultados

| Teste | Resultado |
|-------|-----------|
| Pull anônimo da imagem pública `:v1.1.1` | PASS — baixou sem login |
| Bridge auto-bootstrap (`POSTGRES_ADMIN_URL`) | PASS — `database bootstrap complete`, role+db `bridge` criados sozinhos |
| Auto-migrate no boot (`RUN_MIGRATIONS=1`) | PASS — migrations 0001-0004 aplicadas, 5 tabelas (admins, contacts, messages, settings, tenants) |
| Bridge HTTP | PASS — `listening :8080` |
| Idempotência em restart (`service update --force`) | PASS — 2º boot pula bootstrap existente, re-aplica migrations sem erro |
| Postgres compartilhado | PASS — `bridge` (5 tabelas) + `chatwoot` (89 tabelas) no mesmo serviço `postgres` |
| Chatwoot real v4.14.1-ce | PASS — admin+api+sidekiq+redis 1/1; `rails db:chatwoot_prepare` criou DB `chatwoot` |
| YAMLs swarm-válidos | PASS — todas as stacks aceitas no `docker stack deploy` |

6 serviços simultâneos 1/1: `bridge`, `chatwoot_admin`, `chatwoot_api`,
`chatwoot_sidekiq`, `postgres`, `redis`.

### Evidência (logs do bridge)

```
{"level":"info","role":"bridge","database":"bridge","message":"database bootstrap complete"}
{"level":"info","file":"0001_init.sql","message":"migration applied"}
{"level":"info","file":"0002_chatwoot_ids_bigint.sql","message":"migration applied"}
{"level":"info","file":"0003_admins_and_settings.sql","message":"migration applied"}
{"level":"info","file":"0004_pairing.sql","message":"migration applied"}
{"level":"info","addr":":8080","message":"listening"}
```

### Evidência (Postgres compartilhado)

```
datname  | tabelas (public)
---------+-----------------
bridge   | 5
chatwoot | 89
```

## Não validado localmente (requer ambiente real)

- **Cloudflare Tunnel** — precisa de token de tunnel real + ingress no
  Zero Trust dashboard (`chat.DOMINIO->chatwoot_admin:3000`,
  `bridge.DOMINIO->bridge:8080`). YAML valida em `docker compose config`,
  roteamento não testável sem token.
- **Bridge `/readyz` via HTTP** — imagem é scratch (sem `curl`) e sem porta
  publicada (design tunnel). Inferido OK: `listening` + DB conectado +
  migrations concluídas.

## Pendências para produção real

1. Subir num swarm com a stack cloudflared + token real.
2. Criar os 2 ingress no Cloudflare Zero Trust.
3. `rails db:chatwoot_prepare` (1ª vez) no chatwoot_admin.
4. Criar admin do bridge com senha forte (`bridge admin add`) — ver SCR-201.

## Limpeza pós-teste

Swarm de teste removido (`docker stack rm`, `swarm leave --force`, network
removida). Ambiente de dev compose (`:8090`) preservado e saudável.
