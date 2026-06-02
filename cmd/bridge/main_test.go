package main

import (
	"context"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"

	"github.com/madeinlowcode/chatwoot-megaapi-bridge/internal/bridge"
)

func TestLoadMasterKey_RejectsMissing(t *testing.T) {
	t.Setenv("MASTER_KEY", "")
	_, err := loadMasterKey()
	require.Error(t, err)
	require.Contains(t, err.Error(), "MASTER_KEY")
}

func TestLoadMasterKey_RejectsBadBase64(t *testing.T) {
	t.Setenv("MASTER_KEY", "!!!not-base64!!!")
	_, err := loadMasterKey()
	require.Error(t, err)
	require.Contains(t, err.Error(), "base64")
}

func TestLoadMasterKey_RejectsShortKey(t *testing.T) {
	short := base64.StdEncoding.EncodeToString(make([]byte, 16))
	t.Setenv("MASTER_KEY", short)
	_, err := loadMasterKey()
	require.Error(t, err)
	require.Contains(t, err.Error(), "32 bytes")
}

func TestLoadMasterKey_AcceptsExact32Bytes(t *testing.T) {
	good := base64.StdEncoding.EncodeToString(make([]byte, 32))
	t.Setenv("MASTER_KEY", good)
	key, err := loadMasterKey()
	require.NoError(t, err)
	require.Len(t, key, 32)
}

func TestLoadDSN_RequiresEnv(t *testing.T) {
	t.Setenv("DATABASE_URL", "")
	_, err := loadDSN()
	require.Error(t, err)
}

func TestParseTenantFlags_RequiresAllFlags(t *testing.T) {
	_, err := parseTenantFlags([]string{"--slug", "demo"})
	require.Error(t, err)
}

func TestParseTenantFlags_AcceptsCompleteSet(t *testing.T) {
	args := []string{
		"--slug", "demo",
		"--megaapi-host", "https://m",
		"--megaapi-instance", "i1",
		"--megaapi-token", "mtok",
		"--chatwoot-url", "https://c",
		"--chatwoot-token", "ctok",
		"--chatwoot-account", "1",
		"--chatwoot-inbox", "2",
	}
	f, err := parseTenantFlags(args)
	require.NoError(t, err)
	require.Equal(t, "demo", f.slug)
	require.Equal(t, "https://m", f.megaHost)
	require.Equal(t, int64(1), f.cwAccount)
	require.Equal(t, int64(2), f.cwInbox)
	require.False(t, f.skipReachCheck)
}

func TestBuildTenantInsert_GeneratesUniqueBearerAndHMAC(t *testing.T) {
	key := bridge.RandomBytes(32)
	f := tenantFlags{
		slug: "x", megaHost: "https://m", megaInstance: "i", megaToken: "mtok",
		cwURL: "https://c", cwToken: "ctok", cwAccount: 1, cwInbox: 2,
	}
	bearer1, hmac1, _, err := buildTenantInsert(f, key)
	require.NoError(t, err)
	bearer2, hmac2, _, err := buildTenantInsert(f, key)
	require.NoError(t, err)
	require.NotEqual(t, bearer1, bearer2)
	require.NotEqual(t, hmac1, hmac2)
}

func TestBuildTenantInsert_EncryptsAllSecrets(t *testing.T) {
	key := bridge.RandomBytes(32)
	f := tenantFlags{
		slug: "x", megaHost: "https://m", megaInstance: "i", megaToken: "mega-secret",
		cwURL: "https://c", cwToken: "cw-secret", cwAccount: 1, cwInbox: 2,
	}
	bearer, hmacSecret, ti, err := buildTenantInsert(f, key)
	require.NoError(t, err)
	require.Equal(t, "x", ti.Slug)

	plain, err := bridge.Decrypt(ti.MegaAPITokenEnc, key)
	require.NoError(t, err)
	require.Equal(t, "mega-secret", string(plain))

	plain, err = bridge.Decrypt(ti.ChatwootTokenEnc, key)
	require.NoError(t, err)
	require.Equal(t, "cw-secret", string(plain))

	plain, err = bridge.Decrypt(ti.WebhookBearerEnc, key)
	require.NoError(t, err)
	require.Equal(t, bearer, string(plain))

	plain, err = bridge.Decrypt(ti.HMACSecretEnc, key)
	require.NoError(t, err)
	require.Equal(t, hmacSecret, string(plain))
}

func TestReachCheck_OK(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()
	require.NoError(t, reachCheck(context.Background(), srv.URL))
}

func TestReachCheck_5xxFails(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()
	err := reachCheck(context.Background(), srv.URL)
	require.Error(t, err)
}

func TestReachCheck_BadURL(t *testing.T) {
	err := reachCheck(context.Background(), "://not-a-url")
	require.Error(t, err)
}

func TestBootstrapDatabaseIdempotent(t *testing.T) {
	t.Run("creates role and database when absent", func(t *testing.T) {
		var execed []string
		exists := func(_ context.Context, _ string, _ ...any) (bool, error) { return false, nil }
		exec := func(_ context.Context, sql string) error { execed = append(execed, sql); return nil }
		err := bootstrapDatabase(context.Background(), exists, exec, "bridge", "bridge", "s3cr3t")
		require.NoError(t, err)
		require.Len(t, execed, 2)
		require.Contains(t, execed[0], "CREATE ROLE")
		require.Contains(t, execed[0], `"bridge"`)
		require.Contains(t, execed[1], "CREATE DATABASE")
		require.Contains(t, execed[1], "OWNER")
	})

	t.Run("skips creation when role and database already present", func(t *testing.T) {
		var execed []string
		exists := func(_ context.Context, _ string, _ ...any) (bool, error) { return true, nil }
		exec := func(_ context.Context, sql string) error { execed = append(execed, sql); return nil }
		err := bootstrapDatabase(context.Background(), exists, exec, "bridge", "bridge", "s3cr3t")
		require.NoError(t, err)
		require.Empty(t, execed)
	})

	t.Run("ignores already-exists race errors", func(t *testing.T) {
		exists := func(_ context.Context, _ string, _ ...any) (bool, error) { return false, nil }
		exec := func(_ context.Context, sql string) error {
			if strings.Contains(sql, "DATABASE") {
				return &pgconn.PgError{Code: "42P04"} // duplicate_database
			}
			return &pgconn.PgError{Code: "42710"} // duplicate_object (role)
		}
		err := bootstrapDatabase(context.Background(), exists, exec, "bridge", "bridge", "s3cr3t")
		require.NoError(t, err)
	})

	t.Run("propagates real errors", func(t *testing.T) {
		exists := func(_ context.Context, _ string, _ ...any) (bool, error) { return false, nil }
		exec := func(_ context.Context, _ string) error { return &pgconn.PgError{Code: "42501"} } // insufficient_privilege
		err := bootstrapDatabase(context.Background(), exists, exec, "bridge", "bridge", "s3cr3t")
		require.Error(t, err)
	})
}

func TestServeMigrationsToggle(t *testing.T) {
	log := zerolog.Nop()

	t.Run("runs when RUN_MIGRATIONS unset", func(t *testing.T) {
		t.Setenv("RUN_MIGRATIONS", "")
		called := false
		err := runMigrationsIfEnabled(log, func() error { called = true; return nil })
		require.NoError(t, err)
		require.True(t, called)
	})

	t.Run("runs when RUN_MIGRATIONS=1", func(t *testing.T) {
		t.Setenv("RUN_MIGRATIONS", "1")
		called := false
		err := runMigrationsIfEnabled(log, func() error { called = true; return nil })
		require.NoError(t, err)
		require.True(t, called)
	})

	t.Run("skips when RUN_MIGRATIONS=0", func(t *testing.T) {
		t.Setenv("RUN_MIGRATIONS", "0")
		called := false
		err := runMigrationsIfEnabled(log, func() error { called = true; return nil })
		require.NoError(t, err)
		require.False(t, called)
	})
}
