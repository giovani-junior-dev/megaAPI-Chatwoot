# 🚀 Instalação do zero — Chatwoot + WhatsApp (bridge)

Guia passo a passo para subir **tudo** numa VPS Ubuntu usando Docker Swarm +
Portainer, com **Cloudflare Tunnel** dando o HTTPS (sem mexer em DNS A-record,
sem abrir portas, sem certificado manual).

> **Para quem é:** pessoa não-técnica que tem uma VPS e um domínio na
> Cloudflare.

---

## ⚡ Instalação automática (recomendada)

Um comando na VPS (Ubuntu, root). Ele pergunta só **domínio, token do tunnel
e e-mail/senha do admin**, gera o resto dos segredos sozinho e sobe tudo:

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/giovani-junior-dev/megaAPI-Chatwoot/master/install-pack/bootstrap.sh)
```

Ou clonando o repo:

```bash
git clone https://github.com/giovani-junior-dev/megaAPI-Chatwoot.git
bash megaAPI-Chatwoot/install-pack/setup.sh
```

Quando terminar, **falta só** criar os 2 Public Hostnames no Cloudflare Zero
Trust (passo 5 abaixo): `chat.SEU_DOMINIO → chatwoot_admin:3000` e
`bridge.SEU_DOMINIO → bridge:8080`.

> Prefere ser guiado por uma IA? Veja [`INSTALL-AGENT.md`](INSTALL-AGENT.md):
> cole o prompt no Claude Code na VPS e responda as perguntas.

O passo a passo manual abaixo (colar stacks no Portainer) continua válido como
**alternativa avançada**, caso queira controlar cada etapa.

---

## 📋 O que você precisa antes de começar

1. **Uma VPS** (servidor) Ubuntu, com acesso root por SSH. Mínimo recomendado:
   2 vCPU / 4 GB de RAM.
2. **Um domínio** já adicionado na sua conta **Cloudflare** (ex.:
   `minhaempresa.com.br`).
3. **Um computador** com terminal (para gerar os segredos e acessar a VPS).

Vamos usar **2 subdomínios** (criados no painel, passo 5):

| Subdomínio          | Para quê                       |
|---------------------|--------------------------------|
| `chat.SEU_DOMINIO`  | painel do Chatwoot (atendentes)|
| `bridge.SEU_DOMINIO`| painel de admin do bridge      |

---

## 🔑 Antes de tudo: gere seus segredos

No seu computador (Linux/Mac) ou na própria VPS, rode estes comandos e
**guarde cada saída** no arquivo [`.env.modelo`](.env.modelo):

```bash
# MASTER_KEY (login do painel do bridge)
openssl rand -base64 32

# BRIDGE_ENCRYPTION_KEY (cifra os segredos — NUNCA troque depois)
openssl rand -base64 32

# SECRET_KEY_BASE (Chatwoot)
openssl rand -hex 32

# PBW_ENCRYPTION_KEY (chave do backup)
openssl rand -base64 16
```

E **escolha** (digitando você mesmo) duas senhas fortes:
- `SENHA_POSTGRES` — senha do banco de dados.
- `SENHA_BRIDGE` — senha do usuário `bridge` no banco.

> 💡 Abra o `.env.modelo`, cole cada valor no campo certo, e mantenha esse
> arquivo **em local seguro** (gerenciador de senhas). Você vai copiar esses
> valores para dentro das stacks no Portainer.

---

## Passo 1 — Preparar o servidor (infra)

Conecte na VPS por SSH e rode (como root):

```bash
# Instalar Docker (pule se já tiver)
apt-get update && apt-get install -y docker.io docker-compose-plugin
systemctl enable --now docker

# Subir o Swarm + criar a rede (use o script da pasta 01-infra)
bash 01-infra/setup-swarm.sh
```

✅ Quando aparecer **"Swarm ativo + rede network_swarm_public criada"**, siga.

Detalhes: [`01-infra/LEIA.md`](01-infra/LEIA.md).

---

## Passo 2 — Subir o Portainer (o painel)

O Portainer é a tela web onde você cola as próximas stacks.

```bash
docker stack deploy -c 02-portainer/portainer.yaml portainer
```

Acesse o Portainer pela primeira vez (escolha **uma** opção):

- **Opção A (rápida, pra configuração inicial):** crie um túnel temporário
  só pra você:
  ```bash
  ssh -L 9000:localhost:9000 root@IP_DA_SUA_VPS
  ```
  Depois abra `http://localhost:9000` no navegador.
- **Opção B (SSH simples):** se o firewall permitir, abra a porta 9000 só
  pro seu IP. (Este pack **não** abre portas por padrão — preferimos o túnel.)

Crie o usuário **admin** do Portainer e **escolha uma senha forte**.

> A partir daqui, todas as stacks podem ser coladas pelo Portainer em
> **Stacks → Add stack → Web editor**. Cole o conteúdo do `.yaml`, troque os
> campos MAIÚSCULOS, e clique **Deploy the stack**.

---

## Passo 3 — Conferir seus valores (.env.modelo)

Abra o [`.env.modelo`](.env.modelo) e confira que todos os campos estão
preenchidos: `DOMINIO`, `SENHA_POSTGRES`, `CHAVE_SECRETA_64`, `MASTER_KEY`,
`BRIDGE_ENCRYPTION_KEY`, `SENHA_BRIDGE`, `TOKEN_DO_TUNNEL`,
`PBW_ENCRYPTION_KEY_20`.

Você vai usar esses valores nos passos seguintes.

---

## Passo 4 — Subir o Postgres (banco de dados)

No Portainer: **Stacks → Add stack**, nome `postgres`, cole o conteúdo de
[`03-postgres/postgres.yaml`](03-postgres/postgres.yaml).

**Troque** `SENHA_POSTGRES` pela senha que você definiu. Deploy.

✅ O serviço `postgres` precisa ficar **running** (verde) no Portainer.

---

## Passo 5 — Cloudflare Tunnel (o HTTPS de tudo)

Aqui criamos o túnel e os endereços públicos. **Não abrimos portas 80/443.**

1. Acesse **[Cloudflare Zero Trust](https://one.dash.cloudflare.com/)** →
   **Networks → Tunnels → Create a tunnel** → tipo **Cloudflared**.
2. Dê um nome (ex.: `chatwoot-bridge`) e clique **Save**.
3. Na tela "Install connector", **copie o token** (o texto comprido depois de
   `--token`, começa com `eyJ...`). Guarde como `TOKEN_DO_TUNNEL`.
4. Ainda **não** feche — vá em **Public Hostnames → Add a public hostname** e
   crie **dois**:

   | Subdomain | Domain         | Path | Service (Type / URL)          |
   |-----------|----------------|------|-------------------------------|
   | `chat`    | SEU_DOMINIO    | —    | HTTP → `chatwoot_admin:3000`  |
   | `bridge`  | SEU_DOMINIO    | —    | HTTP → `bridge:8080`          |

   > O **Type** é `HTTP` (não HTTPS) — o tráfego interno entre o tunnel e os
   > containers é HTTP; a Cloudflare entrega HTTPS pro mundo.

5. Agora suba o conector no servidor. No Portainer: **Add stack**, nome
   `cloudflared`, cole [`04-cloudflared/cloudflared.yaml`](04-cloudflared/cloudflared.yaml),
   **troque** `TOKEN_DO_TUNNEL` pelo seu token. Deploy.

✅ No painel da Cloudflare o tunnel deve aparecer **HEALTHY**.

---

## Passo 6 — Subir o Chatwoot

No Portainer: **Add stack**, nome `chatwoot`, cole
[`05-chatwoot/chatwoot.yaml`](05-chatwoot/chatwoot.yaml).

**Troque**:
- `SENHA_POSTGRES` → a mesma senha do passo 4.
- `CHAVE_SECRETA_64` → seu `SECRET_KEY_BASE`.
- `DOMINIO` → seu domínio (no campo `FRONTEND_URL: https://chat.DOMINIO`).

Deploy. Espere os serviços `chatwoot_admin`, `chatwoot_api`,
`chatwoot_sidekiq` e `redis` ficarem **running**.

**Primeira vez — preparar o banco do Chatwoot:** no Portainer, abra o
**console** do container `chatwoot_admin` (Containers → ... → `>_ Console` →
`/bin/bash`) e rode:

```bash
bundle exec rails db:chatwoot_prepare
```

✅ Acesse `https://chat.SEU_DOMINIO` e crie a conta de administrador do
Chatwoot.

> 📎 **Anexos ficam no servidor** (`ACTIVE_STORAGE_SERVICE=local`, volume
> `chatwoot_storage`). Sem Minio/S3 — mais simples, sem serviço extra.

---

## Passo 7 — Subir o bridge

No Portainer: **Add stack**, nome `bridge`, cole
[`06-bridge-admin/bridge.yaml`](06-bridge-admin/bridge.yaml).

**Troque**:
- `MASTER_KEY` → seu valor (login do painel).
- `BRIDGE_ENCRYPTION_KEY` → seu valor.
- `SENHA_BRIDGE` → senha do role `bridge`.
- `SENHA_POSTGRES` → senha do superusuário (em `POSTGRES_ADMIN_URL`).

Deploy. Na **primeira** subida o bridge cria sozinho o role+database `bridge`
e roda as migrations. Depois de subir OK, você **pode** remover a linha
`POSTGRES_ADMIN_URL` e dar **Update the stack**.

✅ Acesse `https://bridge.SEU_DOMINIO` e entre com a `MASTER_KEY`.

Detalhes: [`06-bridge-admin/LEIA.md`](06-bridge-admin/LEIA.md).

---

## Passo 8 — Backup + teste final (smoke)

**Backup** (opcional mas recomendado). No console do container `postgres`,
crie o banco interno do backup uma vez:

```bash
psql -U postgres -c "CREATE DATABASE pgbackweb;"
```

Depois suba a stack: **Add stack**, nome `pgbackupweb`, cole
[`07-backup/pgbackupweb.yaml`](07-backup/pgbackupweb.yaml), **troque**
`PBW_ENCRYPTION_KEY_20` e `SENHA_POSTGRES`. Deploy. No painel do pgbackweb,
crie jobs para os bancos `chatwoot` e `bridge` com retenção
**14 dias / 4 semanas / 6 meses**.

**Teste final (smoke):**
- [ ] `https://chat.SEU_DOMINIO` abre e loga no Chatwoot.
- [ ] `https://bridge.SEU_DOMINIO` abre e loga com a `MASTER_KEY`.
- [ ] No painel do bridge, cadastre um tenant ligando a inbox do Chatwoot à
      sua instância megaAPI e envie uma mensagem de teste no WhatsApp.

🎉 Pronto! Tudo no ar via Cloudflare Tunnel.

---

## 🧯 Problemas comuns (troubleshooting)

| Sintoma | Causa provável / solução |
|---------|--------------------------|
| Tunnel fica **DOWN** no painel | Token errado no `cloudflared.yaml`. Confira o `TOKEN_DO_TUNNEL` e dê **Update the stack**. |
| `chat.DOMINIO` dá 502/Bad Gateway | Chatwoot ainda subindo, ou o **Public Hostname** aponta pra URL errada. Deve ser `HTTP → chatwoot_admin:3000`. |
| `bridge.DOMINIO` não abre | Ingress deve ser `HTTP → bridge:8080`. Confira o serviço `bridge` **running**. |
| Chatwoot reclama de banco | Faltou rodar `bundle exec rails db:chatwoot_prepare` no console do `chatwoot_admin` (passo 6). |
| bridge não conecta no banco | `SENHA_BRIDGE`/`SENHA_POSTGRES` não batem com o `03-postgres`, ou o Postgres não está **running**. |
| bridge não criou o database | Faltou `POSTGRES_ADMIN_URL` na **primeira** subida. Adicione e dê Update. |
| Anexo não carrega no Chatwoot | `FRONTEND_URL` precisa ser `https://chat.SEU_DOMINIO`. Após mudar, reinicie `chatwoot_admin` e `chatwoot_sidekiq`. |
| Mudei senha do Postgres e quebrou | A senha tem que ser **igual** em 03-postgres, 05-chatwoot, 06-bridge e 07-backup. |
| Perdi a `MASTER_KEY` | É só o login do painel do bridge — troque na stack e dê Update. (Já a `BRIDGE_ENCRYPTION_KEY` **nunca** troque com dados reais.) |

---

## 📂 Mapa do pack

```
install-pack/
  00-LEIA-PRIMEIRO.md     <- este guia
  .env.modelo             <- cola dos seus valores/segredos
  01-infra/               <- swarm init + rede network_swarm_public
  02-portainer/           <- painel Portainer (sem Traefik)
  03-postgres/            <- Postgres pgvector pg16 (compartilhado)
  04-cloudflared/         <- Cloudflare Tunnel (HTTPS de tudo)
  05-chatwoot/            <- Chatwoot admin+api+sidekiq+redis (storage local)
  06-bridge-admin/        <- a aplicação bridge
  07-backup/              <- backups do Postgres (pgbackweb)
```

Ordem de instalação: **01 → 02 → (segredos) → 03 → 04 → 05 → 06 → 07**.
