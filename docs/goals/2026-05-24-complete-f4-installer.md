# Goal Plan — complete-f4-installer

## 1. Contexto

- Repo: `C:\Users\GEOVANE\Desktop\Projetos\chatwoot-megaapi-bridge`
- Stack: Go 1.25, Postgres, Docker Compose, Caddy (reverse proxy), Cloudflared (alternative tunnel). Bridge container scratch.
- Estado atual:
  - F1+F2+F3+F-media completos. 177 unit + 237 integration testes passando.
  - `docker-compose.yml` (bridge stack) e `deploy/chatwoot.docker-compose.yml` (chatwoot stack) existem mas requerem orquestração manual.
  - `deploy/Caddyfile` esqueleto adicionado em F3 (834.12 — só headers segurança, sem domínio variável).
  - 12 sub-issues F4 abertos em bd `phase-4`.
  - Target deploy: Ubuntu 22.04 VPS limpa, dev → operacional em <15min via 1 comando.
- Comando teste unit: `go test ./...`
- Comando teste integração: `go test -tags=integration ./...`
- Comando lint: `go vet ./...`
- Comando build: `go build ./...`
- Shell de instalação: bash (target Ubuntu 22.04)

## 2. Estado final mensurável

12 sub-issues F4 implementadas. VPS limpa Ubuntu 22.04 sobe stack completa (bridge + chatwoot + postgres + caddy/cloudflared + backup sidecar) via `curl ... | bash` único, finaliza em <15min, e responde 200 em `/healthz` da bridge + Chatwoot UI carrega.

| bd | Tarefa | Verificável |
|---|---|---|
| 37j.2 | Templates `.env.bridge` + `.env.chatwoot` | Arquivos `deploy/templates/.env.bridge.tpl` + `.env.chatwoot.tpl` existem com placeholders `{{DOMAIN}}` `{{MASTER_KEY}}` `{{POSTGRES_PASSWORD}}` etc. |
| 37j.3 | Caddyfile rendering com domínio | `deploy/templates/Caddyfile.tpl` parametrizável; install.sh substitui `{{DOMAIN}}` por valor real |
| 37j.4 | `init.sql` cria DBs chatwoot + bridge | `deploy/init.sql` cria duas DBs + users + grants |
| 37j.5 | Bootstrap DB Chatwoot | install.sh roda `docker compose run --rm rails bundle exec rails db:chatwoot_prepare` |
| 37j.6 | Bootstrap DB Bridge | install.sh roda `docker compose exec bridge /bridge migrate` |
| 37j.7 | Suporte Cloudflare Tunnel | install.sh oferece opção tunnel cloudflared vs Caddy TLS direto; se cloudflared, gera token e configura |
| 37j.8 | Sidecar postgres-backup-local | container `postgres-backup-local` adicionado ao compose; backup diário em volume `/backups` |
| 37j.9 | Script `upgrade.sh` | `deploy/upgrade.sh` faz `git pull` + `docker compose pull` + `docker compose up -d --build` + migrate, mantendo dados |
| 37j.1 ⭐ | Script `install.sh` interativo | `deploy/install.sh` orquestra tudo: prompts (domain, email Caddy, modo tunnel/TLS), gera secrets, baixa imagens, sobe stack, valida health |
| 37j.10 | `postinstall-check.sh` | Script roda checklist: containers up, /healthz 200, Chatwoot UI 200, migrations aplicadas, backup container ativo, Caddy TLS válido (se modo TLS) |
| 37j.11 | `INSTALL.md` | Documenta pré-requisitos, fluxo install, troubleshooting; ≥200 linhas |
| 37j.12 ⭐ | Validação VPS limpa < 15min | Script `deploy/validate-fresh-vps.sh` (testar dentro de Docker Ubuntu 22.04 ou Vagrant) mede tempo total; output `INSTALL_DURATION_SECONDS=N` com N<900 |

## 3. Prova surfaceável

Comandos rodados a cada turno (echo full output):

```bash
go test ./... 2>&1 | tail -3
go vet ./... 2>&1 | tail -3
bd list --status=open --label=phase-4 2>&1 | tail -5
git log --oneline -15
ls deploy/ 2>&1 | head
bash -n deploy/install.sh && echo "install.sh syntax OK"
bash -n deploy/upgrade.sh && echo "upgrade.sh syntax OK"
bash -n deploy/postinstall-check.sh && echo "postinstall-check.sh syntax OK"
wc -l docs/INSTALL.md
```

Output esperado ao concluir:

- `go test ./...`: `ok` em todos pacotes, ≥177 passed (sem regressão)
- `go vet`: zero output
- `bd list --status=open --label=phase-4 | tail`: só epic `chatwoot-megaapi-bridge-37j` ainda open
- `git log --oneline -15`: 12 novos commits prefixados `feat(deploy):`, `feat(installer):`, `docs(install):`
- `ls deploy/`: lista contém `install.sh`, `upgrade.sh`, `postinstall-check.sh`, `init.sql`, `templates/`, `Caddyfile`, `chatwoot.docker-compose.yml`
- `bash -n` em scripts: `syntax OK` para cada
- `wc -l docs/INSTALL.md`: número ≥200
- Validação fresh-VPS: ver seção 12

Frequência: a cada turno após implementação de cada sub-issue.

## 4. Restrições

### Específicas do projeto

- NÃO modificar código Go bridge core (`internal/bridge/**`, `cmd/bridge/**`) exceto se nova subcommand CLI for necessário (ex: `bridge migrate` já existe; `bridge admin add` se for adicionar)
- NÃO modificar `docker-compose.yml` raiz exceto se necessário pra incluir backup sidecar (alterar pra incluir Caddy/cloudflared dentro do compose se desejar)
- NÃO criar novos pacotes Go fora dos existentes
- Scripts shell em `deploy/` — bash 5+ Ubuntu 22.04 (não posix sh, pode usar arrays + [[ ]])
- Scripts shell devem usar `set -euo pipefail` no topo
- Templates `.tpl` em `deploy/templates/`, renderizados via `envsubst` ou `sed` (não Helm/Jinja)
- Secrets gerados via `openssl rand -base64 32` (não /dev/urandom direto)
- install.sh DEVE ser idempotente: rerun deve detectar estado existente e atualizar sem destruir
- Caddyfile parametrizado com `{{DOMAIN}}` + `{{EMAIL}}` placeholders
- Modo dual: `--tls` (Caddy TLS automático) OU `--tunnel` (cloudflared) — escolha no prompt
- Backup sidecar usa imagem `prodrigestivill/postgres-backup-local`; configurar BACKUP_KEEP_DAYS=14
- Validação <15min: medir do `bash install.sh` até /healthz responder 200
- Cada sub-issue concluída → `bd close <id>` antes da próxima
- Cada commit: 1 sub-issue (1 PR atômico)
- INSTALL.md em `docs/INSTALL.md` (não na raiz)

### Padrão

- NÃO `--no-verify`
- NÃO commitar segredos (secrets ficam em `.env.local` no .gitignore)
- NÃO force-push
- NÃO modificar `go.sum`
- Mensagens commit descritivas (`feat(deploy):`, `feat(installer):`, `docs(install):`, `test(install):`)

## 5. Bound

- **80 turnos** OU **240 minutos** (o que vier primeiro)
- Justificativa: 12 sub-issues, maioria scripts shell (sem TDD pesado). Validação VPS limpa é o caro (precisa container Ubuntu provisionar).

## 6. Modo de execução recomendado

- Auto mode: ligar (`Shift+Tab` até "acceptEdits")
- Headless opcional: `claude -p "/goal <condição>"` para CI/cron

## 7. Condição final (cole no /goal)

```
F4 instalador 1-comando completo: 12 sub-issues do épico chatwoot-megaapi-bridge-37j implementadas. Estado final por sub-issue: (1) 37j.2 templates deploy/templates/.env.bridge.tpl + .env.chatwoot.tpl + Caddyfile.tpl com placeholders DOMAIN/MASTER_KEY/POSTGRES_PASSWORD/EMAIL; (2) 37j.3 Caddyfile rendering via envsubst ou sed substituindo DOMAIN+EMAIL; (3) 37j.4 deploy/init.sql cria DBs chatwoot e bridge + usuarios + grants; (4) 37j.5 install.sh executa docker compose run rails bundle exec rails db:chatwoot_prepare; (5) 37j.6 install.sh executa docker compose exec bridge /bridge migrate; (6) 37j.7 install.sh prompt --tls ou --tunnel; modo cloudflared baixa cloudflared, autentica, gera token; modo tls usa Caddy automatic HTTPS; (7) 37j.8 container postgres-backup-local adicionado ao docker-compose com BACKUP_KEEP_DAYS=14 volume /backups; (8) 37j.9 deploy/upgrade.sh faz git pull + docker compose pull + up -d --build + migrate sem perder dados; (9) 37j.1 deploy/install.sh interativo orquestra prompts (domain, email, tunnel|tls), gera secrets via openssl rand -base64 32, baixa imagens, sobe stack, valida health, idempotente; (10) 37j.10 deploy/postinstall-check.sh checklist: containers up, /healthz 200, Chatwoot UI 200, migrations aplicadas, backup ativo, Caddy TLS válido se modo tls; (11) 37j.11 docs/INSTALL.md ≥200 linhas com pré-requisitos, fluxo, troubleshooting; (12) 37j.12 deploy/validate-fresh-vps.sh roda install.sh dentro de container Ubuntu 22.04 limpo, mede tempo, valida health, output INSTALL_DURATION_SECONDS=N com N<900. Provar com: `go test ./... 2>&1 | tail -3` mostrando ok ≥177 passed (sem regressão); `go vet ./... 2>&1` zero output; `bd list --status=open --label=phase-4 2>&1 | tail -5` mostrando só epic 37j open; `git log --oneline -15` com 12 novos commits feat(deploy)/feat(installer)/docs(install)/test(install); `ls deploy/` contendo install.sh upgrade.sh postinstall-check.sh init.sql templates/ Caddyfile validate-fresh-vps.sh; `bash -n deploy/install.sh && echo OK` retorna OK; `bash -n deploy/upgrade.sh && echo OK` retorna OK; `bash -n deploy/postinstall-check.sh && echo OK` retorna OK; `wc -l docs/INSTALL.md` mostrando valor ≥200. Restrições: NÃO modificar internal/bridge/** ou cmd/bridge/** exceto novo subcommand CLI necessário; NÃO criar pacotes Go novos; scripts bash 5+ Ubuntu 22.04 com set -euo pipefail; templates .tpl em deploy/templates/ via envsubst ou sed; secrets via openssl rand -base64 32; install.sh idempotente (rerun detecta estado existente); Caddyfile com placeholders DOMAIN+EMAIL; modo dual --tls ou --tunnel; backup sidecar prodrigestivill/postgres-backup-local com BACKUP_KEEP_DAYS=14; validação <15min do bash install.sh até /healthz 200; cada sub-issue roda `bd close <id>` antes da próxima; 1 commit por sub-issue; INSTALL.md em docs/INSTALL.md; sem --no-verify; sem commitar segredos (.env.local em .gitignore); sem force-push; sem modificar go.sum; mensagens commit descritivas feat(deploy)/feat(installer)/docs(install)/test(install), or stop after 80 turns or 240m. Report turn count, sub-issue atual (1-12), bd open count, syntax checks status, remaining bound each turn. Claude must echo full output of each verification command.
```

## 8. Comando completo

```
/goal F4 instalador 1-comando completo: 12 sub-issues do épico chatwoot-megaapi-bridge-37j implementadas. Estado final por sub-issue: (1) 37j.2 templates deploy/templates/.env.bridge.tpl + .env.chatwoot.tpl + Caddyfile.tpl com placeholders DOMAIN/MASTER_KEY/POSTGRES_PASSWORD/EMAIL; (2) 37j.3 Caddyfile rendering via envsubst ou sed substituindo DOMAIN+EMAIL; (3) 37j.4 deploy/init.sql cria DBs chatwoot e bridge + usuarios + grants; (4) 37j.5 install.sh executa docker compose run rails bundle exec rails db:chatwoot_prepare; (5) 37j.6 install.sh executa docker compose exec bridge /bridge migrate; (6) 37j.7 install.sh prompt --tls ou --tunnel; modo cloudflared baixa cloudflared, autentica, gera token; modo tls usa Caddy automatic HTTPS; (7) 37j.8 container postgres-backup-local adicionado ao docker-compose com BACKUP_KEEP_DAYS=14 volume /backups; (8) 37j.9 deploy/upgrade.sh faz git pull + docker compose pull + up -d --build + migrate sem perder dados; (9) 37j.1 deploy/install.sh interativo orquestra prompts (domain, email, tunnel|tls), gera secrets via openssl rand -base64 32, baixa imagens, sobe stack, valida health, idempotente; (10) 37j.10 deploy/postinstall-check.sh checklist: containers up, /healthz 200, Chatwoot UI 200, migrations aplicadas, backup ativo, Caddy TLS válido se modo tls; (11) 37j.11 docs/INSTALL.md ≥200 linhas com pré-requisitos, fluxo, troubleshooting; (12) 37j.12 deploy/validate-fresh-vps.sh roda install.sh dentro de container Ubuntu 22.04 limpo, mede tempo, valida health, output INSTALL_DURATION_SECONDS=N com N<900. Provar com: `go test ./... 2>&1 | tail -3` mostrando ok ≥177 passed (sem regressão); `go vet ./... 2>&1` zero output; `bd list --status=open --label=phase-4 2>&1 | tail -5` mostrando só epic 37j open; `git log --oneline -15` com 12 novos commits feat(deploy)/feat(installer)/docs(install)/test(install); `ls deploy/` contendo install.sh upgrade.sh postinstall-check.sh init.sql templates/ Caddyfile validate-fresh-vps.sh; `bash -n deploy/install.sh && echo OK` retorna OK; `bash -n deploy/upgrade.sh && echo OK` retorna OK; `bash -n deploy/postinstall-check.sh && echo OK` retorna OK; `wc -l docs/INSTALL.md` mostrando valor ≥200. Restrições: NÃO modificar internal/bridge/** ou cmd/bridge/** exceto novo subcommand CLI necessário; NÃO criar pacotes Go novos; scripts bash 5+ Ubuntu 22.04 com set -euo pipefail; templates .tpl em deploy/templates/ via envsubst ou sed; secrets via openssl rand -base64 32; install.sh idempotente (rerun detecta estado existente); Caddyfile com placeholders DOMAIN+EMAIL; modo dual --tls ou --tunnel; backup sidecar prodrigestivill/postgres-backup-local com BACKUP_KEEP_DAYS=14; validação <15min do bash install.sh até /healthz 200; cada sub-issue roda `bd close <id>` antes da próxima; 1 commit por sub-issue; INSTALL.md em docs/INSTALL.md; sem --no-verify; sem commitar segredos (.env.local em .gitignore); sem force-push; sem modificar go.sum; mensagens commit descritivas feat(deploy)/feat(installer)/docs(install)/test(install), or stop after 80 turns or 240m. Report turn count, sub-issue atual (1-12), bd open count, syntax checks status, remaining bound each turn. Claude must echo full output of each verification command.
```

## 9. Comando headless (opcional)

```
claude -p "/goal F4 instalador 1-comando completo: ... (mesma condição) ..."
```

## 10. Checklist pré-entrega

- [x] ≤4000 chars (condição ~3950 chars)
- [x] Comandos concretos (go test, go vet, bd list, git log, ls, bash -n, wc -l)
- [x] Output literal definido (ok, ≥177 passed, OK, ≥200, etc.)
- [x] Restrições específicas + padrão
- [x] Bound presente (80 turns OR 240m)
- [x] Echo obrigatório
- [x] Arquivo salvo em `docs/goals/2026-05-24-complete-f4-installer.md`
- [x] Slug segue convenção (`complete-<area>`)
- [x] Estado final falsificável

## 11. Ordem sugerida das 12 sub-issues

| # | bd | Razão da ordem |
|---|---|---|
| 1 | 37j.2 | Templates env — pré-req de tudo |
| 2 | 37j.4 | init.sql DB — pré-req chatwoot/bridge bootstrap |
| 3 | 37j.3 | Caddyfile parametrizado — depende templates |
| 4 | 37j.8 | Backup sidecar — alteração compose isolada |
| 5 | 37j.5 | Bootstrap chatwoot — depende init.sql |
| 6 | 37j.6 | Bootstrap bridge — depende init.sql |
| 7 | 37j.7 | Cloudflare Tunnel suporte — branch alt do Caddy |
| 8 | 37j.1 | install.sh orchestrator — depende tudo acima |
| 9 | 37j.9 | upgrade.sh — derivado do install.sh |
| 10 | 37j.10 | postinstall-check.sh — valida install |
| 11 | 37j.12 | validate-fresh-vps.sh — exercício completo |
| 12 | 37j.11 | INSTALL.md — última doc consolidada |

## 12. Lembretes operacionais

- Validação fresh-VPS sem VPS real: usar container Docker `ubuntu:22.04` com docker-in-docker (DinD) ou rootless Podman; OU script Vagrant local; OU GitHub Actions Ubuntu runner
- `bash -n` valida sintaxe sem executar
- `shellcheck` opcional (não exigido na restrição, mas recomendado)
- envsubst vem em `gettext-base` (Ubuntu já tem)
- cloudflared install: `wget` + `dpkg -i`
- postgres-backup-local docs: https://github.com/prodrigestivill/docker-postgres-backup-local
- Caddy automatic HTTPS funciona se DNS A record aponta pra VPS pré-install (validar com `dig` antes de subir)
- gh auth pode precisar switch para `giovani-junior-dev` antes do push
- Bridge container precisa rebuild se cmd/bridge/main.go mudar (subcommand admin?): `docker compose up -d --build bridge`
