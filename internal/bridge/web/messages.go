package web

import (
	"net/http"
	"strconv"

	"github.com/madeinlowcode/chatwoot-megaapi-bridge/internal/bridge"
)

const messagesPageSize = 25

type messagesView struct {
	Tenant     string
	Page       int
	NextPage   int
	PrevPage   int
	HasPrev    bool
	HasNext    bool
	Items      []bridge.Message
	KnownSlugs []string
}

func (h *Handler) handleMessages(w http.ResponseWriter, r *http.Request) {
	tenant := r.URL.Query().Get("tenant")
	pn := parsePage(r.URL.Query().Get("page"))
	v := messagesView{
		Tenant:   tenant,
		Page:     pn,
		PrevPage: pn - 1,
		NextPage: pn + 1,
		HasPrev:  pn > 1,
	}
	if h.deps.TenantSummaries != nil {
		if sums, err := h.deps.TenantSummaries(r.Context()); err == nil {
			slugs := make([]string, 0, len(sums))
			for _, s := range sums {
				slugs = append(slugs, s.Slug)
			}
			v.KnownSlugs = slugs
		}
	}
	if tenant == "" || h.deps.ListMessages == nil {
		h.renderMessages(w, r, v)
		return
	}
	items, err := h.deps.ListMessages(r.Context(), tenant, messagesPageSize, (pn-1)*messagesPageSize)
	if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	v.Items = items
	v.HasNext = len(items) == messagesPageSize
	h.renderMessages(w, r, v)
}

func (h *Handler) renderMessages(w http.ResponseWriter, r *http.Request, v messagesView) {
	h.render(w, "messages.html", h.shellPage(r, "Mensagens", "/messages", v))
}

func parsePage(raw string) int {
	if raw == "" {
		return 1
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < 1 {
		return 1
	}
	return n
}
