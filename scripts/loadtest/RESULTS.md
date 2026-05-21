# SCR-72 — Load Test Baseline (client-side, k6)

**bd issue:** `chatwoot-megaapi-bridge-8s8.9`
**Linear:** SCR-72
**Date (UTC):** 2026-05-21T00:19:59Z
**RUN_TAG:** `run-20260520211959`
**Verdict:** **PASS** — all thresholds met, zero message loss.

## Goal recap

Validate the hot ACK path `POST /v1/wa/{slug}` sustains:

- **1000 req/s** for **5 min** (300 000 requests total)
- **ACK p99 < 10 ms** measured client-side (k6)
- **Zero loss**: inserted `messages` rows == k6 iteration count (unique payload
  `key.id` per iteration so idempotency dedup never engages)

Scope intentionally client-side only — server-side Prometheus instrumentation
is deferred to bd `8s8.6` / `8s8.7` (both still open P2).

## Hardware / Host

| Field | Value |
| --- | --- |
| OS | Microsoft Windows 11 Pro 10.0.26200 |
| CPU | 13th Gen Intel(R) Core(TM) i7-13620H (10 cores / 16 logical) |
| RAM | 15.7 GiB |
| Docker | 29.3.1 (Compose v5.1.0) |
| Postgres | `postgres:15-alpine` (compose `db` service, default tuning) |
| Bridge | `bridge:latest` built from this repo, `LOG_LEVEL=warn`, `BUFFER_LIMIT=400000` |
| k6 | `grafana/k6:latest` running in docker, same compose network |

## Configuration

`.env` deltas vs `.env.example`:

```
BRIDGE_HOST_PORT=8090   # host 8090 mapped to container 8080 (8080 was busy)
POSTGRES_PORT=5433      # host 5433 mapped to container 5432 (5432 was busy)
BUFFER_LIMIT=400000     # enlarged Inbox/Outbox channel so a single 5min run
                        # at 1000 rps fits even if workers stall on the stub
                        # tenant's unreachable chatwoot URL.
WORKERS=0               # 0 = bridge default (GOMAXPROCS*4)
LOG_LEVEL=warn
```

`cmd/bridge/main.go` reads `BUFFER_LIMIT` and `WORKERS` env vars into
`bridge.Config` so runtime tuning never touches the `internal/bridge`
package (flat-first invariant preserved).

`docker-compose.yml` exposes `BUFFER_LIMIT` / `WORKERS` env vars and adds an
optional `BRIDGE_HOST_PORT` knob to keep the host-side mapping decoupled from
`BRIDGE_PORT` (the bridge listens on the original 8080 inside the container).

Tenant provisioned via:

```
bridge tenant add --slug loadtest \
  --megaapi-host http://stub.local --megaapi-instance test-instance --megaapi-token faketoken \
  --chatwoot-url http://stub.local --chatwoot-token faketoken \
  --chatwoot-account 1 --chatwoot-inbox 1 --skip-reach-check
```

The stub upstreams are unreachable on purpose — the workers will retry and
eventually mark each message `failed`, but only **after** the ACK path returns
200. The load test scope is the ACK, not the worker side.

## How to reproduce

```powershell
# 1. start stack
docker compose up -d

# 2. apply migrations (one-time)
docker run --rm --network chatwoot-megaapi-bridge_default `
  -e DATABASE_URL=postgres://bridge:bridge@db:5432/bridge?sslmode=disable `
  -e MASTER_KEY=<MASTER_KEY> `
  bridge:latest migrate

# 3. create tenant (writes Bearer to stdout)
docker run --rm --network chatwoot-megaapi-bridge_default `
  -e DATABASE_URL=postgres://bridge:bridge@db:5432/bridge?sslmode=disable `
  -e MASTER_KEY=<MASTER_KEY> `
  bridge:latest tenant add --slug loadtest \
    --megaapi-host http://stub.local --megaapi-instance i --megaapi-token t \
    --chatwoot-url http://stub.local --chatwoot-token t \
    --chatwoot-account 1 --chatwoot-inbox 1 --skip-reach-check

# 4. run load test
./scripts/loadtest/run.ps1 -Bearer <bearer-from-step-3> -Rate 1000 -Duration 5m
```

## k6 results

Source: `scripts/loadtest/summary.json` (machine-readable),
`scripts/loadtest/latest-run.log` (full stdout).

### HTTP request duration

| Stat | ms |
| ---: | ---: |
| min | 0.044 |
| avg | 2.307 |
| p50 / med | 2.151 |
| p90 | 2.618 |
| p95 | 2.856 |
| **p99** | **4.867** |
| p99.9 | 19.517 |
| max | 64.877 |

### Throughput / errors

| Metric | Value |
| --- | --- |
| iterations | **300 000** |
| iterations/s avg | 1 000.06 |
| http_reqs | 300 000 |
| http_req_failed rate | **0.0000 %** |
| checks pass rate | **100.0000 %** (300 000 / 300 000 `status 200`) |
| data_sent | 119 369 931 B (~ 398 KB/s) |
| data_received | 38 400 000 B (~ 128 KB/s) |

### Thresholds

| Threshold | Result |
| --- | --- |
| `http_req_duration{expected_response:true}: p(99)<10` | **PASS** (4.87 ms) |
| `http_req_duration{expected_response:true}: p(95)<5`  | **PASS** (2.86 ms) |
| `http_req_failed: rate<0.001` | **PASS** (0.0000) |
| `checks: rate>0.999` | **PASS** (1.0000) |

## Zero-loss verification

```sql
SELECT count(*) FROM messages
 WHERE direction = 'in'
   AND external_id LIKE 'run-20260520211959-%';
```

| Source | Count |
| --- | ---: |
| k6 iterations | 300 000 |
| `messages` rows for this RUN_TAG | **300 000** |
| **Delta (iters − rows)** | **0** |

Every successful k6 request is reflected by one `messages` row. Idempotency
dedup never engages because `key.id = ${RUN_TAG}-${VU}-${ITER}` is unique per
iteration.

## Observed behaviour

- Bridge bridge `Inbox` channel filled to ~ rate × duration over the run since
  workers cannot drain (stub Chatwoot host unreachable, 10s HTTP timeout +
  retry backoff). Sized `BUFFER_LIMIT=400000` accommodates 5 min at 1000 rps
  with headroom. **Without the BUFFER_LIMIT bump the default 1000-slot channel
  would fill in ~1 s and the handler would respond `503 queue full`** —
  documented here so future runs against a live downstream don't need it.
- Worker retry loop logs `chatwoot http://stub.local/... lookup` errors for
  every message after the ACK. Expected and unrelated to the load test scope.
- Tail latency (p99.9 = 19.5 ms, max = 64.9 ms) is consistent with brief
  Go scheduler / GC pauses under sustained 1000 rps. No threshold breach.

## Verdict

**PASS.** All acceptance criteria met:

- p99 ACK 4.87 ms (target < 10 ms) — **2× margin**
- Zero loss: 300 000 / 300 000 (target ≥ iterations)
- 0.000 % HTTP failures, 100 % checks (target < 0.1 % / > 99.9 %)

## Follow-ups

- bd `8s8.6` / `8s8.7` (Prometheus + Grafana for the worker queue / DB) will
  unlock the next iteration — measuring **server-side** latency, queue
  saturation, and worker drain rate against a live Chatwoot stack.
- Re-run against the `deploy/chatwoot.docker-compose.yml` Chatwoot DEV stack
  once instrumentation lands, so the worker drain path is included in the
  measurement instead of being stalled on `stub.local`.
- Consider raising the default `BufferLimit` once a real-world drain rate is
  measured — current 1000 default is too tight for any non-trivial burst when
  downstream slows.
