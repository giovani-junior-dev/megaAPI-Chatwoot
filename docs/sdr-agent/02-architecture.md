# 02 — Arquitetura

## Fluxo ponta-a-ponta (sem tocar no bridge)

```
Cliente WhatsApp
   │  (1) manda msg
   ▼
megaAPI ──► bridge (/v1/wa) ──► Chatwoot: cria msg incoming na conversa
                                     │
                                     │ (2) Chatwoot dispara AgentBot outgoing_url
                                     ▼
                              ┌──────────────────────┐
                              │      SDR AGENT        │
                              │  (este projeto, TS)   │
                              └──────────┬───────────┘
                                     (3) monta contexto + roda loop Claude Agent SDK
                                     (4) decide: responder / escalar / fechar
                                     ▼
                     Chatwoot API: POST message (outgoing, bot token)
                                     │
                                     │ (5) Chatwoot dispara webhook do canal
                                     ▼
                        bridge (/v1/cw) ──► megaAPI ──► WhatsApp (cliente recebe)
```

Ponto-chave: o inbox tem **dois hooks distintos** que coexistem:

- `webhook_url` do canal API → aponta pro **bridge** (`/v1/cw/{slug}`), já
  configurado. Relaya outgoing → WhatsApp.
- `outgoing_url` do **AgentBot** → aponta pro **SDR Agent**. Recebe eventos de
  mensagem.

Quando o SDR posta uma resposta `outgoing`, ela dispara o `webhook_url` do canal
(bridge) — que a manda pro WhatsApp. O SDR nunca fala com megaAPI direto.

> ⚠️ **Validar na implementação**: confirmar que Chatwoot dispara AMBOS os hooks
> pra mesma conversa/inbox (canal + agentbot). Ver [03](03-chatwoot-integration.md).

## Componentes

Cada componente tem 1 propósito e é testável isolado.

### 1. Webhook receiver (Fastify)
- Endpoint que recebe eventos no `outgoing_url` do bot.
- **Filtra**: só `event=message_created` + `message_type=incoming` + `private=false`.
- **Guard anti-loop**: descarta `outgoing` e mensagens do próprio bot (senão o
  agente responde a si mesmo em cascata). Ver [07](07-testing.md) — guard crítico.
- **Dedupe** por `message.id` (webhook do Chatwoot pode reentregar).
- Valida assinatura/secret do webhook.
- Enfileira pro orchestrator (não processa inline — responde 200 rápido).

### 2. Conversation orchestrator
- Por mensagem aceita: monta o **contexto** —
  - config da empresa (playbook, tom, produtos),
  - KB relevante (in-prompt se pequeno, ou via tool `consultar_kb`),
  - memória do contato (ai-memory, por telefone),
  - histórico da conversa (Chatwoot `GET messages`).
- Invoca o **agent core**; executa os tool-calls resultantes.
- **Serializa por-conversa** (lock/fila por `conversation_id`) pra não gerar duas
  respostas concorrentes na mesma conversa.

### 3. Agent core (Claude Agent SDK)
- `@anthropic-ai/claude-agent-sdk`, modo **custom tools only** (desliga
  read/bash/edit/write/grep/find/ls).
- System prompt = playbook da empresa (atender + qualificar + regras escalação).
- MCP servers conectados: **ai-memory**.
- Roda o loop de raciocínio → emite tool-calls.

### 4. Chatwoot client
- Wrapper REST fino sobre a Application API. Métodos:
  - `postMessage(conv, content, {private})` — resposta ou nota interna
  - `setStatus(conv, status)` — open/pending/resolved
  - `assign(conv, {assignee_id, team_id})`
  - `setLabels(conv, labels)` — sobrescreve (mandar lista completa)
  - `listMessages(conv)` — histórico
- Auth: `api_access_token` = **token do AgentBot** (não do usuário).

### 5. Tools do agente
Mapeiam pra Chatwoot client / KB / memória. Detalhe em [04](04-agent-core-and-tools.md).
- `consultar_kb(query)`
- `escalar_humano(motivo, tipo)`
- `fechar_conversa(resumo)`
- `add_label(labels)`
- `add_nota_privada(texto)`
- memória via ai-memory MCP (`memory_query`, `memory_write_page`)

### 6. KB store
- Markdown versionado na config da empresa. Loader simples.
- Começa in-prompt (KB pequena) OU `consultar_kb` sob demanda. Ver [05](05-context-memory-learning.md).

### 7. Memory (ai-memory MCP)
- 1 projeto/workspace ai-memory por empresa. Páginas por contato + resumos.

### 8. Config loader
- Carrega parametrização por empresa. Schema em [06](06-config-deploy-auth.md).

### 9. Summarizer
- Ao `resolved` (ou fim de atendimento): gera resumo + gaps → grava no ai-memory.

## Concorrência & confiabilidade

- **Serialização por conversa**: fila/lock por `conversation_id`.
- **Idempotência**: dedupe por `message.id`.
- **Resposta rápida ao webhook**: aceitar (200) e processar assíncrono.
- **Falha no agente**: registrar erro, opcionalmente escalar pra humano
  (fail-safe: se o agente cair, melhor cair pro humano que sumir).
- **Rate limit** (OAuth subscription): degradar com backoff; se estourar,
  escalar pro humano em vez de travar.

## Fronteiras / interfaces

- SDR ↔ Chatwoot: só REST (Application API) com token do bot.
- SDR ↔ ai-memory: só MCP.
- SDR ↔ bridge: **nenhuma** (desacoplado; comunica indireto via Chatwoot).
- SDR ↔ Claude: via Agent SDK (OAuth).
