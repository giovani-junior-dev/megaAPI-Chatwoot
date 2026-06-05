package web

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/require"

	"github.com/madeinlowcode/chatwoot-megaapi-bridge/internal/bridge"
)

type tenantSink struct {
	mu   sync.Mutex
	last bridge.TenantInsert
	id   uuid.UUID
	err  error
}

func (s *tenantSink) insert(_ context.Context, ti bridge.TenantInsert) (uuid.UUID, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.last = ti
	if s.err != nil {
		return uuid.Nil, s.err
	}
	if s.id == uuid.Nil {
		s.id = uuid.New()
	}
	return s.id, nil
}

func validWizardForm() url.Values {
	return url.Values{
		"slug":                {"acme"},
		"megaapi_host":        {"https://m.example"},
		"megaapi_instance":    {"inst-1"},
		"megaapi_token":       {"mtok"},
		"chatwoot_url":        {"https://cw.example"},
		"chatwoot_token":      {"ctok"},
		"chatwoot_account_id": {"5"},
		"chatwoot_inbox_id":   {"9"},
	}
}

func newWizardHandler(t *testing.T, sink *tenantSink, cfg func(context.Context, MegaAPIWebhookConfig) error) *Handler {
	t.Helper()
	key := bridge.RandomBytes(32)
	store := newStore()
	store.data[settingBaseURL] = "https://bridge.example"
	h, err := New(Deps{
		Key:           key,
		InsertTenant:  sink.insert,
		GetSetting:    store.get,
		SetSetting:    store.set,
		ConfigWebhook: cfg,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return h
}

func TestWizardGETRenders(t *testing.T) {
	h := newWizardHandler(t, &tenantSink{}, nil)
	req := httptest.NewRequest(http.MethodGet, "/tenants/new", nil)
	req.AddCookie(authCookie(t, h, "a@b"))
	rr := httptest.NewRecorder()
	h.Routes().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d", rr.Code)
	}
	body := rr.Body.String()
	if !strings.Contains(body, `name="slug"`) || !strings.Contains(body, `name="megaapi_host"`) {
		t.Fatalf("missing fields")
	}
}

func TestWizardPOSTPersistsAndRedirects(t *testing.T) {
	sink := &tenantSink{}
	var capturedCfg MegaAPIWebhookConfig
	cfg := func(_ context.Context, c MegaAPIWebhookConfig) error {
		capturedCfg = c
		return nil
	}
	h := newWizardHandler(t, sink, cfg)
	req := httptest.NewRequest(http.MethodPost, "/tenants",
		strings.NewReader(validWizardForm().Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(authCookie(t, h, "a@b"))
	rr := httptest.NewRecorder()
	h.Routes().ServeHTTP(rr, req)
	if rr.Code != http.StatusFound {
		t.Fatalf("status=%d body=%q", rr.Code, rr.Body.String())
	}
	if loc := rr.Header().Get("Location"); loc != "/" {
		t.Fatalf("Location=%q", loc)
	}
	if sink.last.Slug != "acme" {
		t.Fatalf("slug=%q", sink.last.Slug)
	}
	if sink.last.ChatwootAccountID != 5 || sink.last.ChatwootInboxID != 9 {
		t.Fatalf("ids=%+v", sink.last)
	}
	if !strings.HasPrefix(capturedCfg.WebhookURL, "https://bridge.example/v1/wa/acme?token=") {
		t.Fatalf("webhook url=%q", capturedCfg.WebhookURL)
	}
	if capturedCfg.Host != "https://m.example" || capturedCfg.Instance != "inst-1" {
		t.Fatalf("cfg=%+v", capturedCfg)
	}
}

func TestWizardPOSTRejectsMissingField(t *testing.T) {
	h := newWizardHandler(t, &tenantSink{}, nil)
	form := validWizardForm()
	form.Del("slug")
	req := httptest.NewRequest(http.MethodPost, "/tenants",
		strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(authCookie(t, h, "a@b"))
	rr := httptest.NewRecorder()
	h.Routes().ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status=%d", rr.Code)
	}
}

func TestValidateSpecSlugPattern(t *testing.T) {
	base := bridge.TenantSpec{
		MegaAPIHost: "h", MegaAPIInstance: "i", MegaAPIToken: "t",
		ChatwootURL: "u", ChatwootToken: "k", ChatwootAccountID: 1, ChatwootInboxID: 1,
	}
	for _, bad := range []string{"empresa x", "Empresa", "ab", "x_y", "-abc", "café"} {
		s := base
		s.Slug = bad
		if _, err := validateSpec(s); err == nil {
			t.Errorf("expected reject for slug %q", bad)
		}
	}
	for _, ok := range []string{"empresa-x", "acme", "a1b2", "tenant-123"} {
		s := base
		s.Slug = ok
		if _, err := validateSpec(s); err != nil {
			t.Errorf("expected accept for slug %q, got %v", ok, err)
		}
	}
}

func TestWizardPOSTRejectsBadSlug(t *testing.T) {
	h := newWizardHandler(t, &tenantSink{}, nil)
	form := validWizardForm()
	form.Set("slug", "empresa x")
	req := httptest.NewRequest(http.MethodPost, "/tenants",
		strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(authCookie(t, h, "a@b"))
	rr := httptest.NewRecorder()
	h.Routes().ServeHTTP(rr, req)
	require.Equal(t, http.StatusBadRequest, rr.Code)
	require.Contains(t, strings.ToLower(rr.Body.String()), "slug")
}

func TestWizardPOSTRejectsMissingBaseURL(t *testing.T) {
	sink := &tenantSink{}
	key := bridge.RandomBytes(32)
	store := newStore()
	h, err := New(Deps{
		Key:          key,
		InsertTenant: sink.insert,
		GetSetting:   store.get,
		SetSetting:   store.set,
	})
	require.NoError(t, err)
	req := httptest.NewRequest(http.MethodPost, "/tenants",
		strings.NewReader(validWizardForm().Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(authCookie(t, h, "a@b"))
	rr := httptest.NewRecorder()
	h.Routes().ServeHTTP(rr, req)
	require.Equal(t, http.StatusBadRequest, rr.Code,
		"wizard must block tenant creation when base_url setting is missing")
	body := rr.Body.String()
	require.Contains(t, body, "Base URL")
	require.Contains(t, body, "/settings")
	require.Equal(t, uuid.Nil, sink.id,
		"InsertTenant must not run without base_url")
}

func TestWizardPOSTFiresChatwootWebhookConfig(t *testing.T) {
	sink := &tenantSink{}
	megaCfgCalled := false
	megaCfg := func(_ context.Context, _ MegaAPIWebhookConfig) error {
		megaCfgCalled = true
		return nil
	}
	var capturedCw ChatwootWebhookConfig
	cwCfg := func(_ context.Context, c ChatwootWebhookConfig) error {
		capturedCw = c
		return nil
	}
	key := bridge.RandomBytes(32)
	store := newStore()
	store.data[settingBaseURL] = "https://bridge.example"
	h, err := New(Deps{
		Key:             key,
		InsertTenant:    sink.insert,
		GetSetting:      store.get,
		SetSetting:      store.set,
		ConfigWebhook:   megaCfg,
		ConfigCwWebhook: cwCfg,
	})
	require.NoError(t, err)
	req := httptest.NewRequest(http.MethodPost, "/tenants",
		strings.NewReader(validWizardForm().Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(authCookie(t, h, "a@b"))
	rr := httptest.NewRecorder()
	h.Routes().ServeHTTP(rr, req)
	require.Equal(t, http.StatusFound, rr.Code)
	require.True(t, megaCfgCalled, "megaAPI webhook must be configured")
	require.Equal(t, "https://bridge.example/v1/cw/acme", capturedCw.WebhookURL,
		"Chatwoot webhook URL must point at bridge tenant CW endpoint")
	require.Equal(t, int64(5), capturedCw.AccountID)
	require.Equal(t, int64(9), capturedCw.InboxID)
	require.Equal(t, "https://cw.example", capturedCw.BaseURL)
	require.Equal(t, "ctok", capturedCw.Token)
}

func TestWizardPOSTPairsHMACFromChatwoot(t *testing.T) {
	sink := &tenantSink{}
	megaCfg := func(_ context.Context, _ MegaAPIWebhookConfig) error { return nil }
	cwCfg := func(_ context.Context, _ ChatwootWebhookConfig) error { return nil }
	fetchedFor := ChatwootWebhookConfig{}
	fetch := func(_ context.Context, c ChatwootWebhookConfig) (string, error) {
		fetchedFor = c
		return "chatwoot-hmac-secret", nil
	}
	var updatedID uuid.UUID
	var updatedEnc []byte
	update := func(_ context.Context, id uuid.UUID, enc []byte) error {
		updatedID = id
		updatedEnc = enc
		return nil
	}
	key := bridge.RandomBytes(32)
	store := newStore()
	store.data[settingBaseURL] = "https://bridge.example"
	h, err := New(Deps{
		Key:              key,
		InsertTenant:     sink.insert,
		GetSetting:       store.get,
		SetSetting:       store.set,
		ConfigWebhook:    megaCfg,
		ConfigCwWebhook:  cwCfg,
		FetchCwHMAC:      fetch,
		UpdateTenantHMAC: update,
	})
	require.NoError(t, err)
	req := httptest.NewRequest(http.MethodPost, "/tenants",
		strings.NewReader(validWizardForm().Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(authCookie(t, h, "a@b"))
	rr := httptest.NewRecorder()
	h.Routes().ServeHTTP(rr, req)
	require.Equal(t, http.StatusFound, rr.Code)
	require.Equal(t, "https://cw.example", fetchedFor.BaseURL)
	require.Equal(t, int64(5), fetchedFor.AccountID)
	require.Equal(t, int64(9), fetchedFor.InboxID)
	require.Equal(t, sink.id, updatedID)
	require.NotEmpty(t, updatedEnc)
	dec, err := bridge.Decrypt(updatedEnc, key)
	require.NoError(t, err)
	require.Equal(t, "chatwoot-hmac-secret", string(dec))
}

func TestWizardPOSTPairHMACFailureDoesNotBlockTenant(t *testing.T) {
	sink := &tenantSink{}
	megaCfg := func(_ context.Context, _ MegaAPIWebhookConfig) error { return nil }
	cwCfg := func(_ context.Context, _ ChatwootWebhookConfig) error { return nil }
	fetch := func(_ context.Context, _ ChatwootWebhookConfig) (string, error) {
		return "", errAllFieldsRequired
	}
	updateCalled := false
	update := func(_ context.Context, _ uuid.UUID, _ []byte) error {
		updateCalled = true
		return nil
	}
	key := bridge.RandomBytes(32)
	store := newStore()
	store.data[settingBaseURL] = "https://bridge.example"
	h, err := New(Deps{
		Key: key, InsertTenant: sink.insert, GetSetting: store.get, SetSetting: store.set,
		ConfigWebhook: megaCfg, ConfigCwWebhook: cwCfg,
		FetchCwHMAC: fetch, UpdateTenantHMAC: update,
	})
	require.NoError(t, err)
	req := httptest.NewRequest(http.MethodPost, "/tenants",
		strings.NewReader(validWizardForm().Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(authCookie(t, h, "a@b"))
	rr := httptest.NewRecorder()
	h.Routes().ServeHTTP(rr, req)
	require.Equal(t, http.StatusFound, rr.Code)
	require.False(t, updateCalled, "update must not be called when fetch fails")
}

func TestWizardPOSTPairHMACEmptyTokenSkipsUpdate(t *testing.T) {
	sink := &tenantSink{}
	megaCfg := func(_ context.Context, _ MegaAPIWebhookConfig) error { return nil }
	cwCfg := func(_ context.Context, _ ChatwootWebhookConfig) error { return nil }
	fetch := func(_ context.Context, _ ChatwootWebhookConfig) (string, error) {
		return "", nil
	}
	updateCalled := false
	update := func(_ context.Context, _ uuid.UUID, _ []byte) error {
		updateCalled = true
		return nil
	}
	key := bridge.RandomBytes(32)
	store := newStore()
	store.data[settingBaseURL] = "https://bridge.example"
	h, err := New(Deps{
		Key: key, InsertTenant: sink.insert, GetSetting: store.get, SetSetting: store.set,
		ConfigWebhook: megaCfg, ConfigCwWebhook: cwCfg,
		FetchCwHMAC: fetch, UpdateTenantHMAC: update,
	})
	require.NoError(t, err)
	req := httptest.NewRequest(http.MethodPost, "/tenants",
		strings.NewReader(validWizardForm().Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(authCookie(t, h, "a@b"))
	rr := httptest.NewRecorder()
	h.Routes().ServeHTTP(rr, req)
	require.Equal(t, http.StatusFound, rr.Code)
	require.False(t, updateCalled)
}

func TestWizardPOSTDuplicateSlugReturnsFriendlyConflict(t *testing.T) {
	sink := &tenantSink{err: &pgconn.PgError{Code: "23505", ConstraintName: "tenants_slug_key"}}
	h := newWizardHandler(t, sink, nil)
	req := httptest.NewRequest(http.MethodPost, "/tenants",
		strings.NewReader(validWizardForm().Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(authCookie(t, h, "a@b"))
	rr := httptest.NewRecorder()
	h.Routes().ServeHTTP(rr, req)
	if rr.Code != http.StatusConflict {
		t.Fatalf("status=%d want 409", rr.Code)
	}
	body := rr.Body.String()
	if strings.Contains(body, "duplicate key value") || strings.Contains(body, "tenants_slug_key") {
		t.Fatalf("response leaks raw SQL error: %s", body)
	}
	if !strings.Contains(body, "acme") || !strings.Contains(body, "já existe") {
		t.Fatalf("expected friendly PT-BR slug error, got: %s", body)
	}
}

func TestWizardRequiresAuth(t *testing.T) {
	h := newWizardHandler(t, &tenantSink{}, nil)
	req := httptest.NewRequest(http.MethodGet, "/tenants/new", nil)
	rr := httptest.NewRecorder()
	h.Routes().ServeHTTP(rr, req)
	if rr.Code != http.StatusFound {
		t.Fatalf("status=%d", rr.Code)
	}
}

func TestWizardHasStepper(t *testing.T) {
	h := newWizardHandler(t, &tenantSink{}, nil)
	req := httptest.NewRequest(http.MethodGet, "/tenants/new", nil)
	req.AddCookie(authCookie(t, h, "a@b"))
	rr := httptest.NewRecorder()
	h.Routes().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d", rr.Code)
	}
	body := rr.Body.String()
	if !strings.Contains(body, `class="stepper"`) {
		t.Fatalf("wizard must render .stepper container; body=%s", body)
	}
	if !strings.Contains(body, `class="step "`) && !strings.Contains(body, `class="step"`) {
		t.Fatalf("wizard must render .step items; body=%s", body)
	}
	if !strings.Contains(body, "step-bullet") {
		t.Fatalf("wizard must render .step-bullet; body=%s", body)
	}
	if !strings.Contains(body, "data-state") {
		t.Fatalf("wizard steps must carry data-state for Alpine to toggle; body=%s", body)
	}
}

func TestWizardUsesDesignTokens(t *testing.T) {
	h := newWizardHandler(t, &tenantSink{}, nil)
	req := httptest.NewRequest(http.MethodGet, "/tenants/new", nil)
	req.AddCookie(authCookie(t, h, "a@b"))
	rr := httptest.NewRecorder()
	h.Routes().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d", rr.Code)
	}
	body := rr.Body.String()
	for _, banned := range []string{
		"bg-slate-", "text-slate-", "border-slate-",
		"text-emerald-", "text-red-", "bg-green-",
		"shadow-", "shadow-sm",
	} {
		if strings.Contains(body, banned) {
			t.Fatalf("wizard must not use hardcoded class %q; body=%s", banned, body)
		}
	}
	if !strings.Contains(body, `class="input"`) {
		t.Fatalf("wizard fields must use .input; body=%s", body)
	}
	if !strings.Contains(body, "btn-primary") {
		t.Fatalf("wizard navigation must use .btn-primary; body=%s", body)
	}
	if !strings.Contains(body, "btn-ghost") {
		t.Fatalf("wizard back button must use .btn-ghost; body=%s", body)
	}
	if !strings.Contains(body, "field__label") {
		t.Fatalf("wizard fields must use .field__label; body=%s", body)
	}
	if !strings.Contains(body, "section-header") {
		t.Fatalf("wizard must use .section-header; body=%s", body)
	}
}

func TestWizardHasStepLabels(t *testing.T) {
	h := newWizardHandler(t, &tenantSink{}, nil)
	req := httptest.NewRequest(http.MethodGet, "/tenants/new", nil)
	req.AddCookie(authCookie(t, h, "a@b"))
	rr := httptest.NewRecorder()
	h.Routes().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d", rr.Code)
	}
	body := rr.Body.String()
	for _, label := range []string{"Identifica", "megaAPI", "Chatwoot", "Revis"}	{
		if !strings.Contains(body, label) {
			t.Fatalf("wizard stepper must label step %q; body=%s", label, body)
		}
	}
}

func TestWizardReviewSummary(t *testing.T) {
	h := newWizardHandler(t, &tenantSink{}, nil)
	req := httptest.NewRequest(http.MethodGet, "/tenants/new", nil)
	req.AddCookie(authCookie(t, h, "a@b"))
	rr := httptest.NewRecorder()
	h.Routes().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d", rr.Code)
	}
	body := rr.Body.String()
	if !strings.Contains(body, "Resumo") && !strings.Contains(body, "Revis") {
		t.Fatalf("wizard step 4 must show review summary heading; body=%s", body)
	}
	if !strings.Contains(body, `x-show="step===4"`) && !strings.Contains(body, `x-show="step == 4"`) {
		t.Fatalf("wizard step 4 must toggle via x-show on step===4; body=%s", body)
	}
}

func TestWizardSnapshot(t *testing.T) {
	h := newWizardHandler(t, &tenantSink{}, nil)
	req := httptest.NewRequest(http.MethodGet, "/tenants/new", nil)
	req.AddCookie(authCookie(t, h, "a@b"))
	rr := httptest.NewRecorder()
	h.Routes().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d", rr.Code)
	}
	goldenPath := filepath.Join("testdata", "wizard.golden.html")
	if os.Getenv("UPDATE_GOLDEN") == "1" {
		if err := os.MkdirAll("testdata", 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.WriteFile(goldenPath, rr.Body.Bytes(), 0o644); err != nil {
			t.Fatalf("write golden: %v", err)
		}
		return
	}
	want, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("golden file missing at %s; run with UPDATE_GOLDEN=1 to create", goldenPath)
	}
	if string(want) != rr.Body.String() {
		t.Fatalf("wizard snapshot drift.\nwant:\n%s\n\ngot:\n%s", want, rr.Body.String())
	}
}
