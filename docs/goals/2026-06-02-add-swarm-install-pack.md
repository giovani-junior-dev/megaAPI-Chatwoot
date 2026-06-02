# Plano — Pack de Instalação Docker Swarm (bundle completo)

## 0. Contexto

- Repo: `C:\Users\GEOVANE\Desktop\Projetos\chatwoot-megaapi-bridge`
- Objetivo: pack de instalação para usuário **não-técnico** subir tudo do zero numa VPS Ubuntu, via Docker Swarm + Portainer, padrão luizeof/SetupOrion.
- Stacks de referência do usuário em `C:\Users\GEOVANE\Desktop\Stack` (01-infra, portainer, postgres/pgvector, chatwoot v4.0.2-ce, cloudflared).

### Decisões travadas (2026-06-02)

1. **Bundle completo** infra→portainer→postgres→cloudflared→chatwoot→bridge.
2. **UX**: stacks numeradas + guia PT-BR, paste no Portainer (não App Template, não script).
3. **Cloudflare Tunnel pra tudo** (sem Traefik no pack novo). Sem abrir 80/443, sem DNS A-record, sem Let's Encrypt. Usuário só cola o token do tunnel + cria ingress no dashboard Zero Trust.
4. **Postgres compartilhado**: bridge reusa o `postgres` (pgvector pg16) do pack, DB próprio `bridge`.
5. **Bridge auto-cria DB `bridge` + roda migrations no boot** (idempotente — verificado: migrations re-run-safe).

## 1. Padrão do ambiente (extraído das stacks de referência)

- Overlay única: `network_swarm_public` (external, attachable), criada em `01-infra`.
- Postgres: service `postgres`, `pgvector/pgvector:pg16`, user `postgres`, no manager, `--wal_level=minimal`. DNS interno `postgres:5432`.
- Orquestração: Portainer CE + agent. Stacks YAML v3.7, `deploy:` com placement manager + resources limits.
- Cloudflared: `tunnel --no-autoupdate --protocol h2mux run --token <TOKEN>`, mode global, na overlay.

## 2. Arquitetura do pack

```
install-pack/              (dentro deste repo)
  00-LEIA-PRIMEIRO.md      guia PT-BR passo a passo + checklist
  .env.modelo              todas as vars (dominios, senhas, tokens, master key)
  01-infra/                swarm init + network_swarm_public  (scripts existentes)
  02-portainer/            portainer-ce + agent
  03-postgres/             pgvector pg16 (compartilhado chatwoot+bridge)
  04-cloudflared/          tunnel token (ingress no dashboard, --protocol auto)
  05-chatwoot/             admin + api + sidekiq v4.14.1-ce, storage local, SEM Traefik
  06-bridge-admin/         stack do bridge (NOVA)
  07-backup/               pgbackupweb (dumps chatwoot + bridge)
```

Roteamento (Cloudflare Zero Trust dashboard, ingress public hostnames):
- `chat.DOMINIO`   → `http://chatwoot_admin:3000`
- `bridge.DOMINIO` → `http://bridge:8080`

## 3. Mudanças no código bridge (repo, TDD)

Arquivo `cmd/bridge/main.go`:

1. **Auto-bootstrap DB** em `cmdServe`: se env `POSTGRES_ADMIN_URL` setado, conectar no DB de manutenção (`postgres`), criar role `bridge` + database `bridge` (owner bridge) se ausentes — idempotente (`SELECT 1 FROM pg_database`/`pg_roles` antes de `CREATE`). Erros de "já existe" ignorados.
2. **Auto-migrate** em `cmdServe`: se env `RUN_MIGRATIONS` != `0` (default on), rodar `applyMigrations` no DB do bridge antes de `serve`. Reusa lógica de `cmdMigrate`.
3. Env novos documentados: `POSTGRES_ADMIN_URL` (opcional, só 1ª subida), `RUN_MIGRATIONS` (default 1).
4. Manter compat: sem `POSTGRES_ADMIN_URL` → comportamento atual (assume DB já existe).

Restrições: sem deps Go novas, sem mudar rotas/handlers, sem quebrar `bridge migrate`/`serve` standalone.

Tests (TDD):
- `TestBootstrapCreatesDBWhenAbsent` (unit sobre a função de bootstrap com fake/admin DSN — ou integração testcontainers se já houver harness PG)
- `TestServeRunsMigrationsWhenEnabled` / `TestServeSkipsMigrationsWhenDisabled`
- Suite existente continua verde.

## 4. Stack do bridge (`06-bridge-admin/bridge.yaml`)

```yaml
version: "3.7"
services:
  bridge:
    image: ghcr.io/madeinlowcode/chatwoot-megaapi-bridge:v1.1.0
    networks: [network_swarm_public]
    environment:
      - MASTER_KEY=${BRIDGE_MASTER_KEY}
      - BRIDGE_ENCRYPTION_KEY=${BRIDGE_ENCRYPTION_KEY}
      - DATABASE_URL=postgres://bridge:${BRIDGE_DB_PASSWORD}@postgres:5432/bridge?sslmode=disable
      - POSTGRES_ADMIN_URL=postgres://postgres:${POSTGRES_PASSWORD}@postgres:5432/postgres?sslmode=disable
      - RUN_MIGRATIONS=1
      - BRIDGE_PORT=8080
    deploy:
      mode: replicated
      replicas: 1
      placement: { constraints: [node.role == manager] }
      restart_policy: { condition: on-failure }
      resources: { limits: { cpus: "0.5", memory: 512M } }
networks:
  network_swarm_public:
    external: true
    name: network_swarm_public
```

Notas:
- `replicas: 1` obrigatório (fila in-process + RecoverPending + auto-migrate assumem instância única).
- Sem volume (estado todo no Postgres) → sem placement por disco.
- Sem labels Traefik (tunnel faz o ingress).

## 5. PRÉ-REQUISITO CRÍTICO — publicar image

Bridge image precisa estar **pública no ghcr.io** (swarm puxa de registry, não builda):
- GitHub Action `docker/build-push-action` no tag push (`v*`) → `ghcr.io/madeinlowcode/chatwoot-megaapi-bridge:vX.Y.Z` + `:latest`, package público.
- OU push manual uma vez da v1.1.0.
- Bloqueia o pack inteiro até resolvido.

## 6. Chatwoot — versão

- Stack de referência usa `v4.0.2-ce` (antigo). Repo dev usa `v4.13.0-ce`.
- Pinar versão estável recente verificada no Docker Hub no momento da build (não usar `latest`).
- Adaptar stacks chatwoot: remover labels Traefik, `FRONTEND_URL=https://chat.DOMINIO`, confiar proxy (tunnel termina TLS no edge, origin http), `FORCE_SSL` conforme comportamento atrás de tunnel.
- Passo pós-deploy documentado: `bundle exec rails db:chatwoot_prepare` no console do container (1ª vez).

## 7. Guia PT-BR (`00-LEIA-PRIMEIRO.md`)

Passo a passo leigo:
1. VPS Ubuntu limpa → rodar `01-infra` (swarm init + network).
2. Deploy `02-portainer` → acessar GUI.
3. Preencher `.env.modelo` (1 lugar: domínios, senhas, master key gerada, token tunnel).
4. Colar stacks 03→06 no Portainer na ordem, preenchendo env.
5. Criar ingress no Cloudflare Zero Trust (2 hostnames).
6. `rails db:chatwoot_prepare` no chatwoot.
7. Criar admin do bridge: `bridge admin add` (senha forte).
8. Smoke: abrir chat.DOMINIO e bridge.DOMINIO.

Incluir: como gerar MASTER_KEY (`openssl rand -base64 32`), como pegar token do tunnel, troubleshooting comum.

## 8. Execução sugerida (2 goals)

- **Goal A (repo, TDD)**: mudanças Go (auto-bootstrap + auto-migrate) + `deploy/swarm/bridge.yaml` + GitHub Action publish ghcr + `docs/INSTALL-SWARM.md`. Mensurável: `go test ./...` verde, image publicada, stack valida (`docker stack config`/`docker-compose config`).
- **Goal B (pack authoring)**: pasta `pack-instalacao/` com stacks numeradas adaptadas (tunnel-only, versões pinadas) + guia PT-BR. Mensurável: cada YAML passa `docker stack config -c <f>` sem erro; guia cobre os 8 passos.

## 9. Decisões resolvidas (2026-06-02)

1. **Backup**: incluir stack `pgbackupweb` (das refs do usuário) → `07-backup/`. Aponta no `postgres`, dumps de `chatwoot` + `bridge`.
2. **Storage chatwoot**: **disco local** (igual ambiente dev que funcionou): `ACTIVE_STORAGE_SERVICE=local` + volume `chatwoot_storage:/app/storage`. Sem Minio/S3. (Anexos WhatsApp inbound/outbound já funcionaram assim no dev.)
3. **Versão Chatwoot**: **`v4.14.1-ce`** (latest stable, lançada 2026-05-29). Verificar tag `-ce` no Docker Hub na build; bump do dev atual (v4.13.0-ce).
4. **Versionamento do pack**: **dentro deste repo**, em `install-pack/`. Pack version-tracka o bridge; Action de publish da image mora aqui; `deploy/` já tem precedente (compose chatwoot + install.sh).

## 11. Comandos /goal prontos (para delegação)

### Goal A — repo (Go + stack bridge + publish), TDD

```
Implementar suporte swarm no bridge (auto-bootstrap DB + auto-migrate) e artefatos de deploy, com TDD, continuando v1.1.0. Stack Go 1.25, html/template; flat-first (1 pacote internal/bridge, cmd/bridge). Estado final: (1) cmd/bridge/main.go: em cmdServe, se env POSTGRES_ADMIN_URL setado, conectar no DB de manutencao e criar role bridge + database bridge (owner bridge) se ausentes, idempotente (checar pg_database/pg_roles antes de CREATE, ignorar erro ja-existe); extrair essa logica para funcao testavel bootstrapDatabase; (2) cmdServe roda migrations pendentes antes de servir quando env RUN_MIGRATIONS != "0" (default ligado), reusando a logica de applyMigrations/cmdMigrate sem duplicar; sem POSTGRES_ADMIN_URL o comportamento atual e preservado (assume DB existente); (3) env novos documentados no usage e no README de deploy: POSTGRES_ADMIN_URL (opcional, 1a subida) e RUN_MIGRATIONS (default 1); (4) install-pack/06-bridge-admin/bridge.yaml: stack swarm v3.7, service bridge image ghcr.io/madeinlowcode/chatwoot-megaapi-bridge:v1.1.0, network external network_swarm_public, env MASTER_KEY/BRIDGE_ENCRYPTION_KEY/DATABASE_URL(postgres://bridge:...@postgres:5432/bridge)/POSTGRES_ADMIN_URL/RUN_MIGRATIONS=1/BRIDGE_PORT=8080, deploy replicas 1 placement manager restart_policy on-failure resources limits 0.5cpu/512M, sem labels traefik; (5) .github/workflows/publish-image.yml: build + push docker para ghcr no push de tag v*, tags vX.Y.Z + latest, usando docker/build-push-action; (6) docs/INSTALL-SWARM.md documentando deploy da stack do bridge + ingress cloudflare bridge.DOMINIO->http://bridge:8080. Tests novos (TDD RED->GREEN->REFACTOR, commit cada): TestBootstrapDatabaseIdempotent e TestServeMigrationsToggle cobrindo parse de env e decisao de rodar/pular (mockando o executor, sem exigir Postgres real no unit); suite existente continua verde. Provar com: `go test ./... 2>&1 | tail -20` mostrando ok em todos pacotes zero --- FAIL; `go vet ./... 2>&1 | tail -5` sem violacao; `golangci-lint run 2>&1 | tail -5` exits 0; `go build -o bridge.exe ./cmd/bridge 2>&1 | tail -3` sem stderr; `docker compose -f install-pack/06-bridge-admin/bridge.yaml config 2>&1 | tail -5` sem erro de parse; `grep -c "POSTGRES_ADMIN_URL" cmd/bridge/main.go` >=1; `grep -c "RUN_MIGRATIONS" cmd/bridge/main.go` >=1; `ls .github/workflows/publish-image.yml install-pack/06-bridge-admin/bridge.yaml docs/INSTALL-SWARM.md`. TDD obrigatorio. Sem adicionar deps Go (go.mod intocado), sem mudar rotas nem handlers HTTP, sem quebrar `bridge migrate`/`serve` standalone, sem migrations novas, sem //nolint, sem --no-verify, sem modificar go.mod/go.sum, sem force-push, sem commit vago (usar feat()/test()/refactor()/docs()/ci()), sem commitar segredos, or stop after 22 turns. Report turn count, testes passando, files alterados e remaining bound each turn. Claude must echo full output of each verification command.
```

### Goal B — pack authoring (depende do Goal A)

```
Montar o install-pack swarm completo (stacks numeradas + guia PT-BR) pro Chatwoot+bridge, padrao luizeof/Portainer, Cloudflare Tunnel pra tudo. Referencia das stacks do usuario ja lida (network_swarm_public, postgres pgvector pg16, cloudflared token). Estado final em install-pack/ dentro do repo: (1) 00-LEIA-PRIMEIRO.md guia PT-BR passo a passo (8 passos: infra, portainer, env, postgres, cloudflared, chatwoot, bridge, smoke) + como gerar MASTER_KEY (openssl rand -base64 32) + pegar token tunnel + criar ingress Zero Trust (chat.DOMINIO->chatwoot_admin:3000, bridge.DOMINIO->bridge:8080) + troubleshooting; (2) .env.modelo com todas as vars comentadas; (3) 01-infra/ scripts swarm init + create network network_swarm_public (overlay attachable); (4) 02-portainer/portainer.yaml (portainer-ce + agent, sem traefik, acesso via tunnel); (5) 03-postgres/postgres.yaml pgvector/pgvector:pg16 service postgres, placement manager, volume postgres_data, wal_level minimal; (6) 04-cloudflared/cloudflared.yaml tunnel --no-autoupdate --protocol auto run --token, global, na overlay; (7) 05-chatwoot/ com chatwoot_admin + chatwoot_api + chatwoot_sidekiq + redis, image chatwoot/chatwoot:v4.14.1-ce, ACTIVE_STORAGE_SERVICE=local + volume chatwoot_storage, FRONTEND_URL=https://chat.DOMINIO, FORCE_SSL=false, POSTGRES_HOST=postgres, SEM labels traefik (roteado por tunnel); (8) 06-bridge-admin/ referencia a stack do bridge (do Goal A) com nota; (9) 07-backup/pgbackupweb.yaml apontando postgres, dumps chatwoot+bridge, retencao 14d/4w/6m. Todas as stacks usam network_swarm_public external e placeholders de senha/dominio em UPPERCASE pro usuario preencher no Portainer. Provar com: para cada arquivo .yaml em install-pack rodar `docker compose -f <arquivo> config 2>&1 | tail -3` sem erro de parse (echar cada um); `ls install-pack/00-LEIA-PRIMEIRO.md install-pack/.env.modelo`; `grep -rl "network_swarm_public" install-pack/ | wc -l` >=5; `grep -c "v4.14.1-ce" install-pack/05-chatwoot/*.yaml` >=1; `grep -rc "ACTIVE_STORAGE_SERVICE=local" install-pack/05-chatwoot/ | tail -1`; `grep -rEc "traefik" install-pack/05-chatwoot/ install-pack/06-bridge-admin/ | grep -v ":0" | wc -l` retorna `0` (zero labels traefik). Sem incluir senhas/tokens reais (so placeholders UPPERCASE), sem Minio/S3 (storage local), sem labels traefik nas stacks, sem abrir portas 80/443 (tunnel), sem --no-verify, sem force-push, sem commit vago (usar feat(deploy)/docs()), sem commitar segredos, or stop after 18 turns. Report turn count, stacks validadas, files criados e remaining bound each turn. Claude must echo full output of each verification command.
```

Ordem: Goal A primeiro (cria stack do bridge + image), depois Goal B (referencia). Pre-requisito de deploy real: Action de publish (Goal A) deve rodar e a image v1.1.0 subir no ghcr publico.

## 10. Riscos remanescentes

- **Image ghcr** (seção 5) — bloqueante, resolver primeiro.
- **Chatwoot atrás de tunnel** — ActionCable (websocket) via tunnel: usar `--protocol auto` no cloudflared (não `h2mux`) se WS falhar; `FRONTEND_URL=https://chat.DOMINIO`, `FORCE_SSL=false` (TLS termina no edge Cloudflare, origin http), confiar `X-Forwarded-Proto` setado pelo tunnel.
- **SMTP** chatwoot — opcional, documentar (sem SMTP: reset de senha/convites não funcionam, mas operação básica sim).
- **Backup retention** — definir dias/semanas (refs usam 14d/4w/6m).
