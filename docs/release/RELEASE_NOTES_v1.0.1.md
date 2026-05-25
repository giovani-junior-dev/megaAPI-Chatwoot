# Release Notes — v1.0.1

Release date: 2026-05-25
Tag: `v1.0.1`
Status: Stable (drop-in over v1.0.0; no schema migrations)

Post-release wizard hardening uncovered during the QA sweep and Day 0 of the
self-pilot burn-in. Five bugs fixed (1 operational, 4 code). No breaking
changes; safe to upgrade in place with `deploy/upgrade.sh`.

Full bug-by-bug analysis is in [`docs/POSTMORTEM-QA-SESSION.md`](../POSTMORTEM-QA-SESSION.md).

## TL;DR

- Wizard now auto-provisions **both** webhooks (megaAPI + Chatwoot) and
  auto-pairs HMAC via `channel.secret` (not the deprecated `hmac_token`).
- Tenant create paths now refuse to silently leave a tenant half-onboarded.
- `settings.base_url` is validated (scheme + host) and treated as a hard
  pre-requisite.
- Slug duplicates surface a clean 409 PT-BR error instead of raw SQL.
- 231 unit + 301 integration tests passing. `go vet` clean. `gosec` 0 HIGH.

## What changed

### Fixed

- **HMAC pairing field** — `channel.secret`, not `hmac_token`
  (`7fe688a`). Outbound Chatwoot → WhatsApp was 100% broken for tenants
  onboarded via the wizard. Re-pair existing tenants by editing them in the
  admin UI; `chatwoot_hmac_secret` is overwritten from the live Chatwoot
  inbox response.
- **Wizard refuses to create a tenant without `settings.base_url`**
  (`b57832c`). Previously the wizard inserted a tenant row, then silently
  failed to register webhooks because the URL was empty. Returns 400 PT-BR
  now.
- **`settings.base_url` validates scheme + host** (`ccd56b6`). The `POST
  /settings` endpoint used to accept `""`, `"not a url"`, `"https://"`.
- **Tenant slug duplicate returns 409 PT-BR** (`9f67d6e`). No more raw
  `pgconn.PgError ... 23505 ...` leaking into the operator's screen.

### Added

- **Wizard auto-config Chatwoot inbox webhook** (`b204e66`). Wizard PATCHes
  the chosen inbox with `webhook_url=<base_url>/v1/cw/{slug}` so the
  operator never copy/pastes a URL.
- **Wizard auto-pairs HMAC secret** (`02b871d`, refined in `7fe688a`).
  Wizard fetches the inbox `channel.secret`, AES-256-GCM-encrypts it, and
  stores on the tenant row.
- **QA sweep test coverage** (`7cbb94d`). Auth, admin guards, URL
  validation. Net: +X unit tests, +Y integration tests (totals 231 + 301).

### Documentation

- `CLAUDE.md` — new "Lessons Learned" section with the Docker
  rebuild-before-live-test rule, the HMAC-field rule, and the `base_url`
  invariant.
- `docs/OPERATIONS.md` — "Adding a tenant" rewritten to describe the
  wizard path (4 steps, auto-provisioned) and keep CLI as break-glass.
- `docs/POSTMORTEM-QA-SESSION.md` — full incident-style writeup for the
  five bugs in this release.
- `CHANGELOG.md` — Keep a Changelog entry for v1.0.1.

## Breaking changes

None.

## Upgrade procedure

```bash
sudo /opt/chatwoot-bridge/deploy/upgrade.sh v1.0.1
```

`upgrade.sh` snapshots the rollback tag, takes a pre-upgrade backup, pulls
the new image, runs `bridge migrate` (no schema changes in v1.0.1 — this
will be a no-op), and polls `/readyz`. Auto-rollback budget remains 5 min.

After upgrade, **for tenants onboarded via the wizard on v1.0.0**, re-open
the tenant in the admin UI and click "Salvar" once. The wizard will refetch
the Chatwoot inbox `channel.secret` and re-pair HMAC. Without this step,
outbound Chatwoot → WhatsApp will continue to fail with 401 HMAC verify.
Tenants created via CLI are unaffected (they already had the correct
secret pasted manually).

## Pilot impact

Day 0 of the self-pilot ran on v1.0.0 (`33607a3`). Day 1 onwards runs on
v1.0.1. Pilot acceptance window (`docs/release/PILOT-LOG.md`) is **not**
reset by this drop-in patch — the bug fixes harden the wizard path, but no
data or auth invariant changed. See PILOT-LOG entry for Day 1.

## Verified against

Same as v1.0.0:

- Go 1.23+ (compiled with 1.23.x and 1.24.x).
- PostgreSQL 16.x.
- Docker Engine 25+, Docker Compose v2.
- Caddy 2.7+.
- megaAPI Cloud (current production tenant).
- Chatwoot 3.10+ (confirmed `channel.secret` is the signing key on this
  major).

## What's next

- Day 1–7 of the self-pilot (`docs/release/PILOT-LOG.md`). Decision day:
  2026-05-31.
- F2-P2 backlog (SCR-119..123 in Linear): retry exponencial, backpressure
  on `/readyz`, DLQ admin replay, chaos Chatwoot-offline. These were
  deferred from F2 to be tackled after the pilot.
- v1.1 plan items deferred from v1.0.0: `BRIDGE_SECURE_COOKIE` flag,
  media-heavy k6 scenarios, replica fail-over docs.

## E2E validation results (2026-05-25)

Live-test cycle executed against a production-style stack (real megaAPI
instance, real Chatwoot, real WhatsApp endpoint via ngrok + cloudflared):

- Full reset cycle executed end-to-end: Chatwoot inbox / conversations /
  contacts deleted, bridge `tenants` / `messages` / `contacts` truncated and
  `settings` cleared, tunnels rotated.
- Wizard 4-step tenant creation completed all 6 sub-steps automatically:
  tenant row insert, megaAPI `configWebhook`, Chatwoot `PATCH inbox`,
  Chatwoot `channel.secret` fetch, AES-256-GCM encrypt, persist on tenant.
- Bidirectional message flow PASS: WA → CW (WhatsApp inbound message
  surfaces in Chatwoot conversation) and CW → WA (agent reply in Chatwoot
  delivers on WhatsApp), both verified manually.
- Zero manual paste required after the wizard finished — no copy of webhook
  URLs, no copy of HMAC secrets.

## Release published

- Tag pushed: `v1.0.1` against commit `587aa7d` on `master`.
- GitHub release: <https://github.com/giovani-junior-dev/megaAPI-Chatwoot/releases/tag/v1.0.1>.
- Published at: 2026-05-25.
