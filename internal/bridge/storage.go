package bridge

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrNotFound = errors.New("bridge: not found")

type Tenant struct {
	ID                uuid.UUID
	Slug              string
	MegaAPIHost       string
	MegaAPIInstance   string
	MegaAPITokenEnc   []byte
	ChatwootURL       string
	ChatwootTokenEnc  []byte
	ChatwootAccountID int64
	ChatwootInboxID   int64
	HMACSecretEnc     []byte
	WebhookBearerEnc  []byte
}

type Contact struct {
	TenantID         uuid.UUID
	WAJid            string
	CWContactID      int64
	CWConversationID int64
	UpdatedAt        time.Time
}

type Admin struct {
	ID           uuid.UUID
	Email        string
	PasswordHash string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type Message struct {
	ID         uuid.UUID
	TenantID   uuid.UUID
	Direction  string
	ExternalID string
	Status     string
	Attempts   int
	LastError  string
	Payload    []byte
	CreatedAt  time.Time
}

type DB struct {
	Pool *pgxpool.Pool
}

func NewDB(ctx context.Context, dsn string) (*DB, error) {
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, err
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	return &DB{Pool: pool}, nil
}

func (d *DB) Close() {
	if d.Pool != nil {
		d.Pool.Close()
	}
}

func (d *DB) GetTenantBySlug(ctx context.Context, slug string) (Tenant, error) {
	const q = `SELECT id, slug, megaapi_host, megaapi_instance, megaapi_token_enc,
chatwoot_url, chatwoot_token_enc, chatwoot_account_id, chatwoot_inbox_id,
hmac_secret_enc, webhook_bearer_enc FROM tenants WHERE slug = $1`
	var t Tenant
	err := d.Pool.QueryRow(ctx, q, slug).Scan(&t.ID, &t.Slug, &t.MegaAPIHost,
		&t.MegaAPIInstance, &t.MegaAPITokenEnc, &t.ChatwootURL, &t.ChatwootTokenEnc,
		&t.ChatwootAccountID, &t.ChatwootInboxID, &t.HMACSecretEnc, &t.WebhookBearerEnc)
	if errors.Is(err, pgx.ErrNoRows) {
		return Tenant{}, ErrNotFound
	}
	return t, err
}

type TenantInsert struct {
	Slug              string
	MegaAPIHost       string
	MegaAPIInstance   string
	MegaAPITokenEnc   []byte
	ChatwootURL       string
	ChatwootTokenEnc  []byte
	ChatwootAccountID int64
	ChatwootInboxID   int64
	HMACSecretEnc     []byte
	WebhookBearerEnc  []byte
}

func (d *DB) InsertTenant(ctx context.Context, t TenantInsert) (uuid.UUID, error) {
	const q = `INSERT INTO tenants (slug, megaapi_host, megaapi_instance,
megaapi_token_enc, chatwoot_url, chatwoot_token_enc, chatwoot_account_id,
chatwoot_inbox_id, hmac_secret_enc, webhook_bearer_enc)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10) RETURNING id`
	var id uuid.UUID
	err := d.Pool.QueryRow(ctx, q, t.Slug, t.MegaAPIHost, t.MegaAPIInstance,
		t.MegaAPITokenEnc, t.ChatwootURL, t.ChatwootTokenEnc, t.ChatwootAccountID,
		t.ChatwootInboxID, t.HMACSecretEnc, t.WebhookBearerEnc).Scan(&id)
	return id, err
}

func (d *DB) UpsertContact(ctx context.Context, c Contact) error {
	const q = `INSERT INTO contacts (tenant_id, wa_jid, cw_contact_id, cw_conversation_id, updated_at)
VALUES ($1,$2,$3,$4,now())
ON CONFLICT (tenant_id, wa_jid) DO UPDATE SET
  cw_contact_id = EXCLUDED.cw_contact_id,
  cw_conversation_id = EXCLUDED.cw_conversation_id,
  updated_at = now()`
	_, err := d.Pool.Exec(ctx, q, c.TenantID, c.WAJid, c.CWContactID, c.CWConversationID)
	return err
}

func (d *DB) GetContact(ctx context.Context, tenantID uuid.UUID, jid string) (Contact, error) {
	const q = `SELECT tenant_id, wa_jid, cw_contact_id, cw_conversation_id, updated_at
FROM contacts WHERE tenant_id = $1 AND wa_jid = $2`
	var c Contact
	err := d.Pool.QueryRow(ctx, q, tenantID, jid).Scan(&c.TenantID, &c.WAJid,
		&c.CWContactID, &c.CWConversationID, &c.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Contact{}, ErrNotFound
	}
	return c, err
}

func (d *DB) InsertMessage(ctx context.Context, m Message) (uuid.UUID, bool, error) {
	const q = `INSERT INTO messages (tenant_id, direction, external_id, status, payload)
VALUES ($1,$2,$3,'pending',$4)
ON CONFLICT (tenant_id, direction, external_id) DO NOTHING
RETURNING id`
	var id uuid.UUID
	err := d.Pool.QueryRow(ctx, q, m.TenantID, m.Direction, m.ExternalID, m.Payload).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, false, nil
	}
	if err != nil {
		return uuid.Nil, false, err
	}
	return id, true, nil
}

func (d *DB) UpsertAdmin(ctx context.Context, email, passwordHash string) error {
	const q = `INSERT INTO admins (email, password_hash)
VALUES ($1, $2)
ON CONFLICT (email) DO UPDATE SET password_hash = EXCLUDED.password_hash, updated_at = now()`
	_, err := d.Pool.Exec(ctx, q, email, passwordHash)
	return err
}

func (d *DB) GetAdmin(ctx context.Context, email string) (Admin, error) {
	const q = `SELECT id, email, password_hash, created_at, updated_at FROM admins WHERE email = $1`
	var a Admin
	err := d.Pool.QueryRow(ctx, q, email).Scan(&a.ID, &a.Email, &a.PasswordHash, &a.CreatedAt, &a.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Admin{}, ErrNotFound
	}
	return a, err
}

func (d *DB) SetSetting(ctx context.Context, key, value string) error {
	const q = `INSERT INTO settings (key, value)
VALUES ($1, $2)
ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value, updated_at = now()`
	_, err := d.Pool.Exec(ctx, q, key, value)
	return err
}

func (d *DB) GetSetting(ctx context.Context, key string) (string, error) {
	const q = `SELECT value FROM settings WHERE key = $1`
	var v string
	err := d.Pool.QueryRow(ctx, q, key).Scan(&v)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil
	}
	return v, err
}

func (d *DB) SweepStale(ctx context.Context, age time.Duration) (int64, error) {
	const q = `UPDATE messages SET status = 'failed', last_error = 'stale: pending beyond ' || $1
WHERE status = 'pending' AND created_at < NOW() - $2::interval`
	tag, err := d.Pool.Exec(ctx, q, age.String(), age.String())
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

func (d *DB) MarkStatus(ctx context.Context, id uuid.UUID, status, lastErr string) error {
	const q = `UPDATE messages SET status = $2, last_error = NULLIF($3,'') WHERE id = $1`
	_, err := d.Pool.Exec(ctx, q, id, status, lastErr)
	return err
}

func (d *DB) IncrementAttempts(ctx context.Context, id uuid.UUID) error {
	const q = `UPDATE messages SET attempts = attempts + 1 WHERE id = $1`
	_, err := d.Pool.Exec(ctx, q, id)
	return err
}

func (d *DB) FailedMessages(ctx context.Context, limit int) ([]Message, error) {
	if limit <= 0 {
		limit = 50
	}
	const q = `SELECT id, tenant_id, direction, external_id, status, attempts,
COALESCE(last_error,''), payload, created_at FROM messages
WHERE status = 'failed' ORDER BY created_at DESC LIMIT $1`
	rows, err := d.Pool.Query(ctx, q, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Message
	for rows.Next() {
		var m Message
		if err := rows.Scan(&m.ID, &m.TenantID, &m.Direction, &m.ExternalID,
			&m.Status, &m.Attempts, &m.LastError, &m.Payload, &m.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func (d *DB) RequeueMessage(ctx context.Context, id uuid.UUID) (Message, error) {
	const q = `UPDATE messages
SET status = 'pending', attempts = 0, last_error = NULL
WHERE id = $1 AND status = 'failed'
RETURNING id, tenant_id, direction, external_id, status, attempts,
  COALESCE(last_error,''), payload, created_at`
	var m Message
	err := d.Pool.QueryRow(ctx, q, id).Scan(&m.ID, &m.TenantID, &m.Direction,
		&m.ExternalID, &m.Status, &m.Attempts, &m.LastError, &m.Payload, &m.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Message{}, ErrNotFound
	}
	return m, err
}

type TenantSummary struct {
	Slug      string
	Count24h  int64
}

func (d *DB) TenantSummaries(ctx context.Context) ([]TenantSummary, error) {
	const q = `SELECT t.slug, COALESCE(c.cnt, 0)
FROM tenants t LEFT JOIN (
  SELECT tenant_id, COUNT(*) cnt FROM messages
  WHERE created_at > now() - interval '24 hours' GROUP BY tenant_id
) c ON c.tenant_id = t.id ORDER BY t.slug`
	rows, err := d.Pool.Query(ctx, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []TenantSummary
	for rows.Next() {
		var s TenantSummary
		if err := rows.Scan(&s.Slug, &s.Count24h); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

func (d *DB) NextPending(ctx context.Context, limit int) ([]Message, error) {
	const q = `SELECT id, tenant_id, direction, external_id, status, attempts,
COALESCE(last_error,''), payload, created_at FROM messages
WHERE status = 'pending' ORDER BY created_at ASC LIMIT $1`
	rows, err := d.Pool.Query(ctx, q, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Message
	for rows.Next() {
		var m Message
		if err := rows.Scan(&m.ID, &m.TenantID, &m.Direction, &m.ExternalID,
			&m.Status, &m.Attempts, &m.LastError, &m.Payload, &m.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}
