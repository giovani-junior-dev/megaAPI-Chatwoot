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
	ConfigCwWebhook  func(context.Context, ChatwootWebhookConfig) error
	FetchCwHMAC      func(context.Context, ChatwootWebhookConfig) (string, error)
	UpdateTenantHMAC func(context.Context, uuid.UUID, []byte) error
	TenantSummaries func(context.Context) ([]bridge.TenantSummary, error)
	GetTenant       func(context.Context, string) (bridge.Tenant, error)
	PingDB          func(context.Context) error
	HeadOK          func(context.Context, string) bool
	ListMessages    func(context.Context, string, int, int) ([]bridge.Message, error)
	ListFailed      func(context.Context, int) ([]bridge.Message, error)
	RequeueMessage  func(context.Context, uuid.UUID) (bridge.Message, error)
	Key             []byte
}

type Handler struct {
	deps Deps
	tpl  *template.Template
}

func New(d Deps) (*Handler, error) {
	strings, err := loadLocale(defaultLocale)
	if err != nil {
		return nil, err
	}
	tpl, err := template.New("").Funcs(i18nFuncMap(strings)).ParseFS(templatesFS, "templates/*.html")
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
		ConfigCwWebhook:  ConfigureChatwootWebhook,
		FetchCwHMAC:      FetchChatwootInboxHMAC,
		UpdateTenantHMAC: db.UpdateTenantHMAC,
		TenantSummaries: db.TenantSummaries,
		GetTenant:       db.GetTenantBySlug,
		PingDB:          db.Pool.Ping,
		HeadOK:          HeadOK,
		ListMessages:    db.MessagesByTenantSlug,
		ListFailed:      db.FailedMessages,
		RequeueMessage:  db.RequeueMessage,
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
	r.Get("/tenants/{slug}/diag", h.handleDiag)
	r.Get("/messages", h.handleMessages)
	r.Get("/dlq", h.handleDLQ)
	r.Post("/dlq/retry/{id}", h.handleDLQRetry)
	r.Get("/pair/{slug}", h.handlePairLanding)
	r.Get("/pair/{slug}/qr", h.handlePairQR)
	r.Post("/pair/{slug}/code", h.handlePairCode)
	r.Post("/pair/{slug}/logout", h.handlePairLogout)
	r.Get("/pair/{slug}/status", h.handlePairStatus)
	return r
}

type indexRow struct {
	Slug     string
	Count24h int64
	PairLink string
	Paired   bool
	LastJID  string
}

func (h *Handler) handleIndex(w http.ResponseWriter, r *http.Request) {
	rows := h.buildIndexRows(r)
	h.render(w, "index.html", page{Title: "Painel", Data: rows})
}

func (h *Handler) buildIndexRows(r *http.Request) []indexRow {
	if h.deps.TenantSummaries == nil {
		return nil
	}
	summaries, err := h.deps.TenantSummaries(r.Context())
	if err != nil {
		return nil
	}
	var base string
	if h.deps.GetSetting != nil {
		base, _ = h.deps.GetSetting(r.Context(), settingBaseURL)
	}
	out := make([]indexRow, 0, len(summaries))
	for _, s := range summaries {
		row := indexRow{Slug: s.Slug, Count24h: s.Count24h, LastJID: s.LastJID, Paired: s.PairedAt != nil}
		if base != "" {
			row.PairLink = BuildPairLink(PairLinkParams{BaseURL: base, Slug: s.Slug}, h.deps.Key)
		}
		out = append(out, row)
	}
	return out
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
