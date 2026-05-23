package web

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/madeinlowcode/chatwoot-megaapi-bridge/internal/bridge"
)

func newMsgHandler(t *testing.T, list func(context.Context, string, int, int) ([]bridge.Message, error)) *Handler {
	t.Helper()
	key := make([]byte, 32)
	h, err := New(Deps{Key: key, ListMessages: list})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return h
}

func TestMessagesEmptyTenant(t *testing.T) {
	h := newMsgHandler(t, nil)
	req := httptest.NewRequest(http.MethodGet, "/messages", nil)
	req.AddCookie(authCookie(t, h, "a@b"))
	rr := httptest.NewRecorder()
	h.Routes().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "Informe um tenant") {
		t.Fatalf("expected hint, body=%q", rr.Body.String())
	}
}

func TestMessagesListsItems(t *testing.T) {
	var gotSlug string
	var gotLimit, gotOffset int
	list := func(_ context.Context, slug string, limit, offset int) ([]bridge.Message, error) {
		gotSlug, gotLimit, gotOffset = slug, limit, offset
		return []bridge.Message{
			{ID: uuid.New(), Direction: "in", Status: "ok", ExternalID: "ext-1",
				CreatedAt: time.Date(2025, 5, 23, 12, 0, 0, 0, time.UTC), Attempts: 1},
		}, nil
	}
	h := newMsgHandler(t, list)
	req := httptest.NewRequest(http.MethodGet, "/messages?tenant=acme&page=2", nil)
	req.AddCookie(authCookie(t, h, "a@b"))
	rr := httptest.NewRecorder()
	h.Routes().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d", rr.Code)
	}
	if gotSlug != "acme" || gotLimit != messagesPageSize || gotOffset != messagesPageSize {
		t.Fatalf("got slug=%q limit=%d offset=%d", gotSlug, gotLimit, gotOffset)
	}
	if !strings.Contains(rr.Body.String(), "ext-1") {
		t.Fatalf("missing external id")
	}
	if !strings.Contains(rr.Body.String(), "anterior") {
		t.Fatalf("page>1 should show prev link")
	}
}

func TestMessagesRequiresAuth(t *testing.T) {
	h := newMsgHandler(t, nil)
	req := httptest.NewRequest(http.MethodGet, "/messages?tenant=x", nil)
	rr := httptest.NewRecorder()
	h.Routes().ServeHTTP(rr, req)
	if rr.Code != http.StatusFound {
		t.Fatalf("status=%d", rr.Code)
	}
}

func TestParsePage(t *testing.T) {
	if parsePage("") != 1 {
		t.Errorf("empty should be 1")
	}
	if parsePage("abc") != 1 {
		t.Errorf("invalid should be 1")
	}
	if parsePage("-3") != 1 {
		t.Errorf("negative should be 1")
	}
	if parsePage("7") != 7 {
		t.Errorf("seven")
	}
}
