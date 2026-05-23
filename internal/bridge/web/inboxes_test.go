package web

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func newInboxHandler(t *testing.T, f func(context.Context, discoverReq) ([]Inbox, error)) *Handler {
	t.Helper()
	key := make([]byte, 32)
	h, err := New(Deps{FetchInboxes: f, Key: key})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return h
}

func TestDiscoverInboxesReturnsJSON(t *testing.T) {
	fetch := func(_ context.Context, req discoverReq) ([]Inbox, error) {
		if req.ChatwootURL != "https://cw.example" || req.Token != "tok" || req.AccountID != 7 {
			t.Fatalf("bad req: %+v", req)
		}
		return []Inbox{{ID: 1, Name: "Sales", ChannelType: "Channel::Api"}}, nil
	}
	h := newInboxHandler(t, fetch)
	body := `{"chatwoot_url":"https://cw.example","chatwoot_token":"tok","account_id":7}`
	req := httptest.NewRequest(http.MethodPost, "/tenants/discover-inboxes", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(authCookie(t, h, "a@b"))
	rr := httptest.NewRecorder()
	h.Routes().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
	if ct := rr.Header().Get("Content-Type"); !strings.Contains(ct, "application/json") {
		t.Fatalf("content-type=%q", ct)
	}
	var got []Inbox
	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(got) != 1 || got[0].Name != "Sales" {
		t.Fatalf("got=%+v", got)
	}
}

func TestDiscoverInboxesRejectsMissingFields(t *testing.T) {
	h := newInboxHandler(t, nil)
	req := httptest.NewRequest(http.MethodPost, "/tenants/discover-inboxes",
		strings.NewReader(`{"chatwoot_url":""}`))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(authCookie(t, h, "a@b"))
	rr := httptest.NewRecorder()
	h.Routes().ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status=%d", rr.Code)
	}
}

func TestDiscoverInboxesFetcherError(t *testing.T) {
	h := newInboxHandler(t, func(context.Context, discoverReq) ([]Inbox, error) {
		return nil, errors.New("upstream 500")
	})
	body := `{"chatwoot_url":"u","chatwoot_token":"t","account_id":1}`
	req := httptest.NewRequest(http.MethodPost, "/tenants/discover-inboxes", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(authCookie(t, h, "a@b"))
	rr := httptest.NewRecorder()
	h.Routes().ServeHTTP(rr, req)
	if rr.Code != http.StatusBadGateway {
		t.Fatalf("status=%d", rr.Code)
	}
}

func TestFetchInboxesHTTPCallsChatwoot(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/accounts/9/inboxes" {
			t.Errorf("path=%q", r.URL.Path)
		}
		if r.Header.Get("api_access_token") != "abc" {
			t.Errorf("missing api_access_token")
		}
		_, _ = w.Write([]byte(`{"payload":[{"id":42,"name":"WA","channel_type":"Channel::Api"}]}`))
	}))
	defer srv.Close()
	out, err := fetchInboxesHTTP(context.Background(), discoverReq{
		ChatwootURL: srv.URL, Token: "abc", AccountID: 9,
	})
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}
	if len(out) != 1 || out[0].ID != 42 {
		t.Fatalf("out=%+v", out)
	}
}

func TestDiscoverInboxesRequiresAuth(t *testing.T) {
	h := newInboxHandler(t, nil)
	req := httptest.NewRequest(http.MethodPost, "/tenants/discover-inboxes",
		strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	h.Routes().ServeHTTP(rr, req)
	if rr.Code != http.StatusFound {
		t.Fatalf("status=%d", rr.Code)
	}
}
