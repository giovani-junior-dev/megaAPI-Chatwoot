# INSTALL — chatwoot-megaapi-bridge

One-command installer that takes a clean Ubuntu 22.04 VPS to a working
chatwoot-megaapi-bridge stack in under 15 minutes.

This guide covers the production deployment path: HTTPS via Caddy automatic
ACME (Let's Encrypt) **or** Cloudflare Tunnel, daily backups, automatic
schema bootstrap, and an idempotent installer you can safely re-run.

---

## 1. Pre-requisites

### Host

- **OS:** Ubuntu 22.04 LTS (also works on 24.04; other distros untested)
- **CPU / RAM:** 2 vCPU / 4 GB minimum (Chatwoot Sidekiq is the hot spot;
  use 4 vCPU / 8 GB for production)
- **Disk:** 40 GB SSD (postgres + chatwoot storage + 14 days of pg_dump
  backups)
- **Ports:**
  - `80`, `443` open from the public internet — only if you choose the
    `--tls` mode (Caddy needs them for ACME HTTP-01 + serving traffic).
  - **No public ports** required for `--tunnel` mode — Cloudflare Tunnel
    initiates the connection.
- **DNS:** an `A` record (or Cloudflare proxied `CNAME`) for your domain
  pointing at the VPS public IP. Must resolve before `install.sh` runs in
  `--tls` mode, or Let's Encrypt issuance will fail.

### Tools the installer expects

`install.sh` will refuse to start if any of these are missing:

- `docker` (engine + `docker compose` v2 plugin)
- `openssl`
- `envsubst` (Debian/Ubuntu: `apt-get install gettext-base`)
- `curl`

Bootstrap a clean VPS with:

```bash
sudo apt-get update
sudo apt-get install -y curl ca-certificates gnupg gettext-base
# Docker (official Ubuntu instructions)
sudo install -m 0755 -d /etc/apt/keyrings
curl -fsSL https://download.docker.com/linux/ubuntu/gpg | \
    sudo gpg --dearmor -o /etc/apt/keyrings/docker.gpg
echo "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.gpg] \
    https://download.docker.com/linux/ubuntu $(. /etc/os-release && echo "$VERSION_CODENAME") stable" | \
    sudo tee /etc/apt/sources.list.d/docker.list >/dev/null
sudo apt-get update
sudo apt-get install -y docker-ce docker-ce-cli containerd.io docker-compose-plugin
sudo usermod -aG docker "$USER"
# Log out + back in so the group change applies, then verify:
docker compose version
```

### Choose your TLS mode

| Mode       | Public ports needed | DNS                              | Best for                          |
|------------|---------------------|----------------------------------|-----------------------------------|
| `--tls`    | 80, 443             | `A` record → VPS IP              | Standard VPS with public IP       |
| `--tunnel` | none                | Managed by Cloudflare (Tunnel)   | NAT-ed hosts, home labs, security |

You can switch later — the secrets stored in `deploy/.env.local` are
mode-agnostic.

---

## 2. Installation

```bash
git clone https://github.com/MadeInLowCode/chatwoot-megaapi-bridge.git
cd chatwoot-megaapi-bridge
bash deploy/install.sh
```

You will be prompted for:

1. **Domain** — e.g. `bridge.example.com`
2. **Admin email** — used for Let's Encrypt notifications and as the
   default Chatwoot mailer sender
3. **TLS mode** — `tls` (Caddy) or `tunnel` (Cloudflare)

If you want fully non-interactive (CI/CD), set env vars first:

```bash
DOMAIN=bridge.example.com \
EMAIL=ops@example.com \
TLS_MODE=tls \
NONINTERACTIVE=1 \
bash deploy/install.sh
```

The installer will:

1. Verify required tools are present.
2. Generate secrets via `openssl rand -base64 32` (`MASTER_KEY`,
   `POSTGRES_PASSWORD`, `REDIS_PASSWORD`) and `openssl rand -hex 64`
   (`CHATWOOT_SECRET_KEY_BASE`).
3. Persist the generated secrets to `deploy/.env.local` with `chmod 600`.
4. Render `deploy/.env.bridge`, `deploy/.env.chatwoot`, `deploy/Caddyfile`
   from the templates in `deploy/templates/`.
5. (Cloudflare Tunnel mode only) Install `cloudflared`, run interactive
   `cloudflared tunnel login`, create or reuse a named tunnel, route DNS,
   write the token to `deploy/.env.tunnel`.
6. `docker compose pull` and `up -d` against the selected profile.
7. Wait for `http://127.0.0.1:8080/healthz` to return 200.
8. Run `rails db:chatwoot_prepare` (Chatwoot schema) and `/bridge migrate`
   (bridge schema) inside the running containers.

The script is **idempotent**: it loads `deploy/.env.local` on every run,
skips Cloudflare tunnel creation if `deploy/.env.tunnel` already exists,
and Docker Compose itself is responsible for recreating only changed
containers.

---

## 3. Post-install verification

```bash
bash deploy/postinstall-check.sh
```

Pass criteria:

- All expected containers running
  (`postgres`, `redis`, `rails`, `sidekiq`, `bridge`, `caddy`-or-`cloudflared`,
  `postgres-backup`)
- `bridge` `/healthz` returns `200`
- Chatwoot UI returns `200`/`301`/`302`
- `bridge` `schema_migrations` and Chatwoot `ar_internal_metadata` tables
  populated
- `postgres-backup` sidecar running with `BACKUP_KEEP_DAYS=14`
- (`--tls` only) `https://<DOMAIN>/healthz` reachable with a valid
  certificate

If any check fails, see **Troubleshooting** below.

---

## 4. Backups

The `postgres-backup` container (image `prodrigestivill/postgres-backup-local:16`)
runs `pg_dump` daily inside the compose network and writes files to
`./deploy/backups/`.

Retention defaults:

- `BACKUP_KEEP_DAYS=14` — last 14 daily dumps
- `BACKUP_KEEP_WEEKS=4` — last 4 weekly dumps
- `BACKUP_KEEP_MONTHS=6` — last 6 monthly dumps

Both `chatwoot` and `bridge` databases are dumped. Restore example:

```bash
docker compose -f deploy/docker-compose.yml exec -T postgres \
    pg_restore -U postgres -d bridge -c < deploy/backups/daily/bridge/bridge-2026-05-24.sql.gz
```

For off-host backups, copy `deploy/backups/` to S3/B2/Restic on your own
schedule — the sidecar only manages the local rotation.

---

## 5. Upgrades

```bash
bash deploy/upgrade.sh --profile tls       # or --profile tunnel
```

`upgrade.sh` will:

1. `git pull --ff-only` the repo
2. `pg_dump` both DBs into `deploy/backups/pre-upgrade-<timestamp>/`
3. `docker compose pull` and `up -d --build` (rebuilds the bridge image
   from source if you've changed it locally)
4. Re-run `bootstrap-chatwoot.sh` (idempotent `rails db:chatwoot_prepare`)
   and `bootstrap-bridge.sh` (`/bridge migrate`)

Volumes are preserved — there is no `docker compose down -v` anywhere
in the upgrade path.

---

## 6. Day-2 operations

### Tail logs

```bash
docker compose -f deploy/docker-compose.yml logs -f bridge
docker compose -f deploy/docker-compose.yml logs -f rails sidekiq
```

### Restart the bridge only

```bash
docker compose -f deploy/docker-compose.yml restart bridge
```

### Add an admin (bridge UI)

```bash
docker compose -f deploy/docker-compose.yml exec bridge \
    /bridge admin add --email you@example.com
```

### Rotate secrets

`MASTER_KEY` cannot be rotated without re-encrypting all stored tenant
secrets — open a beads issue first (`bd create --title 'rotate MASTER_KEY'`)
and plan the migration.

Other secrets (`POSTGRES_PASSWORD`, `REDIS_PASSWORD`,
`CHATWOOT_SECRET_KEY_BASE`) require a full stack restart; coordinate with
your users — Chatwoot sessions will be invalidated.

---

## 7. Troubleshooting

### `install.sh` says "missing tools: …"

Install whatever's listed. The most common omission is
`gettext-base` (provides `envsubst`).

### Let's Encrypt issuance fails (`--tls` mode)

- Confirm `dig +short <DOMAIN>` returns your VPS IP from a clean resolver.
- Confirm port 80 is open: `curl -v http://<DOMAIN>/`.
- Check Caddy logs: `docker compose -f deploy/docker-compose.yml logs caddy`.
- ACME rate limits: 5 failures per account per hostname per hour. Wait
  before retrying.

### Cloudflare Tunnel container restarts in a loop

- `docker compose -f deploy/docker-compose.yml logs cloudflared`
- Most commonly: the token in `deploy/.env.tunnel` is stale.
  Re-run `DOMAIN=<your domain> bash deploy/setup-tunnel.sh` to regenerate.

### `bridge /healthz` is unreachable from inside the host

```bash
docker compose -f deploy/docker-compose.yml ps
docker compose -f deploy/docker-compose.yml logs bridge | tail -100
docker compose -f deploy/docker-compose.yml exec bridge /bridge --help
```

If `MASTER_KEY` was corrupted/regenerated, the bridge will refuse to
start. Restore the previous `deploy/.env.bridge` from your backup.

### `rails db:chatwoot_prepare` hangs

Run the bootstrap manually and inspect the output:

```bash
docker compose -f deploy/docker-compose.yml run --rm rails \
    bundle exec rails db:chatwoot_prepare
```

Usually a postgres readiness issue — `pg_isready` returned ok, but the
init.sql users were not committed yet. Re-running the bootstrap is safe.

### Backup directory keeps growing

Check that `BACKUP_KEEP_DAYS` and friends are set in
`deploy/docker-compose.yml`. If yes, ensure the sidecar can write to
`deploy/backups/` — bind mount permissions are the usual cause.

### Resetting from scratch (DESTRUCTIVE)

```bash
docker compose -f deploy/docker-compose.yml --profile tls down -v
rm -f deploy/.env.local deploy/.env.bridge deploy/.env.chatwoot \
      deploy/Caddyfile deploy/.env.tunnel
rm -rf deploy/backups/ deploy/.cloudflared/
bash deploy/install.sh
```

⚠️ This deletes the postgres volume **and all Chatwoot conversations,
contacts, and bridge messages**. Make sure you have an off-host backup
before running.

---

## 8. Validating a fresh VPS

CI/CD can prove the installer still works end-to-end:

```bash
bash deploy/validate-fresh-vps.sh
```

This spins up a clean Ubuntu 22.04 Docker container, copies the repo in,
runs `install.sh` with synthetic env vars, waits for `/healthz`, and
prints `INSTALL_DURATION_SECONDS=<n>` (must be `<900`).

---

## 9. Where things live

| Path                              | Purpose                                                 |
|-----------------------------------|---------------------------------------------------------|
| `deploy/install.sh`               | One-command installer (orchestrator)                    |
| `deploy/upgrade.sh`               | Upgrade in place                                        |
| `deploy/postinstall-check.sh`     | Validation checklist                                    |
| `deploy/render-templates.sh`      | `envsubst` of `.tpl` files                              |
| `deploy/setup-tunnel.sh`          | Cloudflare Tunnel installer + token writer              |
| `deploy/bootstrap-chatwoot.sh`    | `rails db:chatwoot_prepare`                             |
| `deploy/bootstrap-bridge.sh`      | `/bridge migrate`                                       |
| `deploy/validate-fresh-vps.sh`    | Fresh Ubuntu container end-to-end test                  |
| `deploy/docker-compose.yml`       | Full stack (profiles `tls` / `tunnel`)                  |
| `deploy/templates/*.tpl`          | Source templates                                        |
| `deploy/init.sql`                 | `chatwoot` + `bridge` users/DBs/extensions              |
| `deploy/.env.local`               | Generated secrets (mode 600 — **never commit**)         |
| `deploy/.env.bridge`              | Rendered bridge env (mode 600 — **never commit**)       |
| `deploy/.env.chatwoot`            | Rendered Chatwoot env (mode 600 — **never commit**)     |
| `deploy/.env.tunnel`              | Cloudflare tunnel token (mode 600 — **never commit**)   |
| `deploy/Caddyfile`                | Rendered Caddy config                                   |
| `deploy/backups/`                 | Daily/weekly/monthly pg_dump output                     |

---

## 10. Getting help

- Bridge bug or unexpected behavior: open a beads issue
  (`bd create --type bug --priority 2 ...`).
- Chatwoot-specific question: see <https://www.chatwoot.com/docs>.
- megaAPI question: ask in the megaAPI Slack — the authors built it and
  know the payload shapes better than anyone.
