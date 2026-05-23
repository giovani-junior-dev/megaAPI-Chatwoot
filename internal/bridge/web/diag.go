package web

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/madeinlowcode/chatwoot-megaapi-bridge/internal/bridge"
)

type DiagResult struct {
	MegaAPI  bool `json:"megaapi"`
	Chatwoot bool `json:"chatwoot"`
	Webhook  bool `json:"webhook"`
	DB       bool `json:"db"`
}

func (h *Handler) handleDiag(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")
	if h.deps.GetTenant == nil {
		http.Error(w, "diag disabled", http.StatusServiceUnavailable)
		return
	}
	t, err := h.deps.GetTenant(r.Context(), slug)
	if errors.Is(err, bridge.ErrNotFound) {
		http.Error(w, "tenant not found", http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	res := h.runDiagnostics(r.Context(), t)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(res)
}

func (h *Handler) runDiagnostics(ctx context.Context, t bridge.Tenant) DiagResult {
	res := DiagResult{Webhook: len(t.WebhookBearerEnc) > 0}
	if h.deps.PingDB != nil {
		res.DB = h.deps.PingDB(ctx) == nil
	}
	if h.deps.HeadOK != nil {
		res.MegaAPI = h.deps.HeadOK(ctx, t.MegaAPIHost)
		res.Chatwoot = h.deps.HeadOK(ctx, t.ChatwootURL)
	}
	return res
}

func HeadOK(ctx context.Context, raw string) bool {
	if raw == "" {
		return false
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, raw, nil)
	if err != nil {
		return false
	}
	cl := &http.Client{Timeout: 5 * time.Second}
	resp, err := cl.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode < 500
}
