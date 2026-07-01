# 07 — Testes & Verificação

TDD (mandato `code-craftsman`). Cada componente testável isolado.

## Unit

| Alvo | Casos |
|------|-------|
| Webhook filter | aceita `incoming`+não-privado; **rejeita `outgoing`/próprio bot** (anti-loop); rejeita `private`; dedupe por `message.id` |
| Chatwoot client | monta requests certos (mock HTTP): postMessage outgoing/private, setStatus, assign, setLabels, listMessages; auth com token do bot |
| Config loader | parse/validação do schema; erro claro em campo faltando; segredos não logados |
| KB loader / consultar_kb | carrega markdown; retorna trecho relevante; KB vazia não quebra |
| Playbook builder | monta system prompt da config; inclui temas sensíveis/sinais |
| Summarizer | gera resumo + gap a partir de histórico mock; grava no ai-memory (mock MCP) |
| Escalação | `escalar_humano` chama status+assign+label+nota **com motivo** |

## Integração

- **Chatwoot real de teste** (instância dev) + inbox + AgentBot provisionado.
- Simular mensagem de cliente (Client API ou mandar WhatsApp real via bridge de
  teste) e assertar:
  - agente **responde** (msg outgoing aparece na conversa),
  - **escala** corretamente (status open + assign + label + nota com motivo),
  - **fecha** (resolved),
  - **grava resumo** no ai-memory.
- Espelhar o padrão `scripts/e2e-test.sh` do bridge.

## E2E (fluxo completo)

WhatsApp real → bridge → Chatwoot → SDR → resposta → bridge → WhatsApp. Validar
o loop inteiro numa conversa de teste.

## Guards críticos (testar explícito)

1. **Anti-loop**: o agente **NÃO** responde às próprias mensagens. Injetar um
   evento `outgoing` do próprio bot e provar que é descartado. (Falha aqui =
   flood no cliente.)
2. **Serialização por conversa**: 2 mensagens rápidas na mesma conversa não geram
   2 respostas concorrentes/duplicadas.
3. **Idempotência**: reentrega do mesmo `message.id` não processa 2x.
4. **Fail-safe**: erro/timeout/rate-limit do agente → escala pro humano, não fica
   em silêncio.
5. **Segredos**: nenhum token/segredo em log.

## Validações manuais na instância do Giovani (pré-impl)

Antes/durante a implementação, confirmar na instância real (ver
[09](09-open-questions-for-review.md)):
- Q1: os dois hooks (canal do bridge + agentbot) disparam juntos.
- Q1: shape real do payload do evento do AgentBot.
- Q2: status inicial da conversa nova em inbox com bot.
