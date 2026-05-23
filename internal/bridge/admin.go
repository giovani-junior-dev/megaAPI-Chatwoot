package bridge

import (
	"crypto/subtle"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

const adminListLimit = 50

type failedRow struct {
	ID         uuid.UUID `json:"id"`
	TenantID   uuid.UUID `json:"tenant_id"`
	Direction  string    `json:"direction"`
	ExternalID string    `json:"external_id"`
	Attempts   int       `json:"attempts"`
	LastError  string    `json:"last_error"`
	CreatedAt  time.Time `json:"created_at"`
}

func (s *Server) adminAuthed(r *http.Request) bool {
	if s.Cfg.AdminToken == "" {
		return false
	}
	got := strings.TrimPrefix(r.Header.Get("Authorization"), bearerPrefix)
	if got == "" {
		got = r.URL.Query().Get("token")
	}
	if got == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(got), []byte(s.Cfg.AdminToken)) == 1
}

func (s *Server) handleAdminFailed(w http.ResponseWriter, r *http.Request) {
	if !s.adminAuthed(r) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 {
		limit = adminListLimit
	}
	msgs, err := s.DB.FailedMessages(r.Context(), limit)
	if err != nil {
		s.Log.Err(err).Msg("admin failed list")
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	out := make([]failedRow, 0, len(msgs))
	for _, m := range msgs {
		out = append(out, failedRow{
			ID: m.ID, TenantID: m.TenantID, Direction: m.Direction,
			ExternalID: m.ExternalID, Attempts: m.Attempts,
			LastError: m.LastError, CreatedAt: m.CreatedAt,
		})
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"count": len(out), "items": out})
}

func (s *Server) handleAdminRetry(w http.ResponseWriter, r *http.Request) {
	if !s.adminAuthed(r) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "bad id", http.StatusBadRequest)
		return
	}
	m, err := s.DB.RequeueMessage(r.Context(), id)
	if errors.Is(err, ErrNotFound) {
		http.Error(w, "not found or not failed", http.StatusNotFound)
		return
	}
	if err != nil {
		s.Log.Err(err).Msg("admin retry")
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	if !s.requeueJob(m) {
		http.Error(w, "queue full", http.StatusServiceUnavailable)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "requeued", "direction": m.Direction})
}

func (s *Server) requeueJob(m Message) bool {
	ch := s.Inbox
	if m.Direction == directionOut {
		ch = s.Outbox
	}
	job := Job{TenantID: m.TenantID, MessageID: m.ID, Direction: m.Direction, Payload: m.Payload}
	select {
	case ch <- job:
		return true
	default:
		return false
	}
}
