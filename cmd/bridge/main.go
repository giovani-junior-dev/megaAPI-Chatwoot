package main

import (
	"context"
	"encoding/base64"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/rs/zerolog"

	"github.com/madeinlowcode/chatwoot-megaapi-bridge/internal/bridge"
	"github.com/madeinlowcode/chatwoot-megaapi-bridge/internal/bridge/web"
	"github.com/madeinlowcode/chatwoot-megaapi-bridge/migrations"
)

const usage = `bridge — chatwoot-megaapi-bridge

Usage:
  bridge serve         Run the HTTP server and worker pools
  bridge migrate       Apply embedded migrations
  bridge tenant add    Register a new tenant
  bridge admin add     Create/replace the admin login

Environment (serve):
  DATABASE_URL         Postgres DSN the app connects to (required).
  MASTER_KEY           32-byte base64 AES key for tenant secrets (required).
  POSTGRES_ADMIN_URL   Optional. Maintenance DSN (e.g. postgres://postgres:...@host/postgres)
                       used on first boot to create the bridge role+database
                       parsed from DATABASE_URL. Idempotent; omit once provisioned.
  RUN_MIGRATIONS       Default 1. Set to 0 to skip the boot-time migrate step.
`

func main() {
	if len(os.Args) < 2 {
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}
	log := newLogger()
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	cmd := os.Args[1]
	args := os.Args[2:]
	if err := dispatch(ctx, log, cmd, args); err != nil {
		log.Fatal().Err(err).Msg("command failed")
	}
}

func dispatch(ctx context.Context, log zerolog.Logger, cmd string, args []string) error {
	switch cmd {
	case "serve":
		return cmdServe(ctx, log)
	case "migrate":
		return cmdMigrate(ctx, log)
	case "tenant":
		return cmdTenant(ctx, log, args)
	case "admin":
		return cmdAdmin(ctx, log, args)
	case "-h", "--help", "help":
		fmt.Fprint(os.Stdout, usage)
		return nil
	default:
		return fmt.Errorf("unknown command %q", cmd)
	}
}

func newLogger() zerolog.Logger {
	level := strings.ToLower(getEnv("LOG_LEVEL", "info"))
	lv, err := zerolog.ParseLevel(level)
	if err != nil {
		lv = zerolog.InfoLevel
	}
	zerolog.SetGlobalLevel(lv)
	return zerolog.New(os.Stdout).With().Timestamp().Logger()
}

func getEnv(k, def string) string {
	if v, ok := os.LookupEnv(k); ok && v != "" {
		return v
	}
	return def
}

func loadMasterKey() ([]byte, error) {
	raw := os.Getenv("MASTER_KEY")
	if raw == "" {
		return nil, errors.New("MASTER_KEY env var is required (32 bytes base64)")
	}
	key, err := base64.StdEncoding.DecodeString(raw)
	if err != nil {
		return nil, fmt.Errorf("MASTER_KEY base64 decode: %w", err)
	}
	if len(key) != 32 {
		return nil, fmt.Errorf("MASTER_KEY must decode to 32 bytes, got %d", len(key))
	}
	return key, nil
}

func loadDSN() (string, error) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		return "", errors.New("DATABASE_URL env var is required")
	}
	return dsn, nil
}

func cmdServe(ctx context.Context, log zerolog.Logger) error {
	key, dsn, err := loadKeyAndDSN()
	if err != nil {
		return err
	}
	if err := maybeBootstrap(ctx, log, dsn); err != nil {
		return err
	}
	db, err := bridge.NewDB(ctx, dsn)
	if err != nil {
		return err
	}
	defer db.Close()
	if err := runMigrationsIfEnabled(log, func() error { return runMigrations(ctx, db, log) }); err != nil {
		return err
	}
	shutdown, err := bridge.InitTracer(ctx)
	if err != nil {
		log.Warn().Err(err).Msg("otel init failed; continuing without tracing")
	}
	defer func() {
		sc, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = shutdown(sc)
	}()
	srv := bridge.NewServer(db, key, loadServerConfig(), log)
	wh, err := web.NewFromDB(db, key)
	if err != nil {
		return err
	}
	srv.Web = wh.Routes()
	if err := srv.RecoverPending(ctx); err != nil {
		log.Warn().Err(err).Msg("recover pending failed")
	}
	go srv.RunWorkers(ctx)
	go srv.RunStaleJanitor(ctx)
	go srv.RunQueueDepthUpdater(ctx)
	return runHTTP(ctx, log, srv)
}

// loadServerConfig wires runtime tuning from env so the same binary can be
// pushed under load tests (BUFFER_LIMIT, WORKERS) without touching bridge code.
func loadServerConfig() bridge.Config {
	cfg := bridge.Config{}
	if v, err := strconv.Atoi(getEnv("BUFFER_LIMIT", "")); err == nil && v > 0 {
		cfg.BufferLimit = v
	}
	if v, err := strconv.Atoi(getEnv("WORKERS", "")); err == nil && v >= 0 {
		cfg.Workers = v
	}
	cfg.AdminToken = os.Getenv("ADMIN_TOKEN")
	return cfg
}

func runHTTP(ctx context.Context, log zerolog.Logger, srv *bridge.Server) error {
	addr := ":" + getEnv("BRIDGE_PORT", "8080")
	httpSrv := &http.Server{Addr: addr, Handler: srv.Routes(), ReadHeaderTimeout: 10 * time.Second}
	go func() { // #nosec G118 -- shutdown needs a fresh deadline once the parent ctx is cancelled
		<-ctx.Done()
		c, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = httpSrv.Shutdown(c)
	}()
	log.Info().Str("addr", addr).Msg("listening")
	if err := httpSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

func cmdMigrate(ctx context.Context, log zerolog.Logger) error {
	dsn, err := loadDSN()
	if err != nil {
		return err
	}
	db, err := bridge.NewDB(ctx, dsn)
	if err != nil {
		return err
	}
	defer db.Close()
	return runMigrations(ctx, db, log)
}

func runMigrations(ctx context.Context, db *bridge.DB, log zerolog.Logger) error {
	files, err := fs.ReadDir(migrations.FS, ".")
	if err != nil {
		return err
	}
	return applyMigrations(ctx, db, log, files)
}

// migrationsEnabled gates the boot-time migrate step: any value other than the
// literal "0" keeps it on so a fresh swarm install self-migrates by default.
func migrationsEnabled() bool { return getEnv("RUN_MIGRATIONS", "1") != "0" }

func runMigrationsIfEnabled(log zerolog.Logger, runner func() error) error {
	if !migrationsEnabled() {
		log.Info().Msg("RUN_MIGRATIONS=0; skipping boot-time migrations")
		return nil
	}
	return runner()
}

// maybeBootstrap provisions the bridge role+database on the maintenance server
// pointed at by POSTGRES_ADMIN_URL before the app connects to DATABASE_URL.
// Without POSTGRES_ADMIN_URL it is a no-op (DB assumed to already exist).
func maybeBootstrap(ctx context.Context, log zerolog.Logger, dsn string) error {
	adminURL := os.Getenv("POSTGRES_ADMIN_URL")
	if adminURL == "" {
		return nil
	}
	role, dbname, password, err := parseDBCreds(dsn)
	if err != nil {
		return fmt.Errorf("parse DATABASE_URL for bootstrap: %w", err)
	}
	conn, err := pgx.Connect(ctx, adminURL)
	if err != nil {
		return fmt.Errorf("connect POSTGRES_ADMIN_URL: %w", err)
	}
	defer func() { _ = conn.Close(ctx) }()
	exists := func(ctx context.Context, query string, args ...any) (bool, error) {
		var one int
		qErr := conn.QueryRow(ctx, query, args...).Scan(&one)
		if errors.Is(qErr, pgx.ErrNoRows) {
			return false, nil
		}
		if qErr != nil {
			return false, qErr
		}
		return true, nil
	}
	exec := func(ctx context.Context, sql string) error {
		_, eErr := conn.Exec(ctx, sql)
		return eErr
	}
	if err := bootstrapDatabase(ctx, exists, exec, role, dbname, password); err != nil {
		return err
	}
	log.Info().Str("role", role).Str("database", dbname).Msg("database bootstrap complete")
	return nil
}

func parseDBCreds(dsn string) (role, dbname, password string, err error) {
	u, err := url.Parse(dsn)
	if err != nil {
		return "", "", "", err
	}
	role = u.User.Username()
	password, _ = u.User.Password()
	dbname = strings.TrimPrefix(u.Path, "/")
	if role == "" || dbname == "" {
		return "", "", "", errors.New("DATABASE_URL must contain user and database name")
	}
	return role, dbname, password, nil
}

// bootstrapDatabase creates the role and database when missing. CREATE ROLE/
// DATABASE cannot bind parameters, so identifiers are sanitized via
// pgx.Identifier and the password literal is single-quote escaped. Existence is
// checked first; a concurrent creator's duplicate error is treated as success.
func bootstrapDatabase(
	ctx context.Context,
	exists func(ctx context.Context, query string, args ...any) (bool, error),
	exec func(ctx context.Context, sql string) error,
	role, dbname, password string,
) error {
	roleExists, err := exists(ctx, "SELECT 1 FROM pg_roles WHERE rolname = $1", role)
	if err != nil {
		return err
	}
	if !roleExists {
		stmt := fmt.Sprintf("CREATE ROLE %s LOGIN PASSWORD %s", pgx.Identifier{role}.Sanitize(), quoteLiteral(password)) // #nosec G201 -- identifier sanitized, literal escaped; DDL cannot bind params
		if err := exec(ctx, stmt); err != nil && !isAlreadyExists(err) {
			return err
		}
	}
	dbExists, err := exists(ctx, "SELECT 1 FROM pg_database WHERE datname = $1", dbname)
	if err != nil {
		return err
	}
	if !dbExists {
		stmt := fmt.Sprintf("CREATE DATABASE %s OWNER %s", pgx.Identifier{dbname}.Sanitize(), pgx.Identifier{role}.Sanitize()) // #nosec G201 -- identifiers sanitized; DDL cannot bind params
		if err := exec(ctx, stmt); err != nil && !isAlreadyExists(err) {
			return err
		}
	}
	return nil
}

func quoteLiteral(s string) string { return "'" + strings.ReplaceAll(s, "'", "''") + "'" }

func isAlreadyExists(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == "42710" || pgErr.Code == "42P04" // duplicate_object / duplicate_database
	}
	return false
}

func applyMigrations(ctx context.Context, db *bridge.DB, log zerolog.Logger, files []fs.DirEntry) error {
	for _, f := range files {
		if f.IsDir() || !strings.HasSuffix(f.Name(), ".sql") {
			continue
		}
		body, err := migrations.FS.ReadFile(f.Name())
		if err != nil {
			return err
		}
		if _, err := db.Pool.Exec(ctx, string(body)); err != nil {
			return fmt.Errorf("apply %s: %w", f.Name(), err)
		}
		log.Info().Str("file", f.Name()).Msg("migration applied")
	}
	return nil
}

func cmdTenant(ctx context.Context, log zerolog.Logger, args []string) error {
	if len(args) == 0 || args[0] != "add" {
		return errors.New("usage: bridge tenant add --slug ... --megaapi-host ... etc.")
	}
	return cmdTenantAdd(ctx, log, args[1:])
}

type tenantFlags struct {
	slug, provider                            string
	megaHost, megaInstance, megaToken         string
	wablastKey, wablastAccount, wablastSecret string
	cwURL, cwToken                            string
	cwAccount, cwInbox                        int64
	skipReachCheck                            bool
}

func parseTenantFlags(args []string) (tenantFlags, error) {
	fs := flag.NewFlagSet("tenant add", flag.ContinueOnError)
	var f tenantFlags
	fs.StringVar(&f.slug, "slug", "", "tenant slug (lowercase, dashes)")
	fs.StringVar(&f.provider, "provider", "megaapi", "provider: megaapi | wablast")
	fs.StringVar(&f.megaHost, "megaapi-host", "", "megaAPI base URL")
	fs.StringVar(&f.megaInstance, "megaapi-instance", "", "megaAPI instance ID")
	fs.StringVar(&f.megaToken, "megaapi-token", "", "megaAPI bearer token")
	fs.StringVar(&f.wablastKey, "wablast-api-key", "", "WaBlast API key (wak_...)")
	fs.StringVar(&f.wablastAccount, "wablast-account-id", "", "WaBlast connection id (optional)")
	fs.StringVar(&f.wablastSecret, "wablast-webhook-secret", "", "WaBlast webhook signing secret (whsec_...)")
	fs.StringVar(&f.cwURL, "chatwoot-url", "", "Chatwoot base URL")
	fs.StringVar(&f.cwToken, "chatwoot-token", "", "Chatwoot api_access_token")
	fs.Int64Var(&f.cwAccount, "chatwoot-account", 0, "Chatwoot account id")
	fs.Int64Var(&f.cwInbox, "chatwoot-inbox", 0, "Chatwoot inbox id")
	fs.BoolVar(&f.skipReachCheck, "skip-reach-check", false, "skip HEAD reachability check (test/dev)")
	if err := fs.Parse(args); err != nil {
		return f, err
	}
	return f, validateTenantFlags(f)
}

func validateTenantFlags(f tenantFlags) error {
	if f.slug == "" || f.cwURL == "" || f.cwToken == "" || f.cwAccount == 0 || f.cwInbox == 0 {
		return errors.New("--slug and all --chatwoot-* flags are required")
	}
	if f.provider == "wablast" {
		if f.wablastKey == "" {
			return errors.New("--wablast-api-key is required for --provider wablast")
		}
		return nil
	}
	if f.megaHost == "" || f.megaInstance == "" || f.megaToken == "" {
		return errors.New("all --megaapi-* flags are required for --provider megaapi")
	}
	return nil
}

func cmdTenantAdd(ctx context.Context, log zerolog.Logger, args []string) error {
	f, err := parseTenantFlags(args)
	if err != nil {
		return err
	}
	key, dsn, err := loadKeyAndDSN()
	if err != nil {
		return err
	}
	if !f.skipReachCheck {
		if err := reachForProvider(ctx, f); err != nil {
			return err
		}
	}
	db, err := bridge.NewDB(ctx, dsn)
	if err != nil {
		return err
	}
	defer db.Close()
	bearer, hmacSecret, ti, err := buildTenantInsert(f, key)
	if err != nil {
		return err
	}
	id, err := db.InsertTenant(ctx, ti)
	if err != nil {
		return err
	}
	log.Info().Str("tenant_id", id.String()).Str("slug", f.slug).Msg("tenant created")
	fmt.Printf("Tenant created: %s\nWebhook Bearer: %s\nHMAC Secret: %s\n", id, bearer, hmacSecret)
	return nil
}

func loadKeyAndDSN() ([]byte, string, error) {
	key, err := loadMasterKey()
	if err != nil {
		return nil, "", err
	}
	dsn, err := loadDSN()
	if err != nil {
		return nil, "", err
	}
	return key, dsn, nil
}

func reachForProvider(ctx context.Context, f tenantFlags) error {
	if f.provider != "wablast" {
		if err := reachCheck(ctx, f.megaHost); err != nil {
			return fmt.Errorf("megaapi-host unreachable: %w", err)
		}
	}
	if err := reachCheck(ctx, f.cwURL); err != nil {
		return fmt.Errorf("chatwoot-url unreachable: %w", err)
	}
	return nil
}

func buildTenantInsert(f tenantFlags, key []byte) (string, string, bridge.TenantInsert, error) {
	bearer, hmacSecret, ti, err := bridge.BuildTenantInsert(key, bridge.TenantSpec{
		Slug:              f.slug,
		Provider:          f.provider,
		MegaAPIHost:       f.megaHost,
		MegaAPIInstance:   f.megaInstance,
		MegaAPIToken:      f.megaToken,
		WablastAPIKey:     f.wablastKey,
		WablastAccountID:  f.wablastAccount,
		ChatwootURL:       f.cwURL,
		ChatwootToken:     f.cwToken,
		ChatwootAccountID: f.cwAccount,
		ChatwootInboxID:   f.cwInbox,
	})
	if err != nil {
		return "", "", bridge.TenantInsert{}, err
	}
	if f.provider == "wablast" && f.wablastSecret != "" {
		enc, encErr := bridge.Encrypt([]byte(f.wablastSecret), key)
		if encErr != nil {
			return "", "", bridge.TenantInsert{}, encErr
		}
		ti.WablastWebhookSecretEnc = enc
	}
	return bearer, hmacSecret, ti, nil
}

func reachCheck(ctx context.Context, rawURL string) error {
	cl := &http.Client{Timeout: 5 * time.Second}
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, rawURL, nil)
	if err != nil {
		return err
	}
	resp, err := cl.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 500 {
		return fmt.Errorf("status %d", resp.StatusCode)
	}
	return nil
}
