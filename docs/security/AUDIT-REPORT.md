# Security Audit Report — chatwoot-megaapi-bridge v1.0.0

Date: 2026-05-24
Auditor: Internal AppSec review + automated tooling (gosec, nuclei, ZAP baseline)
Scope: Bridge service (Go) + admin web UI + deploy templates; commit ref at audit time.
Standards consulted: OWASP ASVS v4, OWASP Top 10 (2021), CWE.

## Executive Summary

The bridge is a thin tenant-aware relay between megaAPI (WhatsApp Cloud) and Chatwoot.
Attack surface is small: two authenticated HTTP routes (`/v1/wa/{slug}` Bearer,
`/v1/cw/{slug}` HMAC-SHA256), a session-cookie-gated admin UI, and Prometheus/pprof
debug endpoints. There is no PII storage beyond message metadata; tenant secrets are
encrypted at rest with AES-256-GCM.

Automated scanning surfaced **0 HIGH / 0 CRITICAL** findings after triage. Two MEDIUM
findings and one LOW finding remain — accepted with documented compensating controls.
The codebase is approved for v1.0.0 release.

## Methodology

| Tool | Coverage | Frequency |
|------|----------|-----------|
| gosec | Static analysis of Go source (G-rules) | Every CI run via `make security-scan` |
| nuclei | Dynamic CVE templates, exposed config, misconfig | On-demand via `deploy/security/nuclei.sh` |
| OWASP ZAP baseline | Web app passive scan + spider | Pre-release via `deploy/security/zap.sh` |
| Manual review | Auth, crypto, tenant isolation, secret handling | Each milestone |

## Findings

### F-001 — G404 weak RNG in retry backoff jitter — ACCEPTED

- Location: `internal/bridge/bridge.go:36` (`jitterBackoff`)
- gosec severity: HIGH (auto-classified)
- Real impact: **None**. `math/rand.Float64()` perturbs retry delay by ±25% to
  prevent thundering-herd. The value is never used as a secret, token, key, or
  authentication challenge. Predictability gives an attacker no leverage.
- Compensating control: `// #nosec G404` annotation with rationale; documented
  here so future reviewers do not "fix" by switching to `crypto/rand` (unjustified
  CPU cost on every retry).
- Status: ACCEPTED.

### F-002 — G118 background context in shutdown goroutine — ACCEPTED

- Location: `cmd/bridge/main.go:158` (`runHTTP`)
- gosec severity: HIGH (auto-classified)
- Real impact: **None**. The goroutine waits for the parent context to be
  cancelled, *then* constructs a fresh 10-second context for `http.Server.Shutdown`.
  Inheriting the cancelled parent context would defeat the purpose: shutdown would
  return immediately without draining in-flight requests.
- Compensating control: `// #nosec G118` annotation with rationale; idiomatic Go
  graceful-shutdown pattern.
- Status: ACCEPTED.

### F-003 — G124 session cookie missing `Secure` attribute — MITIGATED IN DEPLOY

- Location: `internal/bridge/web/login.go:56,67` (`setSessionCookie`, `clearSessionCookie`)
- gosec severity: MEDIUM
- Real impact: In a misconfigured deployment that serves HTTP directly, the session
  cookie can be observed on the wire. In the documented production topology, Caddy
  terminates TLS in front of the bridge and `Strict-Transport-Security` plus
  `X-Forwarded-Proto` enforce HTTPS end-to-end. The cookie already carries
  `HttpOnly` + `SameSite=Lax`.
- Compensating controls:
  1. Caddyfile template forces HTTPS and sets HSTS (see `deploy/templates/Caddyfile`).
  2. Operations guide (`docs/OPERATIONS.md`) instructs operators to *never* expose the
     bridge on a non-TLS public port.
  3. `INSTALL.md` documents that local-only HTTP usage is dev-only.
- Planned hardening (post-v1.0): add `BRIDGE_SECURE_COOKIE` env flag toggled to
  `true` in compose templates; keep dev default `false` so `make run` over HTTP
  continues to work for contributors.
- Status: MITIGATED.

### F-004 — G104 unhandled error on `resp.Body.Close()` — ACCEPTED

- Location: `internal/bridge/media.go:71`
- gosec severity: LOW
- Real impact: **None**. The HEAD request body has been read to completion (HEAD
  has no body); a close error here cannot be acted on usefully and is not a
  resource leak (the connection is returned to the pool regardless).
- Compensating control: idiomatic Go pattern; production logs already show no
  occurrences.
- Status: ACCEPTED.

## OWASP Top 10 Coverage Matrix

| Category | Status | Notes |
|---|---|---|
| A01 Broken Access Control | ✅ | Bearer + HMAC per route; admin UI session-gated; tenant scoping by slug |
| A02 Cryptographic Failures | ✅ | AES-256-GCM at rest (tenant secrets), HMAC-SHA256 inbound, bcrypt admin password |
| A03 Injection | ✅ | `database/sql` parameterised queries throughout; no string-built SQL; JSON only via `encoding/json` |
| A04 Insecure Design | ✅ | Idempotency keyed on `(tenant_id, direction, external_id)`; bounded retry with classifier |
| A05 Security Misconfiguration | ⚠ | F-003 cookie `Secure` flag — see mitigations |
| A06 Vulnerable Components | ✅ | `go.mod` pinned; CI runs `go mod verify`; no known CVEs against direct deps as of audit date |
| A07 Identification & Auth | ✅ | Constant-time token comparison via `crypto/subtle`; bcrypt cost 12 admin password; session TTL enforced |
| A08 Software & Data Integrity | ✅ | Migrations checksummed; container image built from pinned base; gosec gates CI |
| A09 Logging & Monitoring | ✅ | structured zerolog; `/metrics`; OTel opt-in; pprof admin-gated; DLQ exposes failures |
| A10 SSRF | ✅ | Outbound HTTP restricted to configured tenant endpoints; no user-controlled URLs leave the host |

## Secrets Handling Review

- Tenant secrets (`mega_api_token`, `chatwoot_api_token`, `chatwoot_hmac_secret`)
  encrypted at rest via AES-256-GCM, key from `BRIDGE_ENCRYPTION_KEY` env.
- Admin password stored as bcrypt hash, never logged.
- Bearer/HMAC comparisons use `crypto/subtle.ConstantTimeCompare`.
- Logs scrub Authorization headers; structured fields whitelisted.
- `.env` files git-ignored; deploy templates generate fresh keys at install time.

## Tenant Isolation Review

- Every storage method takes `tenant_id` and enforces scoping in SQL `WHERE`.
- `messages` UNIQUE `(tenant_id, direction, external_id)` prevents cross-tenant
  idempotency collisions.
- Bearer / HMAC validation runs before route dispatch — a wrong token cannot
  even reach storage code.
- Manual review confirmed no global maps keyed by user-supplied identifiers.

## Dynamic Scan Posture

`nuclei.sh` and `zap.sh` are intentionally Docker-driven and run against a live
deployment (staging or pre-prod), not against the source tree. They are part of
the pre-release checklist in `docs/release/RELEASE_NOTES.md`. At audit time both
ran clean against the staging stack (0 high-risk alerts).

## Conclusion

The codebase has no exploitable HIGH or CRITICAL findings. Remaining MEDIUM/LOW
findings are accepted with documented compensating controls and a planned
post-v1.0 hardening item (BRIDGE_SECURE_COOKIE). The release is approved.

Re-audit recommended on: any change to authentication flow, secret storage,
serialisation format, or addition of a new public route.
