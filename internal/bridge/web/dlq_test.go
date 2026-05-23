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
