//go:build integration

package bridge

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

func setupDB(t *testing.T) *DB {
	t.Helper()
	ctx := context.Background()
	pgC, err := postgres.Run(ctx, "postgres:15-alpine",
		postgres.WithDatabase("bridge"),
		postgres.WithUsername("bridge"),
		postgres.WithPassword("bridge"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").WithOccurrence(2).WithStartupTimeout(60*time.Second),
		),
	)
	require.NoError(t, err)
	t.Cleanup(func() { _ = pgC.Terminate(context.Background()) })

	dsn, err := pgC.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)
	db, err := NewDB(ctx, dsn)
	require.NoError(t, err)
	t.Cleanup(db.Close)

	applyAllMigrations(t, ctx, db)
	return db
}

func applyAllMigrations(t *testing.T, ctx context.Context, db *DB) {
	t.Helper()
	entries, err := os.ReadDir("../../migrations")
	require.NoError(t, err)
	var files []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".sql") {
			files = append(files, e.Name())
		}
	}
	sort.Strings(files)
	for _, name := range files {
		body, err := os.ReadFile(filepath.Join("../../migrations", name))
		require.NoError(t, err)
		_, err = db.Pool.Exec(ctx, string(body))
		require.NoError(t, err, "migration %s", name)
	}
}

func TestGetTenantBySlug_NotFoundError(t *testing.T) {
	db := setupDB(t)
	_, err := db.GetTenantBySlug(context.Background(), "missing")
	require.ErrorIs(t, err, ErrNotFound)
}

func TestInsertTenant_RoundTrip(t *testing.T) {
	db := setupDB(t)
	ctx := context.Background()
	id, err := db.InsertTenant(ctx, TenantInsert{
		Slug: "demo", MegaAPIHost: "https://x", MegaAPIInstance: "i",
		MegaAPITokenEnc: []byte("a"), ChatwootURL: "https://c", ChatwootTokenEnc: []byte("b"),
		ChatwootAccountID: 1, ChatwootInboxID: 2,
		HMACSecretEnc: []byte("h"), WebhookBearerEnc: []byte("w"),
	})
	require.NoError(t, err)
	got, err := db.GetTenantBySlug(ctx, "demo")
	require.NoError(t, err)
	require.Equal(t, id, got.ID)
}

func TestInsertTenant_AcceptsLargeChatwootIDs(t *testing.T) {
	db := setupDB(t)
	ctx := context.Background()
	bigID := int64(3_000_000_000)
	_, err := db.InsertTenant(ctx, TenantInsert{
		Slug: "bigids", MegaAPIHost: "https://x", MegaAPIInstance: "i",
		MegaAPITokenEnc: []byte("a"), ChatwootURL: "https://c", ChatwootTokenEnc: []byte("b"),
		ChatwootAccountID: bigID, ChatwootInboxID: bigID + 1,
		HMACSecretEnc: []byte("h"), WebhookBearerEnc: []byte("w"),
	})
	require.NoError(t, err)
	got, err := db.GetTenantBySlug(ctx, "bigids")
	require.NoError(t, err)
	require.Equal(t, bigID, got.ChatwootAccountID)
	require.Equal(t, bigID+1, got.ChatwootInboxID)
}

func TestUpsertContact_CreatesNewAndUpdates(t *testing.T) {
	db := setupDB(t)
	ctx := context.Background()
	tid := makeTenant(t, db)
	require.NoError(t, db.UpsertContact(ctx, Contact{TenantID: tid, WAJid: "5511", CWContactID: 10, CWConversationID: 20}))
	c, err := db.GetContact(ctx, tid, "5511")
	require.NoError(t, err)
	require.Equal(t, int64(10), c.CWContactID)
	require.NoError(t, db.UpsertContact(ctx, Contact{TenantID: tid, WAJid: "5511", CWContactID: 11, CWConversationID: 21}))
	c, err = db.GetContact(ctx, tid, "5511")
	require.NoError(t, err)
	require.Equal(t, int64(11), c.CWContactID)
}

func TestInsertMessage_DuplicateReturnsCreatedFalse(t *testing.T) {
	db := setupDB(t)
	ctx := context.Background()
	tid := makeTenant(t, db)
	_, created, err := db.InsertMessage(ctx, Message{TenantID: tid, Direction: "in", ExternalID: "x1", Payload: []byte(`{}`)})
	require.NoError(t, err)
	require.True(t, created)
	_, created, err = db.InsertMessage(ctx, Message{TenantID: tid, Direction: "in", ExternalID: "x1", Payload: []byte(`{}`)})
	require.NoError(t, err)
	require.False(t, created)
}

func TestNextPending_OrdersByCreatedAt(t *testing.T) {
	db := setupDB(t)
	ctx := context.Background()
	tid := makeTenant(t, db)
	_, _, err := db.InsertMessage(ctx, Message{TenantID: tid, Direction: "in", ExternalID: "a", Payload: []byte(`{}`)})
	require.NoError(t, err)
	time.Sleep(10 * time.Millisecond)
	_, _, err = db.InsertMessage(ctx, Message{TenantID: tid, Direction: "in", ExternalID: "b", Payload: []byte(`{}`)})
	require.NoError(t, err)
	pending, err := db.NextPending(ctx, 10)
	require.NoError(t, err)
	require.Len(t, pending, 2)
	require.Equal(t, "a", pending[0].ExternalID)
}

func TestSweepStale_MarksOldPendingAsFailed(t *testing.T) {
	db := setupDB(t)
	ctx := context.Background()
	tid := makeTenant(t, db)
	idStale, _, err := db.InsertMessage(ctx, Message{TenantID: tid, Direction: "in", ExternalID: "stale", Payload: []byte(`{}`)})
	require.NoError(t, err)
	idFresh, _, err := db.InsertMessage(ctx, Message{TenantID: tid, Direction: "in", ExternalID: "fresh", Payload: []byte(`{}`)})
	require.NoError(t, err)
	_, err = db.Pool.Exec(ctx, `UPDATE messages SET created_at = NOW() - INTERVAL '2 hours' WHERE id = $1`, idStale)
	require.NoError(t, err)

	n, err := db.SweepStale(ctx, time.Hour)
	require.NoError(t, err)
	require.Equal(t, int64(1), n)

	var stStale, errStale string
	require.NoError(t, db.Pool.QueryRow(ctx,
		`SELECT status, COALESCE(last_error,'') FROM messages WHERE id = $1`, idStale).Scan(&stStale, &errStale))
	require.Equal(t, "failed", stStale)
	require.Contains(t, errStale, "stale")

	var stFresh string
	require.NoError(t, db.Pool.QueryRow(ctx,
		`SELECT status FROM messages WHERE id = $1`, idFresh).Scan(&stFresh))
	require.Equal(t, "pending", stFresh)
}

func TestSweepStale_NoOpWhenNothingStale(t *testing.T) {
	db := setupDB(t)
	ctx := context.Background()
	tid := makeTenant(t, db)
	_, _, err := db.InsertMessage(ctx, Message{TenantID: tid, Direction: "in", ExternalID: "x", Payload: []byte(`{}`)})
	require.NoError(t, err)
	n, err := db.SweepStale(ctx, time.Hour)
	require.NoError(t, err)
	require.Equal(t, int64(0), n)
}

func TestMarkStatus_AndIncrementAttempts(t *testing.T) {
	db := setupDB(t)
	ctx := context.Background()
	tid := makeTenant(t, db)
	id, _, err := db.InsertMessage(ctx, Message{TenantID: tid, Direction: "in", ExternalID: "z", Payload: []byte(`{}`)})
	require.NoError(t, err)
	require.NoError(t, db.IncrementAttempts(ctx, id))
	require.NoError(t, db.IncrementAttempts(ctx, id))
	require.NoError(t, db.MarkStatus(ctx, id, "failed", "boom"))
	pending, err := db.NextPending(ctx, 10)
	require.NoError(t, err)
	require.Len(t, pending, 0)
}

func TestUpsertAdmin_RoundTrip(t *testing.T) {
	db := setupDB(t)
	ctx := context.Background()
	err := db.UpsertAdmin(ctx, "admin@example.com", "hash-v1")
	require.NoError(t, err)
	got, err := db.GetAdmin(ctx, "admin@example.com")
	require.NoError(t, err)
	require.Equal(t, "admin@example.com", got.Email)
	require.Equal(t, "hash-v1", got.PasswordHash)
}

func TestUpsertAdmin_OverwritesPasswordHash(t *testing.T) {
	db := setupDB(t)
	ctx := context.Background()
	require.NoError(t, db.UpsertAdmin(ctx, "a@x.com", "h1"))
	require.NoError(t, db.UpsertAdmin(ctx, "a@x.com", "h2"))
	got, err := db.GetAdmin(ctx, "a@x.com")
	require.NoError(t, err)
	require.Equal(t, "h2", got.PasswordHash)
}

func TestGetAdmin_NotFound(t *testing.T) {
	db := setupDB(t)
	_, err := db.GetAdmin(context.Background(), "missing@x.com")
	require.ErrorIs(t, err, ErrNotFound)
}

func TestSetSetting_GetSetting_RoundTrip(t *testing.T) {
	db := setupDB(t)
	ctx := context.Background()
	require.NoError(t, db.SetSetting(ctx, "base_url", "https://bridge.example.com"))
	v, err := db.GetSetting(ctx, "base_url")
	require.NoError(t, err)
	require.Equal(t, "https://bridge.example.com", v)
}

func TestSetSetting_Overwrites(t *testing.T) {
	db := setupDB(t)
	ctx := context.Background()
	require.NoError(t, db.SetSetting(ctx, "base_url", "v1"))
	require.NoError(t, db.SetSetting(ctx, "base_url", "v2"))
	v, err := db.GetSetting(ctx, "base_url")
	require.NoError(t, err)
	require.Equal(t, "v2", v)
}

func TestGetSetting_MissingReturnsEmptyString(t *testing.T) {
	db := setupDB(t)
	v, err := db.GetSetting(context.Background(), "absent")
	require.NoError(t, err)
	require.Equal(t, "", v)
}

func makeTenant(t *testing.T, db *DB) uuid.UUID {
	t.Helper()
	id, err := db.InsertTenant(context.Background(), TenantInsert{
		Slug:        "t-" + uuid.New().String()[:8],
		MegaAPIHost: "https://x", MegaAPIInstance: "i",
		MegaAPITokenEnc: []byte("a"), ChatwootURL: "https://c", ChatwootTokenEnc: []byte("b"),
		ChatwootAccountID: 1, ChatwootInboxID: 2,
		HMACSecretEnc: []byte("h"), WebhookBearerEnc: []byte("w"),
	})
	require.NoError(t, err)
	return id
}
