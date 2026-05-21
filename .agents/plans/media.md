# Media Bidirectional Support — Plan

**Goal**: Enable image/audio/video/document/sticker messages WA↔CW via megaAPI pass-through URL.
**Branch**: `feat/media-mvp` (off `main` after PR #2 merges)
**Estimate**: ≤2 days work, ≤200 LOC prod, ≤150 LOC tests
**Style**: pass-through URL only. No download. No upload. No stream proxy.

## Requirements (EARS)

| ID | Requirement |
|----|-------------|
| R1 | When megaAPI webhook arrives with `imageMessage`, the system shall extract `url`+`mimetype`+`caption`, post to Chatwoot with `attachments[0].file_url=url` and `content=caption`. |
| R2 | When megaAPI webhook arrives with `audioMessage`/`videoMessage`/`stickerMessage`, the system shall extract `url`+`mimetype` and post to Chatwoot with `attachments[0].file_url=url` (caption from payload if present, else empty). |
| R3 | When megaAPI webhook arrives with `documentMessage`, the system shall extract `url`+`mimetype`+`fileName`+`caption`, post to Chatwoot with `attachments[0].file_url=url` and `content=caption`. |
| R4 | When Chatwoot webhook arrives with `attachments[]` non-empty, the system shall POST `/rest/sendMessage/{instance}/mediaUrl` to megaAPI once per attachment. |
| R5 | When deriving megaAPI media type from Chatwoot `file_type`, the system shall map `image→image`, `audio→audio`, `video→video`, `file→document`. Sticker treated as `image`. |
| R6 | When sending multiple attachments outbound, the system shall set `caption=content` only on the first POST; subsequent posts have empty caption. |
| R7 | If webhook payload contains unknown message type (`location`, `contact`, `reaction`), then the system shall log warn with type+messageID and ACK HTTP 200 without enqueuing. |
| R8 | If attachment URL is empty/missing, then the system shall log warn and skip that attachment without erroring the request. |

## Architecture

| File | Change |
|------|--------|
| `internal/bridge/types.go` | Add `Attachment{URL, MimeType, FileName, MediaType}`. |
| `internal/bridge/megaapi_parser.go` | Add `parseMediaMessage()`; switch on payload key (`imageMessage`/`audioMessage`/`videoMessage`/`documentMessage`/`stickerMessage`) before text fallback. Return unknown-type sentinel for location/contact/reaction. |
| `internal/bridge/chatwoot_client.go` | Extend `CreateMessage` payload to include `attachments[]` with `file_url`+`file_type`. |
| `internal/bridge/megaapi_client.go` | Add `SendMediaURL(instance, to, mediaURL, mediaType, caption, fileName)` POSTing `/rest/sendMessage/{instance}/mediaUrl`. |
| `internal/bridge/chatwoot_webhook.go` | Read `attachments[]` from inbound CW webhook; loop dispatching to megaAPI client. |
| `internal/bridge/dispatcher.go` (or webhook handler) | Loop over attachments; first gets caption, rest get empty caption. |

No new packages. No refactor of existing code. No new tables.

## Trade-offs

| Decision | Reason | Rejected |
|----------|--------|----------|
| Pass-through URL (no download) | MVP, megaAPI URL is signed/CDN-served, fits ≤2 days budget | Stream proxy through bridge (adds storage, bandwidth, encryption-at-rest concerns) |
| Sequential POST per attachment | Simpler, ordered delivery, low MVP volume | Parallel goroutines (out-of-order risk, no measurable gain) |
| Caption on 1st attachment only | Matches WA UX (single caption per album) | Repeat caption (clutter), separate text message (extra round-trip) |
| Unknown types ACK 200 + warn | Prevents megaAPI retries flooding logs | ACK 4xx (retry storms), enqueue & ignore (DB pollution) |
| Sticker → `image` type outbound | megaAPI `/mediaUrl` has no sticker mode | Add sticker endpoint (separate API path, out of scope) |
| Default unknown `file_type` → `document` | Safe fallback, megaAPI accepts arbitrary mime | Hard error (breaks valid CW payloads) |

## Milestones

| # | Name | Days | Deliverables |
|---|------|------|--------------|
| M1 | Inbound parser | 0.5 | `parseMediaMessage()` + 5 unit tests (one per type) + unknown-type sentinel |
| M2 | Inbound dispatch to CW | 0.5 | `attachments[]` in Chatwoot client payload + integration test (image+audio round-trip) |
| M3 | Outbound megaAPI client | 0.25 | `SendMediaURL()` + unit test (httptest assertion on path+body) |
| M4 | Outbound dispatcher loop | 0.5 | Attachment loop in CW webhook handler + integration test (2 attachments, caption-on-first) |
| M5 | Unknown types ACK | 0.25 | Warn-log + ACK branch + unit test (location fixture) |

## TDD Strict (RED → GREEN per milestone)

| M | RED (write first, must fail) | GREEN |
|---|------------------------------|-------|
| M1 | `TestParseMediaMessage_Image/Audio/Video/Document/Sticker` asserting URL+mime+caption+fileName extraction | `parseMediaMessage()` switch in `megaapi_parser.go` |
| M2 | `TestPostToChatwoot_WithImageAttachment` asserting POST body has `attachments[0].file_url` and `content=caption` | Extend `CreateMessage` payload struct + serialization |
| M3 | `TestSendMediaURL_PostsToMegaAPI` httptest server asserting path `/rest/sendMessage/{inst}/mediaUrl` and body fields | `SendMediaURL()` method on megaAPI client |
| M4 | `TestOutbound_TwoAttachments_FirstHasCaption_RestEmpty` integration with mock megaAPI, asserts 2 POSTs, only 1st has caption | Dispatcher loop in CW webhook handler |
| M5 | `TestWebhook_LocationMessage_ACK200_NoEnqueue` integration, asserts no DB row inserted | Warn+ACK branch in webhook parser |

## Tests Matrix

| Type | Tool | Coverage Target | Scope |
|------|------|-----------------|-------|
| Unit | stdlib `testing` | parser 5 cases, type-derive helper, attachment-loop helper, unknown-type sentinel | pure funcs |
| Integration | testcontainers postgres + httptest megaAPI + httptest CW | inbound image E2E, outbound 2-attachment E2E, unknown-type ACK | full webhook → DB → outbound |
| Total new | — | ~12 cases, ≤150 LOC | — |

## Acceptance Criteria

- [ ] WA image → CW shows image attachment with caption
- [ ] WA audio → CW shows audio attachment
- [ ] WA video → CW shows video attachment with caption
- [ ] WA document (pdf) → CW shows file attachment with fileName + caption
- [ ] WA sticker → CW shows image attachment (empty caption)
- [ ] CW agent sends 1 image + text → WA receives image with caption
- [ ] CW agent sends 3 attachments + text → WA receives 3 medias, only 1st captioned
- [ ] CW `file_type=image` → megaAPI POST `mediaType=image`
- [ ] CW `file_type=file` (pdf) → megaAPI POST `mediaType=document`
- [ ] WA location/contact/reaction → bridge logs warn, ACKs 200, zero DB rows
- [ ] All 53 existing tests still pass
- [ ] ≥12 new tests passing
- [ ] `go vet` + `staticcheck` clean
- [ ] Prod LOC delta ≤200, test LOC delta ≤150

## Out of Scope (do NOT touch)

- QW1–QW6: comments, magic numbers, RFC purism, Bearer case-insensitive, json.Marshal error wrap
- WD1–WD5: graceful shutdown, drain backlog, extractor returns error, retry warn-log, doc banners
- FU1–FU10: CI workflow, mutation testing, integration extras, idempotency tests, refactors, ADRs, runbook
- Stream proxy / download-then-upload of media bytes
- Prometheus metrics, extra observability
- DLQ admin endpoint, admin UI
- Cache, rate limit, backpressure tuning
- Refactor or reorganization of existing packages
- Reaction/location/contact handling beyond warn+ACK
- Encryption of attachment URLs at rest (treated as ephemeral pass-through tokens)
- Re-hosting expired megaAPI URLs (future follow-up, not now)

## Risks

| Risk | Mitigation |
|------|------------|
| megaAPI URL expires before CW agent opens it | Single inline code comment near pass-through call documenting caveat. No retry/re-host logic in MVP. |
| CW adds new `file_type` value not in mapping | Default unknown → `document`; log warn with the unknown value once per message |
| `mimetype` missing in payload | Fallback chain: payload mimetype → extension-from-URL → `application/octet-stream` |
| Attachment URL contains query-string secrets | Pass through verbatim. Do NOT log full URL at info level (only host+path). |

## Definition of Done

PR opened `feat/media-mvp` → `main` after PR #2 merges. CI green. Manual smoke test against real megaAPI sandbox instance: (a) WA image round-trip, (b) CW pdf round-trip. Reviewer signs off acceptance checklist. Merge.
