package web

import (
	"context"
	"errors"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/madeinlowcode/chatwoot-megaapi-bridge/internal/bridge"
)

func newTestHandler(t *testing.T, get func(context.Context, string) (bridge.Admin, error)) *Handler {
	t.Helper()
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i + 1)
	}
	h, err := New(Deps{GetAdmin: get, Key: key})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return h
}

func TestLoginFormGET(t *testing.T) {
	h := newTestHandler(t, nil)
	req := httptest.NewRequest(http.MethodGet, "/login", nil)
	rr := httptest.NewRecorder()
	h.Routes().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), `name="password"`) {
		t.Fatalf("form missing password field")
	}
}

func TestLoginUsesDesignTokens(t *testing.T) {
	h := newTestHandler(t, nil)
	req := httptest.NewRequest(http.MethodGet, "/login", nil)
	rr := httptest.NewRecorder()
	h.Routes().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d", rr.Code)
	}
	body := rr.Body.String()
	for _, banned := range []string{"bg-slate-", "text-slate-", "text-emerald-", "text-red-", "bg-green-", "shadow-", "shadow-sm", "rounded-lg"} {
		if strings.Contains(body, banned) {
			t.Fatalf("login must not use hardcoded tailwind class %q", banned)
		}
	}
	if !strings.Contains(body, `class="input"`) {
		t.Fatalf("login email input must use .input class")
	}
	if !strings.Contains(body, "btn-primary") {
		t.Fatalf("login submit must use .btn-primary class; body=%s", body)
	}
	if !strings.Contains(body, `autofocus`) {
		t.Fatalf("login email must autofocus")
	}
	if !strings.Contains(body, `autocomplete="username"`) {
		t.Fatalf("login email must autocomplete username")
	}
	if !strings.Contains(body, `autocomplete="current-password"`) {
		t.Fatalf("login password must autocomplete current-password")
	}
}

func TestLoginPOSTSuccess(t *testing.T) {
	hash, _ := bridge.HashPassword("s3cret")
	get := func(_ context.Context, email string) (bridge.Admin, error) {
		if email != "admin@example.com" {
			return bridge.Admin{}, bridge.ErrNotFound
		}
		return bridge.Admin{Email: email, PasswordHash: hash}, nil
	}
	h := newTestHandler(t, get)
	form := url.Values{"email": {"admin@example.com"}, "password": {"s3cret"}}
	req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()
	h.Routes().ServeHTTP(rr, req)
	if rr.Code != http.StatusFound {
		t.Fatalf("status=%d body=%q", rr.Code, rr.Body.String())
	}
	if loc := rr.Header().Get("Location"); loc != "/" {
		t.Fatalf("Location=%q", loc)
	}
	c := findCookie(rr.Result().Cookies(), cookieName)
	if c == nil {
		t.Fatalf("missing %s cookie", cookieName)
	}
	if !c.HttpOnly {
		t.Errorf("cookie not httponly")
	}
	if c.SameSite != http.SameSiteLaxMode {
		t.Errorf("cookie SameSite=%v", c.SameSite)
	}
	if c.Value == "" {
		t.Errorf("cookie value empty")
	}
}

func TestLoginPOSTWrongPassword(t *testing.T) {
	hash, _ := bridge.HashPassword("right")
	get := func(_ context.Context, _ string) (bridge.Admin, error) {
		return bridge.Admin{Email: "a@b", PasswordHash: hash}, nil
	}
	h := newTestHandler(t, get)
	form := url.Values{"email": {"a@b"}, "password": {"wrong"}}
	req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()
	h.Routes().ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d", rr.Code)
	}
}

func TestLoginPOSTUnknownEmail(t *testing.T) {
	get := func(_ context.Context, _ string) (bridge.Admin, error) {
		return bridge.Admin{}, bridge.ErrNotFound
	}
	h := newTestHandler(t, get)
	form := url.Values{"email": {"x@y"}, "password": {"p"}}
	req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()
	h.Routes().ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d", rr.Code)
	}
}

func TestLoginPOSTGetAdminError(t *testing.T) {
	get := func(_ context.Context, _ string) (bridge.Admin, error) {
		return bridge.Admin{}, errors.New("db down")
	}
	h := newTestHandler(t, get)
	form := url.Values{"email": {"x"}, "password": {"y"}}
	req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()
	h.Routes().ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d", rr.Code)
	}
}

func TestLogoutClearsCookie(t *testing.T) {
	h := newTestHandler(t, nil)
	req := httptest.NewRequest(http.MethodPost, "/logout", nil)
	rr := httptest.NewRecorder()
	h.Routes().ServeHTTP(rr, req)
	if rr.Code != http.StatusFound {
		t.Fatalf("status=%d", rr.Code)
	}
	c := findCookie(rr.Result().Cookies(), cookieName)
	if c == nil || c.MaxAge >= 0 {
		t.Fatalf("expected expired cookie, got %+v", c)
	}
}

func TestLoginSnapshot(t *testing.T) {
	h := newTestHandler(t, nil)
	req := httptest.NewRequest(http.MethodGet, "/login", nil)
	rr := httptest.NewRecorder()
	h.Routes().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d", rr.Code)
	}
	goldenPath := filepath.Join("testdata", "login.golden.html")
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
		if errors.Is(err, fs.ErrNotExist) {
			t.Fatalf("golden file missing at %s; run with UPDATE_GOLDEN=1 to create", goldenPath)
		}
		t.Fatalf("read golden: %v", err)
	}
	if string(want) != rr.Body.String() {
		t.Fatalf("login snapshot drift.\nwant:\n%s\n\ngot:\n%s", want, rr.Body.String())
	}
}

func findCookie(cs []*http.Cookie, name string) *http.Cookie {
	for _, c := range cs {
		if c.Name == name {
			return c
		}
	}
	return nil
}
