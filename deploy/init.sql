-- init.sql — runs on postgres container first boot via /docker-entrypoint-initdb.d.
-- Creates two databases (chatwoot, bridge) + two users with scoped grants.
-- Passwords are substituted from POSTGRES_PASSWORD env at render time by
-- install.sh (this file is also rendered via envsubst when CHATWOOT_DB_PASSWORD
-- and BRIDGE_DB_PASSWORD are set; otherwise both default to POSTGRES_PASSWORD).

CREATE USER chatwoot WITH PASSWORD :'chatwoot_password';
CREATE USER bridge   WITH PASSWORD :'bridge_password';

CREATE DATABASE chatwoot OWNER chatwoot;
CREATE DATABASE bridge   OWNER bridge;

GRANT ALL PRIVILEGES ON DATABASE chatwoot TO chatwoot;
GRANT ALL PRIVILEGES ON DATABASE bridge   TO bridge;

-- pgvector enabled on chatwoot DB for AI features.
\connect chatwoot
CREATE EXTENSION IF NOT EXISTS vector;
CREATE EXTENSION IF NOT EXISTS pg_trgm;

\connect bridge
CREATE EXTENSION IF NOT EXISTS pg_trgm;
