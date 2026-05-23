-- F2/MED-1: Chatwoot account/inbox IDs to BIGINT.
-- Chatwoot internally types these as bigint; bridge stored as INTEGER which
-- would overflow on installs with >2^31 account/inbox auto-increment values.
ALTER TABLE tenants
  ALTER COLUMN chatwoot_account_id TYPE BIGINT,
  ALTER COLUMN chatwoot_inbox_id   TYPE BIGINT;
