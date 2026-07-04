# ADR 0001 — Provider seam for a second WhatsApp gateway (WaBlast)

Date: 2026-07-02
Status: Accepted

## Context

The bridge was hardcoded to megaAPI (unofficial WhatsApp): `processInbound`/
`processOutbound`, `megaapi_*` tenant columns, routes `/v1/wa` + `/v1/cw`.
We needed to **add** WaBlast (`api.wablastmessage.com`, official Meta WhatsApp
Cloud gateway) **without removing** megaAPI. One provider per tenant.

The project convention (`CLAUDE.md`) forbids speculative interfaces "until there
is a second concrete implementation". WaBlast is that second implementation.

## Decision

Introduce a minimal `Provider` interface (`internal/bridge/provider.go`) with
four methods: `SendText`, `SendMedia`, `ParseInbound`, `DownloadMedia`. Two
concrete impls: `megaProvider` (thin wrapper over the existing functions, no
behaviour change) and `wablastProvider`. `providerFor(tenant)` switches on the
new `tenants.provider` column. Worker code (`processInbound`/`processOutbound`)
is provider-agnostic; inbound parsing is normalised to `InboundMessage`.

WaBlast-specific differences are contained in `wablast.go` / `web/wablast.go`:
- Inbound auth = Standard Webhooks (`VerifyStandardWebhook` in `crypto.go`).
- Outbound media = 2-step (upload `/v1/media` → send with `media_id`).
- 24h window: `409 WINDOW_CLOSED` → `errWindowClosed` → private note in Chatwoot,
  message marked failed (non-retriable).
- Webhook registration is fail-closed in the wizard (the `whsec_` secret is
  returned once; failure aborts tenant creation).

## Alternatives rejected

- **Branch on `tenant.Provider` inline** across 4 files — scatters gateway logic;
  the interface localises each gateway to one file.
- **Overload `megaapi_*` columns** for WaBlast — confusing; dedicated
  `wablast_*` columns self-document.
- **Auto-send a template on WINDOW_CLOSED** — needs template mapping + UI; out of
  MVP scope. A private note is enough for now.
- **Two providers per tenant** — no real use case; adds routing complexity.

## Consequences

- megaAPI path is untouched (regression-free; existing tests unchanged).
- Adding a third gateway now means one new `*Provider` file + a `providerFor` arm.
- Chatwoot side (`/v1/cw`, contact/conversation/message) is shared by both.
