# Pilot Program — v1.0.0

Goal: validate v1.0.0 in production with **5 tenants** running for **7 consecutive
days** with **zero SEV1 or SEV2 incidents**. Completion of the pilot is the human
gate for declaring v1.0.0 generally available and pushing the `v1.0.0` tag to
the remote.

This document is the acceptance checklist and the tracking template. The bead
`chatwoot-megaapi-bridge-efn.8` stays open until the pilot completes; closing
it is a deliberate human decision, not an automation step.

## Acceptance Criteria

The pilot is **successful** if and only if all of the following hold for the
full 7-day window:

1. Five distinct tenants have processed at least one production message each.
2. No SEV1 incident has been declared. SEV1 = all tenants down, data loss risk,
   or unauthenticated access to admin UI. See `docs/OPERATIONS.md`.
3. No SEV2 incident has been declared. SEV2 = a single tenant fully down for
   more than 30 consecutive minutes, or sustained error rate >5% for any
   tenant for more than 30 minutes.
4. DLQ growth across all tenants stays below 0.5% of total processed messages.
5. p99 latency on `/v1/wa/{slug}` stays below 500ms (rolling 1h window).
6. No `gosec` HIGH/CRITICAL regressions introduced by hotfixes shipped during
   the pilot.
7. No emergency rollback executed.

If ANY criterion fails, the pilot is **paused**: file an incident-followup
bead, ship the fix, and restart the 7-day clock from zero.

## Tenant Tracking Template

Copy this block per tenant under the "Tenants" section below.

```
### Tenant: <slug>
- Operator: <name / email>
- Region: <hosting region>
- Bridge version: v1.0.0
- Onboarded: YYYY-MM-DD HH:MM UTC
- megaAPI instance: <id>
- Chatwoot instance: <url>
- Webhook registered: [ ] yes / [ ] no
- First production message: YYYY-MM-DD HH:MM UTC
- Day 1 sign-off: [ ]   notes:
- Day 2 sign-off: [ ]   notes:
- Day 3 sign-off: [ ]   notes:
- Day 4 sign-off: [ ]   notes:
- Day 5 sign-off: [ ]   notes:
- Day 6 sign-off: [ ]   notes:
- Day 7 sign-off: [ ]   notes:
- Incidents (link to bd ID): none / <bd-id>
- Pilot outcome: [ ] PASS / [ ] FAIL / [ ] RESET
```

## Daily Health Checklist

Operators run this once per day during the pilot window. Output is captured
to a log file and attached to the closing bead comment.

```bash
# Stack health
docker compose -f /opt/chatwoot-bridge/deploy/docker-compose.yml ps
curl -fsS https://bridge.example.com/healthz
curl -fsS https://bridge.example.com/readyz

# Key counters
curl -s https://bridge.example.com/metrics \
  | grep -E '^(bridge_dlq_total|bridge_inbound_duration_seconds_count|bridge_outbound_failures_total|bridge_worker_active)'

# Backup freshness (must be < 8h old)
ls -lh /var/backups/bridge | head -3

# Per-tenant message count delta (manual SQL via the read-only role)
docker compose exec db psql -U bridge_readonly bridge \
  -c "SELECT tenant_id, direction, COUNT(*) FROM messages
      WHERE created_at > now() - interval '24 hours'
      GROUP BY 1,2 ORDER BY 1,2;"
```

## Incident Triage During Pilot

1. Any reported issue gets a bead opened with label `pilot-incident` and the
   severity classification from `docs/OPERATIONS.md`.
2. SEV1/SEV2: pilot is paused until resolved. After resolution, restart the
   7-day clock and increment the "RESET" counter on the affected tenant.
3. SEV3/SEV4: tracked but do not pause the pilot.

## Pilot Closeout

When all five tenants reach Day-7 sign-off without a reset:

1. Record the final daily checklist output.
2. Update each tenant block above with `Pilot outcome: PASS`.
3. Push the v1.0.0 tag to the remote: `git push origin v1.0.0`.
4. Cut the `bridge:1.0.0` container image and promote to `:latest`.
5. Close `chatwoot-megaapi-bridge-efn.8` with a `bd close` reason that links
   to this file and the per-tenant sign-off log.
6. Close the parent epic `chatwoot-megaapi-bridge-efn`.

## Tenants

(empty — populate at pilot kickoff)
