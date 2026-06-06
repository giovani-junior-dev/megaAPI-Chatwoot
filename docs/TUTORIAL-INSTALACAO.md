# Tutorial de Instalação — WhatsApp + Chatwoot (Bridge)

Guia passo a passo, do servidor zerado ao WhatsApp atendendo no Chatwoot.
Exemplo com **Hetzner Cloud**, mas funciona em qualquer VPS Ubuntu 24.04.

> **Para quem é:** mesmo sem ser técnico. Você cria a VPS, cola **um comando**,
> responde algumas perguntas e configura o roteamento na Cloudflare.

---

## Visão rápida (ordem)

1. Criar a VPS (Hetzner)
2. Criar o Tunnel na Cloudflare (pegar o token)
3. Rodar o instalador na VPS (1 comando)
4. Criar os 4 endereços públicos na Cloudflare
5. Configurar o Chatwoot (conta + caixa de entrada API)
6. Configurar o painel Bridge (Base URL + tenant)
7. Parear o WhatsApp
8. Verificar no Portainer (stacks e containers)

---

## 1. Criar a VPS na Hetzner

1. Acesse o [Hetzner Cloud Console](https://console.hetzner.cloud/) → **New Project** (ou use um existente).
2. **Add Server**:
   - **Location:** a mais próxima dos seus clientes (ex.: Ashburn/EUA ou Alemanha).
   - **Image:** **Ubuntu 24.04**.
   - **Type:** mínimo recomendado **CPX21** (2 vCPU / 4 GB RAM) — Chatwoot + bridge confortáveis.
   - **Networking:** IPv4 público habilitado.
   - **SSH Key:** adicione sua chave (recomendado) ou use senha por e-mail.
3. **Create & Buy now**. Anote o **IP público** do servidor.

### Acessar por SSH
No seu computador:
```bash
ssh root@SEU_IP
```

---

## 2. Criar o Tunnel na Cloudflare

> O Tunnel dá HTTPS automático **sem abrir portas** no servidor e **sem mexer em
> DNS manualmente**.

1. Abra o [Cloudflare Zero Trust](https://one.dash.cloudflare.com/) → **Networks → Tunnels → Create a tunnel**.
2. Tipo: **Cloudflared** → dê um nome (ex.: `atendimento`) → **Save**.
3. Na tela "Install connector", **copie o token** — o texto longo depois de
   `--token`, começa com `eyJ...`. **Guarde**, você vai colar no instalador.
4. **Não feche** — os endereços públicos serão criados no passo 4.

---

## 3. Rodar o instalador na VPS

Na VPS (como root), cole:

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/giovani-junior-dev/megaAPI-Chatwoot/master/install-pack/bootstrap.sh)
```

O instalador vai **perguntar** (responda com calma — digite os domínios com atenção):

| Pergunta | Exemplo | Observação |
|----------|---------|------------|
| Hostname do Chatwoot | `chatwoot.suaempresa.com` | **1 nível de subdomínio** |
| Hostname do Bridge | `bridge.suaempresa.com` | painel de administração |
| Hostname do Portainer | `portainer.suaempresa.com` | gerenciador de containers |
| Hostname do Backup | `backup.suaempresa.com` | painel de backups |
| Token do Tunnel | `eyJ...` | do passo 2 |
| E-mail do admin (Bridge) | `voce@suaempresa.com` | login do painel Bridge |
| Senha do admin (Bridge) | (sua senha) | login do painel Bridge |
| Senha do admin (Portainer) | (mín. **12 caracteres**) | login do Portainer |

> ⚠️ **Erros comuns a evitar:**
> - **Digite o domínio sem typos** (ex.: `chatwoot`, não `chtwoot`). Um erro
>   aqui quebra os links de mídia.
> - Use **subdomínio de 1 nível** (`chatwoot.suaempresa.com`). Dois níveis
>   (`chat.chatwoot.suaempresa.com`) quebram o HTTPS gratuito da Cloudflare.

O resto (senhas de banco, chaves de criptografia) é **gerado automaticamente**.
O instalador sobe tudo, prepara o banco do Chatwoot e cria os administradores.
Leva alguns minutos (a primeira vez baixa as imagens). Ao final, ele mostra um
resumo com os endereços e lembra dos endereços públicos a criar.

---

## 4. Criar os 4 endereços públicos na Cloudflare

Volte ao Tunnel → aba **Public Hostnames** → **Add a public hostname** (faça 4×).
Em todos, o **Type** é **HTTP** (o HTTPS é entregue pela Cloudflare):

| Subdomain | Domain | Type | URL |
|-----------|--------|------|-----|
| `chatwoot` | suaempresa.com | HTTP | `chatwoot_admin:3000` |
| `bridge` | suaempresa.com | HTTP | `bridge:8080` |
| `portainer` | suaempresa.com | HTTP | `portainer:9000` |
| `backup` | suaempresa.com | HTTP | `pgbackupweb:8085` |

> Os subdomínios precisam bater **exatamente** com os hostnames informados no
> instalador. Após salvar, o Tunnel deve aparecer como **HEALTHY**.

---

## 5. Configurar o Chatwoot

1. Acesse `https://chatwoot.suaempresa.com` → **crie a conta de administrador**.
2. Crie a **caixa de entrada do tipo API**:
   - Menu → **Caixas de entrada** → **Adicionar caixa de entrada** → **API**.
   - Dê um nome (ex.: `WhatsApp`) → criar.
   - **Anote o Inbox ID** (aparece na URL da caixa de entrada).
3. Pegue os dados que o Bridge vai pedir:
   - **Account ID:** está na URL `/app/accounts/<ID>`.
   - **Token de acesso:** Perfil → Configurações do perfil → **Token de Acesso**.

> ⚠️ **Crie a caixa de entrada ANTES de criar o tenant no Bridge.** Se inverter,
> o Bridge avisa com uma mensagem clara e não cria nada incompleto.

---

## 6. Configurar o painel Bridge

1. Acesse `https://bridge.suaempresa.com` → entre com o **e-mail e senha** do install.
2. Vá em **Configurações** → preencha a **Base URL** = `https://bridge.suaempresa.com` → salvar.
3. Vá em **Novo tenant** e preencha:
   - **Slug:** identificador curto, minúsculo, com hífen (ex.: `empresa-x`). Sem espaços.
   - **megaAPI:** host, instância e token da sua instância.
   - **Chatwoot:** URL (`https://chatwoot.suaempresa.com`), token de acesso, Account ID e Inbox ID.
4. Salvar. O Bridge configura o webhook do Chatwoot automaticamente (com token de segurança).

---

## 7. Parear o WhatsApp

1. No **Painel** do Bridge, na linha do tenant, clique em **Gerar link de pareamento**.
2. Abra o link → escaneie o **QR Code** com o WhatsApp do celular (ou use o código).
3. Quando o status mudar para conectado, envie uma mensagem de teste:
   - WhatsApp → Chatwoot (recebida)
   - Responda no Chatwoot → WhatsApp (enviada), com **texto e arquivo**.

---

## 8. Verificar no Portainer (saúde da instalação)

1. Acesse `https://portainer.suaempresa.com` → entre com **admin** + a senha do install
   (sem tela de "timeout" — o admin já vem criado).
2. Selecione o ambiente **swarm-local** → menu **Stacks**.
3. Confira:
   - **As stacks NÃO devem estar "Limited".** Devem aparecer como
     gerenciáveis (editáveis pela interface): `postgres`, `cloudflared`,
     `chatwoot`, `bridge`, `pgbackupweb`.
   - Em **Services / Containers**, todos devem estar **running (verde)**:
     `chatwoot_admin`, `chatwoot_api`, `chatwoot_sidekiq`, `chatwoot_redis`,
     `postgres`, `cloudflared`, `bridge`, `pgbackupweb`, `portainer`, `agent`.

> Se alguma stack aparecer como **Limited** ou faltar o ambiente
> **swarm-local**, o registro do ambiente não concluiu — aguarde 1–2 min e
> recarregue; o instalador registra o ambiente assim que o agente sobe.

---

## Referência — o que cada página do painel Bridge faz

> Use esta seção para a página de documentação na landing page.

| Página | Rota | O que faz |
|--------|------|-----------|
| **Login** | `/login` | Autenticação do administrador do painel (e-mail + senha). |
| **Painel** | `/` | Lista todos os **tenants**: slug, mensagens nas últimas 24h, status de pareamento (número conectado ou "não pareado") e ações rápidas (gerar link de pareamento, diagnóstico). É a tela inicial. |
| **Novo tenant** | `/tenants/new` | Assistente em 4 passos para **cadastrar um tenant**: (1) identificação/slug, (2) dados da megaAPI, (3) dados do Chatwoot (URL, token, Account ID, Inbox ID), (4) revisão. Configura o webhook do Chatwoot automaticamente. |
| **Diagnóstico** | `/tenants/{slug}/diag` | Verifica a **saúde do tenant** — conectividade e configuração — para apontar problemas rapidamente. |
| **Mensagens** | `/messages` | Histórico de **mensagens** por tenant, com filtro por tenant e paginação. Útil para auditar o tráfego (recebidas e enviadas). |
| **DLQ** | `/dlq` | Fila de **mensagens que falharam** (Dead Letter Queue), com botão de **reenviar (retry)** por mensagem. |
| **Configurações** | `/settings` | Define a **Base URL pública** do Bridge (usada para registrar os webhooks). Configuração obrigatória antes de criar tenants. |
| **Pareamento** | `/pair/{slug}` | Página (acessível por link) para **conectar o WhatsApp** do tenant: exibe QR Code e código de pareamento e mostra o status da conexão em tempo real. |

---

## Resolvendo problemas

| Sintoma | Causa provável / solução |
|---------|--------------------------|
| Tunnel **DOWN** | Token errado no install. Refaça o passo do token e reinstale o connector. |
| Página não abre / erro de SSL | Subdomínio de 2 níveis. Use **1 nível** (`chatwoot.suaempresa.com`). |
| **Imagem/vídeo "não disponível"** no Chatwoot | Hostname do Chatwoot digitado errado no install (ex.: `chtwoot`). Corrija o `FRONTEND_URL` para o domínio certo e reinicie o Chatwoot. Texto funciona, mídia não = quase sempre é isso. |
| Envio do Chatwoot dá "Falha ao enviar" | A caixa de entrada foi criada **depois** do tenant. Recrie o tenant com a caixa de entrada já existente. |
| Stacks aparecem como **Limited** | O ambiente do Portainer não registrou no momento certo. Aguarde 1–2 min e recarregue; em instalações novas o registro é automático. |
| Mensagem do WhatsApp não chega (de grupo) | Mensagens de **grupos/canais** não são atendimentos 1:1 e podem ser ignoradas. Use conversas diretas para testar. |

---

## Acessos finais

| Ferramenta | URL | Login |
|-----------|-----|-------|
| Chatwoot | `https://chatwoot.suaempresa.com` | conta criada no passo 5 |
| Bridge (painel) | `https://bridge.suaempresa.com` | e-mail + senha do install |
| Portainer | `https://portainer.suaempresa.com` | `admin` + senha do install |
| Backup | `https://backup.suaempresa.com` | admin do pgbackweb |
