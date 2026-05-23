package web

import (
	"context"
	"html/template"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/madeinlowcode/chatwoot-megaapi-bridge/internal/bridge"
)

const (
	cookieName = "bridge_session"
	sessionTTL = 8 * time.Hour
)

type Deps struct {
	GetAdmin        func(context.Context, string) (bridge.Admin, error)
	GetSetting      func(context.Context, string) (string, error)
	SetSetting      func(context.Context, string, string) error
	FetchInboxes    func(context.Context, discoverReq) ([]Inbox, error)
	InsertTenant    func(context.Context, bridge.TenantInsert) (uuid.UUID, error)
	ConfigWebhook   func(context.Context, MegaAPIWebhookConfig) error
	TenantSummaries func(context.Context) ([]bridge.TenantSummary, error)
	Key             []byte
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
		GetAdmin:        db.GetAdmin,
		GetSetting:      db.GetSetting,
		SetSetting:      db.SetSetting,
		InsertTenant:    db.InsertTenant,
		ConfigWebhook:   ConfigureMegaAPIWebhook,
		TenantSummaries: db.TenantSummaries,
		Key:             key,
	})
}

func (h *Handler) Routes() http.Handler {
	r := chi.NewRouter()
	r.Use(h.requireAuth)
	r.Get("/", h.handleIndex)
	r.Get("/login", h.handleLoginForm)
	r.Post("/login", h.handleLoginSubmit)
	r.Post("/logout", h.handleLogout)
	r.Get("/settings", h.handleSettings)
	r.Post("/settings/base_url", h.handleSettingsBaseURL)
	r.Post("/tenants/discover-inboxes", h.handleDiscoverInboxes)
	r.Get("/tenants/new", h.handleTenantNew)
	r.Post("/tenants", h.handleTenantCreate)
	return r
}

func (h *Handler) handleIndex(w http.ResponseWriter, r *http.Request) {
	var summaries []bridge.TenantSummary
	if h.deps.TenantSummaries != nil {
		if s, err := h.deps.TenantSummaries(r.Context()); err == nil {
			summaries = s
		}
	}
	h.render(w, "index.html", page{Title: "Painel", Data: summaries})
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
