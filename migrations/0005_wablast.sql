-- WaBlast provider (official Meta WhatsApp Cloud gateway) as a second, additive
-- provider alongside megaAPI. One provider per tenant. megaAPI rows stay intact
-- (default 'megaapi'). megaapi_* columns become optional so a wablast tenant can
-- persist without them.
ALTER TABLE tenants
  ADD COLUMN IF NOT EXISTS provider                   TEXT  NOT NULL DEFAULT 'megaapi',
  ADD COLUMN IF NOT EXISTS wablast_api_key_enc        BYTEA,
  ADD COLUMN IF NOT EXISTS wablast_account_id         TEXT  NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS wablast_webhook_secret_enc BYTEA;

ALTER TABLE tenants
  ALTER COLUMN megaapi_host      DROP NOT NULL,
  ALTER COLUMN megaapi_instance  DROP NOT NULL,
  ALTER COLUMN megaapi_token_enc DROP NOT NULL;

ALTER TABLE tenants DROP CONSTRAINT IF EXISTS tenants_provider_check;
ALTER TABLE tenants ADD  CONSTRAINT tenants_provider_check
  CHECK (provider IN ('megaapi', 'wablast'));
