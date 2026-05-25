# Goal Plan — harden-qa-sweep-tdd

## 1. Contexto

- Repo: `C:\Users\GEOVANE\Desktop\Projetos\chatwoot-megaapi-bridge`
- Stack: Go 1.25, chi v5, pgx v5, zerolog, htmx+Tailwind+alpine via go:embed.
- Estado atual:
  - v1.0.0 tagged + released GitHub. F1-F6 done.
  - 195 unit + 253 integration tests passando.
  - 2 bugs encontrados QA pass anterior (1 stale container, 1 SQL leak) — corrigidos commit 9f67d6e.
  - Pilot self-pilot Day 0 ativo (`docs/release/PILOT-LOG.md`).
- Comando teste unit: `go test ./...`
- Comando teste integração: `go test -tags=integration ./...`
- Comando lint: `go vet ./...`
- Comando build: `go build ./...`
- Skill ativa: `/code-craftsman` (TDD red-green obrigatório, funcs ≤20 linhas, sem interfaces especulativas, YAGNI).

## 2. Estado final mensurável

Sweep QA full-feature via agent `e2e-testing-specialist` + TDD fix de TODA falha encontrada. Estado final: relatório QA com 0 FAIL across matriz completa + suite testes cresceu (cobertura nova evita regressão) + cleanup do ambiente.

| Categoria | Verificável |
|---|---|
| QA matriz | Relatório agent diz `PASS` em todos 15+ cenários |
| Unit tests | ≥210 passed (atual 195 +15 regressão de cada fix) |
| Integration | ≥265 passed (atual 253 +12) |
| `go vet` | zero output |
| `gosec` HIGH | 0 |
| Bd issues criadas | Todas closed (cada bug encontrado → fix → close) |
| Container | Sempre rebuilt antes de live curl (regra crítica: container stale = falsos positivos) |
| Cleanup | Tenants de teste removidos do DB, settings reset |

## 3. Prova surfaceável

Comandos a cada turno (echo full output):

```bash
go test ./... -count=1 2>&1 | tail -3
go test -tags=integration -count=1 ./... 2>&1 | tail -3
go vet ./... 2>&1
bd list --status=open 2>&1 | grep -E "qa-sweep|pilot-incident" | head
git log --oneline -15
docker compose ps --format '{{.Service}}:{{.Status}}' 2>&1 | head
curl -sS -o /dev/null -w "/healthz:%{http_code}\n" http://localhost:8090/healthz
curl -sS -o /dev/null -w "/login:%{http_code}\n" http://localhost:8090/login
curl -sS -o /dev/null -w "/metrics:%{http_code}\n" http://localhost:8090/metrics
docker compose exec -T db psql -U bridge -d bridge -tA -c "SELECT slug FROM tenants;" 2>&1 | head -5
```

Output esperado ao concluir:

- `go test ./...`: `ok` em todos pacotes, ≥210 passed
- `go test -tags=integration`: `ok`, ≥265 passed
- `go vet`: zero
- bd qa-sweep/pilot-incident: vazio (todos closed)
- `git log`: novos commits prefixados `test(qa):`, `fix(qa):`, `chore(qa):`
- containers: todos `Up` ou `running`
- endpoints HTTP: 200 / 302 conforme esperado
- tenants: somente `e2e-teste` + `loadtest` (cleanup OK)

Frequência: a cada turno.

## 4. Restrições

### Específicas do projeto

- **TDD obrigatório SEMPRE**: para CADA bug encontrado, escrever teste RED reproduzindo → impl GREEN → refactor. Sem teste = bug não fixado.
- Sempre rebuild container antes de curl/QA agent: `docker compose up -d --build bridge`. Container stale = lição BUG-001.
- Funções novas ≤20 linhas, ≤2 parâmetros (ctx exceto). Code-craftsman bind.
- Sem interfaces especulativas — manter flat 1 pkg `internal/bridge`.
- Não modificar testes existentes exceto pra refletir mudança de contrato documentada.
- Cada bug encontrado → `bd create --title="qa-sweep: <descrição>" --parent=chatwoot-megaapi-bridge-efn.8`.
- Cada bug fixado → `bd close <id> --reason="fix commit <hash>"` ANTES da próxima.
- 1 commit por fix atômico (1 bug = 1 commit).
- Não criar PRs — commits direto master conforme padrão F1-F6.
- Não pular pyramid: unit → integration → live curl (rebuild antes do live).
- Erros UI devem ser PT-BR sem vazar implementação (lição BUG-002).
- Cleanup obrigatório ao final: deletar tenants de teste (`qa-sweep-*`), resetar settings de teste.

### Padrão

- NÃO `--no-verify`
- NÃO `//nolint`
- NÃO modificar `go.sum` exceto via `go get`
- NÃO commitar segredos
- NÃO force-push
- Mensagens commit descritivas (`test(qa):`, `fix(qa):`, `chore(qa):`)

## 5. Bound

- **60 turnos** OU **180 minutos**
- Justificativa: matriz 15+ cenários × ~3 turns cada (QA call + análise + se houver bug: red+green+commit+close). 15×3 = 45. +15 buffer para bugs complexos.

## 6. Modo de execução recomendado

- Auto mode: ligar (`Shift+Tab` até "acceptEdits")
- Headless: opcional `claude -p "/goal <condição>"`

## 7. Condição final (cole no /goal)

```
QA full sweep com TDD fix-loop completo. Rodar agente e2e-testing-specialist contra matriz 15+ cenários (auth flow login/logout/expired, dashboard, wizard 4 passos, auto-discovery, diagnóstico, mensagens paginadas, DLQ list+retry, settings persistência, pprof gated logado/deslogado, /healthz, /readyz, /metrics com bridge_*, /admin/failed Bearer, /v1/wa fromMe filter + bad slug 404 + missing extID 400, /v1/cw HMAC, format guards HEIC/GIF/WAV/WEBM, size guards, multipart upload Chatwoot, retry jitter, stale janitor, backpressure 80%, BIGINT migration, OTel opt-in noop, troubleshooting doc, deploy scripts bash -n, Caddyfile valid, gosec scan 0 HIGH, tag v1.0.0 presente). Cada FALHA encontrada vira bd issue + fix TDD red-green obrigatório (teste falhando primeiro depois impl mínima depois refactor) + 1 commit atômico + bd close. Estado final: relatório QA agent PASS em todos cenários, sem bd issues qa-sweep abertas, suite testes cresceu de baseline (≥210 unit ≥265 integration), go vet zero, gosec HIGH zero, containers todos up, endpoints HTTP respondem códigos esperados, cleanup completo (tenants qa-sweep-* removidos do DB, settings de teste resetadas, base_url produção mantida). Provar com: `go test ./... -count=1 2>&1 | tail -3` mostrando ok ≥210 passed; `go test -tags=integration -count=1 ./... 2>&1 | tail -3` mostrando ok ≥265 passed; `go vet ./... 2>&1` sem output; `bd list --status=open 2>&1 | grep qa-sweep` vazio; `git log --oneline -15` com commits test(qa)/fix(qa)/chore(qa); `docker compose ps --format "{{.Service}}:{{.Status}}"` mostrando todos Up; `curl -sS -o /dev/null -w "%{http_code}\n" http://localhost:8090/healthz` retorna 200; `curl -sS -o /dev/null -w "%{http_code}\n" http://localhost:8090/login` retorna 200; `curl -sS -o /dev/null -w "%{http_code}\n" http://localhost:8090/metrics` retorna 200; `docker compose exec -T db psql -U bridge -d bridge -tA -c "SELECT slug FROM tenants;"` listando somente e2e-teste e loadtest. Restrições: TDD obrigatório por fix (teste RED primeiro reproduzindo bug + impl GREEN minimal + refactor); sempre rebuild container antes de live curl (docker compose up -d --build bridge) pois stale container causa falsos positivos lição BUG-001; funções ≤20 linhas ≤2 params (ctx exceto); sem interfaces especulativas mantém flat 1 pkg internal/bridge; não modificar testes existentes exceto refletindo mudança contrato; cada bug bd create --parent chatwoot-megaapi-bridge-efn.8 e bd close ao fixar; 1 commit por fix atômico; mensagens UI PT-BR sem vazar implementação lição BUG-002; cleanup final tenants qa-sweep-* removidos settings teste resetadas base_url produção preservada; sem --no-verify; sem //nolint; sem modificar go.sum exceto via go get; sem segredos; sem force-push; mensagens commit descritivas test(qa)/fix(qa)/chore(qa), or stop after 60 turns or 180m. Report turn count, cenários PASS/FAIL, bugs abertos, testes passed, remaining bound each turn. Claude must echo full output of each verification command.
```

## 8. Comando completo

```
/goal QA full sweep com TDD fix-loop completo. Rodar agente e2e-testing-specialist contra matriz 15+ cenários (auth flow login/logout/expired, dashboard, wizard 4 passos, auto-discovery, diagnóstico, mensagens paginadas, DLQ list+retry, settings persistência, pprof gated logado/deslogado, /healthz, /readyz, /metrics com bridge_*, /admin/failed Bearer, /v1/wa fromMe filter + bad slug 404 + missing extID 400, /v1/cw HMAC, format guards HEIC/GIF/WAV/WEBM, size guards, multipart upload Chatwoot, retry jitter, stale janitor, backpressure 80%, BIGINT migration, OTel opt-in noop, troubleshooting doc, deploy scripts bash -n, Caddyfile valid, gosec scan 0 HIGH, tag v1.0.0 presente). Cada FALHA encontrada vira bd issue + fix TDD red-green obrigatório (teste falhando primeiro depois impl mínima depois refactor) + 1 commit atômico + bd close. Estado final: relatório QA agent PASS em todos cenários, sem bd issues qa-sweep abertas, suite testes cresceu de baseline (≥210 unit ≥265 integration), go vet zero, gosec HIGH zero, containers todos up, endpoints HTTP respondem códigos esperados, cleanup completo (tenants qa-sweep-* removidos do DB, settings de teste resetadas, base_url produção mantida). Provar com: `go test ./... -count=1 2>&1 | tail -3` mostrando ok ≥210 passed; `go test -tags=integration -count=1 ./... 2>&1 | tail -3` mostrando ok ≥265 passed; `go vet ./... 2>&1` sem output; `bd list --status=open 2>&1 | grep qa-sweep` vazio; `git log --oneline -15` com commits test(qa)/fix(qa)/chore(qa); `docker compose ps --format "{{.Service}}:{{.Status}}"` mostrando todos Up; `curl -sS -o /dev/null -w "%{http_code}\n" http://localhost:8090/healthz` retorna 200; `curl -sS -o /dev/null -w "%{http_code}\n" http://localhost:8090/login` retorna 200; `curl -sS -o /dev/null -w "%{http_code}\n" http://localhost:8090/metrics` retorna 200; `docker compose exec -T db psql -U bridge -d bridge -tA -c "SELECT slug FROM tenants;"` listando somente e2e-teste e loadtest. Restrições: TDD obrigatório por fix (teste RED primeiro reproduzindo bug + impl GREEN minimal + refactor); sempre rebuild container antes de live curl (docker compose up -d --build bridge) pois stale container causa falsos positivos lição BUG-001; funções ≤20 linhas ≤2 params (ctx exceto); sem interfaces especulativas mantém flat 1 pkg internal/bridge; não modificar testes existentes exceto refletindo mudança contrato; cada bug bd create --parent chatwoot-megaapi-bridge-efn.8 e bd close ao fixar; 1 commit por fix atômico; mensagens UI PT-BR sem vazar implementação lição BUG-002; cleanup final tenants qa-sweep-* removidos settings teste resetadas base_url produção preservada; sem --no-verify; sem //nolint; sem modificar go.sum exceto via go get; sem segredos; sem force-push; mensagens commit descritivas test(qa)/fix(qa)/chore(qa), or stop after 60 turns or 180m. Report turn count, cenários PASS/FAIL, bugs abertos, testes passed, remaining bound each turn. Claude must echo full output of each verification command.
```

## 9. Comando headless (opcional)

```
claude -p "/goal QA full sweep ..."
```

## 10. Checklist pré-entrega

- [x] ≤4000 chars (condição ~3990)
- [x] Comandos concretos (go test, go vet, bd list, git log, docker compose ps, curl, psql)
- [x] Output literal (ok, ≥210, ≥265, 200, vazio)
- [x] Restrições específicas (TDD obrigatório, rebuild container, ≤20 linhas, etc)
- [x] Restrições padrão (no-verify, lockfiles, segredos, force-push)
- [x] Bound (60 turns OR 180m)
- [x] Echo obrigatório
- [x] Arquivo salvo em `docs/goals/2026-05-24-harden-qa-sweep-tdd.md`
- [x] Slug `harden-<area>`
- [x] Estado final falsificável
- [x] Code-craftsman regras embutidas

## 11. Matriz de cenários (15+ áreas)

| # | Categoria | Cenários |
|---|---|---|
| 1 | Auth | login válido/inválido, logout, sessão expirada |
| 2 | Dashboard | listagem tenants, count 24h, sparkline |
| 3 | Wizard | 4 passos completos, slug duplicado, campos missing |
| 4 | Discovery | auto-discovery Chatwoot inboxes válido/inválido |
| 5 | Diagnóstico | per-tenant flags megaapi/chatwoot/webhook/db |
| 6 | Mensagens | paginação, filtro tenant, ordenação |
| 7 | DLQ | listar failed, retry individual, retry massa |
| 8 | Settings | persistência base_url, validação URL |
| 9 | pprof | gated logado 200, deslogado 401/redirect |
| 10 | Endpoints API | healthz, readyz, metrics |
| 11 | Webhook auth | /v1/wa Bearer ok/fail, fromMe filter, bad slug 404 |
| 12 | Webhook CW | /v1/cw HMAC ok/fail, non-relay ignored |
| 13 | Format guards | HEIC/GIF/WAV/WEBM rejections |
| 14 | Reliability | retry jitter, janitor stale, backpressure 80% |
| 15 | Observability | metrics series, OTel noop, troubleshooting doc |
| 16 | Deploy | bash -n scripts, Caddyfile valid |
| 17 | Security | gosec HIGH=0, tag v1.0.0 |

## 12. Lições aplicáveis

- **BUG-001 (container stale)**: SEMPRE rebuild antes de curl live. Embutido em restrições.
- **BUG-002 (SQL leak)**: erros UI PT-BR sem vazar pg-error. Pattern: `isUniqueViolation(err)` + friendly text. Embutido em restrições.
- **Code-craftsman**: TDD red-green obrigatório por fix. Funções ≤20 linhas. Flat arch.

## 13. Out-of-scope

- Load test 24h real (continua trigger manual)
- Push v1.0.0 tag (já pushed)
- Recrutamento pilotos externos (deferido)
