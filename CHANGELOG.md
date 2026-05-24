# Changelog

All notable changes to chatwoot-megaapi-bridge will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- (none)

### Changed
- (none)

### Deprecated
- (none)

### Removed
- (none)

### Fixed
- (none)

### Security
- (none)

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

[Unreleased]: https://github.com/MadeInLowCode/chatwoot-megaapi-bridge/compare/v1.0.0...HEAD
[1.0.0]: https://github.com/MadeInLowCode/chatwoot-megaapi-bridge/releases/tag/v1.0.0
