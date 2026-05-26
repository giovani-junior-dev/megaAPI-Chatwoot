package bridge

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func newPairServer(t *testing.T, h http.HandlerFunc) (PairClient, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return PairClient{Host: srv.URL, Instance: "inst1", Token: "tok"}, srv
}

func TestFetchQR(t *testing.T) {
	c, _ := newPairServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet ||
			r.URL.Path != "/rest/instance/qrcode_base64/inst1" {
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer tok" {
			t.Fatalf("missing bearer")
		}
		_, _ = w.Write([]byte(`{"qrcode":"data:image/png;base64,AAA"}`))
	})
	qr, err := c.FetchQR(context.Background())
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if qr != "data:image/png;base64,AAA" {
		t.Fatalf("qr=%q", qr)
	}
}

func TestFetchQR_ErrorStatus(t *testing.T) {
	c, _ := newPairServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	})
	if _, err := c.FetchQR(context.Background()); err == nil {
		t.Fatal("expected error")
	}
}

func TestFetchPairingCode(t *testing.T) {
	c, _ := newPairServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/rest/instance/pairingCode/inst1" {
			t.Fatalf("path=%s", r.URL.Path)
		}
		if r.URL.Query().Get("phoneNumber") != "5511999999999" {
			t.Fatalf("phone=%s", r.URL.Query().Get("phoneNumber"))
		}
		_, _ = w.Write([]byte(`{"pairingCode":"ABCD-1234"}`))
	})
	code, err := c.FetchPairingCode(context.Background(), "5511999999999")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if code != "ABCD-1234" {
		t.Fatalf("code=%q", code)
	}
}

func TestFetchInstanceStatus(t *testing.T) {
	c, _ := newPairServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/rest/instance/inst1" {
			t.Fatalf("path=%s", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"instance":{"status":"connected","user":{"id":"5511@s.whatsapp.net","name":"Test"}}}`))
	})
	st, err := c.FetchInstanceStatus(context.Background())
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if st.Status != "connected" {
		t.Fatalf("status=%q", st.Status)
	}
	if !strings.HasPrefix(st.JID, "5511") {
		t.Fatalf("jid=%q", st.JID)
	}
	if st.PushName != "Test" {
		t.Fatalf("name=%q", st.PushName)
	}
}

func TestLogoutInstance(t *testing.T) {
	called := false
	c, _ := newPairServer(t, func(w http.ResponseWriter, r *http.Request) {
		called = true
		if r.Method != http.MethodDelete {
			t.Fatalf("method=%s", r.Method)
		}
		if r.URL.Path != "/rest/instance/inst1/logout" {
			t.Fatalf("path=%s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
	})
	if err := c.LogoutInstance(context.Background()); err != nil {
		t.Fatalf("err: %v", err)
	}
	if !called {
		t.Fatal("not called")
	}
}

func TestRestartInstance(t *testing.T) {
	c, _ := newPairServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete ||
			r.URL.Path != "/rest/instance/inst1/restart" {
			t.Fatalf("got %s %s", r.Method, r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
	})
	if err := c.RestartInstance(context.Background()); err != nil {
		t.Fatalf("err: %v", err)
	}
}

func TestLogoutInstance_ErrorStatus(t *testing.T) {
	c, _ := newPairServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	})
	if err := c.LogoutInstance(context.Background()); err == nil {
		t.Fatal("expected error")
	}
}
