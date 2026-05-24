#!/usr/bin/env bash
# Bootstrap bridge DB. Idempotent: /bridge migrate applies pending migrations,
# noop if schema already current.
#
# Pre-req: docker compose stack already up; bridge service reachable.

set -euo pipefail

COMPOSE_FILE="${COMPOSE_FILE:-docker-compose.yml}"
BRIDGE_SERVICE="${BRIDGE_SERVICE:-bridge}"

echo "[bootstrap-bridge] running /bridge migrate on service=${BRIDGE_SERVICE}"

docker compose -f "$COMPOSE_FILE" exec -T "$BRIDGE_SERVICE" /bridge migrate

echo "[bootstrap-bridge] done"
