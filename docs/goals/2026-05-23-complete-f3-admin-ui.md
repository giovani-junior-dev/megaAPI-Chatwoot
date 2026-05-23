# Goal Plan — complete-f3-admin-ui

## 1. Contexto

- Repo: `C:\Users\GEOVANE\Desktop\Projetos\chatwoot-megaapi-bridge`
- Stack: Go 1.25, chi v5, pgx v5, zerolog. UI nova: htmx + Tailwind + alpine via `//go:embed`. Cookie session auth. Single admin (não SaaS multi-tenant).
- Estado atual:
  - F1 + F2 + F-media completos. 121 unit + 153 integration testes passando.
  - F3 schema (834.13) já entregue: tabelas `admins` + `settings` (migration 0003).
  - 12 sub-issues F3 abertos em bd label `phase-3`.
  - Bridge único pkg `internal/bridge` (flat-first, sem interfaces especulativas).
  - Master key 32 bytes base64 em env `MASTER_KEY`.
- Comando teste unit: `go test ./...`
- Comando teste integração: `go test -tags=integration ./...`
- Comando lint: `go vet ./...`
- Comando build: `go build ./...`

## 2. Estado final mensurável

Todas 12 sub-issues F3 implementadas com TDD red-green-refactor, fechadas no bd, commits pushed. Endpoints HTTP novos respondendo com status esperados.

| bd | Tarefa | Endpoint/feature verificável |
|---|---|---|
| 834.1 | Layout htmx+Tailwind+alpine base | `GET /` retorna HTML com `<html` + `htmx.org` + `tailwindcss` |
| 834.2 | Login admin (argon2id + sessão) | `POST /login` válido seta cookie `bridge_session`; bcrypt/argon2id rejeita senha errada com 401; `bridge admin add` CLI cria admin via password_hash; cookie httponly+secure |
| 834.3 | Wizard "Novo Tenant" 4 passos | `GET /tenants/new` retorna 200 com form; `POST /tenants` cria registro DB + retorna 302 dashboard |
| 834.4 | Auto-discovery inboxes Chatwoot | `POST /tenants/discover-inboxes` recebe `{url, token}` e retorna JSON com array de inboxes |
| 834.5 | Auto-config webhook megaAPI | Bridge chama `POST {host}/rest/webhook/{instance}/configWebhook` body `{messageData:{webhookUrl, webhookEnabled:true}}` ao finalizar wizard; mockado via httptest no teste |
| 834.6 | Dashboard tenant + sparkline 24h | `GET /` autenticado retorna HTML listando tenants com count msgs últimas 24h |
| 834.7 | Diagnóstico 1-clique (checklist live) | `GET /tenants/{slug}/diag` retorna JSON `{megaapi:bool, chatwoot:bool, webhook:bool, db:bool}` |
| 834.8 | Log mensagens paginado | `GET /messages?tenant={slug}&page=N` retorna HTML com tabela + filtros |
| 834.9 | DLQ admin UI | `GET /dlq` lista status=failed; `POST /dlq/retry/{id}` (reusa lógica F2 8s8.4) muda failed→pending |
| 834.10 | Settings: base_url + senha + master key info | `GET /settings` form; `POST /settings/base_url` persiste em `settings` k/v |
| 834.11 | i18n PT-BR | Strings em `web/i18n/pt-BR.json`; helper `t(key)` em templates; teste valida ≥20 chaves presentes |
| 834.12 | Caddy headers CSP/HSTS | `deploy/Caddyfile` adicionado com `Strict-Transport-Security`, `Content-Security-Policy`, `X-Frame-Options: DENY` |

## 3. Prova surfaceável

Comandos a cada turno (echo full output):

```bash
go test ./... 2>&1 | tail -3
go test -tags=integration ./... 2>&1 | tail -3
go vet ./... 2>&1 | tail -3
bd list --status=open --label=phase-3 2>&1 | tail -5
git log --oneline -15
docker compose up -d --build bridge 2>&1 | tail -2
curl -sS -o /dev/null -w "/healthz: %{http_code}\n" http://localhost:8090/healthz
curl -sS -o /dev/null -w "/login: %{http_code}\n" http://localhost:8090/login
curl -sS -o /dev/null -w "/metrics: %{http_code}\n" http://localhost:8090/metrics
```

Output esperado ao concluir:

- `go test ./...`: `ok` em todos pacotes, ≥130 passed
- `go test -tags=integration`: `ok`, ≥175 passed
- `go vet`: zero issues
- `bd list --status=open --label=phase-3 | tail`: só epic `chatwoot-megaapi-bridge-834` ainda open
- `git log --oneline -15`: 12 novos commits prefixados `feat(ui):`, `feat(auth):`, `feat(wizard):`, etc.
- `curl /login`: 200 (form login renderiza)
- `curl /healthz`, `/metrics`: 200

Frequência: a cada turno após implementação de cada sub-issue.

## 4. Restrições

### Específicas do projeto

- NÃO modificar schema das tabelas existentes (tenants, contacts, messages, admins, settings) exceto via migration 0004+
- NÃO criar pacotes novos fora de `internal/bridge/` exceto `internal/bridge/web/` (templates+static embeddados) e `deploy/` (Caddyfile)
- NÃO trocar chi router por outro framework
- NÃO usar React/Vue/Next/build-step JS — somente htmx via CDN copiado pra static, Tailwind compilado pre-build OU CDN, alpine.js standalone
- Templates HTML via `html/template` stdlib + `//go:embed`
- Funções ≤20 linhas, ≤2 parâmetros (ctx exceto)
- Sem interfaces especulativas (mantém flat 1 pkg)
- TDD obrigatório: teste falhando primeiro, então impl
- Bearer/sessão: cookie httponly + samesite=lax + secure quando HTTPS detectado via header X-Forwarded-Proto
- Senha admin: argon2id (golang.org/x/crypto/argon2) — NÃO bcrypt, NÃO MD5
- Endpoints admin existentes (`/admin/failed`, `/admin/retry/{id}` do F2) DEVEM continuar funcionando com Bearer ADMIN_TOKEN — UI nova é alternativa, não substitui
- Cada sub-issue concluída → `bd close <id>` antes da próxima
- Cada commit: 1 sub-issue (1 PR atômico por feature)
- Bridge deve continuar single binary scratch container — NÃO adicionar deps que exijam CGO
- Master key continua única do deploy (não por admin)

### Padrão

- NÃO `--no-verify`
- NÃO `//nolint`
- NÃO modificar `go.sum` exceto via `go get`
- NÃO commitar segredos
- NÃO force-push
- Mensagens commit descritivas (`feat(ui):`, `feat(auth):`, `feat(wizard):`, `test(...)`)

## 5. Bound

- **120 turnos** OU **360 minutos** (o que vier primeiro)
- Justificativa: 12 sub-issues × ~10 turns cada (TDD + commit + close bd). Inclui rebuilds Docker + testcontainers. F3 maior que F2 (UI envolve templates).

## 6. Modo de execução recomendado

- Auto mode: ligar (`Shift+Tab` até "acceptEdits")
- Headless opcional: `claude -p "/goal <condição>"` para CI/cron

## 7. Condição final (cole no /goal)

```
F3 admin UI completa: 12 sub-issues abertas do épico chatwoot-megaapi-bridge-834 implementadas com TDD red-green. Stack: htmx+Tailwind+alpine via //go:embed, cookie session auth single admin, argon2id password. Estado final por sub-issue: (1) 834.1 layout base HTML embeddado retornado em GET /; (2) 834.2 login: POST /login válido seta cookie bridge_session httponly samesite=lax, senha errada 401, argon2id hash, CLI bridge admin add cria admin; (3) 834.3 wizard 4 passos GET /tenants/new + POST /tenants persiste em DB + redirect 302; (4) 834.4 POST /tenants/discover-inboxes retorna JSON array inboxes via Chatwoot API; (5) 834.5 bridge chama POST {host}/rest/webhook/{instance}/configWebhook com body messageData webhookUrl webhookEnabled true ao finalizar wizard (mock httptest); (6) 834.6 GET / autenticado lista tenants com msg count 24h; (7) 834.7 GET /tenants/{slug}/diag JSON com flags megaapi/chatwoot/webhook/db; (8) 834.8 GET /messages?tenant=&page= HTML paginado; (9) 834.9 GET /dlq lista failed + POST /dlq/retry/{id} muda failed para pending; (10) 834.10 GET /settings + POST /settings/base_url persiste em tabela settings k/v; (11) 834.11 web/i18n/pt-BR.json com ≥20 strings + helper t() em templates + teste valida chaves; (12) 834.12 deploy/Caddyfile com Strict-Transport-Security Content-Security-Policy X-Frame-Options DENY. Provar com: `go test ./... 2>&1 | tail -3` mostrando ok em todos pacotes com ≥130 testes passed; `go test -tags=integration ./... 2>&1 | tail -3` mostrando ok com ≥175 passed; `go vet ./... 2>&1` zero output; `bd list --status=open --label=phase-3 2>&1 | tail -5` mostrando só epic 834 ainda open; `git log --oneline -15` com 12 novos commits feat(ui)/feat(auth)/feat(wizard)/feat(diag)/test(...); `curl -sS -o /dev/null -w "%{http_code}\n" http://localhost:8090/login` retorna 200; `curl -sS -o /dev/null -w "%{http_code}\n" http://localhost:8090/healthz` retorna 200. Restrições: schema só via migrations 0004+, pacotes novos só internal/bridge/web/ + deploy/, NÃO React/Vue/Next/build-step JS, html/template stdlib + //go:embed, funções ≤20 linhas ≤2 params (ctx exceto), sem interfaces especulativas, TDD teste-primeiro obrigatório, cookie httponly samesite=lax, argon2id NÃO bcrypt NÃO MD5, endpoints F2 /admin/failed e /admin/retry/{id} continuam funcionando, cada sub-issue concluída roda `bd close <id>` antes da próxima, scratch container sem CGO, sem --no-verify, sem //nolint, sem modificar go.sum exceto via go get, sem segredos, sem force-push, mensagens commit descritivas feat(scope):/test(scope):, or stop after 120 turns or 360m. Report turn count, sub-issue atual (1-12), unit/integration tests passed, bd open count, remaining bound each turn. Claude must echo full output of each verification command.
```

## 8. Comando completo

```
/goal F3 admin UI completa: 12 sub-issues abertas do épico chatwoot-megaapi-bridge-834 implementadas com TDD red-green. Stack: htmx+Tailwind+alpine via //go:embed, cookie session auth single admin, argon2id password. Estado final por sub-issue: (1) 834.1 layout base HTML embeddado retornado em GET /; (2) 834.2 login: POST /login válido seta cookie bridge_session httponly samesite=lax, senha errada 401, argon2id hash, CLI bridge admin add cria admin; (3) 834.3 wizard 4 passos GET /tenants/new + POST /tenants persiste em DB + redirect 302; (4) 834.4 POST /tenants/discover-inboxes retorna JSON array inboxes via Chatwoot API; (5) 834.5 bridge chama POST {host}/rest/webhook/{instance}/configWebhook com body messageData webhookUrl webhookEnabled true ao finalizar wizard (mock httptest); (6) 834.6 GET / autenticado lista tenants com msg count 24h; (7) 834.7 GET /tenants/{slug}/diag JSON com flags megaapi/chatwoot/webhook/db; (8) 834.8 GET /messages?tenant=&page= HTML paginado; (9) 834.9 GET /dlq lista failed + POST /dlq/retry/{id} muda failed para pending; (10) 834.10 GET /settings + POST /settings/base_url persiste em tabela settings k/v; (11) 834.11 web/i18n/pt-BR.json com ≥20 strings + helper t() em templates + teste valida chaves; (12) 834.12 deploy/Caddyfile com Strict-Transport-Security Content-Security-Policy X-Frame-Options DENY. Provar com: `go test ./... 2>&1 | tail -3` mostrando ok em todos pacotes com ≥130 testes passed; `go test -tags=integration ./... 2>&1 | tail -3` mostrando ok com ≥175 passed; `go vet ./... 2>&1` zero output; `bd list --status=open --label=phase-3 2>&1 | tail -5` mostrando só epic 834 ainda open; `git log --oneline -15` com 12 novos commits feat(ui)/feat(auth)/feat(wizard)/feat(diag)/test(...); `curl -sS -o /dev/null -w "%{http_code}\n" http://localhost:8090/login` retorna 200; `curl -sS -o /dev/null -w "%{http_code}\n" http://localhost:8090/healthz` retorna 200. Restrições: schema só via migrations 0004+, pacotes novos só internal/bridge/web/ + deploy/, NÃO React/Vue/Next/build-step JS, html/template stdlib + //go:embed, funções ≤20 linhas ≤2 params (ctx exceto), sem interfaces especulativas, TDD teste-primeiro obrigatório, cookie httponly samesite=lax, argon2id NÃO bcrypt NÃO MD5, endpoints F2 /admin/failed e /admin/retry/{id} continuam funcionando, cada sub-issue concluída roda `bd close <id>` antes da próxima, scratch container sem CGO, sem --no-verify, sem //nolint, sem modificar go.sum exceto via go get, sem segredos, sem force-push, mensagens commit descritivas feat(scope):/test(scope):, or stop after 120 turns or 360m. Report turn count, sub-issue atual (1-12), unit/integration tests passed, bd open count, remaining bound each turn. Claude must echo full output of each verification command.
```

## 9. Comando headless (opcional)

```
claude -p "/goal F3 admin UI completa: ... (mesma condição) ..."
```

## 10. Checklist pré-entrega

- [x] ≤4000 chars (condição ~3900 chars)
- [x] Comandos concretos (go test, go vet, bd list, git log, curl)
- [x] Output literal definido (ok, ≥130 passed, 200, etc.)
- [x] Restrições específicas + padrão
- [x] Bound presente (120 turns OR 360m)
- [x] Echo obrigatório
- [x] Arquivo salvo em `docs/goals/2026-05-23-complete-f3-admin-ui.md`
- [x] Slug segue convenção (`complete-<area>`)
- [x] Estado final falsificável
- [x] Comandos referenciados existem

## 11. Ordem sugerida das 12 sub-issues

| # | bd | Razão da ordem |
|---|---|---|
| 1 | 834.1 | Layout base — pré-req visual de tudo |
| 2 | 834.2 | Login — protege rotas internas |
| 3 | 834.10 | Settings (base_url) — wizard precisa pra gerar webhook URL |
| 4 | 834.4 | Auto-discovery Chatwoot — wizard precisa |
| 5 | 834.5 | Auto-config webhook megaAPI — wizard precisa |
| 6 | 834.3 | Wizard tenant (composto dos 3 anteriores) |
| 7 | 834.6 | Dashboard — visualização básica |
| 8 | 834.7 | Diagnóstico 1-clique |
| 9 | 834.8 | Log mensagens UI |
| 10 | 834.9 | DLQ admin UI (reusa endpoints F2) |
| 11 | 834.12 | Caddy headers (deploy concern) |
| 12 | 834.11 | i18n PT-BR (final polish) |

## 12. Lembretes operacionais

- Rebuild Docker após cada commit: `docker compose up -d --build bridge`
- Migration 0004+ via `docker compose exec bridge /bridge migrate`
- testcontainers requer Docker rodando
- gh auth switch para `giovani-junior-dev` antes do push
- htmx CDN: copiar `https://unpkg.com/htmx.org@2.0.4/dist/htmx.min.js` para `internal/bridge/web/static/htmx.min.js`
- alpine CDN: `https://cdn.jsdelivr.net/npm/alpinejs@3.x.x/dist/cdn.min.js`
- Tailwind: usar play CDN inicialmente (`https://cdn.tailwindcss.com`) — depois compilar pré-build se quiser zero CDN
- Para CLI admin: adicionar subcommand `bridge admin add --email --password` ou via env primeiro-boot
