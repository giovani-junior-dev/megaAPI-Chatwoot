package web

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/madeinlowcode/chatwoot-megaapi-bridge/internal/bridge"
)

func diagDeps(t *testing.T, get func(context.Context, string) (bridge.Tenant, error)) *Handler {
	t.Helper()
	key := make([]byte, 32)
	h, err := New(Deps{
		Key:       key,
		GetTenant: get,
		PingDB:    func(context.Context) error { return nil },
		HeadOK:    func(_ context.Context, u string) bool { return u != "" && u != "http://bad" },
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return h
}

func TestDiagJSONAllGreen(t *testing.T) {
	get := func(_ context.Context, _ string) (bridge.Tenant, error) {
		return bridge.Tenant{
			Slug: "acme", MegaAPIHost: "http://m", ChatwootURL: "http://c",
			WebhookBearerEnc: []byte{1, 2, 3},
		}, nil
	}
	h := diagDeps(t, get)
	req := httptest.NewRequest(http.MethodGet, "/tenants/acme/diag", nil)
	req.AddCookie(authCookie(t, h, "a@b"))
	rr := httptest.NewRecorder()
	h.Routes().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
	var res DiagResult
	if err := json.Unmarshal(rr.Body.Bytes(), &res); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !res.DB || !res.MegaAPI || !res.Chatwoot || !res.Webhook {
		t.Fatalf("res=%+v", res)
	}
}

func TestDiagJSONMegaAPIDown(t *testing.T) {
	get := func(_ context.Context, _ string) (bridge.Tenant, error) {
		return bridge.Tenant{Slug: "x", MegaAPIHost: "http://bad", ChatwootURL: "http://c"}, nil
	}
	h := diagDeps(t, get)
	req := httptest.NewRequest(http.MethodGet, "/tenants/x/diag", nil)
	req.AddCookie(authCookie(t, h, "a@b"))
	rr := httptest.NewRecorder()
	h.Routes().ServeHTTP(rr, req)
	var res DiagResult
	_ = json.Unmarshal(rr.Body.Bytes(), &res)
	if res.MegaAPI {
		t.Fatalf("expected megaapi=false: %+v", res)
	}
	if !res.Chatwoot {
		t.Fatalf("expected chatwoot=true: %+v", res)
	}
	if res.Webhook {
		t.Fatalf("expected webhook=false: %+v", res)
	}
}

func TestDiagTenantNotFound(t *testing.T) {
	get := func(context.Context, string) (bridge.Tenant, error) {
		return bridge.Tenant{}, bridge.ErrNotFound
	}
	h := diagDeps(t, get)
	req := httptest.NewRequest(http.MethodGet, "/tenants/zzz/diag", nil)
	req.AddCookie(authCookie(t, h, "a@b"))
	rr := httptest.NewRecorder()
	h.Routes().ServeHTTP(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("status=%d", rr.Code)
	}
}

func TestDiagRequiresAuth(t *testing.T) {
	h := diagDeps(t, nil)
	req := httptest.NewRequest(http.MethodGet, "/tenants/x/diag", nil)
	rr := httptest.NewRecorder()
	h.Routes().ServeHTTP(rr, req)
	if rr.Code != http.StatusFound {
		t.Fatalf("status=%d", rr.Code)
	}
}

func TestHeadOKReturnsTrueOnHTTP200(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()
	if !HeadOK(context.Background(), srv.URL) {
		t.Fatalf("expected true")
	}
}

func TestHeadOKFalseOnDialFailure(t *testing.T) {
	if HeadOK(context.Background(), "http://127.0.0.1:1") {
		t.Fatalf("expected false")
	}
}
