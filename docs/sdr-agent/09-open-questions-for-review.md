# 09 — Perguntas Abertas (resolver na revisão)

Pendências a fechar antes/durante o plano de implementação. Numeradas p/ referência.

## Validação técnica na instância do Giovani

- **Q1 — Coexistência de hooks + payload.** Confirmar (capturando eventos reais)
  que o Chatwoot dispara o `webhook_url` do canal (bridge) **e** o `outgoing_url`
  do AgentBot pra mesma conversa/inbox. Capturar o shape exato do payload do
  evento do bot.
- **Q2 — Status inicial.** Qual o status de uma conversa nova num inbox com
  AgentBot? Começa `pending` (bot dono) automático, ou o bot precisa setar? Como o
  handoff `pending`→`open` se comporta na fila dos agentes.
- **Q3 — Endpoint de criação do bot.** Na self-hosted do Giovani, criar agent bot
  responde via Application API (`/api/v1/accounts/{id}/agent_bots`) ou só via
  Platform API (`/platform/api/v1/agent_bots`)? Isso muda o provisionamento.

## Produto / comportamento

- **Q4 — `responder` implícito vs explícito.** O texto final do turno vira
  resposta automática, ou `responder` é tool explícita (permite escalar sem
  responder)? Recomendo **explícita** (mais controle).
- **Q5 — Privacidade/LGPD na memória.** O que a memória do contato **pode** e
  **não pode** guardar? Retenção? Isso afeta o que o summarizer grava.
- **Q6 — Formato da config.** YAML + env? Um arquivo por empresa? Onde ficam os
  segredos (reusar cifra do bridge, ou secret manager)?
- **Q7 — Modelo & auth por empresa.** Default Sonnet ou Opus? OAuth subscription
  em todas, ou já prever API key pra alguma? Critério de troca.
- **Q8 — Nome/repo.** `chatwoot-sdr-agent`? Repo separado (recomendado) — confirmar.

## Fase 2

- **Q9 — megaAPI e janela 24h.** A megaAPI aplica regra de janela/template do
  WhatsApp pra proativo, ou manda livre? Apetite de risco de ban pra follow-up?
  (Giovani responde — ele construiu a megaAPI.)

## Escopo / prioridade

- **Q10 — Métrica de sucesso.** Confirmar as métricas propostas em
  [01](01-overview-and-decisions.md) e qual é a nº1 pra Fase 1.
- **Q11 — Primeira empresa piloto.** Qual empresa/segmento vai ser a primeira
  instância? Isso define a KB/playbook inicial de teste real.
