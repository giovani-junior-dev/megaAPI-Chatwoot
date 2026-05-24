# Bridge env — generated from deploy/templates/.env.bridge.tpl by install.sh
# Regenerate secrets via: openssl rand -base64 32

MASTER_KEY=${MASTER_KEY}
DATABASE_URL=postgres://bridge:${POSTGRES_PASSWORD}@db:5432/bridge?sslmode=disable

POSTGRES_USER=bridge
POSTGRES_PASSWORD=${POSTGRES_PASSWORD}
POSTGRES_DB=bridge
POSTGRES_PORT=5432

BRIDGE_PORT=8080
BRIDGE_HOST_PORT=8080
BRIDGE_IMAGE=bridge:latest
LOG_LEVEL=info
BUFFER_LIMIT=1000
WORKERS=0
DEBUG_SKIP_HMAC=0

BRIDGE_DOMAIN=${DOMAIN}
BRIDGE_UPSTREAM=bridge:8080
ACME_EMAIL=${EMAIL}
