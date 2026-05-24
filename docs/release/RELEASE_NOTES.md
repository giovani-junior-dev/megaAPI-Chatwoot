# Release Notes — v1.0.0

Release date: 2026-05-24
Tag: `v1.0.0`
Status: Stable

This is the first stable release of chatwoot-megaapi-bridge. It marks the
completion of feature epics F1 through F5 plus the F6 hardening track and
declares the production contract documented in `docs/VERSIONING.md`.

## TL;DR

- Bidirectional WhatsApp ↔ Chatwoot relay through megaAPI, multi-tenant.
- One-command install (`install.sh`), automated PostgreSQL backups, scripted
  upgrade with auto-rollback on health failure.
- Optional observability overlay (Prometheus, Grafana, AlertManager,
  OpenTelemetry, pprof).
- Security: AES-256-GCM at rest, HMAC-SHA256 on Chatwoot webhooks, gosec gated
  in CI (0 HIGH/CRITICAL).
- Load test harness ready for sustained 24h soak; chaos harness exercises
  container-kill recovery.

## What's in v1.0.0

### F1 — MVP texto

- `POST /v1/wa/{slug}` (Bearer-authed): inbound WhatsApp messages from megaAPI.
- `POST /v1/cw/{slug}` (HMAC-SHA256-authed): outbound message events from Chatwoot.
- Tables: `tenants`, `contacts`, `messages` (idempotency via
  `(tenant_id, direction, external_id)` UNIQUE).
- Crypto primitives in `crypto.go` with constant-time comparisons.

### F2 — Mídia + reliability

- Image / audio / video / document / sticker passthrough using megaAPI
  `POST /rest/instance/downloadMediaMessage/{instance}`.
- In-process retry queue (no Redis, no asynq). Worker pool sized to
  `GOMAXPROCS×4` by default; tunable via `BRIDGE_WORKERS`.
- Error classifier: `retriableError` / `fatalError`. Default is retry.
- DLQ exposed via `/debug/dlq` (admin-gated).
- `RecoverPending` on boot drains in-flight messages — no Redis-style WAL needed.

### F3 — Admin UI

- Wizard for first-time setup.
- Tenant create/edit/disable with masked secrets.
- Message search by tenant + status + direction.
- DLQ viewer + replay.
- Session cookies with `HttpOnly` + `SameSite=Lax` (see `docs/security/AUDIT-REPORT.md`
  for the Secure-flag deployment guidance).
- Admin password as bcrypt cost-12 hash.

### F4 — One-command installer

- `install.sh` walks the operator through domain, admin email, megaAPI
  endpoint, Chatwoot endpoint.
- Generates fresh `BRIDGE_ENCRYPTION_KEY` and `ADMIN_TOKEN` per install.
- Brings up Docker Compose stack (Caddy + bridge + Postgres + backup sidecar).
- `postinstall-check.sh` validates all containers healthy.
- `upgrade.sh` snapshots rollback tag, pre-upgrade backup, runs migrations,
  auto-rollback on `/readyz` failure (5-minute rollback budget).
- Cloudflare Tunnel quick-start via `setup-tunnel.sh`.

### F5 — Observability

- Optional `deploy/observability/` Compose overlay.
- Prometheus scrapes bridge `/metrics`.
- Grafana auto-provisioned with a 5-panel bridge dashboard.
- AlertManager with webhook + email receivers configured.
- `bridge_dlq_total`, `bridge_inbound_duration_seconds`,
  `bridge_outbound_failures_total`, `bridge_worker_active`,
  `bridge_db_pool_in_use` are the headline metrics.
- 3 baseline alert rules (DLQ growth, p99 latency, worker pool exhausted).
- Opt-in OpenTelemetry OTLP/HTTP exporter (`OTEL_EXPORTER_OTLP_ENDPOINT`).
- Admin-gated `/debug/pprof/*` for live profiling.
- 291-line `docs/TROUBLESHOOTING.md` runbook.

### F6 — Hardening + 1.0 release

- `make security-scan` runs gosec; 0 HIGH / 0 CRITICAL findings.
- Dynamic scan harnesses (`nuclei.sh`, `zap.sh`) Docker-driven against a
  live target.
- `make loadtest-smoke` drives k6 with smoke/24h/spike profiles; thresholds
  gate `error_rate<1%`, `p95<300ms`, `p99<500ms`.
- `make chaos-smoke` kills bridge/db/chatwoot containers, verifies `/healthz`
  recovery and DLQ growth tolerance.
- `docs/OPERATIONS.md` (310 lines) — full SRE runbook.
- `docs/VERSIONING.md` (132 lines) — SemVer contract and deprecation cycle.
- `docs/security/AUDIT-REPORT.md` — audit findings + OWASP Top 10 matrix.
- `docs/release/PILOT-PROGRAM.md` — 5-tenant pilot acceptance template.
- `CHANGELOG.md` (Keep a Changelog 1.1.0 format).

## Breaking Changes

None. This is the initial stable release; the 0.x development line is now
declared end-of-life. Operators on 0.x must follow the upgrade path below.

## Upgrade Path from 0.x

0.x releases were development snapshots and did not carry compatibility
guarantees. Treat the 0.x → 1.0.0 hop as a fresh install:

1. Back up the existing 0.x database:
   `pg_dump --format=custom bridge > 0x-final.dump`.
2. Run `install.sh` on the target host. It will bootstrap a fresh
   `BRIDGE_ENCRYPTION_KEY` — DO NOT reuse the 0.x key; tenant secrets must be
   re-entered.
3. For each tenant, run `bridge tenant add` with the original megaAPI and
   Chatwoot credentials.
4. Replay any in-flight messages from the 0.x dump manually via SQL into the
   1.0.0 schema, or accept the gap and let new traffic flow.

Subsequent upgrades within the 1.x line follow the standard `upgrade.sh`
procedure documented in `docs/OPERATIONS.md`.

## Configuration Matrix

| Env var | Default | Required? | Notes |
|---|---|---|---|
| `BRIDGE_DB_DSN` | `postgres://bridge:bridge@db:5432/bridge?sslmode=disable` | Yes | Postgres DSN |
| `BRIDGE_ENCRYPTION_KEY` | — | Yes | 32-byte AES key, base64 |
| `BRIDGE_PORT` | `8080` | No | listen port |
| `BRIDGE_WORKERS` | `GOMAXPROCS×4` | No | worker pool size |
| `BRIDGE_DB_MAX_CONNS` | `25` | No | DB pool size |
| `ADMIN_TOKEN` | — | No | enables `/debug/*` when set |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | — | No | enables OTel tracing |
| `OTEL_SERVICE_NAME` | `chatwoot-megaapi-bridge` | No | OTel resource name |

Full matrix and per-field semantics live in `INSTALL.md`.

## Known Issues

- Session cookie does not set the `Secure` attribute by default. Mitigated by
  deploying behind Caddy with HSTS (default in the supplied template); see
  `docs/security/AUDIT-REPORT.md` F-003. A `BRIDGE_SECURE_COOKIE` flag is
  planned for v1.1.
- Backup retention is fixed at 7 days; off-site replication requires the
  operator to set `BACKUP_REMOTE_TARGET` to an S3-compatible URL.
- The 24h load profile in `k6-bridge.js` and the chaos harness require manual
  triggering against a live staging environment; CI gates only the smoke
  profile.

## Verified Against

- Go 1.23+ (compiled with 1.23.x and 1.24.x).
- PostgreSQL 16.x.
- Docker Engine 25+, Docker Compose v2.
- Caddy 2.7+.
- megaAPI Cloud (current production tenant of upstream provider).
- Chatwoot 3.10+.

## Contributors

- @MadeInLowCode — project lead, F1 through F6 implementation.
- Anthropic Claude — paired authoring across all feature epics.
- megaAPI team — webhook payload + downloadMediaMessage guidance documented in
  `bd remember` notes.

## What's Next

- **Pilot program (F6 sub-issue efn.8).** Five tenants run v1.0.0 for seven
  consecutive days without a SEV1/SEV2 incident. Tracked in
  `docs/release/PILOT-PROGRAM.md`.
- **v1.1 hardening.** Address F-003 with `BRIDGE_SECURE_COOKIE`. Add k6
  scenarios for media-heavy traffic. Document Postgres replica fail-over.
- **v1.2 features.** Tentative: outbound message templating, multi-region
  deploy guidance, automated DLQ replay policies.

## Support and Reporting

- Bug reports: open an issue in beads with `bd create --type=bug` and tag
  `phase-6` if discovered during the pilot.
- Security disclosures: see `SECURITY.md` (private channel preferred).
- Operational questions: `docs/OPERATIONS.md` + `docs/TROUBLESHOOTING.md` cover
  most cases; otherwise reach the on-call rotation listed in
  `docs/OPERATIONS.md`.
