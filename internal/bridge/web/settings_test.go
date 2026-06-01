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

func TestValidateBaseURL(t *testing.T) {
	cases := []struct {
		name    string
		val     string
		wantErr bool
	}{
		{"empty", "", true},
		{"missing_scheme", "example.com", true},
		{"javascript_scheme", "javascript:alert(1)", true},
		{"ftp_scheme", "ftp://example.com", true},
		{"file_scheme", "file:///etc/passwd", true},
		{"http_only_no_host", "http://", true},
		{"https_only_no_host", "https://", true},
		{"http_basic", "http://example.com", false},
		{"https_basic", "https://example.com", false},
		{"https_with_port", "https://example.com:8443", false},
		{"https_with_path", "https://example.com/api/v1", false},
		{"https_with_query", "https://example.com/?x=1", false},
		{"https_localhost", "http://localhost", false},
		{"https_ipv4", "http://10.0.0.1:8080", false},
		{"https_trailing_slash", "https://example.com/", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateBaseURL(tc.val)
			if tc.wantErr && err == nil {
				t.Fatalf("%q: expected error", tc.val)
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("%q: unexpected error: %v", tc.val, err)
			}
		})
	}
}

func TestSettingsPOSTServiceUnavailableWhenNoSetter(t *testing.T) {
	key := make([]byte, 32)
	store := newStore()
	h, err := New(Deps{GetSetting: store.get, Key: key})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	form := url.Values{"base_url": {"https://x.com"}}
	req := httptest.NewRequest(http.MethodPost, "/settings/base_url", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(authCookie(t, h, "a@b"))
	rr := httptest.NewRecorder()
	h.Routes().ServeHTTP(rr, req)
	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 got %d", rr.Code)
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

func TestSettingsUsesDesignTokens(t *testing.T) {
	store := newStore()
	store.data[settingBaseURL] = "https://bridge.example.com"
	h := newSettingsHandler(t, store)
	req := httptest.NewRequest(http.MethodGet, "/settings", nil)
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
			t.Fatalf("settings must not use hardcoded class %q; body=%s", banned, body)
		}
	}
	if !strings.Contains(body, `class="input"`) {
		t.Fatalf("settings must use .input class; body=%s", body)
	}
	if !strings.Contains(body, "btn-primary") {
		t.Fatalf("settings must use .btn-primary for submit; body=%s", body)
	}
	if !strings.Contains(body, "field__label") {
		t.Fatalf("settings must use .field__label for form label; body=%s", body)
	}
	if !strings.Contains(body, "section-header") {
		t.Fatalf("settings must use .section-header; body=%s", body)
	}
}

func TestSettingsShowsToastWhenSaved(t *testing.T) {
	store := newStore()
	store.data[settingBaseURL] = "https://bridge.example.com"
	h := newSettingsHandler(t, store)
	req := httptest.NewRequest(http.MethodGet, "/settings?saved=1", nil)
	req.AddCookie(authCookie(t, h, "a@b"))
	rr := httptest.NewRecorder()
	h.Routes().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d", rr.Code)
	}
	body := rr.Body.String()
	if !strings.Contains(body, `class="toast`) {
		t.Fatalf("settings must render .toast when saved=1; body=%s", body)
	}
	if !strings.Contains(body, "toast-success") {
		t.Fatalf("settings must render .toast-success variant; body=%s", body)
	}
	if !strings.Contains(body, "x-data") {
		t.Fatalf("settings toast must be controlled by Alpine x-data for auto-dismiss; body=%s", body)
	}
	if strings.Contains(body, "text-green-700") {
		t.Fatalf("settings must not use raw text-green-700 inline; body=%s", body)
	}
}

func TestSettingsNoToastWhenNotSaved(t *testing.T) {
	store := newStore()
	store.data[settingBaseURL] = "https://bridge.example.com"
	h := newSettingsHandler(t, store)
	req := httptest.NewRequest(http.MethodGet, "/settings", nil)
	req.AddCookie(authCookie(t, h, "a@b"))
	rr := httptest.NewRecorder()
	h.Routes().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d", rr.Code)
	}
	body := rr.Body.String()
	if strings.Contains(body, `class="toast toast-success"`) {
		t.Fatalf("settings must not render toast when saved query is absent; body=%s", body)
	}
}

func TestSettingsSnapshot(t *testing.T) {
	store := newStore()
	store.data[settingBaseURL] = "https://bridge.example.com"
	h := newSettingsHandler(t, store)
	req := httptest.NewRequest(http.MethodGet, "/settings?saved=1", nil)
	req.AddCookie(authCookie(t, h, "a@b"))
	rr := httptest.NewRecorder()
	h.Routes().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d", rr.Code)
	}
	goldenPath := filepath.Join("testdata", "settings.golden.html")
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
		t.Fatalf("settings snapshot drift.\nwant:\n%s\n\ngot:\n%s", want, rr.Body.String())
	}
}
