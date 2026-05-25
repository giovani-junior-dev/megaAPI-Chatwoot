# Pilot Log — Self-Pilot v1.0.0

**Mode**: self-pilot (1 tenant, operator-only) — informal pre-release smoke real.
**Start date**: 2026-05-24
**Target end date**: 2026-05-31 (7 days)
**Tenant**: `e2e-teste` (Giovani — operador)

## Scope Adjustment

Original PILOT-PROGRAM.md exige 5 tenants distintos. Self-pilot é **reduzido**:
- 1 tenant (operador testa em produção própria)
- Critérios funcionais mantidos (SEV1/SEV2/DLQ/p99)
- Não cumpre gate oficial v1.0.0 — serve como burn-in pré-release
- Após self-pilot OK, decisão: (A) aceitar relaxed gate ou (B) recrutar 4 tenants externos

## Acceptance (relaxed for self-pilot)

| # | Critério | Threshold |
|---|---|---|
| 1 | Tenant processou ≥10 msgs reais/dia | ≥70 msgs total em 7 dias |
| 2 | Zero SEV1 (down total / data loss / auth bypass) | 0 |
| 3 | Zero SEV2 (>30min down / >5% error 30min) | 0 |
| 4 | DLQ growth | <0.5% total processed |
| 5 | p99 latência `/v1/wa/*` | <500ms (1h rolling) |
| 6 | `gosec` HIGH/CRITICAL regressions | 0 |
| 7 | Emergency rollback | 0 |

## Daily Check (run via `deploy/security/gosec.sh` + manual queries)

Comandos diário:
```bash
# Total messages processed
docker compose exec -T db psql -U bridge -d bridge -tA -c \
  "SELECT count(*) FROM messages WHERE created_at >= NOW() - INTERVAL '1 day';"

# Failed count (DLQ growth)
docker compose exec -T db psql -U bridge -d bridge -tA -c \
  "SELECT count(*) FROM messages WHERE status='failed' AND created_at >= NOW() - INTERVAL '1 day';"

# Pending (stale check)
docker compose exec -T db psql -U bridge -d bridge -tA -c \
  "SELECT count(*) FROM messages WHERE status='pending' AND created_at < NOW() - INTERVAL '1 hour';"

# Health
curl -sS http://localhost:8090/healthz
curl -sS http://localhost:8090/readyz

# Metrics snapshot
curl -sS http://localhost:8090/metrics | grep -E "^bridge_(messages|queue|job)"
```

## Daily Log

### Day 0 — 2026-05-24 (kickoff)
- Pipeline operacional: F1-F6 entregues
- Tenant ativo: `e2e-teste`
- Webhooks setados: megaAPI configWebhook + Chatwoot inbox 1
- Baseline: msgs prévias limpas (truncate `messages` + `contacts`)
- `gosec` baseline: 0 HIGH (commit 6f9b164)
- Incidents: 0

### Day 1 — 2026-05-25 (preencher)
- Msgs hoje: __
- Failed hoje: __
- DLQ growth %: __
- p99 latency: __
- Health: __
- Incidents: __
- Notas: __

### Day 2 — 2026-05-26
- (preencher)

### Day 3 — 2026-05-27
- (preencher)

### Day 4 — 2026-05-28
- (preencher)

### Day 5 — 2026-05-29
- (preencher)

### Day 6 — 2026-05-30
- (preencher)

### Day 7 — 2026-05-31 (decision day)
- Total msgs 7d: __
- Total failed 7d: __
- DLQ growth %: __
- Max p99 observada: __
- Incidents (SEV1/SEV2): __
- gosec final: __
- **Decisão**: [PASS reduzido / FAIL → restart / decidir recrutar 4 externos]

## Reset Criteria

Se qualquer critério falhar em qualquer dia:
1. Pause clock
2. File `bd create --type=bug --title="pilot-incident: <descrição>" --parent=chatwoot-megaapi-bridge-efn.8`
3. Ship fix
4. Reiniciar D1 do zero
