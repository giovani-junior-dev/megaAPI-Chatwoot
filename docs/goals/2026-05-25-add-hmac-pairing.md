# Goal Plan — add-hmac-pairing

## 1. Contexto

- Repo: `C:\Users\GEOVANE\Desktop\Projetos\chatwoot-megaapi-bridge`
- Stack: Go 1.25, chi v5, pgx v5. Master key 32 bytes base64 em env `MASTER_KEY`. AES-256-GCM em `crypto.go`.
- Estado atual:
  - F1-F6 done + v1.0.0 released.
  - 223 unit + 289 integration tests passando.
  - Wizard auto-config webhook_url megaAPI (configWebhook) e Chatwoot (PATCH inbox channel.webhook_url) — commit b204e66.
  - HMAC ainda manual: bridge gera secret próprio, Chatwoot gera próprio `hmac_token`. Webhook signature verification quebra se não pareados.
- Bd issue: `chatwoot-megaapi-bridge-3sf` (P2, feature open).
- Comando teste unit: `go test ./...`
- Comando teste integração: `go test -tags=integration ./...`
- Comando lint: `go vet ./...`

## 2. Estado final mensurável

Wizard `POST /tenants` pareia HMAC entre bridge e Chatwoot automaticamente. Após criar tenant:
1. PATCH inbox seta `webhook_url` + `hmac_mandatory: true`
2. GET inbox lê `hmac_token` gerado pelo Chatwoot
3. UPDATE `tenants.hmac_secret_enc = Encrypt(hmac_token, master_key)` no tenant criado
4. Bridge passa a verificar webhooks CW com `hmac_token` real do Chatwoot (zero paste manual)

| bd 3sf entregável | Verificável |
|---|---|
| `FetchChatwootInboxHMAC` func | TestFetchChatwootInboxHMAC_ReturnsToken passa |
| `DB.UpdateTenantHMAC` método | TestUpdateTenantHMAC_RoundTrip passa (testcontainers) |
| Wizard chama pareamento após PATCH | TestWizardPOSTPairsHMACFromChatwoot passa |
| `ConfigureChatwootWebhook` envia `hmac_mandatory: true` | TestConfigureChatwootWebhook_EnablesHMAC passa |
| Suite full pass | 223+4=227 unit, 289+1=290 integration |
| `go vet` zero | sem output |

## 3. Prova surfaceável

Comandos a cada turno (echo full output):

```bash
go test ./... -count=1 2>&1 | tail -3
go test -tags=integration -count=1 ./... 2>&1 | tail -3
go vet ./... 2>&1
bd show chatwoot-megaapi-bridge-3sf 2>&1 | grep -E "Status:|Type:"
git log --oneline -5
```

Output esperado ao concluir:

- `go test ./...`: `ok`, ≥227 passed
- `go test -tags=integration`: `ok`, ≥290 passed
- `go vet`: zero output
- `bd show 3sf`: `Status: closed`
- `git log`: novo commit `feat(wizard): pair HMAC from Chatwoot inbox after tenant create`

Frequência: a cada turno após cada sub-impl.

## 4. Restrições

### Específicas do projeto

- TDD obrigatório: teste RED primeiro reproduzindo gap (verificação webhook falha sem pareamento) → impl GREEN minimal → refactor
- `Encrypt`/`Decrypt` em `internal/bridge/crypto.go` deve ser reusado, NÃO duplicar AES-GCM
- Schema tenants imutável; só atualizar coluna existente `hmac_secret_enc`. NÃO criar migration nova
- Funções ≤20 linhas, ≤2 parâmetros (ctx exceto). Code-craftsman bind
- Sem interfaces especulativas — manter flat 1 pkg `internal/bridge` + `internal/bridge/web`
- Não modificar testes existentes (só adicionar). Existing megaAPI auto-config test deve continuar verde
- `FetchChatwootInboxHMAC` em `internal/bridge/web/chatwoot_webhook.go` (mesmo arquivo de `ConfigureChatwootWebhook`)
- `DB.UpdateTenantHMAC(ctx, id, encryptedSecret)` em `internal/bridge/storage.go` ao lado de `UpsertTenant`/`InsertTenant`
- Deps struct ganha `UpdateTenantHMAC func(...) error` opcional (nil-safe igual outros)
- Wizard handler: após `fireChatwootConfig`, dispara `pairChatwootHMAC` (lê hmac_token + persist)
- Pareamento failure NÃO bloqueia criação tenant (idempotência: tenta novamente em re-submit ou via diagnóstico). Loga warning
- Se Chatwoot API não retornar `hmac_token` (versão antiga), bridge mantém secret próprio + warning log. NÃO crash
- Container rebuild sempre antes de live test (lição BUG-001): `docker compose up -d --build bridge`

### Padrão

- NÃO `--no-verify`
- NÃO `//nolint`
- NÃO modificar `go.sum` exceto via `go get`
- NÃO commitar segredos
- NÃO force-push
- Mensagem commit descritiva: `feat(wizard): pair HMAC ...`

## 5. Bound

- **20 turnos** OU **60 minutos**
- Justificativa: 1 sub-feature pequena (~30 linhas Go), 4 testes TDD, 1 commit. F.I.R.S.T. <5 turns por test cycle.

## 6. Modo de execução recomendado

- Auto mode: ligar (`Shift+Tab` → "acceptEdits")
- Headless: não recomendado (task curta)

## 7. Condição final (cole no /goal)

```
HMAC pairing implementado: wizard POST /tenants após criar tenant chama PATCH Chatwoot inbox com webhook_url+hmac_mandatory true, GET inbox lê hmac_token gerado pelo Chatwoot, UPDATE tenants.hmac_secret_enc = Encrypt(hmac_token, master_key). Bd issue chatwoot-megaapi-bridge-3sf fechada. TDD red-green obrigatório por método novo. Estado final: (1) FetchChatwootInboxHMAC func em internal/bridge/web/chatwoot_webhook.go retorna string hmac_token via GET /api/v1/accounts/{acc}/inboxes/{id} parsing response.hmac_token (test TestFetchChatwootInboxHMAC_ReturnsToken passa); (2) DB.UpdateTenantHMAC(ctx, id, enc) em internal/bridge/storage.go executa UPDATE tenants SET hmac_secret_enc=$1 WHERE id=$2 (test integração TestUpdateTenantHMAC_RoundTrip passa); (3) ConfigureChatwootWebhook agora envia channel.webhook_url + hmac_mandatory=true no body (test TestConfigureChatwootWebhook_EnablesHMAC passa); (4) wizard.go fireChatwootConfig após PATCH chama pairChatwootHMAC que faz GET inbox + Encrypt + UpdateTenantHMAC (test TestWizardPOSTPairsHMACFromChatwoot passa); (5) failure no pareamento NÃO bloqueia criação tenant (warning log, bridge mantém secret próprio até retry); (6) crypto Encrypt/Decrypt reusados de internal/bridge/crypto.go sem duplicação. Provar com: `go test ./... -count=1 2>&1 | tail -3` mostrando ok ≥227 passed; `go test -tags=integration -count=1 ./... 2>&1 | tail -3` mostrando ok ≥290 passed; `go vet ./... 2>&1` zero output; `bd show chatwoot-megaapi-bridge-3sf 2>&1 | grep "Status:"` mostrando Status: closed; `git log --oneline -5` com novo commit feat(wizard): pair HMAC. Restrições: TDD obrigatório red-green (teste falhando primeiro depois impl minimal); reusar Encrypt/Decrypt de crypto.go sem duplicação AES-GCM; schema tenants imutável (NÃO criar migration nova, só UPDATE hmac_secret_enc); funções ≤20 linhas ≤2 params (ctx exceto); sem interfaces especulativas mantendo flat internal/bridge + internal/bridge/web; não modificar testes existentes só adicionar; FetchChatwootInboxHMAC em chatwoot_webhook.go; UpdateTenantHMAC em storage.go; Deps struct nova func opcional nil-safe; pareamento failure NÃO bloqueia criação tenant só loga warning; se Chatwoot não retornar hmac_token bridge mantém secret próprio sem crash; container rebuild antes de live test (docker compose up -d --build bridge) lição BUG-001; sem --no-verify; sem //nolint; sem modificar go.sum exceto via go get; sem segredos; sem force-push; mensagem commit descritiva feat(wizard): pair HMAC, or stop after 20 turns or 60m. Report turn count, sub-impl atual (1-4), testes passed, bd status, remaining bound each turn. Claude must echo full output of each verification command.
```

## 8. Comando completo

```
/goal HMAC pairing implementado: wizard POST /tenants após criar tenant chama PATCH Chatwoot inbox com webhook_url+hmac_mandatory true, GET inbox lê hmac_token gerado pelo Chatwoot, UPDATE tenants.hmac_secret_enc = Encrypt(hmac_token, master_key). Bd issue chatwoot-megaapi-bridge-3sf fechada. TDD red-green obrigatório por método novo. Estado final: (1) FetchChatwootInboxHMAC func em internal/bridge/web/chatwoot_webhook.go retorna string hmac_token via GET /api/v1/accounts/{acc}/inboxes/{id} parsing response.hmac_token (test TestFetchChatwootInboxHMAC_ReturnsToken passa); (2) DB.UpdateTenantHMAC(ctx, id, enc) em internal/bridge/storage.go executa UPDATE tenants SET hmac_secret_enc=$1 WHERE id=$2 (test integração TestUpdateTenantHMAC_RoundTrip passa); (3) ConfigureChatwootWebhook agora envia channel.webhook_url + hmac_mandatory=true no body (test TestConfigureChatwootWebhook_EnablesHMAC passa); (4) wizard.go fireChatwootConfig após PATCH chama pairChatwootHMAC que faz GET inbox + Encrypt + UpdateTenantHMAC (test TestWizardPOSTPairsHMACFromChatwoot passa); (5) failure no pareamento NÃO bloqueia criação tenant (warning log, bridge mantém secret próprio até retry); (6) crypto Encrypt/Decrypt reusados de internal/bridge/crypto.go sem duplicação. Provar com: `go test ./... -count=1 2>&1 | tail -3` mostrando ok ≥227 passed; `go test -tags=integration -count=1 ./... 2>&1 | tail -3` mostrando ok ≥290 passed; `go vet ./... 2>&1` zero output; `bd show chatwoot-megaapi-bridge-3sf 2>&1 | grep "Status:"` mostrando Status: closed; `git log --oneline -5` com novo commit feat(wizard): pair HMAC. Restrições: TDD obrigatório red-green (teste falhando primeiro depois impl minimal); reusar Encrypt/Decrypt de crypto.go sem duplicação AES-GCM; schema tenants imutável (NÃO criar migration nova, só UPDATE hmac_secret_enc); funções ≤20 linhas ≤2 params (ctx exceto); sem interfaces especulativas mantendo flat internal/bridge + internal/bridge/web; não modificar testes existentes só adicionar; FetchChatwootInboxHMAC em chatwoot_webhook.go; UpdateTenantHMAC em storage.go; Deps struct nova func opcional nil-safe; pareamento failure NÃO bloqueia criação tenant só loga warning; se Chatwoot não retornar hmac_token bridge mantém secret próprio sem crash; container rebuild antes de live test (docker compose up -d --build bridge) lição BUG-001; sem --no-verify; sem //nolint; sem modificar go.sum exceto via go get; sem segredos; sem force-push; mensagem commit descritiva feat(wizard): pair HMAC, or stop after 20 turns or 60m. Report turn count, sub-impl atual (1-4), testes passed, bd status, remaining bound each turn. Claude must echo full output of each verification command.
```

## 9. Comando headless (opcional)

```
claude -p "/goal HMAC pairing ..."
```

## 10. Checklist pré-entrega

- [x] ≤4000 chars (~3550)
- [x] Comandos concretos (go test, go vet, bd show, git log)
- [x] Output literal (ok, ≥227, ≥290, "Status: closed")
- [x] Restrições específicas (TDD, reuso crypto, schema imutável, ≤20 linhas)
- [x] Restrições padrão (no-verify, lockfiles, segredos, force-push)
- [x] Bound (20 turns OR 60m)
- [x] Echo obrigatório
- [x] Arquivo salvo em `docs/goals/2026-05-25-add-hmac-pairing.md`
- [x] Slug `add-<feature>`
- [x] Estado final falsificável

## 11. Ordem sugerida (4 sub-impls)

| # | Sub-impl | TDD test name | Linhas~ |
|---|---|---|---|
| 1 | `ConfigureChatwootWebhook` envia `hmac_mandatory:true` | TestConfigureChatwootWebhook_EnablesHMAC | +3 |
| 2 | `FetchChatwootInboxHMAC` GET inbox → hmac_token | TestFetchChatwootInboxHMAC_ReturnsToken | +25 |
| 3 | `DB.UpdateTenantHMAC` UPDATE tenants | TestUpdateTenantHMAC_RoundTrip (integration) | +10 |
| 4 | Wizard `pairChatwootHMAC` orchestrator | TestWizardPOSTPairsHMACFromChatwoot | +20 |

Total ~60 linhas + tests.

## 12. Lembretes operacionais

- Container rebuild: `docker compose up -d --build bridge` ANTES de qualquer live curl
- Chatwoot inbox API GET endpoint: `GET /api/v1/accounts/{acc}/inboxes/{id}` retorna JSON com campo `hmac_token` (verificar via context7 docs Chatwoot se duvidar)
- master_key acessível via `s.Key` no Server; no wizard context via `h.deps.Key`
- Failure pairing: `_ = err` + `log.Warn().Err(err)` (best-effort, não crash)
- BD: integration test reusa `setupDB` + `applyAllMigrations`
- gh auth switch `giovani-junior-dev` antes do push

## 13. Out-of-scope

- Retry/backoff pareamento (1 tentativa só)
- UI para forçar re-pareamento manual (futura iteração)
- Migration backwards-compat (HMAC pareamento opt-in via setting)
