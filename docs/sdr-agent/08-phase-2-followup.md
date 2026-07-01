# 08 — Fase 2 (esboço — não implementar agora)

Fase 2 entra depois da Fase 1 rodando e com dados reais acumulados no ai-memory.

## Follow-up proativo

Reengajar cliente/lead que não respondeu ou ficou de retornar.

- **Precisa**: scheduler + tracking (quem / quando / por quê seguir) + política de
  quantas tentativas.
- **Fonte de quem seguir**: resumos/memória do ai-memory (ex: lead quente que não
  fechou; cliente que ficou de confirmar).
- **⚠️ Constraint — janela 24h do WhatsApp**: no WhatsApp **oficial**, mensagem
  proativa fora de 24h desde a última msg do cliente exige **template aprovado**.
  - **megaAPI é não-oficial** (Baileys?) → pode mandar livre, SEM gate de template,
    mas com **maior risco de ban** (risco R1 já mapeado no bridge).
  - **Decisão pendente pro Giovani** (ele construiu a megaAPI): a megaAPI aplica
    regra de janela/template ou manda livre? Qual o apetite de risco de ban pra
    proativo? Ver Q9.
  - Isso **não bloqueia a Fase 1** — resposta a inbound está sempre dentro da
    janela de 24h.

## Loop de melhoria assistido (completar o de [05](05-context-memory-learning.md))

- Agente batch periódico: lê gaps/resumos acumulados → agrega recorrências → gera
  **sugestão de atualização de KB**.
- Giovani revisa e aprova (fluxo de aval — como? PR nos .md da KB? painel?).
- Aprovado → KB markdown atualizada → atendimento melhora.

## Analytics (opcional)

- Export jsonl dos resumos pra métricas (deflexão, temas, leads, gaps top-N).
- Dashboard simples se houver demanda.

## Possíveis extras (backlog, não decididos)

- Múltiplas inboxes por instância.
- Áudio/mídia no atendimento (transcrição de áudio do cliente).
- A/B de playbook.
