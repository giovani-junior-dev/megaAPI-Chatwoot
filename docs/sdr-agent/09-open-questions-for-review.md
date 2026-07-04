# 09 — Perguntas: resolvidas / gates / abertas

Status pós-revisão. Planejamento **fechado**: decidíveis resolvidas, restantes
viram gate de fase ou 1 decisão de negócio do Giovani.

## ✅ Resolvidas (decisão travada)

| Q | Decisão |
|---|---------|
| Q4 — `responder` implícito vs explícito | **Tool explícita.** Permite escalar sem responder; mais controle. Fixado na Phase 4. |
| Q6 — formato da config | **`config/<empresa>/config.yaml`** (perfil, ids, flags) + **`AGENT.md`** + `kb/`. Segredos (`bot_access_token`, `webhook_secret`, OAuth token) **só via env/secret-manager**, nunca em git. Serviço próprio — não depende da cifra do bridge. |
| Q7 — modelo & auth | Default **Sonnet**; **OAuth subscription** em todas as instâncias; trocar por **API key** só na instância que escalar tráfego (path documentado, muda só env). |
| Q8 — nome/repo | **`chatwoot-sdr-agent`**, repo **separado**. (Veta se quiser outro nome.) |
| Q10 — métrica nº1 | **% de conversas resolvidas sem humano** (deflexão) como norte da Fase 1. Secundárias: leads quentes entregues, tempo até 1ª resposta. |
| Q11 — piloto | **Produto do próprio Giovani** (ele atende hoje). 1ª instância = montar `AGENT.md` + `kb/` reais desse produto. |

## 🚧 Gates de implementação (validar na instância real — Phase 7, não bloqueiam Phases 0–6)

| Q | Gate |
|---|------|
| Q1 — coexistência hooks + payload | Capturar evento real: confirmar que canal (bridge) **e** `outgoing_url` do AgentBot disparam juntos; travar shape do payload. Gate p/ fechar Phase 2/7. |
| Q2 — status inicial da conversa | Confirmar se conversa nova em inbox com bot começa `pending`; ajustar handoff. |
| Q3 — endpoint criação do bot | Application (`/api/v1/accounts/{id}/agent_bots`) vs Platform API na self-hosted. Muda provisionamento. |

## 🟡 Aberta — 1 decisão de negócio do Giovani

- **Q5 — LGPD / retenção na memória.** Proposta default (confirmar): memória do
  contato guarda **intenção, resumo, status, preferências** — **NÃO** guarda dado
  sensível cru (documento, dado de pagamento, saúde). Retenção configurável.
  O summarizer segue essa regra. Confirmar antes da Phase 6.

## 🔵 Fase 2 (não bloqueia Fase 1)

- **Q9 — megaAPI e janela 24h.** Follow-up proativo: megaAPI aplica janela/template
  ou manda livre? Apetite de risco de ban? Giovani responde na Fase 2.
