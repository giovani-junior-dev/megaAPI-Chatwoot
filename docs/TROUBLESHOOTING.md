# Troubleshooting

Operational runbook for the `chatwoot-megaapi-bridge`. Each section follows
the same shape:

1. Symptom — what you actually see.
2. Why — most common root causes, in order of probability.
3. Check — concrete commands to confirm the diagnosis.
4. Fix — the smallest change that resolves it.

If the symptom is not listed, jump to [General diagnostic flow](#general-diagnostic-flow)
at the bottom.

---

## Bridge não sobe (container CrashLoopBackOff / `bridge serve` exits)

**Symptom**
- `docker compose logs bridge` shows the process exiting immediately.
- `docker compose ps` reports the bridge service as `Restarting (1)`.
- Locally, `bridge serve` returns a non-zero exit code in under one second.

**Why**
- `MASTER_KEY` missing or not 32 bytes after base64 decode.
- `DATABASE_URL` malformed, points at an unreachable host, or the database
  has no `pgcrypto` extension.
- Migrations not applied (table `messages` does not exist).
- Port 8080 already bound by another process.

**Check**
```bash
docker compose logs --tail=80 bridge
echo "$MASTER_KEY" | base64 -d | wc -c   # must print 32
psql "$DATABASE_URL" -c "select 1"
ss -ltnp | grep :8080
```

**Fix**
- Regenerate the key: `openssl rand -base64 32` and update the secret store.
- Run `bridge migrate` before `bridge serve` (the installer does both).
- Free the port or set `BRIDGE_PORT=8081` and update the reverse proxy.

---

## `/healthz` falha (returns non-200 or hangs)

**Symptom**
- `curl http://localhost:8080/healthz` hangs, times out, or returns 5xx.
- Kubernetes restart loop fires `Liveness probe failed`.

**Why**
- The HTTP listener never started — usually a panic in `cmdServe` before
  `ListenAndServe`. `/healthz` itself never touches the DB, so a failure here
  means the process is dead.
- Reverse proxy is in front and the bridge is unreachable from it.

**Check**
```bash
curl -sv http://localhost:8080/healthz
docker compose exec bridge wget -qO- http://localhost:8080/healthz
docker compose logs --tail=200 bridge | grep -E "panic|fatal|error"
```

**Fix**
- Restart the container; if it still fails, see "Bridge não sobe" above.
- Check the reverse proxy upstream target (Caddyfile / nginx `proxy_pass`).

---

## `/readyz` retorna 503

**Symptom**
- `curl http://localhost:8080/readyz` returns 503 with body
  `{"db":"down"}` or `{"queue":"near_full"}`.
- The bridge stops accepting webhooks after a traffic spike.

**Why**
- DB ping failed — Postgres restarted, network partition, or pool exhausted.
- The in-process queue crossed 80% of `BUFFER_LIMIT`; workers are not
  keeping up with ingest.

**Check**
```bash
curl -s http://localhost:8080/readyz
curl -s http://localhost:8080/metrics | grep -E "bridge_queue_depth|bridge_messages"
psql "$DATABASE_URL" -c "select count(*) from messages where status='pending';"
```

**Fix**
- If `db:down`: restart Postgres, verify `pg_isready`, check pool size
  (`max_connections` vs configured pool).
- If `queue:near_full`: increase `WORKERS` (default 4) and/or `BUFFER_LIMIT`
  (default 1000), then redeploy. If downstream (megaAPI / Chatwoot) is slow,
  fix that first — the queue is a symptom.

---

## Webhook `unauthorized` (401 from `/v1/wa/{slug}` or `/v1/cw/{slug}`)

**Symptom**
- megaAPI delivery dashboard shows the bridge replying `401 unauthorized`.
- Chatwoot inbox automation log shows webhook delivery failed with 401.

**Why**
- WA path: the Bearer token in the request does not match the encrypted
  `webhook_bearer` for that tenant. megaAPI does not accept custom headers;
  the token must be in the query string (`?token=…`) — see
  [megaapi-webhook-config-nao-aceita-custom-headers-bearer]. The bridge
  accepts either `Authorization: Bearer …` or `?token=…`.
- CW path: HMAC signature missing or computed with the wrong secret.
  Header name is `X-Chatwoot-Signature` and the value is `sha256=<hex>`.

**Check**
```bash
# Diagnostic log line printed on every WA 401 (server.go logs raw_query/header lengths)
docker compose logs --tail=200 bridge | grep "WA webhook unauthorized"
# Confirm tenant exists with the expected slug
psql "$DATABASE_URL" -c "select slug from tenants where slug='<slug>';"
# Verify the secret stored matches what you configured upstream
bridge tenant add --slug ... --skip-reach-check   # to regenerate
```

**Fix**
- Re-issue the tenant with `bridge tenant add` and copy the new Bearer / HMAC
  back into megaAPI / Chatwoot. Old creds become inert.
- For CW, set `DEBUG_SKIP_HMAC=1` *only* in development to confirm the path
  is otherwise correct; remove it before production.

---

## megaAPI retorna 401 (outbound from bridge to megaAPI)

**Symptom**
- Worker logs show `megaapi POST … status=401`.
- Outgoing Chatwoot replies never reach WhatsApp.

**Why**
- The megaAPI token stored for the tenant is wrong, expired, or was rotated
  on the megaAPI side without updating the bridge.
- Wrong instance ID in the tenant config.

**Check**
```bash
# Manual call against megaAPI with the same credentials
curl -sS -H "Authorization: Bearer $MEGAAPI_TOKEN" \
  "$MEGAAPI_HOST/rest/instance/<instance>/info"
# Bridge-side: confirm the encrypted token decrypts
docker compose logs --tail=200 bridge | grep -E "decrypt|megaapi"
```

**Fix**
- Re-register the tenant: `bridge tenant add --slug <s> --megaapi-token <new>`
  (and other flags). The bridge will encrypt and store the new value.
- If only the token rotated, you can also `UPDATE tenants SET …` but prefer
  the CLI so encryption is correct.

---

## Chatwoot retorna 422 (outbound from bridge to Chatwoot)

**Symptom**
- Worker logs show `chatwoot POST … status=422` with body like
  `{"errors":["Inbox not found"]}` or `{"errors":["Content is required"]}`.
- Inbound WhatsApp messages do not appear in the Chatwoot conversation.

**Why**
- `chatwoot_account_id` or `chatwoot_inbox_id` in the tenant row does not
  match a real inbox in Chatwoot (typo, deleted inbox, wrong account).
- The message body is empty — usually a sticker or unsupported media type
  that the bridge accepted but Chatwoot rejected.
- `api_access_token` lacks permission on that account.

**Check**
```bash
curl -sS -H "api_access_token: $CW_TOKEN" \
  "$CW_URL/api/v1/accounts/$ACCOUNT_ID/inboxes" | jq '.payload[] | {id,name}'
docker compose logs --tail=200 bridge | grep -E "chatwoot.*422"
```

**Fix**
- Update the tenant with the correct `--chatwoot-account` / `--chatwoot-inbox`
  values.
- For empty content, either drop the media type upstream or extend the
  bridge's media path to attach the binary instead of sending plain text.

---

## Mensagens ficam `pending` / DLQ cresce (failed)

**Symptom**
- `select count(*) from messages where status='pending'` keeps climbing.
- `/admin/failed` returns a growing list of failed message IDs.
- `bridge_messages_failed_total` rate exceeds 0.1/s (triggers `high_failed_rate`).

**Why**
- Workers are stuck on a downstream that is slow or returning 5xx.
- A specific tenant's credentials are bad and every message for that tenant
  drains the retry budget.
- The worker pool crashed silently — `RecoverPending` only runs at startup.

**Check**
```bash
psql "$DATABASE_URL" -c "
  select tenant_id, status, count(*) from messages
  group by tenant_id, status order by 3 desc limit 10;"
curl -s http://localhost:8080/admin/failed | jq '.[] | {id, tenant_id, last_error}'
curl -s http://localhost:8080/metrics | grep -E "bridge_messages_failed_total|bridge_queue_depth"
```

**Fix**
- Triage by tenant: if one tenant dominates, fix its credentials first.
- Replay a single message: `curl -X POST http://localhost:8080/admin/retry/<id>`.
- Bulk requeue via the admin UI's DLQ page, then watch failed-rate drop.

---

## `/metrics` vazio (no `bridge_*` series scraped)

**Symptom**
- Prometheus shows `up{job="bridge"} == 0` or scrape errors.
- Grafana dashboard panels say "No data".

**Why**
- The Prometheus scrape target points at a host the container cannot reach.
  Default in `deploy/observability/prometheus/prometheus.yml` is
  `host.docker.internal:8080`, which requires `extra_hosts: host-gateway` on
  Linux (already configured in the overlay).
- The bridge is running but on a different port (`BRIDGE_PORT`).
- The bridge is behind TLS and Prometheus is configured for http.

**Check**
```bash
docker compose -f deploy/observability/docker-compose.yml exec prometheus \
  wget -qO- http://host.docker.internal:8080/metrics | head
docker compose -f deploy/observability/docker-compose.yml \
  logs --tail=80 prometheus | grep -i scrape
curl -s http://localhost:9090/api/v1/targets | jq '.data.activeTargets[] | {job: .labels.job, health}'
```

**Fix**
- Update `scrape_configs[0].static_configs[0].targets` to the correct host.
- Reload Prometheus: `curl -X POST http://localhost:9090/-/reload`.
- For TLS, add a `scheme: https` and `tls_config` block to the scrape job.

---

## OTel não exporta (no spans in collector / Tempo / Jaeger)

**Symptom**
- The OTLP collector receives nothing from the bridge.
- Bridge logs show no errors but no spans appear downstream either.

**Why**
- `OTEL_EXPORTER_OTLP_ENDPOINT` is unset — the bridge runs in no-op mode by
  design (`InitTracer` skips exporter wiring when the env var is empty).
- The endpoint is set but uses the wrong scheme. `otlptracehttp` defaults to
  `http://` and port `4318`; for `https://` you need a full URL.
- The collector is up but refuses unauthenticated traffic.

**Check**
```bash
docker compose exec bridge env | grep OTEL
docker compose logs --tail=100 bridge | grep -i otel
# From the collector side
docker compose logs --tail=100 otel-collector | grep -i bridge
```

**Fix**
- Set the env var: `OTEL_EXPORTER_OTLP_ENDPOINT=http://otel-collector:4318`
  and redeploy.
- For TLS or auth, also export `OTEL_EXPORTER_OTLP_HEADERS=…`.
- Confirm the exporter is wired by hitting an instrumented route while
  tailing the collector logs.

---

## General diagnostic flow

If the symptom is not above:

1. `curl http://localhost:8080/healthz` — process up?
2. `curl http://localhost:8080/readyz` — DB and queue healthy?
3. `curl http://localhost:8080/metrics | grep bridge_` — counters moving?
4. `docker compose logs --tail=200 bridge` — recent errors?
5. `psql "$DATABASE_URL" -c "select status, count(*) from messages group by 1;"`
   — pipeline shape.
6. Reproduce with a single curl against `/v1/wa/<slug>` or `/v1/cw/<slug>`
   using the exact payload the upstream sends.

Escalate with: the failing curl, the bridge log block around the failure,
and the output of step 5.
