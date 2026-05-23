# Goal Plan — complete-f2-reliability

## 1. Contexto

- Repo: `C:\Users\GEOVANE\Desktop\Projetos\chatwoot-megaapi-bridge`
- Stack: Go 1.25, Postgres (pgx), zerolog, chi, testcontainers
- Estado atual:
  - 2/8 tarefas F2 concluídas (BIGINT migration, stale janitor)
  - 6 restantes: retry jitter, backpressure tune, /metrics Prometheus, DLQ endpoints, integration tests, runCreateTenant CLI tests
  - 102 unit tests + 128 integration tests passando
- Comando teste unit: `go test ./...`
- Comando teste integração: `go test -tags=integration ./...`
- Comando lint: `go vet ./...` (golangci-lint opcional)
- Comando build: `go build ./...`

## 2. Estado final mensurável

Todas 6 tarefas F2 restantes implementadas com TDD red-green-refactor, fechadas no bd, commits pushed:

1. **8s8.3 retry jitter**: `retryBackoff` recebe jitter ±25% aleatório por retry; teste novo `TestRunRetryLoop_AppliesJitter` passa
2. **8s8.5 backpressure tune**: `/readyz` retorna 503 quando inbox OU outbox ≥ 80% (threshold configurável); teste `TestHandleReady_ReturnsServiceUnavailableAt80PercentFull` passa
3. **8s8.6 métricas Prometheus**: endpoint `GET /metrics` exposto; 5 metrics — `bridge_messages_in_total`, `bridge_messages_out_total`, `bridge_messages_failed_total`, `bridge_job_duration_seconds` (histogram), `bridge_queue_depth` (gauge); curl `localhost:8090/metrics` retorna 200 + linhas `bridge_*`
4. **8s8.4 DLQ endpoints**: `GET /admin/failed?limit=N` lista msgs status=failed; `POST /admin/retry/{id}` muda failed → pending; auth via Bearer (mesmo token tenant); 2 testes integração novos
5. **8s8.8 integration tests**: cobertura testcontainers expandida — retry classification (retriable vs notRetriable), recovery após restart, dedup ON CONFLICT; mínimo 5 novos test cases
6. **8s8.13 runCreateTenant CLI tests**: `cmd/bridge/main_test.go` cobre `runCreateTenant` com sucesso + erros (slug duplicado, host inválido, master key ausente); mínimo 4 novos test cases

## 3. Prova surfaceável

Comandos rodados a cada turno (echo full output):

```bash
go test ./... 2>&1 | tail -3
go test -tags=integration ./... 2>&1 | tail -3
go vet ./... 2>&1 | tail -3
bd list --status=open --label=phase-2 2>&1 | tail -5
git log --oneline -10
```

Output literal esperado no transcript ao concluir:

- `go test ./...`: `ok` em todos pacotes, ≥110 passed
- `go test -tags=integration`: `ok` em todos pacotes, ≥135 passed
- `go vet ./...`: sem output (zero issues)
- `bd list --status=open --label=phase-2`: somente epic `chatwoot-megaapi-bridge-8s8` ainda open (epic fecha por último)
- `git log --oneline -10`: 6 novos commits com prefixos `feat(reliability):`, `feat(observability):`, `feat(admin):`, `test(integration):`, `test(cli):`

Frequência: a cada turno após implementação de cada tarefa.

## 4. Restrições

### Específicas do projeto

- NÃO modificar `Tenant` ou `Message` struct shapes (compatibilidade dump-restore Postgres)
- NÃO adicionar deps fora de stdlib + pacotes já em `go.mod` exceto `github.com/prometheus/client_golang` (necessário para 8s8.6)
- NÃO mexer em `migrations/0001_init.sql` ou `0002_chatwoot_ids_bigint.sql` (criar `0003_*.sql` se schema change necessário)
- NÃO criar interfaces especulativas — manter flat (1 pkg `internal/bridge`)
- Functions ≤20 linhas, ≤2 parâmetros (`ctx` exceto)
- Endpoints admin devem usar mesma autenticação Bearer já existente
- Workflow TDD obrigatório: teste falhando primeiro, commit do teste primeiro se preferir
- Cada tarefa concluída → `bd close <id> --reason "..."` antes de prosseguir

### Padrão

- NÃO usar `--no-verify` em commits
- NÃO desabilitar lint (sem `//nolint`)
- NÃO modificar `go.sum` exceto via `go get`
- NÃO commitar segredos
- NÃO force-push
- Mensagens commit descritivas (`feat(scope): ...`, `test(scope): ...`)

## 5. Bound

- **60 turnos** OU **180 minutos** (o que vier primeiro)
- Justificativa: 6 tarefas × ~10 turnos cada (TDD red-green + commit + close bd). Tempo cobre rebuilds Docker + testcontainers downloads.

## 6. Modo de execução recomendado

- Auto mode: ligar antes (`Esc Esc` para acceptEdits)
- Headless opcional: `claude -p "/goal <condição>"` se quiser CI/cron

## 7. Condição final (cole no /goal)

```
F2 reliability completa: 6 tarefas restantes do épico chatwoot-megaapi-bridge-8s8 implementadas com TDD red-green. Estado final: (1) retry jitter ±25% em retryBackoff com teste novo passando; (2) /readyz retorna 503 ao atingir ≥80% de inbox ou outbox com teste novo passando; (3) endpoint GET /metrics expõe bridge_messages_in_total, bridge_messages_out_total, bridge_messages_failed_total, bridge_job_duration_seconds (histogram), bridge_queue_depth (gauge) — curl localhost:8090/metrics deve retornar 200 com linhas bridge_*; (4) endpoints admin GET /admin/failed?limit=N e POST /admin/retry/{id} autenticados via Bearer existente, 2+ testes integração novos; (5) integration tests testcontainers expandidos com 5+ casos novos cobrindo retry classification, recovery após restart, dedup ON CONFLICT; (6) runCreateTenant CLI testado com 4+ casos cobrindo sucesso e erros (slug duplicado, host inválido, master key ausente). Provar com: `go test ./... 2>&1 | tail -3` mostrando ok em todos pacotes com ≥110 testes passed; `go test -tags=integration ./... 2>&1 | tail -3` mostrando ok com ≥135 testes passed; `go vet ./... 2>&1` sem output (zero issues); `bd list --status=open --label=phase-2 2>&1 | tail -5` mostrando somente epic chatwoot-megaapi-bridge-8s8 ainda open (subtasks fechadas: .3 .4 .5 .6 .8 .13); `git log --oneline -10` mostrando 6 novos commits com prefixos feat(reliability)/feat(observability)/feat(admin)/test(integration)/test(cli). Restrições: não modificar Tenant ou Message struct shapes, deps novas só prometheus/client_golang, não mexer em migrations 0001/0002 (criar 0003 se necessário), manter flat (1 pkg internal/bridge sem interfaces especulativas), funções ≤20 linhas e ≤2 params (ctx exceto), endpoints admin reusam Bearer auth existente, TDD teste-primeiro obrigatório, cada tarefa concluída deve rodar `bd close <id>` antes da próxima, sem --no-verify, sem //nolint, sem modificar go.sum exceto via go get, sem segredos, sem force-push, mensagens commit descritivas (feat(scope):/test(scope):), or stop after 60 turns or 180m. Report turn count, qual tarefa atual (1-6), testes passed/integration passed e remaining bound each turn. Claude must echo full output of each verification command.
```

## 8. Comando completo

```
/goal F2 reliability completa: 6 tarefas restantes do épico chatwoot-megaapi-bridge-8s8 implementadas com TDD red-green. Estado final: (1) retry jitter ±25% em retryBackoff com teste novo passando; (2) /readyz retorna 503 ao atingir ≥80% de inbox ou outbox com teste novo passando; (3) endpoint GET /metrics expõe bridge_messages_in_total, bridge_messages_out_total, bridge_messages_failed_total, bridge_job_duration_seconds (histogram), bridge_queue_depth (gauge) — curl localhost:8090/metrics deve retornar 200 com linhas bridge_*; (4) endpoints admin GET /admin/failed?limit=N e POST /admin/retry/{id} autenticados via Bearer existente, 2+ testes integração novos; (5) integration tests testcontainers expandidos com 5+ casos novos cobrindo retry classification, recovery após restart, dedup ON CONFLICT; (6) runCreateTenant CLI testado com 4+ casos cobrindo sucesso e erros (slug duplicado, host inválido, master key ausente). Provar com: `go test ./... 2>&1 | tail -3` mostrando ok em todos pacotes com ≥110 testes passed; `go test -tags=integration ./... 2>&1 | tail -3` mostrando ok com ≥135 testes passed; `go vet ./... 2>&1` sem output (zero issues); `bd list --status=open --label=phase-2 2>&1 | tail -5` mostrando somente epic chatwoot-megaapi-bridge-8s8 ainda open (subtasks fechadas: .3 .4 .5 .6 .8 .13); `git log --oneline -10` mostrando 6 novos commits com prefixos feat(reliability)/feat(observability)/feat(admin)/test(integration)/test(cli). Restrições: não modificar Tenant ou Message struct shapes, deps novas só prometheus/client_golang, não mexer em migrations 0001/0002 (criar 0003 se necessário), manter flat (1 pkg internal/bridge sem interfaces especulativas), funções ≤20 linhas e ≤2 params (ctx exceto), endpoints admin reusam Bearer auth existente, TDD teste-primeiro obrigatório, cada tarefa concluída deve rodar `bd close <id>` antes da próxima, sem --no-verify, sem //nolint, sem modificar go.sum exceto via go get, sem segredos, sem force-push, mensagens commit descritivas (feat(scope):/test(scope):), or stop after 60 turns or 180m. Report turn count, qual tarefa atual (1-6), testes passed/integration passed e remaining bound each turn. Claude must echo full output of each verification command.
```

## 9. Comando headless (opcional)

```
claude -p "/goal F2 reliability completa: ... (mesma condição) ..."
```

## 10. Checklist pré-entrega

- [x] ≤4000 chars (condição ~3500 chars)
- [x] Comandos concretos presentes (go test, go vet, bd list, git log, curl)
- [x] Output literal definido (ok, ≥110 passed, sem output vet, etc.)
- [x] Restrições específicas + padrão
- [x] Bound presente (60 turns OR 180m)
- [x] Echo obrigatório (`Claude must echo full output`)
- [x] Arquivo salvo em `docs/goals/2026-05-23-complete-f2-reliability.md`
- [x] Slug segue convenção (`complete-<area>`)
- [x] Estado final falsificável
- [x] Comandos referenciados existem no projeto (Makefile + bd)

## 11. Ordem sugerida das 6 tarefas

| # | bd | Tarefa | Razão ordem |
|---|---|---|---|
| 1 | 8s8.3 | Retry jitter ±25% | Mínimo invasivo, modifica `runRetryLoop` |
| 2 | 8s8.5 | Backpressure tune /readyz 503 ≥80% | Pequeno tune em handler existente |
| 3 | 8s8.6 | Métricas Prometheus + /metrics | Adiciona dep, isolado |
| 4 | 8s8.4 | DLQ endpoints /admin/failed + retry | Depende de Bearer auth pattern existente |
| 5 | 8s8.8 | Integration tests testcontainers expand | Wrapper TDD para itens 1-4 |
| 6 | 8s8.13 | runCreateTenant CLI tests | Débito de TDD pré-existente |

## 12. Lembretes operacionais

- Bridge container precisa rebuild após cada commit: `docker compose up -d --build bridge`
- Migrations novas devem ser aplicadas: `docker compose exec -T bridge //bridge migrate`
- testcontainers requer Docker rodando
- gh auth pode precisar switch para `giovani-junior-dev` antes do push
