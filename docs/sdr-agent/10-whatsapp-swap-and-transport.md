# 10 — Troca de core WhatsApp & camada de transporte

Registro da decisão **D10/D11** (rodada 2 do brainstorming) pra não reabrir.

## O ponto que Giovani levantou

> "megaAPI é não-oficial, mas quero poder usar a oficial também. Arquitetura que
> troque o core WhatsApp sem parar o produto nem reescrever. Igual LLM compatível
> com API OpenAI." + "Sou tech provider Meta, tenho minha API oficial
> (`wablastmessage.com`)."

## A resposta: Chatwoot JÁ é a camada de abstração

O agente **nunca toca WhatsApp**:
- **Envia** "como se fosse Giovani no chat" → Chatwoot (integrado via bridge→megaAPI)
  entrega no WhatsApp.
- **Recebe** a msg do cliente pelo webhook do AgentBot (a msg já está no Chatwoot).

Logo o WhatsApp está **100% na camada do Chatwoot**, não do agente. Trocar
megaAPI ↔ oficial acontece **embaixo** do agente. Mesmo princípio "OpenAI-
compatível": o agente fala **uma** interface (Chatwoot); o transporte troca livre
atrás. O agente é **automaticamente à prova de troca** — zero reescrita.

```
Cliente WhatsApp
   ⇅  [core trocável: megaAPI | wablast oficial]   ← troca aqui (fora do agente)
Chatwoot   (camada de abstração — o "OpenAI-compatível")
   ⇅  AgentBot
SDR AGENT  (fala só Chatwoot — nunca vê WhatsApp)
```

## D10 — decisão

**Nenhum gateway/abstração WhatsApp dentro do agente.** Seria código à toa
(YAGNI). O agente depende só do contrato Chatwoot.

## D11 — wablast oficial é workstream SEPARADO

Giovani vai adicionar a wablast como core **dentro do Chatwoot** (como fez com
megaAPI — o Chatwoot nativo oficial-Meta é engessado pro caso dele). Isso é
**outra sessão / outro projeto**, fora do escopo do SDR Agent.

Quando existir, o inbox no Chatwoot troca de canal/core e o agente **continua
funcionando sem tocar em nada**. É exatamente o "trocar sem reescrever" que ele
queria — entregue pela fronteira do Chatwoot, não por código no agente.

## Onde uma "classe por provider" caberia (se algum dia)

Só na camada de transporte não-oficial (megaAPI/Evolution/WAHA que alimentam o
canal `api` do Chatwoot) — domínio do **bridge (Go)**, interface
`WhatsAppProvider`. Não é deste projeto. WhatsApp oficial nem precisa disso
(wablast/Chatwoot resolvem).
