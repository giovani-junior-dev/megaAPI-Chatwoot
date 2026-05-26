package web

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/madeinlowcode/chatwoot-megaapi-bridge/internal/bridge"
)

func newPairHandler(t *testing.T, megaSrv *httptest.Server) (*Handler, []byte) {
	t.Helper()
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}
	tenant := bridge.Tenant{
		ID:              uuid.New(),
		Slug:            "acme",
		MegaAPIHost:     megaSrv.URL,
		MegaAPIInstance: "inst1",
	}
	tokEnc, err := bridge.Encrypt([]byte("mtok"), key)
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	tenant.MegaAPITokenEnc = tokEnc
	h, err := New(Deps{
		Key: key,
		GetTenant: func(_ context.Context, slug string) (bridge.Tenant, error) {
			if slug != tenant.Slug {
				return bridge.Tenant{}, bridge.ErrNotFound
			}
			return tenant, nil
		},
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return h, key
}

func validPairURL(slug string, key []byte, path string) string {
	exp := time.Now().Add(time.Hour).Unix()
	tok := SignPairToken(PairClaim{Slug: slug, Exp: exp}, key)
	return path + "?t=" + tok + "&exp=" + strconv.FormatInt(exp, 10)
}

func TestSignPairToken_RoundTrip(t *testing.T) {
	key := []byte("0123456789abcdef0123456789abcdef")
	c := PairClaim{Slug: "acme", Exp: time.Now().Add(time.Hour).Unix()}
	c.Token = SignPairToken(c, key)
	if !VerifyPairToken(c, key) {
		t.Fatal("verify failed")
	}
}

func TestVerifyPairToken_Tampered(t *testing.T) {
	key := []byte("0123456789abcdef0123456789abcdef")
	exp := time.Now().Add(time.Hour).Unix()
	tok := SignPairToken(PairClaim{Slug: "acme", Exp: exp}, key)
	if VerifyPairToken(PairClaim{Slug: "other", Exp: exp, Token: tok}, key) {
		t.Fatal("should reject tampered slug")
	}
	if VerifyPairToken(PairClaim{Slug: "acme", Exp: exp + 1, Token: tok}, key) {
		t.Fatal("should reject tampered exp")
	}
}

func TestVerifyPairToken_Expired(t *testing.T) {
	key := []byte("0123456789abcdef0123456789abcdef")
	exp := time.Now().Add(-time.Hour).Unix()
	tok := SignPairToken(PairClaim{Slug: "acme", Exp: exp}, key)
	if VerifyPairToken(PairClaim{Slug: "acme", Exp: exp, Token: tok}, key) {
		t.Fatal("should reject expired")
	}
}

func TestPairLanding_MissingToken(t *testing.T) {
	megaSrv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	defer megaSrv.Close()
	h, _ := newPairHandler(t, megaSrv)
	req := httptest.NewRequest(http.MethodGet, "/pair/acme", nil)
	rr := httptest.NewRecorder()
	h.Routes().ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
}

func TestPairLanding_ExpiredToken(t *testing.T) {
	megaSrv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	defer megaSrv.Close()
	h, key := newPairHandler(t, megaSrv)
	exp := time.Now().Add(-time.Hour).Unix()
	tok := SignPairToken(PairClaim{Slug: "acme", Exp: exp}, key)
	req := httptest.NewRequest(http.MethodGet,
		"/pair/acme?t="+tok+"&exp="+strconv.FormatInt(exp, 10), nil)
	rr := httptest.NewRecorder()
	h.Routes().ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d", rr.Code)
	}
}

func TestPairLanding_Renders(t *testing.T) {
	megaSrv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	defer megaSrv.Close()
	h, key := newPairHandler(t, megaSrv)
	req := httptest.NewRequest(http.MethodGet, validPairURL("acme", key, "/pair/acme"), nil)
	rr := httptest.NewRecorder()
	h.Routes().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "acme") {
		t.Fatalf("body missing slug")
	}
}

func TestPairQR_ProxiesMegaAPI(t *testing.T) {
	megaSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/rest/instance/qrcode_base64/inst1" {
			t.Fatalf("path=%s", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"qrcode":"data:image/png;base64,XYZ"}`))
	}))
	defer megaSrv.Close()
	h, key := newPairHandler(t, megaSrv)
	req := httptest.NewRequest(http.MethodGet, validPairURL("acme", key, "/pair/acme/qr"), nil)
	rr := httptest.NewRecorder()
	h.Routes().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
	var out map[string]string
	if err := json.Unmarshal(rr.Body.Bytes(), &out); err != nil {
		t.Fatalf("json: %v", err)
	}
	if out["qrcode"] != "data:image/png;base64,XYZ" {
		t.Fatalf("qrcode=%q", out["qrcode"])
	}
}

func TestPairCode_ProxiesMegaAPI(t *testing.T) {
	megaSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("phoneNumber") != "5511999" {
			t.Fatalf("phone=%s", r.URL.Query().Get("phoneNumber"))
		}
		_, _ = w.Write([]byte(`{"pairingCode":"ZZZ-9"}`))
	}))
	defer megaSrv.Close()
	h, key := newPairHandler(t, megaSrv)
	body := strings.NewReader("phone=5511999")
	req := httptest.NewRequest(http.MethodPost, validPairURL("acme", key, "/pair/acme/code"), body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()
	h.Routes().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "ZZZ-9") {
		t.Fatalf("body=%s", rr.Body.String())
	}
}

func TestPairStatus_ProxiesMegaAPI(t *testing.T) {
	megaSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"instance":{"status":"connected","user":{"id":"5511@s.whatsapp.net","name":"X"}}}`))
	}))
	defer megaSrv.Close()
	h, key := newPairHandler(t, megaSrv)
	req := httptest.NewRequest(http.MethodGet, validPairURL("acme", key, "/pair/acme/status"), nil)
	rr := httptest.NewRecorder()
	h.Routes().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "connected") {
		t.Fatalf("body=%s", rr.Body.String())
	}
}

func TestPairLogout_ProxiesMegaAPI(t *testing.T) {
	called := false
	megaSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		if r.Method != http.MethodDelete {
			t.Fatalf("method=%s", r.Method)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer megaSrv.Close()
	h, key := newPairHandler(t, megaSrv)
	req := httptest.NewRequest(http.MethodPost, validPairURL("acme", key, "/pair/acme/logout"), nil)
	rr := httptest.NewRecorder()
	h.Routes().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
	if !called {
		t.Fatal("mega not called")
	}
}

func TestPairCode_MissingPhone(t *testing.T) {
	megaSrv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("megaapi should not be called")
	}))
	defer megaSrv.Close()
	h, key := newPairHandler(t, megaSrv)
	req := httptest.NewRequest(http.MethodPost, validPairURL("acme", key, "/pair/acme/code"), nil)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()
	h.Routes().ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status=%d", rr.Code)
	}
}

func TestPairCode_JSONBody(t *testing.T) {
	megaSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("phoneNumber") != "551188888" {
			t.Fatalf("phone=%s", r.URL.Query().Get("phoneNumber"))
		}
		_, _ = w.Write([]byte(`{"pairingCode":"JS-1"}`))
	}))
	defer megaSrv.Close()
	h, key := newPairHandler(t, megaSrv)
	body := strings.NewReader(`{"phone":"551188888"}`)
	req := httptest.NewRequest(http.MethodPost, validPairURL("acme", key, "/pair/acme/code"), body)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	h.Routes().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "JS-1") {
		t.Fatalf("body=%s", rr.Body.String())
	}
}

func TestPairQR_MegaAPIError(t *testing.T) {
	megaSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer megaSrv.Close()
	h, key := newPairHandler(t, megaSrv)
	req := httptest.NewRequest(http.MethodGet, validPairURL("acme", key, "/pair/acme/qr"), nil)
	rr := httptest.NewRecorder()
	h.Routes().ServeHTTP(rr, req)
	if rr.Code != http.StatusBadGateway {
		t.Fatalf("status=%d", rr.Code)
	}
}

func TestPairStatus_MegaAPIError(t *testing.T) {
	megaSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer megaSrv.Close()
	h, key := newPairHandler(t, megaSrv)
	req := httptest.NewRequest(http.MethodGet, validPairURL("acme", key, "/pair/acme/status"), nil)
	rr := httptest.NewRecorder()
	h.Routes().ServeHTTP(rr, req)
	if rr.Code != http.StatusBadGateway {
		t.Fatalf("status=%d", rr.Code)
	}
}

func TestPairLogout_MegaAPIError(t *testing.T) {
	megaSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer megaSrv.Close()
	h, key := newPairHandler(t, megaSrv)
	req := httptest.NewRequest(http.MethodPost, validPairURL("acme", key, "/pair/acme/logout"), nil)
	rr := httptest.NewRecorder()
	h.Routes().ServeHTTP(rr, req)
	if rr.Code != http.StatusBadGateway {
		t.Fatalf("status=%d", rr.Code)
	}
}

func TestPair_TenantNotFound(t *testing.T) {
	megaSrv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	defer megaSrv.Close()
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}
	h, err := New(Deps{
		Key: key,
		GetTenant: func(_ context.Context, _ string) (bridge.Tenant, error) {
			return bridge.Tenant{}, bridge.ErrNotFound
		},
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	req := httptest.NewRequest(http.MethodGet, validPairURL("nope", key, "/pair/nope/qr"), nil)
	rr := httptest.NewRecorder()
	h.Routes().ServeHTTP(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("status=%d", rr.Code)
	}
}

func TestBuildPairLink(t *testing.T) {
	key := []byte("0123456789abcdef0123456789abcdef")
	link := BuildPairLink(PairLinkParams{BaseURL: "https://bridge.example", Slug: "acme", TTLSeconds: 3600}, key)
	if !strings.Contains(link, "/pair/acme?t=") {
		t.Fatalf("link=%s", link)
	}
	if !strings.Contains(link, "&exp=") {
		t.Fatalf("link missing exp: %s", link)
	}
}
