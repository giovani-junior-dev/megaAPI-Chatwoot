# Goal Plan — complete-f5-observability

## 1. Contexto

- Repo: `C:\Users\GEOVANE\Desktop\Projetos\chatwoot-megaapi-bridge`
- Stack: Go 1.25, Postgres, Docker Compose. F2 já entregou `/metrics` Prometheus (5 series) + logs zerolog estruturados.
- Estado atual:
  - F1+F2+F3+F4+F-media completos. 183 unit + 243 integration tests passando.
  - 7 sub-issues F5 abertas em bd `phase-5`.
  - Bridge tem `/metrics`, `/healthz`, `/readyz`, `/admin/failed`, `/admin/retry/{id}`, `/login`, dashboard.
- Comando teste unit: `go test ./...`
- Comando teste integração: `go test -tags=integration ./...`
- Comando lint: `go vet ./...`
- Comando build: `go build ./...`

## 2. Estado final mensurável

7 sub-issues F5 implementadas. Stack opcional de observabilidade (Grafana+Prometheus+AlertManager) entregue como overlay docker-compose separado (NÃO bundle no stack principal). Bridge ganha pprof gated + OTel opt-in.

| bd | Tarefa | Verificável |
|---|---|---|
| 2u4.4 | `deploy/observability/docker-compose.yml` (Prometheus+Grafana+AlertManager overlay) | `docker compose -f deploy/observability/docker-compose.yml config` exits 0 |
| 2u4.1 | Grafana dashboard pré-construído | `deploy/observability/grafana/dashboards/bridge.json` parseia JSON e contém ≥4 painéis referenciando metrics `bridge_messages_*` |
| 2u4.2 | AlertManager rules | `deploy/observability/prometheus/alerts.yml` parseia YAML; ≥3 rules: high_failed_rate, queue_depth_high, msg_rate_drop |
| 2u4.3 | Webhook alerta configurável (Slack/Discord/email) | `deploy/observability/alertmanager/alertmanager.yml` template com placeholders `WEBHOOK_URL`, `EMAIL_TO`; doc explica `webhook_configs` vs `email_configs` |
| 2u4.5 | OpenTelemetry traces (opt-in) | Bridge ganha env `OTEL_EXPORTER_OTLP_ENDPOINT`; quando setada, exporta spans para endpoint; quando vazia, no-op. Teste valida ambos modos |
| 2u4.6 | Profiling endpoint admin-gated | `GET /debug/pprof/` retorna 401 sem session cookie, 200 com cookie válido. Endpoints `/debug/pprof/heap`, `/debug/pprof/goroutine`, etc. expostos via stdlib `net/http/pprof` |
| 2u4.7 | Troubleshooting docs | `docs/TROUBLESHOOTING.md` ≥150 linhas cobrindo: bridge não sobe, /healthz falha, /readyz 503, webhook unauthorized, megaAPI 401, Chatwoot 422, msg fica pending, DLQ cresce, métricas vazias, OTel não exporta |

## 3. Prova surfaceável

Comandos a cada turno (echo full output):

```bash
go test ./... 2>&1 | tail -3
go vet ./... 2>&1 | tail -3
bd list --status=open --label=phase-5 2>&1 | tail -5
git log --oneline -10
docker compose -f deploy/observability/docker-compose.yml config > /dev/null && echo "obs compose OK"
python -c "import json; json.load(open('deploy/observability/grafana/dashboards/bridge.json')); print('dashboard JSON OK')"
python -c "import yaml; yaml.safe_load(open('deploy/observability/prometheus/alerts.yml')); print('alerts YAML OK')"
python -c "import yaml; yaml.safe_load(open('deploy/observability/alertmanager/alertmanager.yml')); print('alertmanager YAML OK')"
wc -l docs/TROUBLESHOOTING.md
curl -sS -o /dev/null -w "/debug/pprof/ no-auth: %{http_code}\n" http://localhost:8090/debug/pprof/
```

Output esperado ao concluir:

- `go test ./...`: `ok`, ≥190 passed (sem regressão)
- `go vet`: zero output
- `bd list --status=open --label=phase-5 | tail`: só epic `chatwoot-megaapi-bridge-2u4` ainda open
- `git log --oneline -10`: 7 novos commits `feat(observability):`, `feat(otel):`, `feat(debug):`, `docs(troubleshooting):`
- `docker compose config`: `obs compose OK`
- JSON/YAML parses: `... OK`
- `wc -l docs/TROUBLESHOOTING.md`: ≥150
- `/debug/pprof/` sem auth: `401`

## 4. Restrições

### Específicas do projeto

- NÃO bundle stack observability no `docker-compose.yml` raiz; usar `deploy/observability/docker-compose.yml` como overlay opcional (user roda separado)
- NÃO adicionar deps Go fora de stdlib + atual + `go.opentelemetry.io/otel` family (necessário para OTel)
- pprof DEVE ser admin-gated (reusar middleware session F3); 401 quando não autenticado
- OTel DEVE ser no-op quando `OTEL_EXPORTER_OTLP_ENDPOINT` vazio (não crashar, não exportar)
- Funções ≤20 linhas, ≤2 parâmetros (ctx exceto)
- Sem interfaces especulativas
- TDD obrigatório para código Go novo (OTel init, pprof handler gated)
- Cada sub-issue concluída → `bd close <id>` antes da próxima
- 1 commit por sub-issue
- Grafana dashboard JSON: schema válido Grafana 10+ (testar via `python -c json.load`)
- AlertManager + Prometheus config YAML: schema válido (testar via `python -c yaml.safe_load`)
- Bridge container scratch sem CGO (OTel deps sem CGO)
- Bridge `/metrics` endpoint F2 NÃO pode ser quebrado (regressão)

### Padrão

- NÃO `--no-verify`
- NÃO `//nolint`
- NÃO modificar `go.sum` exceto via `go get`
- NÃO commitar segredos
- NÃO force-push
- Mensagens commit descritivas (`feat(observability):`, `feat(otel):`, `feat(debug):`, `docs(troubleshooting):`)

## 5. Bound

- **50 turnos** OU **150 minutos**
- Justificativa: 7 sub-issues, maioria config files (Grafana JSON, Prometheus/AlertManager YAML). Código Go limitado a OTel init + pprof gating + tests.

## 6. Modo de execução recomendado

- Auto mode: ligar (`Shift+Tab` até "acceptEdits")
- Headless opcional: `claude -p "/goal <condição>"` para CI/cron

## 7. Condição final (cole no /goal)

```
F5 observabilidade avançada completa: 7 sub-issues do épico chatwoot-megaapi-bridge-2u4 implementadas. Stack observability como overlay separado (NÃO bundle no docker-compose raiz). Estado final por sub-issue: (1) 2u4.4 deploy/observability/docker-compose.yml com Prometheus+Grafana+AlertManager validado por docker compose config exits 0; (2) 2u4.1 deploy/observability/grafana/dashboards/bridge.json JSON válido com ≥4 painéis referenciando bridge_messages_in_total bridge_messages_out_total bridge_messages_failed_total bridge_job_duration_seconds bridge_queue_depth; (3) 2u4.2 deploy/observability/prometheus/alerts.yml YAML válido com ≥3 rules high_failed_rate queue_depth_high msg_rate_drop; (4) 2u4.3 deploy/observability/alertmanager/alertmanager.yml YAML válido com placeholders WEBHOOK_URL EMAIL_TO + doc explicando webhook_configs vs email_configs; (5) 2u4.5 bridge ganha env OTEL_EXPORTER_OTLP_ENDPOINT; quando setada exporta spans via OTLP/HTTP usando go.opentelemetry.io/otel; quando vazia no-op não crasha; testes cobrem ambos modos; (6) 2u4.6 GET /debug/pprof/ retorna 401 sem session cookie 200 com cookie; endpoints heap/goroutine/profile expostos via net/http/pprof stdlib; (7) 2u4.7 docs/TROUBLESHOOTING.md ≥150 linhas cobrindo bridge não sobe /healthz falha /readyz 503 webhook unauthorized megaAPI 401 Chatwoot 422 msg pending DLQ cresce métricas vazias OTel não exporta. Provar com: `go test ./... 2>&1 | tail -3` mostrando ok ≥190 passed sem regressão; `go vet ./... 2>&1` zero output; `bd list --status=open --label=phase-5 2>&1 | tail -5` mostrando só epic 2u4 open; `git log --oneline -10` com 7 novos commits feat(observability)/feat(otel)/feat(debug)/docs(troubleshooting); `docker compose -f deploy/observability/docker-compose.yml config` exits 0 mostrando obs compose OK; `python -c "import json; json.load(open('deploy/observability/grafana/dashboards/bridge.json'))"` sem erro mostrando dashboard JSON OK; `python -c "import yaml; yaml.safe_load(open('deploy/observability/prometheus/alerts.yml'))"` mostrando alerts YAML OK; `python -c "import yaml; yaml.safe_load(open('deploy/observability/alertmanager/alertmanager.yml'))"` mostrando alertmanager YAML OK; `wc -l docs/TROUBLESHOOTING.md` mostrando ≥150; `curl -sS -o /dev/null -w "%{http_code}\n" http://localhost:8090/debug/pprof/` retorna 401 sem auth. Restrições: NÃO bundle observability stack no docker-compose.yml raiz; usar deploy/observability/docker-compose.yml como overlay opcional; NÃO adicionar deps Go fora de stdlib + go.opentelemetry.io/otel family; pprof admin-gated via middleware session F3 retorna 401 sem cookie; OTel no-op quando OTEL_EXPORTER_OTLP_ENDPOINT vazio; funções ≤20 linhas ≤2 params (ctx exceto); sem interfaces especulativas; TDD obrigatório código Go novo; cada sub-issue roda `bd close <id>` antes da próxima; 1 commit por sub-issue; Grafana dashboard JSON schema Grafana 10+; YAML AlertManager/Prometheus schema válido; container scratch sem CGO; /metrics F2 não pode regredir; sem --no-verify; sem //nolint; sem modificar go.sum exceto via go get; sem segredos; sem force-push; mensagens commit descritivas feat(observability)/feat(otel)/feat(debug)/docs(troubleshooting), or stop after 50 turns or 150m. Report turn count, sub-issue atual (1-7), tests passed, bd open count, remaining bound each turn. Claude must echo full output of each verification command.
```

## 8. Comando completo

```
/goal F5 observabilidade avançada completa: 7 sub-issues do épico chatwoot-megaapi-bridge-2u4 implementadas. Stack observability como overlay separado (NÃO bundle no docker-compose raiz). Estado final por sub-issue: (1) 2u4.4 deploy/observability/docker-compose.yml com Prometheus+Grafana+AlertManager validado por docker compose config exits 0; (2) 2u4.1 deploy/observability/grafana/dashboards/bridge.json JSON válido com ≥4 painéis referenciando bridge_messages_in_total bridge_messages_out_total bridge_messages_failed_total bridge_job_duration_seconds bridge_queue_depth; (3) 2u4.2 deploy/observability/prometheus/alerts.yml YAML válido com ≥3 rules high_failed_rate queue_depth_high msg_rate_drop; (4) 2u4.3 deploy/observability/alertmanager/alertmanager.yml YAML válido com placeholders WEBHOOK_URL EMAIL_TO + doc explicando webhook_configs vs email_configs; (5) 2u4.5 bridge ganha env OTEL_EXPORTER_OTLP_ENDPOINT; quando setada exporta spans via OTLP/HTTP usando go.opentelemetry.io/otel; quando vazia no-op não crasha; testes cobrem ambos modos; (6) 2u4.6 GET /debug/pprof/ retorna 401 sem session cookie 200 com cookie; endpoints heap/goroutine/profile expostos via net/http/pprof stdlib; (7) 2u4.7 docs/TROUBLESHOOTING.md ≥150 linhas cobrindo bridge não sobe /healthz falha /readyz 503 webhook unauthorized megaAPI 401 Chatwoot 422 msg pending DLQ cresce métricas vazias OTel não exporta. Provar com: `go test ./... 2>&1 | tail -3` mostrando ok ≥190 passed sem regressão; `go vet ./... 2>&1` zero output; `bd list --status=open --label=phase-5 2>&1 | tail -5` mostrando só epic 2u4 open; `git log --oneline -10` com 7 novos commits feat(observability)/feat(otel)/feat(debug)/docs(troubleshooting); `docker compose -f deploy/observability/docker-compose.yml config` exits 0 mostrando obs compose OK; `python -c "import json; json.load(open('deploy/observability/grafana/dashboards/bridge.json'))"` sem erro mostrando dashboard JSON OK; `python -c "import yaml; yaml.safe_load(open('deploy/observability/prometheus/alerts.yml'))"` mostrando alerts YAML OK; `python -c "import yaml; yaml.safe_load(open('deploy/observability/alertmanager/alertmanager.yml'))"` mostrando alertmanager YAML OK; `wc -l docs/TROUBLESHOOTING.md` mostrando ≥150; `curl -sS -o /dev/null -w "%{http_code}\n" http://localhost:8090/debug/pprof/` retorna 401 sem auth. Restrições: NÃO bundle observability stack no docker-compose.yml raiz; usar deploy/observability/docker-compose.yml como overlay opcional; NÃO adicionar deps Go fora de stdlib + go.opentelemetry.io/otel family; pprof admin-gated via middleware session F3 retorna 401 sem cookie; OTel no-op quando OTEL_EXPORTER_OTLP_ENDPOINT vazio; funções ≤20 linhas ≤2 params (ctx exceto); sem interfaces especulativas; TDD obrigatório código Go novo; cada sub-issue roda `bd close <id>` antes da próxima; 1 commit por sub-issue; Grafana dashboard JSON schema Grafana 10+; YAML AlertManager/Prometheus schema válido; container scratch sem CGO; /metrics F2 não pode regredir; sem --no-verify; sem //nolint; sem modificar go.sum exceto via go get; sem segredos; sem force-push; mensagens commit descritivas feat(observability)/feat(otel)/feat(debug)/docs(troubleshooting), or stop after 50 turns or 150m. Report turn count, sub-issue atual (1-7), tests passed, bd open count, remaining bound each turn. Claude must echo full output of each verification command.
```

## 9. Comando headless (opcional)

```
claude -p "/goal F5 observabilidade ..."
```

## 10. Checklist pré-entrega

- [x] ≤4000 chars
- [x] Comandos concretos (go test, go vet, bd list, git log, docker compose config, python json/yaml parse, wc -l, curl)
- [x] Output literal definido (ok, OK, ≥190, ≥150, 401)
- [x] Restrições específicas + padrão
- [x] Bound presente (50 turns OR 150m)
- [x] Echo obrigatório
- [x] Arquivo salvo em `docs/goals/2026-05-24-complete-f5-observability.md`
- [x] Slug `complete-<area>`
- [x] Estado final falsificável

## 11. Ordem sugerida das 7 sub-issues

| # | bd | Razão da ordem |
|---|---|---|
| 1 | 2u4.7 | Troubleshooting doc (sem deps; cria base mental) |
| 2 | 2u4.6 | pprof admin-gated (código Go isolado, TDD limpo) |
| 3 | 2u4.5 | OTel opt-in (código Go + deps novas isoladas) |
| 4 | 2u4.4 | Compose overlay (esqueleto containers) |
| 5 | 2u4.1 | Grafana dashboard JSON (depende compose) |
| 6 | 2u4.2 | Prometheus alerts (depende compose) |
| 7 | 2u4.3 | AlertManager routing (depende alerts) |

## 12. Lembretes operacionais

- Grafana 10+ dashboard JSON schema: campo `schemaVersion: 38+`
- AlertManager YAML root keys: `route`, `receivers`, `inhibit_rules`
- Prometheus rules: `groups[].rules[].{alert,expr,for,labels,annotations}`
- OTel Go SDK: `go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp`
- pprof stdlib: `import _ "net/http/pprof"` registra rotas em DefaultServeMux; bridge usa chi → registrar handler explícito por path
- gh auth pode precisar switch para `giovani-junior-dev` antes do push
- Bridge container precisa rebuild se cmd/bridge/main.go ou deps mudarem: `docker compose up -d --build bridge`

## 13. Risco YAGNI

F5 inteiro é opcional pra MVP. Cliente típico self-host pode rodar bridge sem Grafana/AlertManager se só usa `/metrics` raw + logs. Considerar **versão reduzida**:

| Item | Manter? | Razão |
|---|---|---|
| 2u4.7 troubleshooting | ✓ | Doc operacional alto valor |
| 2u4.6 pprof gated | ✓ | Útil debug produção, baixo custo |
| 2u4.5 OTel | ⚠ | Útil pra quem usa OTLP — keep como opt-in |
| 2u4.4 compose overlay | ✓ | Material pronto que cliente layer-on |
| 2u4.1 Grafana dashboard | ✓ | JSON estático, alto valor |
| 2u4.2 alerts | ✓ | Rules úteis pra qualquer scrape Prometheus |
| 2u4.3 AlertManager routing | ⚠ | Template básico OK, evitar over-config |

Manter 7 itens mas escopo enxuto cada um.
