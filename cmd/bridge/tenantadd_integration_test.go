//go:build integration

package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/madeinlowcode/chatwoot-megaapi-bridge/internal/bridge"
)

func startTestDB(t *testing.T) string {
	t.Helper()
	ctx := context.Background()
	pgC, err := postgres.Run(ctx, "postgres:15-alpine",
		postgres.WithDatabase("bridge"),
		postgres.WithUsername("bridge"),
		postgres.WithPassword("bridge"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).WithStartupTimeout(60*time.Second),
		),
	)
	require.NoError(t, err)
	t.Cleanup(func() { _ = pgC.Terminate(context.Background()) })

	dsn, err := pgC.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)

	db, err := bridge.NewDB(ctx, dsn)
	require.NoError(t, err)
	t.Cleanup(db.Close)

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
	return dsn
}

func TestCmdTenantAdd_HappyPathInsertsTenant(t *testing.T) {
	dsn := startTestDB(t)
	t.Setenv("MASTER_KEY", validKey32())
	t.Setenv("DATABASE_URL", dsn)
	mega, closeMega := reachableHost(t)
	defer closeMega()
	cw, closeCW := reachableHost(t)
	defer closeCW()

	ctx := context.Background()
	require.NoError(t, cmdTenantAdd(ctx, zerolog.Nop(),
		validTenantArgs("t-happy", mega, cw)))

	db, err := bridge.NewDB(ctx, dsn)
	require.NoError(t, err)
	defer db.Close()
	tn, err := db.GetTenantBySlug(ctx, "t-happy")
	require.NoError(t, err)
	require.Equal(t, "t-happy", tn.Slug)
	require.Equal(t, int64(10), tn.ChatwootAccountID)
	require.Equal(t, int64(20), tn.ChatwootInboxID)
}

func TestCmdTenantAdd_DuplicateSlugReturnsError(t *testing.T) {
	dsn := startTestDB(t)
	t.Setenv("MASTER_KEY", validKey32())
	t.Setenv("DATABASE_URL", dsn)
	mega, closeMega := reachableHost(t)
	defer closeMega()
	cw, closeCW := reachableHost(t)
	defer closeCW()

	ctx := context.Background()
	require.NoError(t, cmdTenantAdd(ctx, zerolog.Nop(),
		validTenantArgs("t-dup", mega, cw)))

	err := cmdTenantAdd(ctx, zerolog.Nop(),
		validTenantArgs("t-dup", mega, cw))
	require.Error(t, err, "second insert with same slug must fail on unique constraint")
}

func TestCmdTenantAdd_SkipReachCheckBypassesUnreachableHost(t *testing.T) {
	dsn := startTestDB(t)
	t.Setenv("MASTER_KEY", validKey32())
	t.Setenv("DATABASE_URL", dsn)

	args := append(validTenantArgs("t-skip-reach", "http://127.0.0.1:1", "http://127.0.0.1:1"),
		"--skip-reach-check")
	require.NoError(t, cmdTenantAdd(context.Background(), zerolog.Nop(), args))

	db, err := bridge.NewDB(context.Background(), dsn)
	require.NoError(t, err)
	defer db.Close()
	tn, err := db.GetTenantBySlug(context.Background(), "t-skip-reach")
	require.NoError(t, err)
	require.Equal(t, "t-skip-reach", tn.Slug)
}

// Confirms cmdTenantAdd respects ctx cancellation surfacing from the reach check
// rather than blocking on a slow upstream forever.
func TestCmdTenantAdd_ContextCanceledFails(t *testing.T) {
	t.Setenv("MASTER_KEY", validKey32())
	t.Setenv("DATABASE_URL", "postgres://valid")

	slow := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	defer slow.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := cmdTenantAdd(ctx, zerolog.Nop(),
		validTenantArgs("t-canceled", slow.URL, slow.URL))
	require.Error(t, err)
}
