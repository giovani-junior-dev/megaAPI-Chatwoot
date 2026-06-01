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

func TestPainelTableResponsive(t *testing.T) {
	summaries := func(context.Context) ([]bridge.TenantSummary, error) {
		return []bridge.TenantSummary{{Slug: "acme", Count24h: 3}}, nil
	}
	key := make([]byte, 32)
	h, _ := New(Deps{Key: key, TenantSummaries: summaries})
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(authCookie(t, h, "a@b"))
	rr := httptest.NewRecorder()
	h.Routes().ServeHTTP(rr, req)
	body := rr.Body.String()
	if !strings.Contains(body, `class="data-table"`) {
		t.Fatalf("painel must use .data-table class; body=%s", body)
	}
	if !strings.Contains(body, `class="list-item`) {
		t.Fatalf("painel must have mobile list-item card class; body=%s", body)
	}
	for _, banned := range []string{"bg-slate-", "text-slate-", "text-emerald-", "text-red-", "bg-green-", "shadow-", "shadow-sm"} {
		if strings.Contains(body, banned) {
			t.Fatalf("painel must not use hardcoded tailwind class %q", banned)
		}
	}
}

func TestPainelPairedShowsBadge(t *testing.T) {
	summaries := func(context.Context) ([]bridge.TenantSummary, error) {
		return []bridge.TenantSummary{{Slug: "acme", Count24h: 0, LastJID: "5511999@s.whatsapp.net"}}, nil
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
	if !strings.Contains(body, `badge-success`) {
		t.Fatalf("paired tenant must render .badge-success; body=%s", body)
	}
	if !strings.Contains(body, "5511999@s.whatsapp.net") {
		t.Fatalf("paired tenant must show JID inside badge; body=%s", body)
	}
}

func TestEmptyStateRendersCTA(t *testing.T) {
	summaries := func(context.Context) ([]bridge.TenantSummary, error) {
		return []bridge.TenantSummary{}, nil
	}
	key := make([]byte, 32)
	h, _ := New(Deps{Key: key, TenantSummaries: summaries})
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(authCookie(t, h, "a@b"))
	rr := httptest.NewRecorder()
	h.Routes().ServeHTTP(rr, req)
	body := rr.Body.String()
	if !strings.Contains(body, "empty-state") {
		t.Fatalf("empty state must use .empty-state class; body=%s", body)
	}
	if !strings.Contains(body, `href="/tenants/new"`) {
		t.Fatalf("empty state must link to /tenants/new; body=%s", body)
	}
}
