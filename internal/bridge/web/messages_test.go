package web

import (
	"context"
	"errors"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
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

func TestMessagesUsesDesignTokens(t *testing.T) {
	list := func(context.Context, string, int, int) ([]bridge.Message, error) {
		return []bridge.Message{
			{ID: uuid.New(), Direction: "in", Status: "delivered", ExternalID: "ext-1",
				CreatedAt: time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC), Attempts: 1},
		}, nil
	}
	h := newMsgHandler(t, list)
	req := httptest.NewRequest(http.MethodGet, "/messages?tenant=acme", nil)
	req.AddCookie(authCookie(t, h, "a@b"))
	rr := httptest.NewRecorder()
	h.Routes().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d", rr.Code)
	}
	body := rr.Body.String()
	for _, banned := range []string{"bg-slate-", "text-slate-", "text-emerald-", "text-red-", "bg-green-", "shadow-", "shadow-sm"} {
		if strings.Contains(body, banned) {
			t.Fatalf("messages must not use hardcoded tailwind class %q", banned)
		}
	}
	if !strings.Contains(body, `class="data-table"`) {
		t.Fatalf("messages must use .data-table class; body=%s", body)
	}
	if !strings.Contains(body, "list-item") {
		t.Fatalf("messages must have mobile list-item card; body=%s", body)
	}
	if !strings.Contains(body, "badge ") {
		t.Fatalf("messages must render status via .badge partial; body=%s", body)
	}
	if !strings.Contains(body, `class="input"`) {
		t.Fatalf("tenant filter must use .input class; body=%s", body)
	}
	if !strings.Contains(body, "btn-primary") {
		t.Fatalf("filter submit must use .btn-primary; body=%s", body)
	}
	if !strings.Contains(body, "section-header") {
		t.Fatalf("messages must use .section-header for h1; body=%s", body)
	}
}

func TestMessagesStatusBadge(t *testing.T) {
	list := func(context.Context, string, int, int) ([]bridge.Message, error) {
		return []bridge.Message{
			{ID: uuid.New(), Direction: "in", Status: "failed", ExternalID: "ext-x",
				CreatedAt: time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC), Attempts: 3},
		}, nil
	}
	h := newMsgHandler(t, list)
	req := httptest.NewRequest(http.MethodGet, "/messages?tenant=acme", nil)
	req.AddCookie(authCookie(t, h, "a@b"))
	rr := httptest.NewRecorder()
	h.Routes().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d", rr.Code)
	}
	body := rr.Body.String()
	if !strings.Contains(body, "badge-danger") {
		t.Fatalf("failed status must render badge-danger; body=%s", body)
	}
	if !strings.Contains(body, "badge-neutral") {
		t.Fatalf("direction must render badge-neutral; body=%s", body)
	}
}

func TestMessagesSnapshot(t *testing.T) {
	pinTime := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	restore := SetNowFunc(func() time.Time { return pinTime })
	defer restore()
	fixedA := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	fixedB := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	list := func(context.Context, string, int, int) ([]bridge.Message, error) {
		return []bridge.Message{
			{ID: fixedA, Direction: "in", Status: "failed", ExternalID: "ext-1",
				CreatedAt: pinTime, Attempts: 2},
			{ID: fixedB, Direction: "out", Status: "delivered", ExternalID: "ext-2",
				CreatedAt: pinTime, Attempts: 1},
		}, nil
	}
	h := newMsgHandler(t, list)
	req := httptest.NewRequest(http.MethodGet, "/messages?tenant=acme", nil)
	req.AddCookie(authCookie(t, h, "a@b"))
	rr := httptest.NewRecorder()
	h.Routes().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d", rr.Code)
	}
	goldenPath := filepath.Join("testdata", "messages.golden.html")
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
		t.Fatalf("messages snapshot drift.\nwant:\n%s\n\ngot:\n%s", want, rr.Body.String())
	}
}
