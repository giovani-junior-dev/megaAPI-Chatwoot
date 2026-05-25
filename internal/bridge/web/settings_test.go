package web

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
)

type settingsStore struct {
	mu   sync.Mutex
	data map[string]string
}

func newStore() *settingsStore { return &settingsStore{data: map[string]string{}} }

func (s *settingsStore) get(_ context.Context, k string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.data[k], nil
}

func (s *settingsStore) set(_ context.Context, k, v string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data[k] = v
	return nil
}

func newSettingsHandler(t *testing.T, store *settingsStore) *Handler {
	t.Helper()
	key := make([]byte, 32)
	h, err := New(Deps{GetSetting: store.get, SetSetting: store.set, Key: key})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return h
}

func TestSettingsGETShowsCurrentValue(t *testing.T) {
	store := newStore()
	store.data[settingBaseURL] = "https://existing.example.com"
	h := newSettingsHandler(t, store)
	req := httptest.NewRequest(http.MethodGet, "/settings", nil)
	req.AddCookie(authCookie(t, h, "a@b"))
	rr := httptest.NewRecorder()
	h.Routes().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "https://existing.example.com") {
		t.Fatalf("body missing existing value")
	}
}

func TestSettingsPOSTPersists(t *testing.T) {
	store := newStore()
	h := newSettingsHandler(t, store)
	form := url.Values{"base_url": {"https://new.example.com"}}
	req := httptest.NewRequest(http.MethodPost, "/settings/base_url", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(authCookie(t, h, "a@b"))
	rr := httptest.NewRecorder()
	h.Routes().ServeHTTP(rr, req)
	if rr.Code != http.StatusFound {
		t.Fatalf("status=%d", rr.Code)
	}
	if got := store.data[settingBaseURL]; got != "https://new.example.com" {
		t.Fatalf("persisted=%q", got)
	}
}

func TestSettingsPOSTRejectsEmpty(t *testing.T) {
	store := newStore()
	h := newSettingsHandler(t, store)
	req := httptest.NewRequest(http.MethodPost, "/settings/base_url", strings.NewReader("base_url="))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(authCookie(t, h, "a@b"))
	rr := httptest.NewRecorder()
	h.Routes().ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status=%d", rr.Code)
	}
}

func TestSettingsPOSTRejectsInvalidURL(t *testing.T) {
	cases := []struct {
		name string
		val  string
	}{
		{"no_scheme", "not-a-url"},
		{"javascript", "javascript:alert(1)"},
		{"ftp_scheme", "ftp://example.com"},
		{"only_spaces_invalid", "http://"},
		{"missing_host", "https://"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			store := newStore()
			h := newSettingsHandler(t, store)
			form := url.Values{"base_url": {tc.val}}
			req := httptest.NewRequest(http.MethodPost, "/settings/base_url", strings.NewReader(form.Encode()))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			req.AddCookie(authCookie(t, h, "a@b"))
			rr := httptest.NewRecorder()
			h.Routes().ServeHTTP(rr, req)
			if rr.Code != http.StatusBadRequest {
				t.Fatalf("expected 400 for %q got %d", tc.val, rr.Code)
			}
			if _, persisted := store.data[settingBaseURL]; persisted {
				t.Fatalf("invalid value %q was persisted", tc.val)
			}
		})
	}
}

func TestSettingsPOSTAcceptsValidHTTPSURL(t *testing.T) {
	store := newStore()
	h := newSettingsHandler(t, store)
	form := url.Values{"base_url": {"https://app.example.com:8443/path"}}
	req := httptest.NewRequest(http.MethodPost, "/settings/base_url", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(authCookie(t, h, "a@b"))
	rr := httptest.NewRecorder()
	h.Routes().ServeHTTP(rr, req)
	if rr.Code != http.StatusFound {
		t.Fatalf("expected 302 got %d body=%s", rr.Code, rr.Body.String())
	}
	if got := store.data[settingBaseURL]; got != "https://app.example.com:8443/path" {
		t.Fatalf("persisted=%q", got)
	}
}

func TestSettingsGETRequiresAuth(t *testing.T) {
	h := newSettingsHandler(t, newStore())
	req := httptest.NewRequest(http.MethodGet, "/settings", nil)
	rr := httptest.NewRecorder()
	h.Routes().ServeHTTP(rr, req)
	if rr.Code != http.StatusFound {
		t.Fatalf("status=%d", rr.Code)
	}
}
