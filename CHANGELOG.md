# Changelog

All notable changes to chatwoot-megaapi-bridge will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- (none)

### Changed
- (none)

### Fixed
- (none)

## [1.1.2] - 2026-06-05

### Fixed
- **Chatwoot -> bridge webhook authentication.** The `/v1/cw/{slug}` endpoint
  required an `X-Chatwoot-Signature` HMAC header, but Chatwoot 4.x does not sign
  the API-channel outgoing webhook, so every outbound (agent reply) message
  failed with 401 ("Falha ao enviar"). The bridge now authenticates by a token
  in the webhook URL (`?token=`), matched in constant time against the tenant
  secret — the same pattern already used for the megaAPI direction. The HMAC
  header remains accepted as a fallback when present. The wizard now appends the
  token to the Chatwoot inbox `webhook_url`.
- **Tenant slug validation.** The wizard rejected invalid slugs only at the DB
  check constraint (opaque 500); it now returns a friendly PT-BR 400 mirroring
  `^[a-z0-9][a-z0-9-]{2,63}$`.

## [1.1.1] - 2026-06-02

Swarm deploy enablement. No app behaviour change; drop-in over 1.1.0.

### Added
- **Boot-time DB bootstrap + migrations** for swarm installs. `bridge serve`
  auto-creates the `bridge` role/database when `POSTGRES_ADMIN_URL` is set
  (idempotent) and runs pending migrations unless `RUN_MIGRATIONS=0`.
- **`install-pack/`** — numbered Docker Swarm stacks (infra, Portainer,
  Postgres, Cloudflare Tunnel, Chatwoot v4.14.1-ce, bridge, backup) plus a
  PT-BR step-by-step guide. Cloudflare Tunnel routing, no Traefik, local
  storage.
- **`publish-image` GitHub Action** builds and pushes the bridge image to
  `ghcr.io/giovani-junior-dev/chatwoot-megaapi-bridge` on every `v*` tag.

### Fixed
- GHCR image namespace corrected from `madeinlowcode` to the repo owner
  `giovani-junior-dev` so the Actions `GITHUB_TOKEN` can publish it.

## [1.1.0] - 2026-06-01

Admin UI overhaul (design system + responsive shell) plus the self-service
pairing flow. No breaking changes; drop-in over 1.0.1. Schema migration
`0004_pairing.sql` is required (adds `tenants.paired_at`, `tenants.last_jid`).

### Added
- **Admin shell redesign (phases 1-5).** Full visual overhaul of the admin UI
  against `DESIGN.md` (OKLCH tokens, Inter, restrained palette, accent hue 250):
  - **Shell.** Vertical sidebar (240px) with active-item `aria-current`, brand,
    user card + logout. Off-canvas drawer under 1024px via Alpine, hamburger
    trigger. CSS design tokens declared inline; `prefers-reduced-motion`
    honoured. New `HideShell` / `ActivePath` / `Email` page fields.
  - **Login + Painel.** Reusable template partials (`button`, `badge`, `empty`,
    `datatable`). Login carries product identity; dashboard tenant table
    collapses to cards under 768px with status badges and an empty state.
  - **Mensagens + DLQ.** Responsive tables, status/direction badges, ghost-button
    pagination, tenant `datalist`, inline Alpine confirm on DLQ retry, DLQ
    errors rendered in `--danger`.
  - **Settings + Wizard.** Settings save feedback as an auto-dismissing toast.
    Wizard gains a real vertical stepper (`data-state` active/completed/upcoming)
    with per-step validation and a review summary.
  - **Polish.** SVG-data-URI favicon, meta description, global `aria-live` toast
    region, skeleton classes, `g`+`d/m/q/s` keyboard navigation, skip-link, and
    `:focus-visible` outlines across interactive elements.
- **Self-service WhatsApp pairing flow.** Admins generate a per-tenant
  HMAC-signed link (`/pair/{slug}?t=<hmac>&exp=<unix>`, 24h TTL) and share it
  with the end customer. The pairing page exposes both QR code and pairing-code
  flows, polls every 3s, and surfaces the live connection state. megaAPI tokens
  never leave the server.
- Migration `0004_pairing.sql` adds `tenants.paired_at` and `tenants.last_jid`.
- Webhook handler now reacts to `messageType=connection_update` with
  `message=phone_connected` and persists pairing state.

### Changed
- Admin dashboard lists each tenant's pairing link with copy-to-clipboard and
  last-paired timestamp.
- Admin pages no longer use the central `max-w-6xl` container; content uses the
  full shell width with a fixed sidebar.

### Security
- Pairing endpoints require HMAC-SHA256 over `slug|exp` keyed with
  `BRIDGE_ENCRYPTION_KEY`. Missing / invalid / expired token returns 401.
  Slug alone is insufficient to access pairing controls.

### Notes
- Tailwind is still loaded via CDN (no build step added). UI partials live in
  `internal/bridge/web/templates/partials/`; golden snapshots cover the
  rendered layout and pages.

## [1.0.1] - 2026-05-25

Post-release wizard hardening uncovered during QA sweep + the first day of the
self-pilot burn-in. No schema migrations; safe drop-in over v1.0.0.

Tag pushed and GitHub release published 2026-05-25:
<https://github.com/giovani-junior-dev/megaAPI-Chatwoot/releases/tag/v1.0.1>.

### Added

- **Wizard auto-config Chatwoot inbox webhook** (commit `b204e66`). On tenant
  create the bridge now PATCHes the Chatwoot API inbox so the operator no
  longer has to paste a webhook URL by hand.
- **Wizard auto-pairs HMAC secret** (commit `02b871d`). Bridge reads the
  Chatwoot inbox `channel.secret` after create, encrypts it with
  `BRIDGE_ENCRYPTION_KEY`, and persists it on the tenant row — no manual
  copy/paste of the HMAC token.
- **QA sweep test coverage** (commit `7cbb94d`). Expanded unit tests around
  auth, admin guards, and URL validation. Total now: **231 unit tests + 301
  integration tests** passing.
- **Validated end-to-end on a production-style environment with a full reset
  cycle** (2026-05-25). Chatwoot inbox/conversations/contacts cleared, bridge
  `tenants`/`messages`/`contacts`/`settings` truncated, tunnels rotated, new
  tenant onboarded via the 4-step wizard against a real megaAPI instance.
  WA→CW and CW→WA both PASS with zero manual paste after the wizard.

### Fixed

- **Tenant slug duplicate now returns 409 + PT-BR friendly message**
  (commit `9f67d6e`). Previously a `pgconn.PgError` with code `23505` leaked
  raw SQL details (BUG-002). Wizard now maps the unique-violation to a clean
  user-facing error.
- **`settings.base_url` is validated for scheme + host** (commit `ccd56b6`).
  The wizard rejected silently when `base_url` was missing or malformed
  (BUG-QA-01); it now returns a 400 with a PT-BR explanation.
- **Wizard refuses to create a tenant when `settings.base_url` is empty**
  (commit `b57832c`). Closes BUG-WIZARD-01 — the previous behaviour silently
  skipped Chatwoot inbox provisioning and left the tenant half-onboarded.
- **HMAC pairing uses Chatwoot `channel.secret`, not the deprecated
  `hmac_token` field** (commit `7fe688a`). Closes BUG-HMAC-01 — the bridge
  was reading a field that Chatwoot no longer uses to sign
  `api_inbox_webhook` events, so signature verification failed for every
  inbound CW message.

### Notes

- Operational lesson captured in `CLAUDE.md`: rebuild the Docker image before
  every live-test cycle (BUG-001, non-code). Stale containers were the root
  cause of multiple "fix doesn't seem to work" loops during the QA sweep.
- `docs/POSTMORTEM-QA-SESSION.md` collects the five bugs above with timeline,
  root cause, and remediation.

## [1.0.0] - 2026-05-24

### Added

- **F1 — MVP texto.** Bidirectional text messages between megaAPI (WhatsApp Cloud)
  and Chatwoot, with tenant isolation, Bearer auth on `/v1/wa/{slug}`, HMAC-SHA256
  auth on `/v1/cw/{slug}`, idempotency via `(tenant_id, direction, external_id)`.
- **F2 — Mídia + reliability.** Image/audio/video/document/sticker passthrough
  using megaAPI `downloadMediaMessage`. In-process retry queue with classifier
  (`retriableError` / `fatalError`). DLQ exposed via `/debug/dlq`. `RecoverPending`
  on boot drains in-flight rows.
- **F3 — Admin UI.** Session-cookie-gated web UI for tenant onboarding, message
  search, DLQ inspection, and settings. bcrypt admin password storage.
- **F4 — One-command installer.** `install.sh` with interactive prompts,
  template rendering, automatic Caddy + Postgres + bridge bring-up.
- **F4 — Backup sidecar.** `postgres-backup` runs `pg_dump --format=custom`
  every 6 hours with 7-day retention.
- **F4 — Upgrade script.** `deploy/upgrade.sh` snapshots rollback tag, pre-upgrade
  backup, runs migrations, auto-rollback on `/readyz` failure.
- **F5 — Observability stack.** Optional `deploy/observability/` overlay
  (Prometheus + Grafana + AlertManager). 5-panel Grafana dashboard. 3 Prometheus
  alert rules wired to AlertManager.
- **F5 — OpenTelemetry.** Opt-in OTLP/HTTP tracer initialised when
  `OTEL_EXPORTER_OTLP_ENDPOINT` is set.
- **F5 — pprof debug.** Admin-gated `/debug/pprof/*` for live profiling.
- **F5 — Troubleshooting runbook.** `docs/TROUBLESHOOTING.md` (291 lines) covering
  bridge, /healthz, /readyz, webhooks, DLQ, metrics, OTel.
- **F6 — Security scan harness.** `make security-scan` runs gosec; `nuclei.sh`
  and `zap.sh` Docker-driven for dynamic scans.
- **F6 — Load test harness.** `deploy/loadtest/k6-bridge.js` with smoke/24h/spike
  profiles and threshold gating.
- **F6 — Chaos harness.** `deploy/chaos/chaos.sh` kills bridge/db/chatwoot,
  verifies `/healthz` recovery and DLQ tolerance.
- **F6 — Operations runbook.** `docs/OPERATIONS.md` (310 lines) covering deploy,
  upgrade, backup, restore, rollback, monitoring, scaling, incident response.
- **F6 — Versioning policy.** This file + `docs/VERSIONING.md`.
- **F6 — Security audit report.** `docs/security/AUDIT-REPORT.md` documenting
  4 gosec findings with OWASP Top 10 coverage matrix.
- **F6 — Release notes.** `docs/release/RELEASE_NOTES.md` for v1.0.0.
- **F6 — Pilot program.** `docs/release/PILOT-PROGRAM.md` template for 5-tenant
  7-day pilot.

### Security

- All tenant secrets encrypted at rest with AES-256-GCM
  (`BRIDGE_ENCRYPTION_KEY`).
- Bearer + HMAC comparisons use `crypto/subtle.ConstantTimeCompare`.
- Admin password stored as bcrypt cost-12 hash.
- 0 HIGH / 0 CRITICAL gosec findings as of v1.0.0 cut.

### Notes

- This is the initial stable release. The 0.x line is now EOL.
- See `docs/release/RELEASE_NOTES.md` for the full upgrade story from 0.x and
  for the supported configuration matrix.

[Unreleased]: https://github.com/MadeInLowCode/chatwoot-megaapi-bridge/compare/v1.1.2...HEAD
[1.1.2]: https://github.com/MadeInLowCode/chatwoot-megaapi-bridge/compare/v1.1.1...v1.1.2
[1.1.1]: https://github.com/MadeInLowCode/chatwoot-megaapi-bridge/compare/v1.1.0...v1.1.1
[1.1.0]: https://github.com/MadeInLowCode/chatwoot-megaapi-bridge/compare/v1.0.1...v1.1.0
[1.0.1]: https://github.com/MadeInLowCode/chatwoot-megaapi-bridge/compare/v1.0.0...v1.0.1
[1.0.0]: https://github.com/MadeInLowCode/chatwoot-megaapi-bridge/releases/tag/v1.0.0
