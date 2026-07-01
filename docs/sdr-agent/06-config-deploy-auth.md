# 06 — Config, Deploy & Auth

## Modelo: instance-per-company

1 processo/container = 1 empresa/Chatwoot. Sem multi-tenant no código. Subir nova
empresa = nova config + novo deploy. Isolamento total (credencial, memória, KB,
playbook, rate-limit, auth).

## Schema de config (por empresa)

Proposta (formato final — YAML/env — decidir na impl, Q6):

```yaml
chatwoot:
  base_url: https://chat.empresa.com
  account_id: 1
  inbox_id: 7
  bot_id: 12
  bot_access_token: <cifrado>      # token do AgentBot
  webhook_secret: <cifrado>        # valida eventos no outgoing_url

company:
  nome: "Empresa X"
  descricao: "..."
  tom_de_voz: "próximo, direto, PT-BR"

playbook:
  kb_dir: ./kb                     # markdown files
  temas_sensiveis: [reclamacao, juridico, reembolso, cancelamento]
  sinais_lead_quente: ["quero comprar", "qual o valor", "fechar", ...]
  escala_team_id: 2                # time humano destino
  max_turnos_sem_resolver: 6

model:
  provider: claude
  model: claude-sonnet-5           # opus se precisar
  # auth via env CLAUDE_CODE_OAUTH_TOKEN

memory:
  aimemory_project: empresa-x      # escopo ai-memory
```

Segredos (`bot_access_token`, `webhook_secret`, OAuth token) **nunca** em git —
env/secret manager. Espelhar o padrão de cifra do bridge se rodar junto.

## Auth (Claude Agent SDK)

- **Subscription OAuth**: `claude setup-token` → `CLAUDE_CODE_OAUTH_TOKEN`
  (token ~1 ano) → env do processo.
- **Cuidado**: se `ANTHROPIC_API_KEY` estiver no ambiente, ela **vence
  silenciosamente** o OAuth. Garantir que NÃO esteja setada (ou setar explícito o
  auth desejado).
- **Caveat ToS (documentar)**: OAuth de subscription é licenciado pra **uso
  individual**; rate limit calibrado pra 1 humano. Instância single-tenant da
  própria empresa mantém isso defensável. **Se uma empresa escalar tráfego** →
  trocar aquela instância por **API key** com billing (só muda env, sem
  reescrever). Ver Q7.
- **Fail-safe rate limit**: se estourar limite, escalar pro humano (não travar).

## Deploy

- 1 container por empresa. Config via env + `kb_dir` montado (volume).
- Roda **ao lado do bridge** (mesma stack Swarm/Portainer) ou em qualquer host com
  acesso de rede à API do Chatwoot + saída internet (Claude).
- `outgoing_url` do bot precisa ser **alcançável pelo Chatwoot** (mesma exposição
  que o bridge já usa — Cloudflare Tunnel / host público).
- Healthcheck `/healthz`. Logs estruturados (espelhar zerolog-style do bridge, mas
  em TS: pino).

## Estrutura do repo (separado)

```
chatwoot-sdr-agent/            # NestJS
  src/
    webhook/        # controller outgoing_url + filtro/anti-loop/dedupe
    orchestrator/   # monta contexto, roda loop, serializa por conversa
    agent/          # Claude Agent SDK config + tools + system-prompt(AGENT.md)
    chatwoot/       # client REST
    kb/             # loader markdown + consultar_kb
    memory/         # wrapper ai-memory MCP
    summarizer/     # resumo + gaps
    config/         # loader + schema
  config/<empresa>/ # AGENT.md (personalidade) + kb/*.md  (replicar = copiar+editar)
  test/
```

Repo **separado** do bridge (linguagem diferente, deploy independente,
instance-per-company). Nome proposto: `chatwoot-sdr-agent` (confirmar, Q8).
