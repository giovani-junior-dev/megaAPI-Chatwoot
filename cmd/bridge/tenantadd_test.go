package main

import (
	"context"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"

	"github.com/madeinlowcode/chatwoot-megaapi-bridge/internal/bridge"
)

func validKey32() string {
	return base64.StdEncoding.EncodeToString(make([]byte, 32))
}

func reachableHost(t *testing.T) (string, func()) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	return srv.URL, srv.Close
}

func validTenantArgs(slug, megaHost, cwHost string) []string {
	return []string{
		"--slug", slug,
		"--megaapi-host", megaHost,
		"--megaapi-instance", "inst-1",
		"--megaapi-token", "mega-tok",
		"--chatwoot-url", cwHost,
		"--chatwoot-token", "cw-tok",
		"--chatwoot-account", "10",
		"--chatwoot-inbox", "20",
	}
}

func TestCmdTenantAdd_MissingMasterKeyFails(t *testing.T) {
	t.Setenv("MASTER_KEY", "")
	t.Setenv("DATABASE_URL", "postgres://x")
	mega, closeMega := reachableHost(t)
	defer closeMega()
	cw, closeCW := reachableHost(t)
	defer closeCW()

	err := cmdTenantAdd(context.Background(), zerolog.Nop(),
		validTenantArgs("t-miss-key", mega, cw))
	require.Error(t, err)
	require.Contains(t, err.Error(), "MASTER_KEY")
}

func TestCmdTenantAdd_MissingDatabaseURLFails(t *testing.T) {
	t.Setenv("MASTER_KEY", validKey32())
	t.Setenv("DATABASE_URL", "")
	mega, closeMega := reachableHost(t)
	defer closeMega()
	cw, closeCW := reachableHost(t)
	defer closeCW()

	err := cmdTenantAdd(context.Background(), zerolog.Nop(),
		validTenantArgs("t-miss-dsn", mega, cw))
	require.Error(t, err)
	require.Contains(t, err.Error(), "DATABASE_URL")
}

func TestCmdTenantAdd_InvalidHostFails(t *testing.T) {
	t.Setenv("MASTER_KEY", validKey32())
	t.Setenv("DATABASE_URL", "postgres://valid-but-unused")
	// megaapi-host points at a closed port (port 1 is reserved/unreachable),
	// so reachCheck fails before any DB code runs.
	cw, closeCW := reachableHost(t)
	defer closeCW()

	err := cmdTenantAdd(context.Background(), zerolog.Nop(),
		validTenantArgs("t-bad-host", "http://127.0.0.1:1", cw))
	require.Error(t, err)
	require.Contains(t, err.Error(), "megaapi-host unreachable")
}

func TestCmdTenantAdd_ParseFlagErrorSurfaces(t *testing.T) {
	t.Setenv("MASTER_KEY", validKey32())
	t.Setenv("DATABASE_URL", "postgres://valid")
	err := cmdTenantAdd(context.Background(), zerolog.Nop(),
		[]string{"--slug", "only-slug"})
	require.Error(t, err)
}

func TestCmdTenantAdd_ChatwootHostUnreachableFails(t *testing.T) {
	t.Setenv("MASTER_KEY", validKey32())
	t.Setenv("DATABASE_URL", "postgres://valid")
	mega, closeMega := reachableHost(t)
	defer closeMega()

	err := cmdTenantAdd(context.Background(), zerolog.Nop(),
		validTenantArgs("t-bad-cw", mega, "http://127.0.0.1:1"))
	require.Error(t, err)
	require.Contains(t, err.Error(), "chatwoot-url unreachable")
}

// buildTenantInsert+Decrypt round-trip already covered, but verify a
// generated bearer is base64url-safe per provisioning contract.
func TestCmdTenantAdd_GeneratedBearerIsURLSafe(t *testing.T) {
	key := bridge.RandomBytes(32)
	f := tenantFlags{
		slug: "x", megaHost: "https://m", megaInstance: "i", megaToken: "t",
		cwURL: "https://c", cwToken: "t", cwAccount: 1, cwInbox: 2,
	}
	bearer, _, _, err := buildTenantInsert(f, key)
	require.NoError(t, err)
	_, decErr := base64.RawURLEncoding.DecodeString(bearer)
	require.NoError(t, decErr, "bearer must be base64url-safe for use in webhook config")
}
