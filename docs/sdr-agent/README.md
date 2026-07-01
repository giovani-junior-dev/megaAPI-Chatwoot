# SDR Agent — Design (Fase 1)

Agente de IA de atendimento que atende WhatsApp via **AgentBot do Chatwoot**,
resolve rotina, qualifica intenção de compra (híbrido) e **escala pra humano com
contexto** quando falta base. Acoplado ao ecossistema `chatwoot-megaapi-bridge`
já em produção — **zero mudança no bridge**.

> Status: **design em revisão**. Nada implementado. Ordem: planejar → revisar →
> plano de implementação → executar.

## Índice

| Doc | Assunto |
|-----|---------|
| [01-overview-and-decisions](01-overview-and-decisions.md) | Contexto, dor, decisões travadas, goals/non-goals |
| [02-architecture](02-architecture.md) | Componentes, fluxo ponta-a-ponta, sequência |
| [03-chatwoot-integration](03-chatwoot-integration.md) | AgentBot, endpoints, webhook, anti-loop |
| [04-agent-core-and-tools](04-agent-core-and-tools.md) | Claude Agent SDK, tools, playbook, escalação |
| [05-context-memory-learning](05-context-memory-learning.md) | KB, ai-memory, resumos, learning assistido |
| [06-config-deploy-auth](06-config-deploy-auth.md) | Schema config, auth OAuth, deploy instance-per-company |
| [07-testing](07-testing.md) | Unit, integração, E2E, guards críticos |
| [08-phase-2-followup](08-phase-2-followup.md) | Follow-up proativo, loop melhoria, constraint 24h |
| [09-open-questions-for-review](09-open-questions-for-review.md) | Pendências a resolver na revisão |
| [10-whatsapp-swap-and-transport](10-whatsapp-swap-and-transport.md) | **D10/D11** — Chatwoot como camada de troca de core; sem WhatsApp no agente |
| [sdr-agent-fase1-plan.html](sdr-agent-fase1-plan.html) | **Plano de implementação** (planf3 + code-craftsman): fases, EARS, TDD, matriz de testes |

> **Backend: NestJS** · **Personalidade: `AGENT.md` (estilo CLAUDE.md) + `kb/` por agente, replicável por cópia** (decisões rodada 2).

## Resumo em 1 parágrafo

Serviço TypeScript (Fastify) que registra um **AgentBot** no inbox WhatsApp do
Chatwoot. Quando chega mensagem de cliente, o Chatwoot dispara o `outgoing_url`
do bot → o serviço monta contexto (config da empresa + KB markdown + memória do
contato no **ai-memory** + histórico da conversa) → roda um loop do **Claude
Agent SDK** (custom tools only) → o agente responde, aplica labels, escala pra
humano ou fecha a conversa via API do Chatwoot. A resposta sai pelo bridge atual
→ megaAPI → WhatsApp. Pós-conversa, grava resumo + gaps no ai-memory pra melhoria
assistida. Uma instância por empresa, parametrizada por config.

## Rastreio

- Epic bd: `chatwoot-megaapi-bridge-08o`
- Design bd: `chatwoot-megaapi-bridge-08o.1`
- Plano brainstorming aprovado: `~/.claude/plans/vamos-criar-agora-um-misty-clarke.md`
