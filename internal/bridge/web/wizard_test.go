package web

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"

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
