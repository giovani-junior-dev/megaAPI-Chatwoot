# Operations Runbook — chatwoot-megaapi-bridge

Audience: SREs and operators running the bridge in production. Pairs with
`docs/TROUBLESHOOTING.md` (symptom → diagnosis) and `docs/INSTALL.md` (first install).

This runbook covers: deploy, upgrade, backup, restore, rollback, monitoring,
scaling, incident response. Every command assumes the bridge is deployed via the
provided Docker Compose stack at `/opt/chatwoot-bridge` unless noted otherwise.

## Table of Contents

1. [Topology Overview](#topology-overview)
2. [Deploy](#deploy)
3. [Upgrade](#upgrade)
4. [Backup](#backup)
5. [Restore](#restore)
6. [Rollback](#rollback)
7. [Troubleshooting Entry Points](#troubleshooting-entry-points)
8. [Monitoring](#monitoring)
9. [Scaling](#scaling)
10. [Incident Response](#incident-response)
11. [Routine Maintenance](#routine-maintenance)
12. [Security Operations](#security-operations)

## Topology Overview

```
[megaAPI Cloud] ──webhook──► [Caddy :443] ──► [bridge :8080] ──► [Chatwoot]
                                                  │
                                                  ├──► [postgres:5432]
                                                  └──► [/metrics] scraped by Prometheus
```

- `bridge` — Go service, stateless, horizontally scalable behind Caddy.
- `db` — PostgreSQL 16, single primary; tables: `tenants`, `contacts`, `messages`.
- `caddy` — TLS termination, ACME, HTTP/3, reverse proxy.
- `postgres-backup` — sidecar that runs `pg_dump` on a cron schedule.
- `chatwoot` — external dependency, may be co-located or remote.

All services run in the `bridge-net` overlay network; only Caddy publishes ports.

## Deploy

### Fresh install (operator workflow)

```bash
ssh ops@new-host
curl -fsSL https://raw.githubusercontent.com/.../install.sh -o install.sh
sudo bash install.sh
# Follow interactive prompts: domain, admin email, megaAPI URL, Chatwoot URL.
```

The installer:
1. Validates Docker + Compose version.
2. Renders templates from `deploy/templates/` with operator answers.
3. Generates fresh `BRIDGE_ENCRYPTION_KEY` and `ADMIN_TOKEN`.
4. Brings up the stack with `docker compose up -d`.
5. Runs `deploy/postinstall-check.sh` to verify all containers are healthy.

### Adding a tenant

```bash
docker compose exec bridge bridge tenant add \
  --slug demo \
  --mega-instance-id <megaapi-instance> \
  --mega-token <megaapi-token> \
  --chatwoot-url https://chatwoot.example.com \
  --chatwoot-token <chatwoot-api-token> \
  --chatwoot-hmac-secret <hmac>
```

Webhook URL to register in megaAPI:
`https://bridge.example.com/v1/wa/demo?token=<bridge-bearer>`

## Upgrade

Always run `deploy/upgrade.sh`. It:
1. Snapshots the current Docker image tag and writes it to `/var/lib/bridge/rollback.tag`.
2. Triggers `postgres-backup` to take an immediate pre-upgrade dump.
3. Pulls the new image and runs migrations via `bridge migrate`.
4. Recreates the bridge container with `docker compose up -d bridge`.
5. Polls `/readyz` for 30s — non-200 triggers automatic rollback.

```bash
sudo /opt/chatwoot-bridge/deploy/upgrade.sh v1.0.1
```

Manual upgrade (advanced):

```bash
docker compose pull bridge
docker compose run --rm bridge bridge migrate
docker compose up -d bridge
curl -fsS https://bridge.example.com/readyz
```

## Backup

Automated: `postgres-backup` sidecar runs `pg_dump --format=custom` every
6 hours and rotates with 7-day retention into `/var/backups/bridge/`.

Manual snapshot before risky operations:

```bash
docker compose exec postgres-backup /backup.sh now
```

What's backed up:
- Full `bridge` database (`tenants`, `contacts`, `messages`).
- `/opt/chatwoot-bridge/.env` and `/opt/chatwoot-bridge/deploy/.env.bridge`.
- Generated config templates under `/opt/chatwoot-bridge/deploy/`.

What's **not** backed up automatically:
- Docker images themselves (re-pulled from registry).
- Caddy state (regenerated on restart; certificates re-issued via ACME).
- Local audit logs (`/var/log/bridge.log` rotates via logrotate).

Off-site copy: configure `BACKUP_REMOTE_TARGET=s3://...` in `.env.bridge`.

## Restore

From the most recent backup:

```bash
docker compose stop bridge
docker compose exec postgres-backup /restore.sh latest
docker compose up -d bridge
```

From a specific dump file:

```bash
docker compose stop bridge
docker compose exec postgres-backup /restore.sh /var/backups/bridge/bridge-20260524-0600.dump
docker compose up -d bridge
```

Verify after restore:

```bash
docker compose exec bridge bridge tenant list   # counts match expectation
curl -fsS https://bridge.example.com/readyz
```

## Rollback

If `upgrade.sh` failed health checks, it already rolled back automatically.

To rollback manually after a successful but problematic upgrade:

```bash
PREVIOUS_TAG=$(cat /var/lib/bridge/rollback.tag)
docker compose stop bridge
# Restore DB if the new release ran destructive migrations (rare; check release notes)
docker compose exec postgres-backup /restore.sh pre-upgrade
# Pin the previous image
sed -i "s|image: bridge:.*|image: bridge:${PREVIOUS_TAG}|" docker-compose.yml
docker compose up -d bridge
```

Rollback budget: 5 minutes from decision to /readyz=200.

## Troubleshooting Entry Points

| Symptom | First check |
|---|---|
| `/healthz` 500 | container logs; DB reachability |
| `/readyz` 503 | DB migration state; Chatwoot connectivity |
| Inbound webhook rejected 401 | Bearer token mismatch in tenant row |
| Outbound to Chatwoot 401 | tenant `chatwoot_api_token` expired |
| Outbound to megaAPI 5xx | megaAPI status page; DLQ growth |
| Messages stuck in `messages` with status=pending for >5m | worker restart loop; check goroutine count via /debug/pprof/goroutine |
| Sudden p99 spike | `/metrics`: `bridge_inbound_duration_bucket`; postgres slow query log |

Full symptom matrix lives in `docs/TROUBLESHOOTING.md`.

## Monitoring

### Endpoints

- `/healthz` — liveness, returns 200 if process is up.
- `/readyz` — readiness, returns 200 only if DB ping succeeds within 1s.
- `/metrics` — Prometheus exposition format.
- `/debug/pprof/*` — admin-gated profiling (set `ADMIN_TOKEN` to enable).

### Key metrics to alert on

| Metric | Threshold | Action |
|---|---|---|
| `bridge_dlq_total` rate>0 | warn after 5 in 10m | inspect DLQ table |
| `bridge_inbound_duration_seconds` p99>500ms | warn for 10m | check Chatwoot latency |
| `bridge_outbound_failures_total` rate>1/s | page | upstream outage |
| `bridge_worker_active` ==0 | page | worker pool crashed |
| `bridge_db_pool_in_use` >0.9 of max | warn | scale db pool / investigate slow queries |

Pre-built alerts live in `deploy/observability/prometheus/alerts.yml`.
Dashboard JSON: `deploy/observability/grafana/dashboards/bridge.json`.

### Logs

Structured JSON via zerolog. Useful queries:

```bash
docker compose logs bridge | jq 'select(.level=="error")'
docker compose logs bridge | jq 'select(.tenant_id=="demo")'
docker compose logs bridge | jq 'select(.event=="dlq_enqueue")'
```

## Scaling

The bridge is stateless. Horizontal scale via `docker compose up -d --scale bridge=N`
behind Caddy load balancing. Bottlenecks in order of likelihood:

1. **PostgreSQL connection pool** — increase `BRIDGE_DB_MAX_CONNS` (default 25).
2. **Chatwoot inbox API rate limits** — coalesce by tenant or request rate-limit lift.
3. **megaAPI instance quotas** — out of bridge's control; communicate with megaAPI ops.
4. **Worker goroutine count** — `BRIDGE_WORKERS` env (default GOMAXPROCS×4).

Vertical signals before scaling out:
- `bridge_db_pool_in_use` consistently >0.7.
- `bridge_worker_queue_depth` p95>50.
- p99 latency creep with stable RPS.

## Incident Response

### Severity matrix

| Sev | Definition | Response time |
|---|---|---|
| SEV1 | All tenants down or data loss risk | immediate page; bridge call within 5m |
| SEV2 | Single tenant fully down, or partial cluster impact | 15m response |
| SEV3 | Elevated error rate, no full outage | next business hour |
| SEV4 | Cosmetic / non-customer-facing | standard backlog |

### SEV1 playbook

1. Acknowledge page; declare on the ops channel.
2. Snapshot diagnostics: `docker compose ps`, `/metrics` dump, last 1000 log lines.
3. If DB unreachable → fail over to read replica or restore from latest dump.
4. If bridge crash-looping → check OOM, then disable that tenant's webhook in
   Caddy (`@badtenant` matcher) to isolate.
5. Decide rollback vs. forward-fix within 15 minutes.
6. Post-incident: write postmortem within 72h, file remediation issues with
   `bd create --label=incident-followup`.

### SEV2 playbook

1. Identify blast radius (which tenant, what fraction of messages).
2. Drain DLQ manually if backlog grows: `bridge dlq replay --tenant <slug>`.
3. Communicate ETA to affected tenant operator.

### Communication

- Internal status: ops Slack channel `#bridge-ops`.
- External: status page entry within 30m of SEV1/SEV2 declaration.
- Customer: tenant operators paged via PagerDuty service `bridge-tenant-impact`.

## Routine Maintenance

### Weekly

- Review `bd ready --label=phase-6 --label=routine` for hygiene items.
- Inspect Grafana dashboard for slow drift in p95/p99.
- Confirm latest backup restored cleanly in staging (smoke run).

### Monthly

- Patch base images: `docker compose pull && docker compose up -d`.
- Rotate `ADMIN_TOKEN` via `.env.bridge` and restart bridge.
- Audit tenant list: `bridge tenant list --inactive 90d` and disable stale tenants.

### Quarterly

- Re-run `make security-scan` and review `security-reports/`.
- Re-run `deploy/security/zap.sh` against staging.
- Restore drill: full backup → fresh host → verify business-critical flows.

## Security Operations

- Rotate `BRIDGE_ENCRYPTION_KEY`: requires re-encrypting tenant secrets — use
  `bridge tenant rekey` (offline command, locks tenants table briefly).
- Compromised tenant token: `bridge tenant rotate-token --slug <slug>` regenerates
  the inbound bearer; coordinate with tenant operator to update megaAPI webhook URL.
- Admin password reset: `bridge admin reset-password` (requires shell access).
- Audit access: `/debug/pprof` requires `ADMIN_TOKEN` cookie session; review
  Caddy access log for `/debug/*` hits.

### Disabling a tenant in an emergency

```bash
docker compose exec bridge bridge tenant disable --slug <slug>
```

Disabled tenants reject all inbound webhooks with 410 Gone and stop dispatching
outbound. Messages already in the queue are NOT cancelled — drain them first
with `bridge dlq replay` or let them retire to DLQ.

### Out-of-band access for support

Never share `BRIDGE_ENCRYPTION_KEY` or the bcrypt admin hash. Support sessions
should:
1. SSH bastion with audited shell history.
2. `docker compose exec` rather than direct DB connections where possible.
3. Use read-only DB credentials (`bridge_readonly` role) for diagnostics.

---

Owner: Bridge SRE on-call rotation.
Last reviewed: 2026-05-24 (v1.0.0 release readiness).
Re-review trigger: any change to the deploy topology, secret model, or worker model.
