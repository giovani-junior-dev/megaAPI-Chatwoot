# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Mandatory Skills & Tools

- **`/code-craftsman`** — always ON. Enforce TDD, clean code, flat architecture every session.
- **`chatwoot-api` skill** — use for ALL Chatwoot API questions. Has full API reference.
- **context7 MCP** — use for Chatwoot library/framework docs (`mcp__plugin_context7_context7__query-docs`). Never guess API shape.
- **megaAPI questions** — ask the user. They built megaAPI and have authoritative answers on payload shapes, endpoints, auth behavior.

## Project Instructions for AI Agents

<!-- BEGIN BEADS INTEGRATION v:1 profile:minimal hash:ca08a54f -->
## Beads Issue Tracker

This project uses **bd (beads)** for issue tracking. Run `bd prime` to see full workflow context and commands.

### Quick Reference

```bash
bd ready              # Find available work
bd show <id>          # View issue details
bd update <id> --claim  # Claim work
bd close <id>         # Complete work
```

### Rules

- Use `bd` for ALL task tracking — do NOT use TodoWrite, TaskCreate, or markdown TODO lists
- Run `bd prime` for detailed command reference and session close protocol
- Use `bd remember` for persistent knowledge — do NOT use MEMORY.md files

## Session Completion

**When ending a work session**, you MUST complete ALL steps below. Work is NOT complete until `git push` succeeds.

**MANDATORY WORKFLOW:**

1. **File issues for remaining work** - Create issues for anything that needs follow-up
2. **Run quality gates** (if code changed) - Tests, linters, builds
3. **Update issue status** - Close finished work, update in-progress items
4. **PUSH TO REMOTE** - This is MANDATORY:
   ```bash
   git pull --rebase
   bd dolt push
   git push
   git status  # MUST show "up to date with origin"
   ```
5. **Clean up** - Clear stashes, prune remote branches
6. **Verify** - All changes committed AND pushed
7. **Hand off** - Provide context for next session

**CRITICAL RULES:**
- Work is NOT complete until `git push` succeeds
- NEVER stop before pushing - that leaves work stranded locally
- NEVER say "ready to push when you are" - YOU must push
- If push fails, resolve and retry until it succeeds
<!-- END BEADS INTEGRATION -->


## Build & Test

```bash
make test          # unit tests (no Docker required)
make integration   # testcontainers-based DB tests (Docker required)
make lint          # go vet + golangci-lint
make build         # static binary -> ./bridge
make run           # build + run `bridge serve`
```

Go 1.23+ required. CI gate: `make lint test`.

## Architecture Overview

Flat-first MVP — see [`.agents/plans/reset-mvp.md`](.agents/plans/reset-mvp.md)
for the deliberate scope-reduction decisions.

- **1 package** `internal/bridge/` — server, storage, crypto, bridge core
  (no `internal/server`, no `internal/queue`, no premature subdomain split).
- **1 binary** `cmd/bridge/` with subcommands `serve`, `migrate`, `tenant add`.
- **3 tables** `tenants`, `contacts`, `messages`. Idempotency lives in the
  `messages` UNIQUE `(tenant_id, direction, external_id)` — no `idempotency_keys`
  table.
- **In-process channels** for the worker queue (no Redis, no asynq). Restart
  recovery uses `RecoverPending` over `messages.payload` (Deviation 1).
- **AES-256-GCM** at-rest for tenant secrets; **HMAC-SHA256** for inbound
  Chatwoot webhook signatures.

Two HTTP routes drive everything: `POST /v1/wa/{slug}` (Bearer-authed,
megaAPI → Chatwoot) and `POST /v1/cw/{slug}` (HMAC-authed, Chatwoot → megaAPI).
Health: `/healthz`, `/readyz`.

## Conventions & Patterns

Hard constraints from the reset-MVP plan — do not violate without updating
the plan first:

- **No speculative interfaces.** `Repository`, `Service`, `Manager` placeholders
  are forbidden until there is a second concrete implementation.
- **Concrete types end-to-end.** `*bridge.DB`, `*bridge.Server` — no
  `interface{}` boundaries that exist "just in case".
- **Functions ≤ 2 params** unless `ctx context.Context` plus idiomatic args
  (e.g. handlers, struct receivers).
- **`bridge` is one package** until a second consumer of its types appears.
  Don't split into subpackages on aesthetic grounds.
- **No comments without a WHY.** Identifier names already say _what_; comments
  should explain non-obvious invariants, hidden constraints, or workarounds.
- **No backwards-compat shims.** This is pre-1.0 — change the call site, don't
  keep a renamed wrapper around.
- **Errors:** `retriableError` / `fatalError` sentinels; default is retry.
  Wrap with `notRetriable(err)` to short-circuit the retry loop.
- **Crypto primitives** (`Encrypt`, `Decrypt`, `VerifyHMAC`) live in
  `crypto.go` and use `crypto/subtle` for comparisons.

## Lessons Learned (post-v1.0.0 session, 2026-05-24 → 2026-05-25)

These were paid for in real debugging time. Read them before live-testing.

- **Rebuild the Docker image before every live-test cycle.** A stale
  `bridge` container will silently run a previous binary even after `git pull`
  and `docker compose restart`. Use
  `docker compose build bridge && docker compose up -d bridge` (or
  `docker compose up -d --build bridge`) every time. Multiple "fix didn't
  apply" loops in the QA sweep traced back to this.
- **HMAC pairing uses Chatwoot `channel.secret`, NOT `hmac_token`.** Chatwoot
  3.x signs `api_inbox_webhook` events with the inbox-level `channel.secret`
  field on `GET /api/v1/accounts/{id}/inboxes/{id}`. The deprecated top-level
  `hmac_token` is no longer used. The wizard fetches `channel.secret` after
  inbox provisioning, encrypts it with `BRIDGE_ENCRYPTION_KEY`, and persists
  on the tenant row. Do not "fix" this back to `hmac_token`.
- **`settings.base_url` is required before any tenant is created.** The
  wizard now hard-fails with a PT-BR 400 if it is empty (and validates
  scheme + host when present). Without it the bridge cannot register
  `api_inbox_webhook` on Chatwoot — silent skip used to leave tenants
  half-onboarded.
- **Slug duplicates surface `pgconn.PgError.Code == "23505"`.** Map it to a
  409 with a clean PT-BR message in the wizard; do not let the raw SQL
  error escape to the operator.
