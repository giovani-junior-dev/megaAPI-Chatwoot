# Reset MVP — chatwoot-megaapi-bridge

## Goal

Bridge HTTP **flat-first** entre megaAPI e Chatwoot, **open-source**, substituindo
integração via n8n. Suporte a múltiplos tenants (cada cliente da megaAPI), alta vazão
(1000+ msg/s sustentado), instalação simples (`docker compose up`), sem over-engineering.

**Substitui completamente a Fase 1 anterior** (4848 LOC, 14 pacotes, 8 tabelas) por
implementação flat com ~640 LOC produção + ~280 LOC tests = **920 LOC total**.

## Context (por que reset)

A Fase 1 entregue via archon resultou em:
- 14 pacotes Go internos (camada-mania) → deveria ser 1 pacote
- 8 tabelas DB (audit_events, idempotency_keys, admin_users, etc) → 3 bastam
- sqlc + queries/*.sql + repo/queries.go (drift detectado no review) → SQL inline pgx
- asynq+Redis pra filas → channels in-process suportam até 5k msg/s
- 2 binários (bridge-api + bridge-worker) → 1 binário, goroutines internas
- 4 P1 issues "interface refactor" no F2 → só existem porque arquitetura camada-mania exige interfaces pra mockar; flat design não cria esse problema
- 126 issues bd, 14 docs antes de 1 linha de código

Custo de descartar: **zero** (sem cliente em produção). Refactor radical via
`code-craftsman` skill: YAGNI + flat-first.

## Architecture (alvo)

```
chatwoot-megaapi-bridge/
├── cmd/bridge/main.go              ~120 LOC: bootstrap + flag parser + 3 subcomandos
├── internal/bridge/
│   ├── server.go                   ~120 LOC: chi router + 4 handlers
│   ├── bridge.go                   ~150 LOC: WA↔CW logic + worker pool
│   ├── storage.go                  ~100 LOC: pgx pool + 5 funções SQL
│   ├── crypto.go                   ~50 LOC:  AES-GCM + HMAC
│   ├── server_test.go              ~80 LOC:  http handlers branches (httptest)
│   ├── storage_test.go             ~60 LOC:  testcontainers PG real
│   ├── bridge_test.go              ~100 LOC: E2E mock servers
│   └── crypto_test.go              ~40 LOC:  encrypt roundtrip + HMAC timing
├── migrations/0001_init.sql        3 tabelas
├── Dockerfile                      multi-stage scratch
├── docker-compose.yml              PG + bridge
├── .env.example
├── README.md                       ~80 linhas
├── LICENSE                         MIT
└── Makefile                        test, lint, build, integration
```

**Boundaries:**
1. Tudo em `internal/bridge/`. Sem subdomain folders.
2. SQL inline em storage.go. Sem queries/*.sql duplicado.
3. Sem interface preventiva. Mock só se 2º caso real surgir.
4. Sem worker binary separado. 1 processo, channels + goroutines.
5. Sem CLI framework — `flag` stdlib.

## Stack

| Pkg | Versão | Razão |
|---|---|---|
| Go | 1.23+ (1.26 já instalado) | scratch image, goroutines, stdlib HTTP forte |
| github.com/go-chi/chi/v5 | v5 | router leve |
| github.com/jackc/pgx/v5 | v5 | driver PG nativo, sem ORM |
| github.com/rs/zerolog | v1 | log JSON estruturado |
| github.com/google/uuid | v1 | UUID gen/parse |
| github.com/testcontainers/testcontainers-go | v0 | PG real em test |
| github.com/stretchr/testify | v1 | assertions |

**Removido vs F1 antiga:** asynq, redis client, sqlc, goose, koanf, golang-lru,
spf13/cobra, modernc/sqlite, sethvargo/go-retry, robfig/cron, multiple internal pkgs
(httpx, queue, observability, repo, tenant, megaapi, chatwoot, handler, worker, config,
db).

## Schemas

```sql
-- migrations/0001_init.sql
CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE tenants (
  id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  slug                TEXT UNIQUE NOT NULL CHECK (slug ~ '^[a-z0-9][a-z0-9-]{2,63}$'),
  megaapi_host        TEXT NOT NULL,
  megaapi_instance    TEXT NOT NULL,
  megaapi_token_enc   BYTEA NOT NULL,
  chatwoot_url        TEXT NOT NULL,
  chatwoot_token_enc  BYTEA NOT NULL,
  chatwoot_account_id INTEGER NOT NULL,
  chatwoot_inbox_id   INTEGER NOT NULL,
  hmac_secret_enc     BYTEA NOT NULL,
  webhook_bearer_enc  BYTEA NOT NULL,
  created_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE contacts (
  tenant_id           UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  wa_jid              TEXT NOT NULL,
  cw_contact_id       BIGINT NOT NULL,
  cw_conversation_id  BIGINT NOT NULL,
  updated_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (tenant_id, wa_jid)
);

CREATE TABLE messages (
  id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id    UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  direction    TEXT NOT NULL CHECK (direction IN ('in','out')),
  external_id  TEXT NOT NULL,
  status       TEXT NOT NULL CHECK (status IN ('pending','done','failed')),
  attempts     SMALLINT NOT NULL DEFAULT 0,
  last_error   TEXT,
  created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE(tenant_id, direction, external_id)
);

CREATE INDEX idx_messages_pending ON messages(tenant_id) WHERE status = 'pending';
```

3 tabelas. Idempotência via `UNIQUE(tenant_id, direction, external_id)`. Sem
`idempotency_keys`, `audit_events`, `admin_users`.

## Endpoints

| Método | Path | Auth | Função |
|---|---|---|---|
| POST | `/v1/wa/{slug}` | `Authorization: Bearer <token>` | webhook megaAPI |
| POST | `/v1/cw/{slug}` | `X-Chatwoot-Signature: <hmac-sha256-hex>` | webhook Chatwoot |
| GET | `/healthz` | none | liveness 200 |
| GET | `/readyz` | none | ping PG + queue depth; 503 se cheio |

ACK 200 em <10ms. Channel cheio → 503.

## Requirements (EARS)

```
REQ-1 (event-driven): When megaAPI webhook arrives at /v1/wa/{slug} with valid Bearer,
  bridge shall enqueue inbound job and respond 200 within 10ms p99.

REQ-2 (event-driven): When inbound job processes, bridge shall create/find Chatwoot
  contact, create/find conversation, post message with content_attributes.external_id.

REQ-3 (event-driven): When Chatwoot webhook arrives at /v1/cw/{slug} with valid HMAC,
  bridge shall enqueue outbound job (filtering only outgoing user-typed messages)
  and respond 200 within 10ms p99.

REQ-4 (event-driven): When outbound job processes, bridge shall POST sendMessage to
  {tenant.megaapi_host}/rest/sendMessage/{instance}/text with decrypted bearer token.

REQ-5 (state-driven): While message.status='pending', bridge shall retry up to 3
  times with backoff (1s, 5s, 30s) before transitioning to 'failed'.

REQ-6 (unwanted): If Bearer invalid, bridge shall reject with 401.

REQ-7 (unwanted): If HMAC invalid, bridge shall reject with 401.

REQ-8 (unwanted): If queue depth > BUFFER_LIMIT (default 1000), /readyz shall 503.

REQ-9 (unwanted): If message external_id already exists for tenant+direction,
  bridge shall ACK 200 without re-enqueueing (idempotent via UNIQUE constraint).

REQ-10 (ubiquitous): bridge shall encrypt all tokens at rest with AES-256-GCM
  using MASTER_KEY env var (32 bytes base64).

REQ-11 (event-driven): When bridge starts, it shall query messages WHERE status='pending'
  and re-enqueue them (recovery from restart).
```

## Components

```
storage.go
├── type DB struct { pool *pgxpool.Pool }
├── func NewDB(ctx, dsn) (*DB, error)
├── func (DB) GetTenantBySlug(ctx, slug) (Tenant, error)
├── func (DB) UpsertContact(ctx, tenantID, jid, cwContact, cwConv) error
├── func (DB) GetContact(ctx, tenantID, jid) (Contact, error)
├── func (DB) InsertMessage(ctx, m Message) (created bool, error)
├── func (DB) MarkStatus(ctx, id, status, err string) error
├── func (DB) IncrementAttempts(ctx, id) error
└── func (DB) NextPending(ctx) ([]Message, error)

crypto.go
├── func Encrypt(plaintext, key []byte) ([]byte, error)
├── func Decrypt(ciphertext, key []byte) ([]byte, error)
├── func VerifyHMAC(body []byte, signature, secret string) bool
└── func RandomBytes(n int) []byte

server.go
├── type Config struct { ... }
├── type Server struct { db *DB; key []byte; in, out chan Job }
├── func NewServer(db *DB, key []byte, cfg Config) *Server
├── func (s *Server) Routes() http.Handler
├── func (s *Server) handleWAWebhook(w, r)
├── func (s *Server) handleCWWebhook(w, r)
├── func (s *Server) handleHealth(w, r)
└── func (s *Server) handleReady(w, r)

bridge.go
├── type Job struct { TenantID uuid.UUID; MessageID uuid.UUID; Payload []byte }
├── func (s *Server) RunWorkers(ctx) — 2 pools: inbound + outbound
├── func (s *Server) processInbound(ctx, job) error
├── func (s *Server) processOutbound(ctx, job) error
├── func (s *Server) resolveContact(ctx, tenant, jid, name) (cwContactID, cwConvID, error)
├── func (s *Server) postChatwootMessage(ctx, tenant, conv, content, externalID) error
├── func (s *Server) sendMegaAPIText(ctx, tenant, to, text) error
└── func (s *Server) recoverPending(ctx) error  — startup

main.go
├── func main()
├── func cmdServe(ctx, cfg)
├── func cmdTenantAdd(ctx, cfg)  — interactive prompt + validate
└── func cmdMigrate(ctx, cfg)    — embed migrations
```

## Concurrency

```
HTTP req                       Channel             Worker (×N=GOMAXPROCS×4)
────────                       ───────             ─────────────────────
auth check                     buf=1000           for job := range chan {
parse JSON minimal                                  for attempt := 1..3 {
db.InsertMessage(pending)                             err := process(job)
chan <- Job (or 503 if full)                          if err == nil { break }
ACK 200                                               sleep(backoff[attempt])
                                                    }
                                                    db.MarkStatus(done|failed)
                                                  }
```

- 2 channels: `inboundQueue`, `outboundQueue`, buf=1000 cada.
- N workers cada (GOMAXPROCS×4 default).
- Backpressure via canal.
- Recovery: startup busca `pending` no DB, re-enfileira.

## Test matrix (TDD mandatory)

| Tipo | Tool | Coverage | Scope |
|---|---|---|---|
| Unit | testify | ≥80% novo cód | crypto.go (encrypt/decrypt/HMAC), helpers em bridge |
| Integration | testcontainers-go | 100% storage | storage.go contra PG real |
| HTTP | httptest | todas branches | server.go (auth, body limit, idempotência, 503 backpressure) |
| E2E | testcontainers + httptest mock | golden + 3 erros | bridge.go (WA→CW + CW→WA via mocks) |
| Mutation | go-mutesting | ≥70% | crypto.go, auth |

**TDD strict:** RED test → GREEN minimal → REFACTOR. Cada milestone abaixo segue.

## Implementation tasks (TDD order)

### M1 — Schema + storage (1 dia)
1. **RED**: `TestUpsertContact_CreatesNew` em `storage_test.go` (testcontainers)
2. **GREEN**: minimal `migrations/0001_init.sql` + `DB.UpsertContact`
3. RED: `TestInsertMessage_DuplicateReturnsCreatedFalse`
4. GREEN: `DB.InsertMessage` com `ON CONFLICT DO NOTHING RETURNING true`
5. RED: `TestNextPending_OrdersByCreatedAt`
6. GREEN: `DB.NextPending`
7. RED: `TestMarkStatus_PreservesAttempts` + `TestIncrementAttempts`
8. GREEN: `MarkStatus`, `IncrementAttempts`
9. RED: `TestGetTenantBySlug_NotFoundError`
10. GREEN: `GetTenantBySlug`
11. REFACTOR: extrair `scanTenant`, `scanMessage` helpers se duplicação >2x

### M2 — Crypto (0.5 dia)
1. RED: `TestEncrypt_RoundtripWithSameKey`, `TestDecrypt_WrongKeyFails`,
   `TestDecrypt_TamperedFails`, `TestDecrypt_TruncatedFails`
2. GREEN: `Encrypt`, `Decrypt` (AES-256-GCM, 96-bit nonce prepended)
3. RED: `TestVerifyHMAC_ValidAccepted`, `TestVerifyHMAC_InvalidRejected`,
   `TestVerifyHMAC_TimingSafe` (uses subtle.ConstantTimeCompare)
4. GREEN: `VerifyHMAC`
5. RED: `TestRandomBytes_LengthN` + uniqueness sample
6. GREEN: `RandomBytes`

### M3 — Server + handlers (1 dia)
1. RED: `TestHealthz_Always200`
2. GREEN: `handleHealth`
3. RED: `TestWAWebhook_MissingBearer401`, `TestWAWebhook_WrongBearer401`
4. GREEN: `handleWAWebhook` + auth
5. RED: `TestWAWebhook_ValidBearerEnqueues`
6. GREEN: parse + db.InsertMessage + chan send
7. RED: `TestWAWebhook_DuplicateExternalID_200NoEnqueue`
8. GREEN: idempotência via `created` flag de InsertMessage
9. RED: `TestWAWebhook_QueueFull_503`
10. GREEN: select com default → 503
11. RED: `TestCWWebhook_InvalidHMAC401`, `TestCWWebhook_ValidHMACEnqueues`
12. GREEN: `handleCWWebhook` (com filtragem `event=message_created` +
    `message_type=outgoing` + `private=false`)
13. RED: `TestReadyz_QueueOK_200`, `TestReadyz_QueueFull_503`,
    `TestReadyz_DBDown_503`
14. GREEN: `handleReady`

### M4 — Bridge core (1.5 dia)
1. RED: `TestProcessInbound_CreatesContactAndPostsMessage` (mocks Chatwoot via
   httptest)
2. GREEN: `processInbound`, `resolveContact`, `postChatwootMessage`
3. RED: `TestProcessInbound_ReusesExistingContact`
4. GREEN: lookup `contacts` antes de criar
5. RED: `TestProcessInbound_ChatwootError_RetryOnce_ThenFails`
6. GREEN: retry loop com 3 attempts, exp backoff
7. RED: `TestProcessOutbound_SendsMegaAPIText`
8. GREEN: `processOutbound`, `sendMegaAPIText`
9. RED: `TestProcessOutbound_4xxNoRetry`, `TestProcessOutbound_5xxRetries`
10. GREEN: classificação retriable
11. RED: `TestRunWorkers_DrainsChannel`
12. GREEN: `RunWorkers` com errgroup + ctx.Done
13. RED: `TestRecoverPending_RequeuesPendingMessages`
14. GREEN: `recoverPending` no startup

### M5 — CLI + main (0.5 dia)
1. RED: `TestCmdTenantAdd_GeneratesBearer_HMAC_EncryptsTokens` (table-driven)
2. GREEN: `cmdTenantAdd` com prompt stdin (ou `--non-interactive` flag pra test)
3. RED: `TestCmdTenantAdd_InvalidMegaAPIHost_Aborts` (HEAD timeout)
4. GREEN: pre-flight HEAD check com timeout 5s
5. RED: `TestCmdMigrate_ApplyEmbedded`
6. GREEN: embed.FS + `tern`/manual exec via pgx
7. Wire `main.go` com flag.Parse + dispatch

### M6 — Docker + compose + README (0.5 dia)
1. Dockerfile multi-stage (build + scratch)
2. `docker-compose.yml`: postgres:15-alpine + bridge service
3. `.env.example`
4. `Makefile`: test, lint, build, integration, run-local
5. `README.md`: 1 página com quickstart + arch 1 parágrafo + link `/docs/`
6. `LICENSE` MIT

### M7 — E2E real validação (1 dia)
1. Subir compose local
2. `bridge tenant add` interativo (test com megaAPI demo se possível)
3. Mock megaAPI server local (httptest standalone) pra teste sem rede
4. Test E2E: payload simulado WhatsApp → Chatwoot mock recebe POST
5. Test E2E: payload Chatwoot → megaAPI mock recebe sendMessage
6. Documentar em README screencast 1min

## Trade-offs

| Decisão | Razão | Rejeitado |
|---|---|---|
| 1 pacote `internal/bridge` | Flat-first; refactor quando 2º caso pedir | 14 pacotes (F1 anterior) |
| Channels in-process | 1000+ msg/s sem dep extra | asynq+Redis |
| SQL inline pgx | 5 funções; sqlc dá drift > valor | sqlc + queries/*.sql |
| Sem DLQ table | `WHERE status='failed'` é DLQ | tabela dedicada |
| Sem audit_events | logs zerolog stdout | append-only table |
| Sem cache LRU | 1 query indexed/req aceitável até 10k req/s | golang-lru |
| AES-GCM mantido | open-source: DBA != admin | sem cifragem |
| HMAC mantido | obrigatório produção | bearer-only |
| 1 binário | bridge-api + bridge-worker era ritual | 2 binários |
| stdlib `flag` | 3 subcomandos não justifica framework | cobra |
| testcontainers PG | tests confiáveis SQL real | mocks de DB |
| Sem interface preventiva | mock direto httptest | repo/lookuper interfaces |

## Migration plan (do F1 atual → reset)

| Passo | Ação |
|---|---|
| 1 | Worktree archon parte de `master` (que tem só docs + bd + plans) |
| 2 | archon cria branch `feat/reset-mvp` |
| 3 | archon implementa do zero seguindo M1–M7 (TDD) |
| 4 | archon abre PR contra master |
| 5 | Após merge: fechar PR #1 antigo, deletar branch `archon/task-feat-phase-1-mvp` |
| 6 | Atualizar `/docs/02-architecture.md` (deletar sub-pacotes) |
| 7 | Atualizar `/docs/05-data-model.md` (3 tabelas) |
| 8 | Atualizar `/docs/12-roadmap.md` (revisar fases) |
| 9 | Adicionar ADRs 11–14 em `/docs/13-risks-and-decisions.md` |
| 10 | Fechar bd issues 8s8.11–24 (não aplicáveis) |

## ADRs novos (criar em /docs/13-risks-and-decisions.md)

- **ADR-011**: Reset radical do F1 — flat-first sobre Clean Arch
- **ADR-012**: Channels in-process sobre asynq+Redis até 5k msg/s
- **ADR-013**: SQL inline pgx sobre sqlc (drift > valor)
- **ADR-014**: 1 binário 1 processo sobre 2 binários

## Acceptance criteria

- [ ] `go test ./...` 100% verde
- [ ] `go vet ./...` zero issues
- [ ] `golangci-lint run` zero violations
- [ ] Coverage novo cód ≥80%
- [ ] Mutation score ≥70% (crypto.go, auth)
- [ ] Imagem Docker `<30 MB`
- [ ] `docker compose up -d` saudável `<30s`
- [ ] CLI `bridge tenant add` funcional + valida megaAPI/Chatwoot HEAD
- [ ] `bridge serve` aceita webhook teste, processa via worker pool, retorna 200
- [ ] Idempotência testada (replay = 200 sem duplicar)
- [ ] HMAC inválido → 401
- [ ] Bearer inválido → 401
- [ ] Queue cheia → 503 em /readyz
- [ ] Recovery: matar processo com pending → restart → drena
- [ ] README quickstart 5 comandos
- [ ] LICENSE MIT
- [ ] Linhas de código: produção ≤700, tests ≤350
- [ ] Imports por arquivo ≤15
- [ ] Funções ≤20 linhas
- [ ] Indentação ≤2 níveis

## Out of scope (deferido — só se cliente pedir)

| Feature | Quando trazer |
|---|---|
| UI admin web | 3+ clientes pedirem |
| Mídia (image/audio/video/doc) | Próxima fase após texto E2E real |
| asynq+Redis | Quando > 5k msg/s sustentado |
| Métricas Prometheus | Quando observabilidade ativa pedida |
| AlertManager | Idem |
| Múltiplos admins | Quando time crescer |
| 2FA TOTP | Idem |
| Helm chart K8s | Cliente pedir K8s |
| Cloudflare Tunnel | Doc apenas, sem bundle |
| Driver pluggável (Evolution/WAHA) | 1º cliente pedir |
| Auto-registro webhook | v0.2 |

YAGNI rigoroso. Só implementar feature quando 2º caso real surgir.

## Verification

```bash
# Auto
make test
make integration   # testcontainers
make lint
make build

# E2E manual (após auto verde)
docker compose up -d
./bridge migrate
./bridge tenant add --slug demo \
  --megaapi-host https://apibusiness7.megaapi.com.br \
  --megaapi-instance abc \
  --megaapi-token ... \
  --chatwoot-url https://atendimento.local \
  --chatwoot-token ... \
  --chatwoot-account 1 \
  --chatwoot-inbox 5
./bridge serve

# Test webhook simulado
curl -X POST -H "Authorization: Bearer $TOK" \
     -H "Content-Type: application/json" \
     -d @testdata/wa_text.json \
     http://localhost:8080/v1/wa/demo
# Verificar log + Chatwoot recebeu
```

## Constraints (enforced)

- Funções ≤ 20 linhas
- Indentação ≤ 2 níveis
- Parâmetros ≤ 2 (3+ usar struct)
- Arquivos ≤ 500 linhas (média ~150)
- Imports ≤ 15 por arquivo
- Sem `Manager`/`Util`/`Helper`/`Service` genérico
- Sem interface sem 2ª implementação real
- Sem comentário sem WHY
- TDD obrigatório: RED test antes de production code
