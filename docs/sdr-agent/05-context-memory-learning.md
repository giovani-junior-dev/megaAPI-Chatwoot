# 05 — Contexto, Memória & Aprendizado

Três stores distintos, cada um 1 propósito. Não misturar.

## 1. Knowledge Base (estática, curada) — lever nº1 de qualidade

- **O quê**: produto, empresa, FAQ, playbook/script, políticas (troca, prazo,
  preço), regras de escalação, temas sensíveis, sinais de lead quente.
- **Formato**: **markdown versionado** na config da empresa:
  `config/<empresa>/AGENT.md` (personalidade/playbook, estilo CLAUDE.md) +
  `config/<empresa>/kb/*.md` (contexto). Replicar agente = copiar a pasta + editar.
- **Como o agente usa**:
  - KB pequena → embutida no system prompt.
  - KB maior → tool `consultar_kb(query)` sob demanda.
- **Sem RAG/vetor agora** (YAGNI). Só se a KB crescer a ponto de não caber /
  busca textual falhar. Nesse caso, adicionar índice simples antes de vetor.
- **Autoria**: Giovani edita os .md. Mudou o negócio → edita a KB. É o principal
  ponto de controle de qualidade.

## 2. Memória por-contato (dinâmica) — ai-memory via MCP

- **O quê**: "já falei com esse cliente; ele perguntou X; quer Y; ficou de
  retornar Z".
- **Tech**: **ai-memory** conectado como **MCP server** ao Claude Agent SDK.
- **Escopo**: 1 projeto/workspace ai-memory **por empresa** (isolamento). Dentro,
  **1 página por contato** (chave = telefone/JID normalizado).
- **Fluxo**:
  - Início do atendimento → `memory_query` pela página do contato → injeta no
    contexto ("o que sabemos desse cliente").
  - Durante/fim → `memory_write_page` atualiza a página do contato com o que
    mudou.
- **Privacidade/LGPD**: memória guarda dado de cliente. Definir retenção e o que
  NÃO guardar (ex: dado sensível cru). Ver [09](09-open-questions-for-review.md) Q5.

## 3. Resumos de atendimento & aprendizado assistido

- **Resumo por conversa**: ao fechar (`resolved`), o **summarizer** gera:
  - resumo do atendimento (o que o cliente queria, o que foi resolvido),
  - **gap capturado** se escalou ("faltou info sobre política de troca"),
  - sinais (lead quente? objeção? dúvida recorrente?).
- **Onde grava**: **ai-memory** (mesmo projeto da empresa; página de resumo +
  atualiza página do contato). **Sem jsonl paralelo** — DRY. Export jsonl fica
  pra Fase 2 se precisar analytics.

### Loop de melhoria — ASSISTIDO (decisão travada)

Nada de mutação autônoma de playbook (risco de drift/regressão). O loop é:

```
atendimento → captura gap + resumo (ai-memory)
   → (periódico) agrega gaps recorrentes
   → gera SUGESTÃO de mudança na KB ("clientes perguntam muito sobre X, não temos")
   → Giovani REVISA e aprova
   → KB markdown atualizada → próximos atendimentos melhoram
```

Na Fase 1, o serviço só **captura** (gap + resumo). A agregação/sugestão e o fluxo
de aprovação são **Fase 2** ([08](08-phase-2-followup.md)). Isso mantém a Fase 1
enxuta e já acumula os dados crus certos desde o dia 1.

## Resumo dos stores

| Store | Tech | Escopo | Escreve quando |
|-------|------|--------|----------------|
| KB | markdown | por empresa (curado por humano) | Giovani edita |
| Memória contato | ai-memory (MCP) | por empresa → por contato | durante/fim do atendimento |
| Resumos/gaps | ai-memory (MCP) | por empresa | ao fechar conversa |
