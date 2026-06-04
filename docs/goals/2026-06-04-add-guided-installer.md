# Plano — Instalador guiado (setup.sh + prompt IA)

## 0. Contexto

- Pack atual `install-pack/` = paste manual no Portainer (placeholders MAIUSCULOS trocados à mão).
- Objetivo: reduzir fricção pro leigo, estilo SetupOrion (oriondesign): um comando instala tudo.
- Decisões (2026-06-04): construir **setup.sh + INSTALL-AGENT.md** com motor único; entrega via **curl one-liner**; converter placeholders para `${VAR}`.

## 1. Por que ${VAR}

`docker stack deploy -c f.yml` interpola `${VAR}` do ambiente nativamente
(igual compose). Logo:
- **setup.sh**: `set -a; source .env; set +a; docker stack deploy -c ...` → vars entram, zero sed/envsubst.
- **Portainer**: ao colar stack com `${VAR}`, o editor expõe campo de Environment variables.
- **Agente IA**: só preenche `.env` e chama setup.sh.

Um formato, três rostos. Sem duplicar lógica.

## 2. Entregáveis

### 2.1 Converter stacks para `${VAR}`
Em `03-postgres`, `04-cloudflared`, `05-chatwoot`, `06-bridge-admin`,
`07-backup`: trocar placeholders literais por `${...}`:
- `SENHA_POSTGRES` → `${SENHA_POSTGRES}`
- `CHAVE_SECRETA_64` → `${SECRET_KEY_BASE}`
- `chat.DOMINIO` → `chat.${DOMINIO}` (FRONTEND_URL)
- `MASTER_KEY` (valor) → `${MASTER_KEY}`; `BRIDGE_ENCRYPTION_KEY` → `${BRIDGE_ENCRYPTION_KEY}`
- `SENHA_BRIDGE` → `${SENHA_BRIDGE}`; `TOKEN_DO_TUNNEL` → `${TOKEN_DO_TUNNEL}`
- `PBW_ENCRYPTION_KEY_20` → `${PBW_ENCRYPTION_KEY}`
- Imagem do bridge permanece pinada (`:v1.1.1`), não vira var.

### 2.2 `install-pack/setup.sh`
Bash interativo, idempotente:
1. Detecta/instala docker + compose plugin; `swarm init` se inativo; cria `network_swarm_public` se ausente.
2. Pergunta: `DOMINIO`, `TOKEN_DO_TUNNEL`; **gera** segredos (openssl): MASTER_KEY, BRIDGE_ENCRYPTION_KEY (base64 32), SECRET_KEY_BASE (hex 32), PBW_ENCRYPTION_KEY (base64 16), SENHA_POSTGRES, SENHA_BRIDGE (base64 24). Pergunta email+senha do admin do bridge.
3. Grava `install-pack/.env` (chmod 600) com tudo.
4. `source .env` + `docker stack deploy` na ordem: portainer, postgres, (espera postgres healthy), cloudflared, chatwoot, bridge, backup.
5. Espera chatwoot_admin subir → `docker exec ... rails db:chatwoot_prepare`.
6. Espera bridge subir → `docker exec ... /bridge admin add --email <> --password <>`.
7. Imprime resumo: URLs (`https://chat.$DOMINIO`, `https://bridge.$DOMINIO`), lembrete de criar os 2 ingress no Cloudflare Zero Trust, e onde o `.env` foi salvo.
8. Reexecução: detecta serviços já existentes e dá `service update` em vez de falhar.

### 2.3 `install-pack/bootstrap.sh` (curl one-liner)
Mínimo: instala git se faltar, `git clone` o repo público em `/opt/megaapi-chatwoot` (ou pwd), `cd install-pack`, `exec bash setup.sh`. Documentar:
```
bash <(curl -fsSL https://raw.githubusercontent.com/giovani-junior-dev/megaAPI-Chatwoot/master/install-pack/bootstrap.sh)
```

### 2.4 `install-pack/INSTALL-AGENT.md`
Prompt pro agente IA (Claude Code/outro) na VPS. Estrutura:
- Papel: instalar o stack guiando o usuário.
- Passos: perguntar DOMINIO + TOKEN_DO_TUNNEL (e email/senha admin); rodar `setup.sh` (ou preencher `.env` e chamar); após deploy, instruir os 2 ingress Cloudflare; validar `docker service ls` 1/1 + smoke.
- Regra de segurança: NUNCA commitar `.env`; segredos só no servidor.

### 2.5 Atualizar `00-LEIA-PRIMEIRO.md`
Adicionar no topo o caminho **Instalação automática (recomendada)**:
- one-liner curl OU `git clone && bash install-pack/setup.sh`
- manter o passo a passo manual (Portainer paste) como **alternativa avançada**.

## 3. Provas (mensuráveis)

- `bash -n install-pack/setup.sh` exits 0; idem `bootstrap.sh`.
- (se disponível) `shellcheck install-pack/setup.sh` sem erro nível error.
- Render check: `set -a; source install-pack/.env.exemplo; set +a` e para cada stack `docker compose -f <stack> config` exits 0 (vars interpolam).
- `ls install-pack/setup.sh install-pack/bootstrap.sh install-pack/INSTALL-AGENT.md`.
- `grep -c '${SENHA_POSTGRES}' install-pack/03-postgres/postgres.yaml` >=1 (conversão feita).
- `grep -c 'setup.sh' install-pack/00-LEIA-PRIMEIRO.md` >=1.
- `go test ./...` continua verde (sem mudança Go, sanity).

## 4. Restrições

- Sem mudar código Go, rotas, handlers.
- Sem quebrar o caminho manual Portainer (stacks com `${VAR}` ainda coláveis — Portainer pede env vars).
- Sem hardcode de segredos; `.env` gerado é chmod 600 e fica no `.gitignore`.
- Sem abrir portas (tunnel-only mantido).
- Sem deps novas além de utilitários base (openssl, git, docker já assumidos).
- Sem `--no-verify`, sem force-push, commits `feat(deploy)/docs()`.

## 5. Bound

- 18 turns.

## 6. /goal pronto

```
Adicionar instalador guiado ao install-pack (setup.sh + bootstrap curl + prompt IA) estilo SetupOrion, sem mudar codigo Go. Estado final: (1) stacks 03-postgres/04-cloudflared/05-chatwoot/06-bridge-admin/07-backup com placeholders convertidos para ${VAR} (SENHA_POSTGRES->${SENHA_POSTGRES}, CHAVE_SECRETA_64->${SECRET_KEY_BASE}, chat.DOMINIO->chat.${DOMINIO}, MASTER_KEY/BRIDGE_ENCRYPTION_KEY/SENHA_BRIDGE/TOKEN_DO_TUNNEL/PBW_ENCRYPTION_KEY como ${...}); imagem do bridge segue pinada :v1.1.1; (2) install-pack/setup.sh bash interativo idempotente: detecta/instala docker, swarm init se inativo, cria network_swarm_public, pergunta DOMINIO e TOKEN_DO_TUNNEL e email/senha admin, gera os demais segredos com openssl, grava install-pack/.env com chmod 600, faz source .env e docker stack deploy na ordem portainer->postgres->(espera healthy)->cloudflared->chatwoot->bridge->backup, roda rails db:chatwoot_prepare no chatwoot_admin e /bridge admin add no bridge, imprime resumo com URLs e os 2 ingress Cloudflare a criar; reexecucao usa service update; (3) install-pack/bootstrap.sh que instala git, clona o repo publico e exec bash setup.sh (pro one-liner bash <(curl -fsSL .../install-pack/bootstrap.sh)); (4) install-pack/INSTALL-AGENT.md prompt para agente IA conduzir a instalacao reusando setup.sh, perguntando credenciais e validando docker service ls 1/1, com regra de nunca commitar .env; (5) install-pack/.env.exemplo com valores ficticios validos para teste de render; (6) 00-LEIA-PRIMEIRO.md ganha secao Instalacao automatica (recomendada) no topo com o one-liner + git clone, mantendo o paste manual como alternativa; (7) .gitignore inclui install-pack/.env. Provar com: `bash -n install-pack/setup.sh 2>&1 | tail -3` exits 0; `bash -n install-pack/bootstrap.sh 2>&1` exits 0; `set -a; . install-pack/.env.exemplo; set +a; for f in install-pack/0[3-7]*/*.yaml; do docker compose -f $f config >/dev/null && echo OK $f || echo FAIL $f; done` todos OK; `ls install-pack/setup.sh install-pack/bootstrap.sh install-pack/INSTALL-AGENT.md install-pack/.env.exemplo`; `grep -c "\${SENHA_POSTGRES}" install-pack/03-postgres/postgres.yaml` >=1; `grep -c "setup.sh" install-pack/00-LEIA-PRIMEIRO.md` >=1; `grep -c "install-pack/.env" .gitignore` >=1; `go test ./... 2>&1 | tail -5` mostrando ok sem --- FAIL. Sem mudar codigo Go nem rotas, sem quebrar paste manual no Portainer, sem hardcode de segredos (so .env.exemplo ficticio), sem abrir portas, sem adicionar deps Go, sem //nolint, sem --no-verify, sem modificar go.mod/go.sum, sem force-push, sem commit vago (usar feat(deploy)/docs()), sem commitar .env real nem segredos, or stop after 18 turns. Report turn count, provas passando, files alterados e remaining bound each turn. Claude must echo full output of each verification command.
```
