#!/usr/bin/env bash
# Bootstrap Chatwoot DB. Idempotent: rails db:chatwoot_prepare creates schema
# only if missing, otherwise runs pending migrations.
#
# Pre-req: docker compose stack already up; rails service reachable.

set -euo pipefail

COMPOSE_FILE="${COMPOSE_FILE:-docker-compose.yml}"
RAILS_SERVICE="${RAILS_SERVICE:-rails}"

echo "[bootstrap-chatwoot] running rails db:chatwoot_prepare on service=${RAILS_SERVICE}"

docker compose -f "$COMPOSE_FILE" run --rm "$RAILS_SERVICE" \
    bundle exec rails db:chatwoot_prepare

echo "[bootstrap-chatwoot] done"
