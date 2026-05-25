# Postmortem — QA Sweep & Wizard Hardening (2026-05-24 → 2026-05-25)

Janela: 2026-05-24 (kickoff self-pilot v1.0.0) até 2026-05-25 (cut v1.0.1).
Owner: @MadeInLowCode + Claude (paired).
Status: encerrado. Todos os bugs abaixo têm fix mergeado em `master`.

Este postmortem cobre o QA sweep pós-v1.0.0 + o pareamento HMAC entre
Chatwoot e a bridge. Cinco bugs foram encontrados, dois deles de severidade
alta (BUG-HMAC-01 quebrava 100% das mensagens Chatwoot → WA num tenant
recém-criado; BUG-WIZARD-01 deixava o tenant meio-onboarded em silêncio).

## Resumo

| ID | Sev | Título | Status | Fix commit |
|---|---|---|---|---|
| BUG-001 | Operacional | Stale container rodando binário antigo após `restart` | Documentado | CLAUDE.md (sem código) |
| BUG-002 | Alta | Slug duplicado vaza `pgconn.PgError` SQL puro | Fix | `9f67d6e` |
| BUG-QA-01 | Média | `settings.base_url` aceito sem validar scheme/host | Fix | `ccd56b6` |
| BUG-HMAC-01 | Alta | Wizard pareava o campo errado (`hmac_token` em vez de `channel.secret`) | Fix | `7fe688a` |
| BUG-WIZARD-01 | Alta | Wizard cria tenant silenciosamente quando `settings.base_url` está vazio | Fix | `b57832c` |

Suítes de testes pós-fix: **231 unit tests + 301 integration tests** (todos
verdes). `go vet` zero issues. `gosec` 0 HIGH (2 MEDIUM cookie-flag falsos
positivos já documentados em `docs/security/AUDIT-REPORT.md`).

## Timeline

- **2026-05-24 — Self-pilot v1.0.0 kickoff** (`33607a3`). Operador (Giovani)
  cria primeiro tenant real `e2e-teste` via wizard. Mensagem inbound funciona;
  outbound Chatwoot → WA retorna 401 na verificação HMAC.
- **2026-05-24 — BUG-001 descoberto.** Várias horas perdidas porque
  `docker compose restart bridge` sobe o **mesmo** binário antigo. Fix de
  processo: `docker compose up -d --build bridge` antes de cada live-test.
  Documentado em `CLAUDE.md` "Lessons Learned".
- **2026-05-24 — QA sweep iniciado** (goal plan
  `docs/goals/2026-05-24-harden-qa-sweep-tdd.md`). Cobertura de auth +
  admin + URL validation (`7cbb94d`).
- **2026-05-24 — BUG-002 (slug duplicate).** Operador tenta recriar tenant
  com mesmo slug; UI retorna 500 + `pgconn.PgError: duplicate key value
  violates unique constraint "tenants_slug_key" (SQLSTATE 23505)`. Fix
  (`9f67d6e`): captura `pgconn.PgError` com `Code == "23505"`, mapeia para
  409 + corpo PT-BR amigável.
- **2026-05-24 — BUG-QA-01 (`base_url` sem validação).** `POST /settings`
  aceitava qualquer string (até string vazia, `not a url`, host vazio). Fix
  (`ccd56b6`): valida `url.Parse` + `Scheme in ("http","https")` + `Host`
  não-vazio antes de gravar.
- **2026-05-25 — Auto-config Chatwoot inbox webhook** (`b204e66`). Wizard
  passa a chamar `PATCH /api/v1/accounts/{id}/inboxes/{id}` com
  `webhook_url=<base_url>/v1/cw/{slug}` automaticamente. Operador não cola
  URL na mão mais.
- **2026-05-25 — BUG-HMAC-01.** Primeira tentativa de pareamento usava
  o campo `hmac_token` retornado pelo Chatwoot inbox. Mensagens chegavam mas
  toda assinatura batia 401 na bridge. Investigação revelou: Chatwoot 3.x
  assina `api_inbox_webhook` events com `channel.secret` (campo aninhado),
  não com o `hmac_token` no topo da resposta. Fix (`7fe688a`): troca o campo
  lido; refaz encrypt + persist. Re-pareamento valida com smoke real
  `scripts/e2e/smoke-doc.ps1`.
- **2026-05-25 — BUG-WIZARD-01.** Edge case descoberto rodando wizard sem
  `settings.base_url` configurado. `InsertTenant` rodava, mas as chamadas
  `configWebhook` (megaAPI) e `PATCH inbox` (Chatwoot) abortavam sem
  feedback — tenant ficava meio-onboarded e a UI sinalizava sucesso. Fix
  (`b57832c`): wizard verifica `settings.base_url` **antes** do
  `InsertTenant` e retorna 400 PT-BR se vazio.
- **2026-05-25 — QA sweep closeout** (`225f908`). Goal plan fechado, bd
  state alinhado, `.gitignore` ajustado para artifacts Playwright.

## Bugs detalhados

### BUG-001 — Stale container após restart

- **Sintoma:** fix aparentemente aplicado no código mas comportamento na UI
  não muda. `docker compose restart bridge` sozinho não rebuild a imagem.
- **Root cause:** o serviço `bridge` no `docker-compose.yml` usa `build`
  local. `restart` reinicia o container existente mas mantém a imagem que
  já estava na cache.
- **Fix:** processo / documentação. Adicionado bloco "Lessons Learned" em
  `CLAUDE.md` exigindo `docker compose up -d --build bridge` antes de cada
  live-test cycle.
- **Prevenção futura:** considerar Makefile target `make live-test` que
  encadeia `docker compose build` + `up -d` + smoke contra `/healthz`.

### BUG-002 — Slug duplicate leak `pgconn.PgError`

- **Sintoma:** UI retorna 500 com payload contendo `duplicate key value
  violates unique constraint "tenants_slug_key" (SQLSTATE 23505)`.
- **Severidade:** Alta (vazamento de detalhe SQL para usuário leigo; UX
  ruim para operador retentando).
- **Root cause:** handler de tenant create não capturava `pgconn.PgError`;
  o erro voltava direto pro JSON encoder.
- **Fix (`9f67d6e`):**
  ```go
  var pgErr *pgconn.PgError
  if errors.As(err, &pgErr) && pgErr.Code == "23505" {
      writeJSONError(w, http.StatusConflict, "Já existe um tenant com esse slug.")
      return
  }
  ```
- **Test:** novo unit test cobre a path de 23505 → 409.

### BUG-QA-01 — `settings.base_url` sem validação

- **Sintoma:** `POST /settings` aceita `base_url=""`, `base_url="not a url"`,
  `base_url="https://"`. Resultado: webhooks gerados com URL malformada.
- **Severidade:** Média (operador pode shoot themselves in the foot).
- **Root cause:** falta de validação no handler.
- **Fix (`ccd56b6`):** valida `url.Parse` + `Scheme in ("http","https")` +
  `Host != ""`. Retorna 400 PT-BR com lista do problema.

### BUG-HMAC-01 — Wizard pareava `hmac_token` em vez de `channel.secret`

- **Sintoma:** todo `POST /v1/cw/{slug}` retornava 401 (HMAC verify fail)
  para tenants criados via wizard.
- **Severidade:** Alta — outbound Chatwoot → WhatsApp 100% quebrado para
  tenants novos.
- **Root cause:** documentação Chatwoot ambígua. O response de
  `GET /api/v1/accounts/{id}/inboxes/{id}` contém **dois** campos
  parecidos:
  - `hmac_token` (top-level, legado, hoje opcional/`nil` em inboxes API)
  - `channel.secret` (aninhado, é o que de fato assina
    `api_inbox_webhook` events em Chatwoot 3.x)
- **Fix (`7fe688a`):** wizard agora lê `channel.secret` (com fallback de
  erro claro se ausente), encrypta com `BRIDGE_ENCRYPTION_KEY`, persiste
  como `chatwoot_hmac_secret` no tenant.
- **Test:** smoke real `scripts/e2e/smoke-doc.ps1` valida a assinatura
  end-to-end antes do close.

### BUG-WIZARD-01 — Tenant meio-onboarded quando `base_url` vazio

- **Sintoma:** wizard mostra "tenant criado" mas:
  - megaAPI webhook NÃO foi registrado
  - Chatwoot inbox webhook NÃO foi PATCHed
  - tenant fica órfão no DB
- **Severidade:** Alta (silent failure; operador não percebe até tentar
  enviar mensagem).
- **Root cause:** wizard chamava `InsertTenant` antes de validar
  `settings.base_url`. As chamadas seguintes (`configWebhook`,
  `PATCH inbox`) então abortavam silenciosamente porque a URL gerada era
  vazia, mas a UI já havia recebido 201.
- **Fix (`b57832c`):** validação subiu pra **antes** do `InsertTenant`.
  Retorna 400 PT-BR explicando que `settings.base_url` é obrigatório.
- **Test:** unit test cobre 3 cenários: vazio, malformado, scheme errado.

## Lições

1. **Rebuild explicitamente, sempre.** Não confie em `restart`. Ver
   CLAUDE.md "Lessons Learned" e considerar `make live-test`.
2. **Chatwoot 3.x assina com `channel.secret`.** Documentado em CLAUDE.md.
   Não regredir para `hmac_token`.
3. **`settings.base_url` é pré-requisito hard para qualquer tenant.** O
   wizard agora trata como invariante; manter assim.
4. **Capture erros Postgres específicos.** Sempre `errors.As(err,
   &pgconn.PgError)` antes de devolver 500. Adicionar lint custom no
   futuro?
5. **Smoke real end-to-end pega bugs que unit tests não pegam.**
   `smoke-doc.ps1` foi decisivo para o BUG-HMAC-01.

## Follow-ups

- [x] Cortar v1.0.1 com os fixes (esta sessão).
- [ ] Considerar `Makefile` target `live-test` que rebuilda + faz smoke
  contra `/healthz` antes de devolver shell.
- [ ] Adicionar test E2E que simule "tenant criado sem `base_url`" para
  garantir que BUG-WIZARD-01 não regrida.
- [ ] Verificar se Chatwoot 4.x manterá `channel.secret` ou voltará a
  `hmac_token`. Caso mude, adicionar feature-flag.

## Referências

- Commits: `9f67d6e`, `ccd56b6`, `7cbb94d`, `225f908`, `02b871d`, `b204e66`,
  `7fe688a`, `b57832c`.
- Goal plan: `docs/goals/2026-05-24-harden-qa-sweep-tdd.md`,
  `docs/goals/2026-05-25-add-hmac-pairing.md`.
- Smoke: `scripts/e2e/smoke-doc.ps1`.
- Pilot log: `docs/release/PILOT-LOG.md` (Day 1, 2026-05-25).
