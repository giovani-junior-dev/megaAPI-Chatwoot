# Goal Plan — harden-v1-release

## 1. Contexto

- Repo: `C:\Users\GEOVANE\Desktop\Projetos\chatwoot-megaapi-bridge`
- Stack: Go 1.25, Postgres, Docker Compose, Caddy/Cloudflared. F1-F5+F-media completos.
- Estado atual:
  - 193 unit + 253 integration tests passando.
  - 8 sub-issues F6 abertas em bd `phase-6`.
  - **efn.8 (5 clientes piloto 7 dias)** é HUMAN-only — NÃO automatizável via goal loop (requer pilotos reais). Será closed manualmente após dry-run pelos clientes.
  - **efn.3 (Load test 24h)** o run real de 24h é triggered manual; goal entrega harness + smoke (1h) validando setup.
- Comando teste unit: `go test ./...`
- Comando teste integração: `go test -tags=integration ./...`
- Comando lint: `go vet ./...`
- Comando build: `go build ./...`

## 2. Estado final mensurável

7 sub-issues F6 implementadas (efn.8 deferida pra trigger humano pós-merge):

| bd | Tarefa | Verificável |
|---|---|---|
| efn.1 | Pen test scanners: gosec + nuclei + ZAP CLI | `make security-scan` exits 0 (ou produz JSON sem critical findings); scripts em `deploy/security/*.sh` + relatórios em `security-reports/` |
| efn.2 | Code review appsec-elite-auditor | `docs/security/AUDIT-REPORT.md` produzido (skill ou manual review documentado) ≥100 linhas com findings + remediation status |
| efn.3 | Load test harness (24h real triggered manual; goal entrega 1h smoke) | `deploy/loadtest/k6-bridge.js` ou `vegeta-bridge.sh` + `make loadtest-smoke` roda 1h e gera relatório `loadtest-results/smoke.json` com `error_rate < 1%` e `p99_latency < 500ms` |
| efn.4 (+8s8.10) | Chaos test: kill containers under load | `deploy/chaos/chaos.sh` mata bridge/db/chatwoot containers durante load test smoke; bridge auto-recupera (RecoverPending); script exits 0 |
| efn.5 | Documentação de operação completa | `docs/OPERATIONS.md` ≥300 linhas cobrindo: deploy inicial, upgrade, backup/restore, rollback, troubleshooting, monitoring, scaling, incident response runbook |
| efn.6 | Política versão + breaking changes | `docs/VERSIONING.md` ≥80 linhas com SemVer policy, deprecation cycle, breaking change checklist, supported versions matrix; `CHANGELOG.md` formatado Keep-a-Changelog |
| efn.7 | Tag v1.0.0 + release notes | `git tag -a v1.0.0 -m "v1.0.0"` criada (não pushed automaticamente — usuário decide quando); `RELEASE_NOTES.md` ≥150 linhas com features F1-F5+F-media, breaking changes, upgrade path, contributors |
| efn.8 | 5 clientes piloto 7 dias | **OUT-OF-SCOPE** goal automation — deferido. Documentado em `docs/release/PILOT-PROGRAM.md` ≥50 linhas com critério aceitação + tracking sheet template |

## 3. Prova surfaceável

Comandos a cada turno (echo full output):

```bash
go test ./... 2>&1 | tail -3
go vet ./... 2>&1 | tail -3
bd list --status=open --label=phase-6 2>&1 | tail -5
git log --oneline -10
ls deploy/security/ deploy/loadtest/ deploy/chaos/ 2>&1
wc -l docs/OPERATIONS.md docs/VERSIONING.md docs/release/RELEASE_NOTES.md 2>&1
gosec -fmt=json -out=/tmp/gosec.json -severity=high ./... 2>&1; jq '.Stats.found' /tmp/gosec.json
test -f CHANGELOG.md && head -20 CHANGELOG.md
git tag -l v1.0.0
```

Output esperado ao concluir:

- `go test ./...`: `ok`, ≥193 passed (sem regressão)
- `go vet`: zero output
- `bd list --status=open --label=phase-6 | tail`: só epic `efn` + `efn.8` (piloto manual) open
- `git log --oneline -10`: 7 novos commits prefixados `feat(security):`, `feat(loadtest):`, `feat(chaos):`, `docs(ops):`, `docs(release):`, `chore(release):`
- `ls deploy/security/`: scripts gosec/nuclei/zap
- `wc -l docs/OPERATIONS.md`: ≥300; `VERSIONING.md`: ≥80; `RELEASE_NOTES.md`: ≥150
- `gosec` Stats.found: 0 (high severity)
- `CHANGELOG.md`: arquivo presente
- `git tag v1.0.0`: tag local criada

## 4. Restrições

### Específicas do projeto

- NÃO modificar código Go bridge core sem CVE/finding documentado (security fixes só com referência explícita em AUDIT-REPORT.md)
- NÃO push `v1.0.0` tag para remote — só criar local (usuário decide push)
- NÃO criar PRs novas; tudo via commits direto no master (mesmo padrão F1-F5)
- NÃO modificar `Makefile` exceto pra adicionar targets `security-scan`, `loadtest-smoke`, `chaos-smoke`
- Scripts security/loadtest/chaos em `deploy/{security,loadtest,chaos}/` com `set -euo pipefail`
- Load test 24h NÃO roda no goal loop; ship harness + smoke 1h apenas
- gosec instalado via `go install github.com/securego/gosec/v2/cmd/gosec@latest`
- nuclei + ZAP rodados via Docker (não instalar host)
- `RELEASE_NOTES.md` em `docs/release/` (não raiz pra não poluir)
- Tag v1.0.0 deve ser anotada (`git tag -a`), não lightweight
- Findings high/critical do gosec devem ser zero antes de criar tag v1.0.0
- Cada sub-issue concluída → `bd close <id>` antes da próxima
- 1 commit por sub-issue

### Padrão

- NÃO `--no-verify`
- NÃO commitar segredos
- NÃO force-push
- NÃO modificar `go.sum` exceto via `go get`
- Mensagens commit descritivas (`feat(security):`, `feat(loadtest):`, `feat(chaos):`, `docs(ops):`, `docs(release):`, `chore(release):`)

## 5. Bound

- **70 turnos** OU **240 minutos**
- Justificativa: 7 sub-issues automatizáveis. Scanners+docs dominam tempo. Load smoke 1h roda em background. efn.8 fora do loop.

## 6. Modo de execução recomendado

- Auto mode: ligar (`Shift+Tab` até "acceptEdits")
- Headless opcional: `claude -p "/goal <condição>"` para CI/cron

## 7. Condição final (cole no /goal)

```
F6 hardening + v1.0 release completo: 7 sub-issues automatizáveis do épico chatwoot-megaapi-bridge-efn implementadas (efn.8 piloto 7 dias deferida pra trigger humano). Estado final por sub-issue: (1) efn.1 deploy/security/{gosec,nuclei,zap}.sh + Makefile target make security-scan + relatórios em security-reports/; gosec high severity findings = 0; (2) efn.2 docs/security/AUDIT-REPORT.md ≥100 linhas com findings + remediation; (3) efn.3 deploy/loadtest/{k6-bridge.js OR vegeta-bridge.sh} + make loadtest-smoke roda 1h e gera loadtest-results/smoke.json com error_rate<1% e p99_latency<500ms; harness pronto para 24h real disparado manual; (4) efn.4 deploy/chaos/chaos.sh mata bridge/db/chatwoot containers durante smoke load; bridge auto-recupera via RecoverPending; script exits 0; (5) efn.5 docs/OPERATIONS.md ≥300 linhas cobrindo deploy upgrade backup restore rollback troubleshooting monitoring scaling incident response; (6) efn.6 docs/VERSIONING.md ≥80 linhas com SemVer policy deprecation cycle breaking change checklist supported versions matrix + CHANGELOG.md formato Keep-a-Changelog; (7) efn.7 git tag -a v1.0.0 criada localmente (NÃO push remote) + docs/release/RELEASE_NOTES.md ≥150 linhas com features F1-F5 F-media breaking changes upgrade path contributors; (8) efn.8 docs/release/PILOT-PROGRAM.md ≥50 linhas com critério aceitação + tracking template (bd issue mantida open para trigger humano). Provar com: `go test ./... 2>&1 | tail -3` mostrando ok ≥193 passed sem regressão; `go vet ./... 2>&1` zero output; `bd list --status=open --label=phase-6 2>&1 | tail -5` mostrando apenas epic efn + efn.8 open; `git log --oneline -10` com 7 novos commits feat(security)/feat(loadtest)/feat(chaos)/docs(ops)/docs(release)/chore(release); `ls deploy/security/ deploy/loadtest/ deploy/chaos/ 2>&1` listando scripts; `wc -l docs/OPERATIONS.md` ≥300; `wc -l docs/VERSIONING.md` ≥80; `wc -l docs/release/RELEASE_NOTES.md` ≥150; `wc -l docs/release/PILOT-PROGRAM.md` ≥50; `wc -l docs/security/AUDIT-REPORT.md` ≥100; gosec relatório com 0 findings high; `test -f CHANGELOG.md && head -1 CHANGELOG.md` mostrando header Keep-a-Changelog; `git tag -l v1.0.0` retornando v1.0.0. Restrições: NÃO modificar código Go core sem CVE/finding documentado; NÃO push v1.0.0 tag remoto (só local); NÃO criar PRs; NÃO modificar Makefile exceto targets security-scan/loadtest-smoke/chaos-smoke; scripts em deploy/{security,loadtest,chaos}/ com set -euo pipefail; load test 24h NÃO no goal loop apenas smoke 1h; gosec via go install; nuclei+ZAP via Docker; RELEASE_NOTES.md em docs/release/; tag v1.0.0 anotada (git tag -a); gosec high/critical zero antes da tag; cada sub-issue roda `bd close <id>` antes da próxima; 1 commit por sub-issue; sem --no-verify; sem segredos; sem force-push; sem modificar go.sum exceto via go get; mensagens commit descritivas feat(security)/feat(loadtest)/feat(chaos)/docs(ops)/docs(release)/chore(release), or stop after 70 turns or 240m. Report turn count, sub-issue atual (1-8), tests passed, bd open count, remaining bound each turn. Claude must echo full output of each verification command.
```

## 8. Comando completo

```
/goal F6 hardening + v1.0 release completo: 7 sub-issues automatizáveis do épico chatwoot-megaapi-bridge-efn implementadas (efn.8 piloto 7 dias deferida pra trigger humano). Estado final por sub-issue: (1) efn.1 deploy/security/{gosec,nuclei,zap}.sh + Makefile target make security-scan + relatórios em security-reports/; gosec high severity findings = 0; (2) efn.2 docs/security/AUDIT-REPORT.md ≥100 linhas com findings + remediation; (3) efn.3 deploy/loadtest/{k6-bridge.js OR vegeta-bridge.sh} + make loadtest-smoke roda 1h e gera loadtest-results/smoke.json com error_rate<1% e p99_latency<500ms; harness pronto para 24h real disparado manual; (4) efn.4 deploy/chaos/chaos.sh mata bridge/db/chatwoot containers durante smoke load; bridge auto-recupera via RecoverPending; script exits 0; (5) efn.5 docs/OPERATIONS.md ≥300 linhas cobrindo deploy upgrade backup restore rollback troubleshooting monitoring scaling incident response; (6) efn.6 docs/VERSIONING.md ≥80 linhas com SemVer policy deprecation cycle breaking change checklist supported versions matrix + CHANGELOG.md formato Keep-a-Changelog; (7) efn.7 git tag -a v1.0.0 criada localmente (NÃO push remote) + docs/release/RELEASE_NOTES.md ≥150 linhas com features F1-F5 F-media breaking changes upgrade path contributors; (8) efn.8 docs/release/PILOT-PROGRAM.md ≥50 linhas com critério aceitação + tracking template (bd issue mantida open para trigger humano). Provar com: `go test ./... 2>&1 | tail -3` mostrando ok ≥193 passed sem regressão; `go vet ./... 2>&1` zero output; `bd list --status=open --label=phase-6 2>&1 | tail -5` mostrando apenas epic efn + efn.8 open; `git log --oneline -10` com 7 novos commits feat(security)/feat(loadtest)/feat(chaos)/docs(ops)/docs(release)/chore(release); `ls deploy/security/ deploy/loadtest/ deploy/chaos/ 2>&1` listando scripts; `wc -l docs/OPERATIONS.md` ≥300; `wc -l docs/VERSIONING.md` ≥80; `wc -l docs/release/RELEASE_NOTES.md` ≥150; `wc -l docs/release/PILOT-PROGRAM.md` ≥50; `wc -l docs/security/AUDIT-REPORT.md` ≥100; gosec relatório com 0 findings high; `test -f CHANGELOG.md && head -1 CHANGELOG.md` mostrando header Keep-a-Changelog; `git tag -l v1.0.0` retornando v1.0.0. Restrições: NÃO modificar código Go core sem CVE/finding documentado; NÃO push v1.0.0 tag remoto (só local); NÃO criar PRs; NÃO modificar Makefile exceto targets security-scan/loadtest-smoke/chaos-smoke; scripts em deploy/{security,loadtest,chaos}/ com set -euo pipefail; load test 24h NÃO no goal loop apenas smoke 1h; gosec via go install; nuclei+ZAP via Docker; RELEASE_NOTES.md em docs/release/; tag v1.0.0 anotada (git tag -a); gosec high/critical zero antes da tag; cada sub-issue roda `bd close <id>` antes da próxima; 1 commit por sub-issue; sem --no-verify; sem segredos; sem force-push; sem modificar go.sum exceto via go get; mensagens commit descritivas feat(security)/feat(loadtest)/feat(chaos)/docs(ops)/docs(release)/chore(release), or stop after 70 turns or 240m. Report turn count, sub-issue atual (1-8), tests passed, bd open count, remaining bound each turn. Claude must echo full output of each verification command.
```

## 9. Comando headless (opcional)

```
claude -p "/goal F6 hardening + v1.0 release ..."
```

## 10. Checklist pré-entrega

- [x] ≤4000 chars (~3960)
- [x] Comandos concretos (go test, go vet, bd list, git log, ls, wc -l, gosec, git tag)
- [x] Output literal (ok, ≥193, ≥300/80/150/50/100, 0 findings)
- [x] Restrições específicas + padrão
- [x] Bound (70 turns OR 240m)
- [x] Echo obrigatório
- [x] Arquivo salvo em `docs/goals/2026-05-24-harden-v1-release.md`
- [x] Slug `harden-<area>`
- [x] Estado final falsificável
- [x] efn.8 explicitamente marcada OUT-OF-SCOPE com handoff humano

## 11. Ordem sugerida das 7 + 1 sub-issues

| # | bd | Razão da ordem |
|---|---|---|
| 1 | efn.1 | Pen test scanners — base segurança antes de tag |
| 2 | efn.2 | Audit report — depende scans |
| 3 | efn.4 | Chaos script — pequeno isolado |
| 4 | efn.3 | Load test harness + smoke — depende chaos opcional |
| 5 | efn.5 | docs/OPERATIONS.md — depende todo deploy/F4 entendido |
| 6 | efn.6 | VERSIONING + CHANGELOG — pré-req release |
| 7 | efn.7 | Tag v1.0.0 + RELEASE_NOTES — última coisa antes piloto |
| 8 | efn.8 | PILOT-PROGRAM.md doc (sub-issue bd mantida open) |

## 12. Lembretes operacionais

- `gosec` instala via: `go install github.com/securego/gosec/v2/cmd/gosec@latest` (binário em `$GOPATH/bin`)
- nuclei: `docker run -v $PWD:/work projectdiscovery/nuclei -u https://localhost:8090`
- OWASP ZAP CLI: `docker run -t owasp/zap2docker-stable zap-baseline.py -t https://localhost:8090`
- k6 ou vegeta — escolher conforme stack: k6 é JS friendly, vegeta é Go nativo
- Load test 24h: trigger manual `make loadtest-24h` (não no loop)
- 5 piloto 7 dias: trigger manual após release (issue bd fica open com critério escrito)
- git tag NÃO push: `git tag -a v1.0.0 -m "..."` só local. Push é `git push origin v1.0.0` manual
- gh auth switch `giovani-junior-dev` antes do push final
- Verify `gosec -fmt=json -severity=high ./...` retorna 0 findings antes da tag

## 13. Out-of-scope explícito

- **efn.8 5 clientes piloto 7 dias**: deferido pós-release. Bd issue mantida open. Documento `docs/release/PILOT-PROGRAM.md` ship junto.
- **Load test 24h real**: harness ship. Run real triggered manual após release.
- Push da tag v1.0.0 ao remote: decisão humana (não automatizar).
