# 01 — Overview & Decisões

## Contexto / dor

Giovani opera sozinho e não dá conta do atendimento constante no Chatwoot. Cada
minuto em atendimento manual é minuto fora do resto do negócio. A dor é
**urgente** e **interna**. Prioridades declaradas, nessa ordem:

1. **Qualidade** do atendimento (não pode parecer robô burro nem dar informação
   errada).
2. **Agilidade** de entrega (reusar o que já funciona, shippar rápido).
3. **Custo previsível** de produção.

## O que é

Um agente de IA que:

- **Atende** o cliente no WhatsApp (via Chatwoot) resolvendo o que estiver na base
  de conhecimento (dúvidas, informações, rotina).
- **Qualifica** intenção de compra e marca lead quente (papel **híbrido**:
  atendente + SDR).
- **Escala pra humano** quando falta contexto, o tema é sensível, o cliente pede,
  ou detecta lead quente — sempre deixando o **motivo** registrado pro humano
  assumir sem reler tudo.
- **Fecha** a conversa quando resolve.
- **Lembra** interações anteriores com o mesmo cliente.
- **Coleta** resumos + gaps de cada atendimento pra melhoria contínua assistida.

Tudo dentro do Chatwoot — Giovani acompanha cada conversa no painel, e a resposta
sai como mensagem do número WhatsApp da empresa (via bridge → megaAPI).

## Decisões travadas (do brainstorming)

| # | Tema | Decisão | Porquê (resumo) |
|---|------|---------|-----------------|
| 1 | Tenancy | **Single-tenant, instance-per-company** | Isolamento real; auth "uso individual" defensável por instância; playbook por segmento; deploy = copiar config. Multi-tenant = over-engineering pro volume atual. |
| 2 | Integração | **AgentBot nativo do Chatwoot** | Feito pra isso: webhook próprio, token próprio, handoff `pending`↔`open`, não gasta assento, separa IA/humano no painel. |
| 3 | Framework | **Claude Agent SDK (TypeScript)** | Reusa infra Pro Max OAuth que já roda (agilidade). MCP nativo p/ ai-memory. Modelos de topo. |
| 4 | Auth | **Subscription OAuth** (`CLAUDE_CODE_OAUTH_TOKEN`) | Já funciona. Individual-use ok por instância. Path documentado p/ API key se escalar. |
| 5 | Contexto | **KB markdown + ai-memory (MCP) + learning assistido** | 3 stores separados, cada um 1 propósito. Sem jsonl paralelo (DRY). |
| 6 | Aprendizado | **Assistido com aval** | Captura gaps+resumos; humano aprova antes de virar regra. Sem mutação autônoma (evita drift). |
| 7 | Papel | **Híbrido** | Resolve rotina E qualifica compra, roteando lead quente com tag. |
| 8 | Escalação | Status + assign + label + **nota privada com motivo** | Humano assume com contexto instantâneo. |
| 9 | Stack | **TypeScript/Node (Fastify)** | Preferência do dev + ecossistema SDK/MCP/REST. |

Detalhe do porquê de cada uma está no plano aprovado do brainstorming.

## Goals (Fase 1)

- Atender inbound ponta-a-ponta: responder, qualificar, escalar, fechar, resumir.
- Reativo a mensagem de cliente (sem proatividade ainda).
- Uma instância parametrizada pra uma empresa/Chatwoot.
- Zero mudança no `chatwoot-megaapi-bridge`.

## Non-goals (Fase 1 — YAGNI)

- Multi-tenant / painel de tenants.
- RAG/busca vetorial na KB (só se a KB crescer demais).
- Mutação autônoma de playbook.
- Follow-up proativo (Fase 2).
- Export jsonl de resumos / analytics (Fase 2, se precisar).

## Métrica de sucesso (proposta — validar na revisão)

- **% de conversas resolvidas sem intervenção humana** (deflexão).
- **Leads quentes entregues** com contexto (qualificação).
- **Tempo até primeira resposta** (deve cair pra ~segundos).
- **Taxa de escalação com motivo claro** (todo escalado tem nota explicando).
