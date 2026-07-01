# 03 — Integração Chatwoot (AgentBot)

Fonte: skill `chatwoot-api` + context7 `/websites/developers_chatwoot`.

## Por que AgentBot (recap)

- Webhook próprio (`outgoing_url`) + token próprio (`access_token`).
- Handoff nativo bot→humano via status de conversa.
- Não gasta assento de agente.
- Separa IA de humano no painel (bom pra revisão e escalação).

## Setup (provisionamento por empresa)

### 1. Criar o bot
```http
POST /api/v1/accounts/{account_id}/agent_bots
{
  "name": "SDR IA",
  "description": "Atendimento automatizado",
  "outgoing_url": "https://<sdr-agent-host>/webhook",
  "bot_type": 0            // 0 = webhook
}
```
Resposta inclui `id` e **`access_token`** (token do bot — guardar cifrado).

### 2. Grudar no inbox WhatsApp
```http
POST /api/v1/accounts/{account_id}/inboxes/{inbox_id}/set_agent_bot
{ "agent_bot": <bot_id> }
```
(pra remover: `{ "agent_bot": null }`)

### 3. Verificar
```http
GET /api/v1/accounts/{account_id}/inboxes/{inbox_id}/agent_bot
```

> Nota: em algumas versões, criar agent bot é via **Platform API**
> (`/platform/api/v1/agent_bots`) com token de platform app; em outras via
> Application API no account. **Validar na instância self-hosted do Giovani** qual
> caminho responde. Ver [09](09-open-questions-for-review.md) Q1.

## Evento recebido (no `outgoing_url`)

O bot recebe eventos de mensagem/conversa do inbox. O SDR só age em:
- `event = message_created`
- `message_type = incoming` (mensagem do cliente)
- `private = false`

Payload traz (entre outros): `id` (message id), `conversation.id`, `content`,
`sender`/contato, `message_type`. **Confirmar shape exato** capturando um evento
real na instância (Q1).

## Resposta / ações (Application API, token do bot)

```http
# responder ao cliente (sai pro WhatsApp via bridge)
POST /api/v1/accounts/{id}/conversations/{conv}/messages
{ "content": "...", "message_type": "outgoing", "private": false }

# nota interna (NÃO vai pro cliente; não dispara bridge pois private=true)
POST .../messages
{ "content": "motivo da escalação...", "message_type": "outgoing", "private": true }

# mudar status (handoff / fechar)
PATCH /api/v1/accounts/{id}/conversations/{conv}
{ "status": "open" }     // open | pending | resolved | snoozed

# assinar pra humano/time
POST /api/v1/accounts/{id}/conversations/{conv}/assignments
{ "assignee_id": 5, "team_id": 2 }

# labels (sobrescreve — sempre lista completa)
POST /api/v1/accounts/{id}/conversations/{conv}/labels
{ "labels": ["lead-quente", "orcamento"] }

# histórico da conversa
GET /api/v1/accounts/{id}/conversations/{conv}/messages
```

## Modelo de handoff (status)

- **`pending`** = bot é dono (atendendo). Novas conversas de inbox com bot podem
  começar `pending`.
- **`open`** = escalado pro humano; entra na fila dos agentes.
- **`resolved`** = fechado.

Escalar = `pending`→`open` + assign + label + **nota privada com o motivo**.
Fechar = `resolved` (com resumo).

> **Validar (Q2)**: qual o status inicial de uma conversa nova num inbox com
> AgentBot (`pending` automático?) e se o bot deve setar `pending` explicitamente
> ao começar a atender.

## Guard anti-loop (crítico)

O bot pode receber eventos das **próprias** mensagens `outgoing`. Se não filtrar,
ele responde a si mesmo → cascata infinita → flood no cliente. Regra dura:
**só processar `message_type=incoming`**. Testar isso explicitamente ([07](07-testing.md)).

## Coexistência com o bridge

- O `webhook_url` do **canal** (bridge) segue registrado e intocado — é ele que
  leva `outgoing` → WhatsApp.
- O `outgoing_url` do **bot** (SDR) é adicional.
- Ambos são hooks separados no Chatwoot. **Validar (Q1)** que os dois disparam.

## Provisionamento — build vs manual

Fase 1 pode provisionar o bot **manualmente** (rodar os POSTs uma vez por empresa)
e guardar `bot_id`+`access_token` na config. Automatizar o provisionamento
(script/CLI) é melhoria, não bloqueia. Ver [09](09-open-questions-for-review.md) Q3.
