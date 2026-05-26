package web

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/madeinlowcode/chatwoot-megaapi-bridge/internal/bridge"
)

func TestDashboardListsTenantsWithCounts(t *testing.T) {
	summaries := func(context.Context) ([]bridge.TenantSummary, error) {
		return []bridge.TenantSummary{
			{Slug: "acme", Count24h: 12},
			{Slug: "globex", Count24h: 0},
		}, nil
	}
	key := make([]byte, 32)
	h, err := New(Deps{Key: key, TenantSummaries: summaries})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(authCookie(t, h, "a@b"))
	rr := httptest.NewRecorder()
	h.Routes().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d", rr.Code)
	}
	body := rr.Body.String()
	if !strings.Contains(body, "acme") || !strings.Contains(body, "globex") {
		t.Fatalf("missing tenants in body")
	}
	if !strings.Contains(body, "12") {
		t.Fatalf("missing count")
	}
}

func TestDashboardShowsPairLink(t *testing.T) {
	summaries := func(context.Context) ([]bridge.TenantSummary, error) {
		return []bridge.TenantSummary{{Slug: "acme", Count24h: 1}}, nil
	}
	getSetting := func(_ context.Context, k string) (string, error) {
		if k == settingBaseURL {
			return "https://bridge.example", nil
		}
		return "", nil
	}
	key := make([]byte, 32)
	h, _ := New(Deps{Key: key, TenantSummaries: summaries, GetSetting: getSetting})
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(authCookie(t, h, "a@b"))
	rr := httptest.NewRecorder()
	h.Routes().ServeHTTP(rr, req)
	body := rr.Body.String()
	if !strings.Contains(body, "https://bridge.example/pair/acme?t=") {
		t.Fatalf("pair link missing: %s", body)
	}
	if !strings.Contains(body, "Gerar link de pareamento") {
		t.Fatalf("link label missing")
	}
}

func TestDashboardEmptyShowsCTA(t *testing.T) {
	summaries := func(context.Context) ([]bridge.TenantSummary, error) {
		return nil, nil
	}
	key := make([]byte, 32)
	h, _ := New(Deps{Key: key, TenantSummaries: summaries})
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(authCookie(t, h, "a@b"))
	rr := httptest.NewRecorder()
	h.Routes().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "Nenhum tenant") {
		t.Fatalf("missing empty CTA")
	}
}
