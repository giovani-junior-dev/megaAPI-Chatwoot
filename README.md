# chatwoot-megaapi-bridge

Open-source HTTP bridge between **megaAPI** (WhatsApp) and **Chatwoot**, multi-tenant,
flat-first Go implementation. One binary, three tables, in-process channels —
no Redis, no Worker pool service, no microservices.

**Latest stable: [`v1.0.1`](https://github.com/giovani-junior-dev/megaAPI-Chatwoot/releases/tag/v1.0.1)** — wizard hardening (auto-config Chatwoot inbox webhook, HMAC pairing via `channel.secret`, slug duplicate friendly 409, `base_url` validation). Published 2026-05-25, drop-in over v1.0.0, no schema migrations. See [v1.0.1 release notes](./docs/release/RELEASE_NOTES_v1.0.1.md), [CHANGELOG](./CHANGELOG.md), and [v1.0.0 release notes](./docs/release/RELEASE_NOTES.md).

> **Operator note:** before opening the tenant wizard, set `settings.base_url` (Admin → Configurações). The wizard refuses to create a tenant if it is empty or malformed.

## v1.0.x feature set

| Epic | Status | What you get |
|------|--------|--------------|
| F1 — MVP texto | Done | Bidirectional WhatsApp ↔ Chatwoot text, multi-tenant, Bearer/HMAC, idempotency |
| F2 — Mídia + reliability | Done | Image / audio / video / document / sticker + GIF + multi-attachment, in-process retry queue, DLQ |
| F3 — Admin UI | Done | Tenant wizard (4 steps, auto-provisions both webhooks), message log, DLQ, diagnostic 1-click, pt-BR |
| F4 — One-command installer | Done | `install.sh` interactive, Caddy or Cloudflare Tunnel, backup sidecar, `upgrade.sh` with auto-rollback |
| F5 — Observability | Done | Prometheus + Grafana + AlertManager overlay, OTel opt-in, admin-gated `/debug/pprof` |
| F6 — Hardening + 1.0 | Done (gates) / self-pilot in progress | gosec/nuclei/ZAP harness, k6 smoke/24h/spike, chaos kill-and-recover, full runbooks |

## Quickstart (5 commands)

```bash
cp .env.example .env
echo "MASTER_KEY=$(openssl rand -base64 32)" >> .env
docker compose up -d --build
docker compose exec bridge /bridge migrate
docker compose exec bridge /bridge tenant add \
  --slug demo \
  --megaapi-host https://apibusiness7.megaapi.com.br \
  --megaapi-instance YOUR_INSTANCE \
  --megaapi-token YOUR_MEGA_TOKEN \
  --chatwoot-url https://your-chatwoot.example.com \
  --chatwoot-token YOUR_CW_TOKEN \
  --chatwoot-account 1 \
  --chatwoot-inbox 5
```

The `tenant add` command prints a **Webhook Bearer** (configure on megaAPI) and
an **HMAC Secret** (configure on Chatwoot webhook integration).

## Endpoints

| Method | Path                | Auth                      |
|--------|---------------------|---------------------------|
| `POST` | `/v1/wa/{slug}`     | `Authorization: Bearer …` |
| `POST` | `/v1/cw/{slug}`     | `X-Chatwoot-Signature: …` |
| `GET`  | `/healthz`          | _none_                    |
| `GET`  | `/readyz`           | _none_                    |

## Architecture

```
megaAPI ──▶ POST /v1/wa/{slug}  ─▶ DB.InsertMessage(pending) ─▶ inboxChan ──▶ workerPool ─▶ Chatwoot REST
Chatwoot ─▶ POST /v1/cw/{slug}  ─▶ DB.InsertMessage(pending) ─▶ outboxChan ─▶ workerPool ─▶ megaAPI sendMessage
```

- **1 package** `internal/bridge/` — server, storage, crypto, bridge core
- **1 binary** `bridge` — subcommands `serve`, `migrate`, `tenant add`
- **3 tables** `tenants`, `contacts`, `messages` (idempotency via `UNIQUE(tenant_id, direction, external_id)`)
- **AES-256-GCM** at-rest for all tokens, **HMAC-SHA256** for inbound signatures

## Development

```bash
make test          # unit tests
make integration   # testcontainers (Docker required)
make lint          # go vet + golangci-lint
make build         # static binary
```

Go 1.25+ required (the `testcontainers-go` indirect dep sets the floor).

## License

MIT — see [LICENSE](./LICENSE).
