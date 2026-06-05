# Install — Docker Swarm

Deploy `chatwoot-megaapi-bridge` as a single-replica Swarm service that
self-provisions its database on first boot and ingresses through Cloudflare.

## Prerequisites

- A Swarm manager node (`docker swarm init` already run).
- An external overlay network shared with your Postgres + Chatwoot stack:
  ```bash
  docker network create --driver overlay --attachable network_swarm_public
  ```
- A Postgres service reachable as `postgres:5432` on that network.
- The published image `ghcr.io/giovani-junior-dev/chatwoot-megaapi-bridge:v1.1.3`
  (produced by the `publish-image` workflow on every `v*` tag).

## 1. Secrets / environment

The stack reads secrets from the deploying shell — nothing sensitive lives in
`bridge.yaml`. Export them on the manager before deploy:

```bash
export MASTER_KEY="$(openssl rand -base64 32)"       # 32-byte base64 AES key (authoritative)
export BRIDGE_ENCRYPTION_KEY="$MASTER_KEY"            # kept aligned with MASTER_KEY
export POSTGRES_PASSWORD="<bridge role password>"     # used in DATABASE_URL

# FIRST DEPLOY ONLY — lets the bridge create its own role + database:
export POSTGRES_ADMIN_URL="postgres://postgres:<admin-pw>@postgres:5432/postgres"
```

> `MASTER_KEY` is the key the binary actually reads to encrypt tenant secrets.
> `BRIDGE_ENCRYPTION_KEY` is carried for install-convention parity; keep it
> equal to `MASTER_KEY`.

## 2. First boot — auto-bootstrap

With `POSTGRES_ADMIN_URL` set, the bridge connects to the maintenance database,
then idempotently creates the `bridge` role and `bridge` database (parsed from
`DATABASE_URL`) if they are absent. `RUN_MIGRATIONS=1` (default) applies the
embedded migrations before the HTTP server starts.

```bash
docker stack deploy -c install-pack/06-bridge-admin/bridge.yaml bridge
docker service logs -f bridge_bridge   # expect "database bootstrap complete" then "listening"
```

Once the role + database exist, **unset `POSTGRES_ADMIN_URL` and redeploy** so
the service no longer needs admin credentials:

```bash
unset POSTGRES_ADMIN_URL
docker stack deploy -c install-pack/06-bridge-admin/bridge.yaml bridge
```

Subsequent boots skip bootstrap (no admin URL) and re-run migrations harmlessly
(every migration is `IF NOT EXISTS` / idempotent). Set `RUN_MIGRATIONS=0` to
disable the boot-time migrate step.

## 3. Ingress — Cloudflare

The stack ships **no Traefik labels**. Route a hostname to the service through
your Cloudflare tunnel / ingress, targeting the in-overlay service port:

```yaml
# cloudflared config.yml
ingress:
  - hostname: bridge.DOMINIO
    service: http://bridge:8080
  - service: http_status:404
```

`bridge` resolves via Swarm's internal DNS on `network_swarm_public`; port
`8080` matches `BRIDGE_PORT`. Health probes: `GET /healthz`, `GET /readyz`.

## 4. Verify

```bash
docker stack services bridge
curl -fsS https://bridge.DOMINIO/healthz
```

## Environment reference

| Variable               | Required        | Purpose                                                        |
| ---------------------- | --------------- | -------------------------------------------------------------- |
| `DATABASE_URL`         | yes             | App DSN: `postgres://bridge:<pw>@postgres:5432/bridge`.        |
| `MASTER_KEY`           | yes             | 32-byte base64 AES key for tenant secrets.                     |
| `BRIDGE_ENCRYPTION_KEY`| yes (parity)    | Held equal to `MASTER_KEY`.                                    |
| `POSTGRES_ADMIN_URL`   | first boot only | Maintenance DSN used to create the bridge role+database.       |
| `RUN_MIGRATIONS`       | no (default 1)  | `0` skips boot-time migrations.                                |
| `BRIDGE_PORT`          | no (default 8080)| HTTP listen port.                                             |
