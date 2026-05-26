-- Self-service pairing: track WhatsApp pairing state per tenant.
ALTER TABLE tenants ADD COLUMN IF NOT EXISTS paired_at TIMESTAMPTZ NULL;
ALTER TABLE tenants ADD COLUMN IF NOT EXISTS last_jid TEXT NULL;
