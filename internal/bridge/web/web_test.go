package web

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/madeinlowcode/chatwoot-megaapi-bridge/internal/bridge"
)

func authCookie(t *testing.T, h *Handler, email string) *http.Cookie {
	t.Helper()
	tok, err := bridge.NewSession(email, h.deps.Key, time.Hour)
	if err != nil {
		t.Fatalf("NewSession: %v", err)
	}
	return &http.Cookie{Name: cookieName, Value: tok}
}

func TestHandleIndexReturnsLayout(t *testing.T) {
	h := newTestHandler(t, nil)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(authCookie(t, h, "admin@example.com"))
	rr := httptest.NewRecorder()
	h.Routes().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d body=%q", rr.Code, rr.Body.String())
	}
	body := rr.Body.String()
	wants := []string{"htmx.org", "alpinejs", "tailwindcss", "Bridge Admin", "<html", "</html>"}
	for _, w := range wants {
		if !strings.Contains(body, w) {
			t.Errorf("body missing %q", w)
		}
	}
	if ct := rr.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
		t.Errorf("content-type=%q", ct)
	}
}

func TestUnauthenticatedRedirectsToLogin(t *testing.T) {
	h := newTestHandler(t, nil)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	h.Routes().ServeHTTP(rr, req)
	if rr.Code != http.StatusFound {
		t.Fatalf("status=%d", rr.Code)
	}
	if loc := rr.Header().Get("Location"); loc != "/login" {
		t.Fatalf("Location=%q", loc)
	}
}
