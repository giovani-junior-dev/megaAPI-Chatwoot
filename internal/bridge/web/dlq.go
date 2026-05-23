package web

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/madeinlowcode/chatwoot-megaapi-bridge/internal/bridge"
)

func (h *Handler) handleDLQ(w http.ResponseWriter, r *http.Request) {
	var items []bridge.Message
	if h.deps.ListFailed != nil {
		if got, err := h.deps.ListFailed(r.Context(), adminListMax); err == nil {
			items = got
		}
	}
	h.render(w, "dlq.html", page{Title: "DLQ", Data: items})
}

const adminListMax = 100

func (h *Handler) handleDLQRetry(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "bad id", http.StatusBadRequest)
		return
	}
	if h.deps.RequeueMessage == nil {
		http.Error(w, "dlq disabled", http.StatusServiceUnavailable)
		return
	}
	if _, err := h.deps.RequeueMessage(r.Context(), id); err != nil {
		if errors.Is(err, bridge.ErrNotFound) {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/dlq", http.StatusFound)
}
