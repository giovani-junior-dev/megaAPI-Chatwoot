package web

import (
	"context"
	"html/template"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/madeinlowcode/chatwoot-megaapi-bridge/internal/bridge"
)

const (
	cookieName = "bridge_session"
	sessionTTL = 8 * time.Hour
)

type Deps struct {
	GetAdmin func(context.Context, string) (bridge.Admin, error)
	Key      []byte
}

type Handler struct {
	deps Deps
	tpl  *template.Template
}

func New(d Deps) (*Handler, error) {
	tpl, err := template.ParseFS(templatesFS, "templates/*.html")
	if err != nil {
		return nil, err
	}
	return &Handler{deps: d, tpl: tpl}, nil
}

func NewFromDB(db *bridge.DB, key []byte) (*Handler, error) {
	return New(Deps{
		GetAdmin: db.GetAdmin,
		Key:      key,
	})
}

func (h *Handler) Routes() http.Handler {
	r := chi.NewRouter()
	r.Use(h.requireAuth)
	r.Get("/", h.handleIndex)
	r.Get("/login", h.handleLoginForm)
	r.Post("/login", h.handleLoginSubmit)
	r.Post("/logout", h.handleLogout)
	return r
}

func (h *Handler) handleIndex(w http.ResponseWriter, _ *http.Request) {
	h.render(w, "index.html", page{Title: "Painel"})
}

type page struct {
	Title string
	Error string
	Data  any
}

func (h *Handler) render(w http.ResponseWriter, name string, p page) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	t, err := h.tpl.Clone()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if _, err := t.ParseFS(templatesFS, "templates/"+name); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if err := t.ExecuteTemplate(w, "layout", p); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
