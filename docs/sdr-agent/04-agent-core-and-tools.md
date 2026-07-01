# 04 — Agent Core & Tools

## Claude Agent SDK — config

- Pacote: `@anthropic-ai/claude-agent-sdk` (TypeScript).
- Backend: Claude Code CLI, auth via `CLAUDE_CODE_OAUTH_TOKEN` (subscription).
  Ver [06](06-config-deploy-auth.md).
- **Custom tools only**: desligar tools nativas de código (read/bash/edit/write/
  grep/find/ls). O SDR não mexe em filesystem.
- **System prompt** = playbook da empresa (montado da config, ver abaixo).
- **MCP servers**: ai-memory (memória por-contato + resumos).
- Modelo: configurável por empresa (default Sonnet; Opus se precisar raciocínio
  mais pesado). Ver Q7.

## Tools expostas ao agente

Cada tool é fina e mapeia pra Chatwoot client, KB ou memória.

| Tool | Args | Efeito |
|------|------|--------|
| `consultar_kb` | `query` | Busca na KB markdown; retorna trechos relevantes |
| `responder` | `texto` | Posta `outgoing` na conversa (vai pro WhatsApp) |
| `add_nota_privada` | `texto` | Posta nota interna (`private=true`) |
| `add_label` | `labels[]` | Seta labels na conversa |
| `escalar_humano` | `motivo`, `tipo`, `team_id?` | status→open + assign + label + nota com motivo |
| `fechar_conversa` | `resumo` | status→resolved + dispara summarizer |
| memória | (via ai-memory MCP) | `memory_query` / `memory_write_page` |

Nota: `responder` pode ser implícito (o texto final do turno vira a resposta) OU
explícito como tool. Definir na implementação — tool explícita dá mais controle
sobre quando/se responder (ex: escalar sem responder). Ver Q4.

## Playbook (system prompt) — estrutura

Montado da config da empresa. Seções:

1. **Identidade & tom** — quem é o agente, tom de voz, idioma (PT-BR), limites
   ("nunca invente preço/prazo; se não sabe, escale").
2. **Sobre a empresa & produtos** — resumo curado (o essencial in-prompt; detalhe
   grande via `consultar_kb`).
3. **Como atender** — passo a passo do atendimento rotineiro.
4. **Como qualificar** (híbrido) — sinais de intenção de compra, perguntas de
   qualificação, quando marcar `lead-quente`.
5. **Quando escalar** — gatilhos (abaixo), e SEMPRE deixar nota com motivo.
6. **Quando fechar** — critérios de resolução.
7. **Regras duras** — não prometer o que não está na KB; não dar informação
   sensível; respeitar LGPD/privacidade.

## Gatilhos de escalação (híbrido)

| Gatilho | Ação | Label sugerida |
|---------|------|----------------|
| Pedido explícito ("quero humano") | escalar | `pedido-humano` |
| Falta de contexto na KB / baixa confiança | escalar | `escalado-sem-contexto` |
| Tema sensível (reclamação, jurídico, reembolso, cancelamento) | escalar | `tema-sensivel` |
| **Lead quente** (intenção clara de compra) | escalar/rota vendas | `lead-quente` |
| N turnos sem resolver | escalar | `sem-resolucao` |

Mecanismo único de escalação: `pending`→`open` + `assign(team/humano)` +
`add_label` + `add_nota_privada(motivo)`. O motivo na nota é obrigatório — é o que
faz o humano assumir sem reler tudo.

> Lista de temas sensíveis e sinais de lead quente são **configuráveis por
> empresa** (config), não hardcoded.

## Loop de decisão (por mensagem)

```
recebe msg incoming
  → carrega contexto (config + KB + memória contato + histórico conversa)
  → agente raciocina:
      ├─ sei responder pela KB?  → responder → (talvez fechar se resolveu)
      ├─ falta contexto / sensível / pediu humano / lead quente?
      │     → add_nota_privada(motivo) → add_label → assign → status open
      └─ preciso de mais info do cliente? → responder (pergunta) → aguarda
  → grava/atualiza memória do contato (ai-memory)
```

## Fail-safe

Se o agente falhar (erro, timeout, rate limit): **escalar pro humano** com nota
"falha técnica do agente" em vez de deixar o cliente sem resposta. Silêncio é o
pior resultado pra um atendimento.
