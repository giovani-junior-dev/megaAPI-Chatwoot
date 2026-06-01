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

func newDLQHandler(
	t *testing.T,
	list func(context.Context, int) ([]bridge.Message, error),
	req func(context.Context, uuid.UUID) (bridge.Message, error),
) *Handler {
	t.Helper()
	key := make([]byte, 32)
	h, err := New(Deps{Key: key, ListFailed: list, RequeueMessage: req})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return h
}

func TestDLQListsFailed(t *testing.T) {
	list := func(context.Context, int) ([]bridge.Message, error) {
		return []bridge.Message{{
			ID: uuid.New(), Direction: "in", ExternalID: "boom-1",
			Status: "failed", LastError: "kaboom",
			CreatedAt: time.Now(),
		}}, nil
	}
	h := newDLQHandler(t, list, nil)
	req := httptest.NewRequest(http.MethodGet, "/dlq", nil)
	req.AddCookie(authCookie(t, h, "a@b"))
	rr := httptest.NewRecorder()
	h.Routes().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d", rr.Code)
	}
	body := rr.Body.String()
	if !strings.Contains(body, "boom-1") || !strings.Contains(body, "kaboom") {
		t.Fatalf("body missing: %q", body)
	}
}

func TestDLQEmpty(t *testing.T) {
	h := newDLQHandler(t, func(context.Context, int) ([]bridge.Message, error) { return nil, nil }, nil)
	req := httptest.NewRequest(http.MethodGet, "/dlq", nil)
	req.AddCookie(authCookie(t, h, "a@b"))
	rr := httptest.NewRecorder()
	h.Routes().ServeHTTP(rr, req)
	if !strings.Contains(rr.Body.String(), "DLQ vazia") {
		t.Fatalf("expected empty message")
	}
}

func TestDLQRetryRequeues(t *testing.T) {
	var gotID uuid.UUID
	requeue := func(_ context.Context, id uuid.UUID) (bridge.Message, error) {
		gotID = id
		return bridge.Message{ID: id, Status: "pending"}, nil
	}
	h := newDLQHandler(t, nil, requeue)
	id := uuid.New()
	req := httptest.NewRequest(http.MethodPost, "/dlq/retry/"+id.String(), nil)
	req.AddCookie(authCookie(t, h, "a@b"))
	rr := httptest.NewRecorder()
	h.Routes().ServeHTTP(rr, req)
	if rr.Code != http.StatusFound {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
	if gotID != id {
		t.Fatalf("got=%s want=%s", gotID, id)
	}
}

func TestDLQRetryNotFound(t *testing.T) {
	requeue := func(context.Context, uuid.UUID) (bridge.Message, error) {
		return bridge.Message{}, bridge.ErrNotFound
	}
	h := newDLQHandler(t, nil, requeue)
	req := httptest.NewRequest(http.MethodPost, "/dlq/retry/"+uuid.New().String(), nil)
	req.AddCookie(authCookie(t, h, "a@b"))
	rr := httptest.NewRecorder()
	h.Routes().ServeHTTP(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("status=%d", rr.Code)
	}
}

func TestDLQRetryBadID(t *testing.T) {
	h := newDLQHandler(t, nil, nil)
	req := httptest.NewRequest(http.MethodPost, "/dlq/retry/not-a-uuid", nil)
	req.AddCookie(authCookie(t, h, "a@b"))
	rr := httptest.NewRecorder()
	h.Routes().ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status=%d", rr.Code)
	}
}

func TestDLQRequiresAuth(t *testing.T) {
	h := newDLQHandler(t, nil, nil)
	req := httptest.NewRequest(http.MethodGet, "/dlq", nil)
	rr := httptest.NewRecorder()
	h.Routes().ServeHTTP(rr, req)
	if rr.Code != http.StatusFound {
		t.Fatalf("status=%d", rr.Code)
	}
}

func TestDLQUsesDesignTokens(t *testing.T) {
	list := func(context.Context, int) ([]bridge.Message, error) {
		return []bridge.Message{{
			ID: uuid.New(), Direction: "out", ExternalID: "ext-dlq",
			Status: "failed", LastError: "kaboom",
			CreatedAt: time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC),
		}}, nil
	}
	h := newDLQHandler(t, list, nil)
	req := httptest.NewRequest(http.MethodGet, "/dlq", nil)
	req.AddCookie(authCookie(t, h, "a@b"))
	rr := httptest.NewRecorder()
	h.Routes().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d", rr.Code)
	}
	body := rr.Body.String()
	for _, banned := range []string{"bg-slate-", "text-slate-", "text-emerald-", "text-red-", "bg-green-", "shadow-", "shadow-sm"} {
		if strings.Contains(body, banned) {
			t.Fatalf("dlq must not use hardcoded tailwind class %q", banned)
		}
	}
	if !strings.Contains(body, `class="data-table"`) {
		t.Fatalf("dlq must use .data-table class; body=%s", body)
	}
	if !strings.Contains(body, "list-item") {
		t.Fatalf("dlq must have mobile list-item card; body=%s", body)
	}
	if !strings.Contains(body, "section-header") {
		t.Fatalf("dlq must use .section-header for h1; body=%s", body)
	}
}

func TestDLQRetryHasConfirm(t *testing.T) {
	list := func(context.Context, int) ([]bridge.Message, error) {
		return []bridge.Message{{
			ID: uuid.New(), Direction: "out", ExternalID: "ext-dlq",
			Status: "failed", LastError: "kaboom",
			CreatedAt: time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC),
		}}, nil
	}
	h := newDLQHandler(t, list, nil)
	req := httptest.NewRequest(http.MethodGet, "/dlq", nil)
	req.AddCookie(authCookie(t, h, "a@b"))
	rr := httptest.NewRecorder()
	h.Routes().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d", rr.Code)
	}
	body := rr.Body.String()
	hasAlpine := strings.Contains(body, "x-data") && strings.Contains(body, "x-on:click") || strings.Contains(body, "@click")
	if !hasAlpine {
		t.Fatalf("dlq retry must use Alpine for inline confirm; body=%s", body)
	}
	idx := strings.Index(body, "alpine")
	if idx == -1 {
		idx = strings.Index(body, "x-data")
	}
	if idx == -1 {
		idx = strings.Index(body, "@click")
	}
	if idx == -1 {
		idx = strings.Index(body, "Retry")
	}
	retryIdx := strings.Index(body, "Retry")
	if idx < 0 || retryIdx < 0 || idx > retryIdx {
		t.Fatalf("dlq must place confirm attribute before Retry submit; confirmIdx=%d retryIdx=%d body=%s", idx, retryIdx, body)
	}
	if !strings.Contains(body, "aria-busy") {
		t.Fatalf("dlq retry must signal loading via aria-busy; body=%s", body)
	}
}

func TestDLQSnapshot(t *testing.T) {
	pinTime := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	fixedID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	list := func(context.Context, int) ([]bridge.Message, error) {
		return []bridge.Message{{
			ID: fixedID, Direction: "out", ExternalID: "ext-dlq",
			Status: "failed", LastError: "kaboom",
			CreatedAt: pinTime,
		}}, nil
	}
	h := newDLQHandler(t, list, nil)
	req := httptest.NewRequest(http.MethodGet, "/dlq", nil)
	req.AddCookie(authCookie(t, h, "a@b"))
	rr := httptest.NewRecorder()
	h.Routes().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d", rr.Code)
	}
	goldenPath := filepath.Join("testdata", "dlq.golden.html")
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
		t.Fatalf("dlq snapshot drift.\nwant:\n%s\n\ngot:\n%s", want, rr.Body.String())
	}
}
