package web

import (
	"html/template"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/madeinlowcode/chatwoot-megaapi-bridge/internal/bridge"
)

type Deps struct {
	DB *bridge.DB
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

func (h *Handler) Routes() http.Handler {
	r := chi.NewRouter()
	r.Get("/", h.handleIndex)
	return r
}

func (h *Handler) handleIndex(w http.ResponseWriter, _ *http.Request) {
	h.render(w, "index.html", page{Title: "Painel"})
}

type page struct {
	Title string
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
