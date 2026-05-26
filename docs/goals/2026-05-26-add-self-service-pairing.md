# Goal Plan — add-self-service-pairing

## 1. Contexto

- Repo: `C:\Users\GEOVANE\Desktop\Projetos\chatwoot-megaapi-bridge`
- Stack: Go 1.23+, chi router, PostgreSQL, html/template, AES-256-GCM + HMAC-SHA256
- Estado atual: bridge multi-tenant validado v1.0.1. Wizard admin cria tenants, mas pareamento WhatsApp (QR/pairing code) é feito manualmente no dashboard megaAPI antes do wizard. Cliente final não tem fluxo self-service.
- Comando teste: `make test` (= `go test ./...`)
- Comando integration: `make integration` (= `go test -tags=integration ./...`, Docker required)
- Comando lint: `make lint` (= `go vet ./... && golangci-lint run`)
- Comando build: `make build` (= `CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o bridge ./cmd/bridge`)

## 2. Estado final mensurável

Feature self-service pairing implementada com TDD estrito (RED-GREEN-REFACTOR). Critérios:

1. Migration `migrations/0004_pairing.sql` adiciona colunas `paired_at TIMESTAMPTZ NULL`, `last_jid TEXT NULL` na tabela `tenants`
2. Pacote `internal/bridge/megaapi_pair.go` expõe 5 funções: `FetchQR`, `FetchPairingCode`, `FetchInstanceStatus`, `LogoutInstance`, `RestartInstance` — cada uma com teste unitário usando `httptest.NewServer`
3. Pacote `internal/bridge/web/pairing.go` expõe 5 handlers públicos: `GET /pair/{slug}`, `GET /pair/{slug}/qr`, `POST /pair/{slug}/code`, `POST /pair/{slug}/logout`, `GET /pair/{slug}/status` — todos validam HMAC token (`HMAC-SHA256(slug|exp, BRIDGE_ENCRYPTION_KEY)`)
4. Handler `/v1/wa/{slug}` trata `messageType=connection_update` com `message=phone_connected` gravando `paired_at=now()` e `last_jid` no tenant
5. Admin UI ganha botão "Gerar link de pareamento" em `/` (lista tenants) que mostra URL `https://{base_url}/pair/{slug}?t=<hmac>&exp=<unix>` (default exp 24h)
6. Template `internal/bridge/web/templates/pair.html` renderiza tabs QR + Pairing Code, polling 3s, status badge
7. `make test` exits 0 com ≥15 testes novos passando (5 megaapi_pair + 5 pairing handler + 2 hmac sign/verify + 2 connection_update webhook + 1 admin link generation)
8. `make lint` exits 0 sem violações
9. `make build` exits 0 produzindo `./bridge` ou `./bridge.exe`
10. Cobertura dos arquivos novos ≥80%

## 3. Prova surfaceável

- Comandos a cada turno:
  - `make test 2>&1 | tail -20`
  - `make lint 2>&1 | tail -5`
  - `go test -cover ./internal/bridge/... 2>&1 | tail -10`
- Comandos no turno final:
  - `make build 2>&1 | tail -3`
  - `ls migrations/0004_pairing.sql`
  - `ls internal/bridge/megaapi_pair.go internal/bridge/megaapi_pair_test.go`
  - `ls internal/bridge/web/pairing.go internal/bridge/web/pairing_test.go`
  - `ls internal/bridge/web/templates/pair.html`
  - `grep -c "connection_update" internal/bridge/bridge.go internal/bridge/bridge_test.go`
- Output literal esperado:
  - `ok` ou `PASS` em todos pacotes
  - `--- FAIL` ausente em qualquer turno após GREEN
  - Coverage line tipo `coverage: 8X.X% of statements` ≥ 80.0
  - Build sem stderr
  - Cada `ls` retorna o path (não "No such file")
- Echo obrigatório: sim

## 4. Restrições

### Específicas do projeto

- NÃO modificar migrations existentes (`0001`, `0002`, `0003`) — só adicionar `0004`
- NÃO criar pacotes novos fora de `internal/bridge/` e `internal/bridge/web/` (regra flat-first MVP)
- NÃO criar interfaces `Repository`/`Service`/`Manager` (regra reset-MVP)
- NÃO usar mais de 2 parâmetros não-ctx em funções novas
- NÃO exceder 20 linhas por função; 500 linhas por arquivo
- NÃO armazenar token megaAPI ou Chatwoot em response do browser — todo proxy server-side
- NÃO criar rota pública sem validação HMAC obrigatória (401 se token ausente/inválido/expirado)
- NÃO tocar em `cmd/bridge/`, `crypto.go`, `storage.go` exceto adicionar campos `PairedAt` + `LastJID` no struct Tenant e SELECT/INSERT correspondentes
- NÃO adicionar dependências externas além do `go.mod` atual (usar `crypto/hmac`, `crypto/sha256`, `encoding/base64`, `encoding/hex` da stdlib)
- NÃO commitar sem rodar `make lint test` localmente

### TDD obrigatório (code-craftsman)

- RED primeiro: cada função/handler novo começa com teste falhando commitado
- GREEN: implementação mínima pra passar
- REFACTOR: limpar mantendo testes verdes
- Sequência por feature: write test → run test (FAIL) → implement → run test (PASS) → commit
- Mocks externos via `httptest.NewServer` para megaAPI, não interface mock genérica
- 1 assert por teste (preferencial), tabela-driven OK para casos paralelos
- Coverage ratchet: nunca diminuir
- Nada de `// TODO: add test later`

### Padrão (sempre)

- NÃO usar `--no-verify` em commits
- NÃO desabilitar lint (`//nolint` proibido sem ADR)
- NÃO modificar `go.mod`/`go.sum` sem comando explícito (stdlib only para este goal)
- NÃO commitar segredos
- NÃO force-push
- Mensagens de commit descritivas (`feat(pair): ...`, `test(pair): ...`, `fix(pair): ...`), nunca `wip`/`fix`/`update`
- NÃO suprimir warnings com flags

## 5. Bound

- **40 turnos** — justificativa: 5 funções megaAPI + 5 handlers + 2 hmac + 2 webhook + 1 admin link + template + migration + integração admin UI. Cada par RED-GREEN consome ~2 turnos. Buffer pra REFACTOR e debugging de coverage.

## 6. Modo de execução recomendado

- Auto mode: ligar antes do `/goal` (`/auto on` ou flag CLI)
- Headless opcional: `claude -p "/goal <condição>"` se rodar via CI/cron

## 7. Condição final (cole no /goal)

```
Feature self-service pairing implementada via TDD estrito. Estado final: (1) migration migrations/0004_pairing.sql adiciona paired_at TIMESTAMPTZ NULL e last_jid TEXT NULL em tenants; (2) internal/bridge/megaapi_pair.go expõe FetchQR, FetchPairingCode, FetchInstanceStatus, LogoutInstance, RestartInstance com teste httptest cada; (3) internal/bridge/web/pairing.go expõe handlers GET /pair/{slug}, GET /pair/{slug}/qr, POST /pair/{slug}/code, POST /pair/{slug}/logout, GET /pair/{slug}/status, todos validando HMAC-SHA256(slug|exp, BRIDGE_ENCRYPTION_KEY); (4) handler /v1/wa/{slug} processa messageType=connection_update message=phone_connected gravando paired_at=now() e last_jid; (5) admin UI lista tenants com botão Gerar link de pareamento (URL https://{base_url}/pair/{slug}?t=<hmac>&exp=<unix>, default 24h); (6) template internal/bridge/web/templates/pair.html com tabs QR + Pairing Code + polling 3s; (7) cobertura ≥80% arquivos novos. Provar com: `make test 2>&1 | tail -20` mostrando `ok` em todos pacotes e zero `--- FAIL`; `make lint 2>&1 | tail -5` exits 0 sem violação; `go test -cover ./internal/bridge/... 2>&1 | tail -10` mostrando `coverage: 8X.X%` ≥80.0 nos arquivos novos; turno final `make build 2>&1 | tail -3` sem stderr; `ls migrations/0004_pairing.sql internal/bridge/megaapi_pair.go internal/bridge/megaapi_pair_test.go internal/bridge/web/pairing.go internal/bridge/web/pairing_test.go internal/bridge/web/templates/pair.html` retornando todos paths. TDD obrigatório: para cada função/handler novo escrever teste falhando primeiro (commitar RED), implementar mínimo (commitar GREEN), refatorar (commitar REFACTOR). Sem modificar migrations existentes (0001-0003), sem criar pacotes fora de internal/bridge/ e internal/bridge/web/, sem criar interfaces Repository/Service/Manager, sem funções com mais de 2 params não-ctx, sem funções >20 linhas, sem arquivos >500 linhas, sem armazenar tokens megaAPI/Chatwoot em response do browser, sem rota pública sem HMAC obrigatório (401 se ausente/inválido/expirado), sem adicionar deps externas (stdlib only: crypto/hmac, crypto/sha256, encoding/base64, encoding/hex), sem //nolint, sem // TODO add test later, sem --no-verify, sem modificar go.mod/go.sum, sem force-push, sem mensagens de commit vagas (usar feat(pair)/test(pair)/fix(pair)), sem commitar segredos, or stop after 40 turns. Report turn count, testes passando, coverage atual dos arquivos novos e remaining bound each turn. Claude must echo full output of each verification command.
```

## 8. Comando completo

```
/goal Feature self-service pairing implementada via TDD estrito. Estado final: (1) migration migrations/0004_pairing.sql adiciona paired_at TIMESTAMPTZ NULL e last_jid TEXT NULL em tenants; (2) internal/bridge/megaapi_pair.go expõe FetchQR, FetchPairingCode, FetchInstanceStatus, LogoutInstance, RestartInstance com teste httptest cada; (3) internal/bridge/web/pairing.go expõe handlers GET /pair/{slug}, GET /pair/{slug}/qr, POST /pair/{slug}/code, POST /pair/{slug}/logout, GET /pair/{slug}/status, todos validando HMAC-SHA256(slug|exp, BRIDGE_ENCRYPTION_KEY); (4) handler /v1/wa/{slug} processa messageType=connection_update message=phone_connected gravando paired_at=now() e last_jid; (5) admin UI lista tenants com botão Gerar link de pareamento (URL https://{base_url}/pair/{slug}?t=<hmac>&exp=<unix>, default 24h); (6) template internal/bridge/web/templates/pair.html com tabs QR + Pairing Code + polling 3s; (7) cobertura ≥80% arquivos novos. Provar com: `make test 2>&1 | tail -20` mostrando `ok` em todos pacotes e zero `--- FAIL`; `make lint 2>&1 | tail -5` exits 0 sem violação; `go test -cover ./internal/bridge/... 2>&1 | tail -10` mostrando `coverage: 8X.X%` ≥80.0 nos arquivos novos; turno final `make build 2>&1 | tail -3` sem stderr; `ls migrations/0004_pairing.sql internal/bridge/megaapi_pair.go internal/bridge/megaapi_pair_test.go internal/bridge/web/pairing.go internal/bridge/web/pairing_test.go internal/bridge/web/templates/pair.html` retornando todos paths. TDD obrigatório: para cada função/handler novo escrever teste falhando primeiro (commitar RED), implementar mínimo (commitar GREEN), refatorar (commitar REFACTOR). Sem modificar migrations existentes (0001-0003), sem criar pacotes fora de internal/bridge/ e internal/bridge/web/, sem criar interfaces Repository/Service/Manager, sem funções com mais de 2 params não-ctx, sem funções >20 linhas, sem arquivos >500 linhas, sem armazenar tokens megaAPI/Chatwoot em response do browser, sem rota pública sem HMAC obrigatório (401 se ausente/inválido/expirado), sem adicionar deps externas (stdlib only: crypto/hmac, crypto/sha256, encoding/base64, encoding/hex), sem //nolint, sem // TODO add test later, sem --no-verify, sem modificar go.mod/go.sum, sem force-push, sem mensagens de commit vagas (usar feat(pair)/test(pair)/fix(pair)), sem commitar segredos, or stop after 40 turns. Report turn count, testes passando, coverage atual dos arquivos novos e remaining bound each turn. Claude must echo full output of each verification command.
```

## 9. Comando headless (opcional)

```
claude -p "/goal <colar string da seção 7>"
```

## 10. Checklist pré-entrega

- [x] ≤4000 chars na condição final (medido: ~3700 chars)
- [x] Comandos concretos presentes (`make test`, `make lint`, `make build`, `go test -cover`, `ls`)
- [x] Output literal definido (`ok`, `--- FAIL` ausente, `coverage: 8X.X%`, paths)
- [x] Restrições específicas (10) + padrão (7) + TDD (8)
- [x] Bound presente (40 turns)
- [x] Echo obrigatório explícito
- [x] Slug segue convenção (`add-<feature>`)
- [x] Arquivo salvo em `docs/goals/`
- [x] Estado final falsificável (exit codes + grep + ls)
- [x] Comandos verificados no Makefile real
- [x] TDD red-green-refactor exigido explicitamente
- [x] Sem dependências externas novas (stdlib only)

## 11. Pré-condições antes de colar `/goal`

1. **megaAPI endpoints confirmados** (já recebidos):
   - `GET /rest/instance/qrcode_base64/{instance}` — QR base64
   - `GET /rest/instance/pairingCode/{instance}?phoneNumber=X&customPairingCode=Y` — pairing code
   - `GET /rest/instance/{instance}` — status (payload `messageType=connection_update`, `message=phone_connected`, `jid`, `pushName`)
   - `DELETE /rest/instance/{instance}/logout` — logout
   - `DELETE /rest/instance/{instance}/restart` — restart
2. **Webhook event shape confirmado**: `connection_update` chega no `/v1/wa/{slug}` no mesmo payload dos eventos de mensagem
3. **Ativar auto mode** na sessão alvo
4. **Branch limpa**: criar `feature/self-service-pairing` antes do `/goal`
5. **DB up**: rodar migration 0004 manualmente NÃO — deixar Claude criar o arquivo, validação real em integration test fica fora deste goal

## 12. Pós-goal (manual)

Após `/goal` completar:
- Rodar `make integration` (Docker) pra validar migration end-to-end
- Smoke manual: gerar link pelo admin UI, abrir incognito, conferir QR renderiza
- Atualizar `CHANGELOG.md` + `RELEASE_NOTES_v1.1.0.md`
- Atualizar `CLAUDE.md` lessons learned se algo novo surgir
- E2E decision ADR via `/code-craftsman:e2e-decide self-service-pairing`
