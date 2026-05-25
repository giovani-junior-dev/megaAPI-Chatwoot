package web

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/madeinlowcode/chatwoot-megaapi-bridge/internal/bridge"
)

type wizardView struct {
	BaseURL string
}

func (h *Handler) handleTenantNew(w http.ResponseWriter, r *http.Request) {
	v := wizardView{}
	if h.deps.GetSetting != nil {
		v.BaseURL, _ = h.deps.GetSetting(r.Context(), settingBaseURL)
	}
	h.render(w, "wizard.html", page{Title: "Novo tenant", Data: v})
}

func (h *Handler) handleTenantCreate(w http.ResponseWriter, r *http.Request) {
	spec, err := readWizardForm(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if h.deps.InsertTenant == nil {
		http.Error(w, "wizard disabled", http.StatusServiceUnavailable)
		return
	}
	bearer, _, ti, err := bridge.BuildTenantInsert(h.deps.Key, spec)
	if err != nil {
		http.Error(w, "crypto error", http.StatusInternalServerError)
		return
	}
	if _, err := h.deps.InsertTenant(r.Context(), ti); err != nil {
		if isUniqueViolation(err) {
			http.Error(w, fmt.Sprintf("Slug %q já existe — escolha outro identificador.", spec.Slug), http.StatusConflict)
			return
		}
		http.Error(w, "erro ao salvar tenant", http.StatusInternalServerError)
		return
	}
	h.fireMegaAPIConfig(r, spec, bearer)
	h.fireChatwootConfig(r, spec)
	http.Redirect(w, r, "/", http.StatusFound)
}

func (h *Handler) fireChatwootConfig(r *http.Request, spec bridge.TenantSpec) {
	if h.deps.ConfigCwWebhook == nil || h.deps.GetSetting == nil {
		return
	}
	base, _ := h.deps.GetSetting(r.Context(), settingBaseURL)
	if base == "" {
		return
	}
	url := strings.TrimRight(base, "/") + "/v1/cw/" + spec.Slug
	_ = h.deps.ConfigCwWebhook(r.Context(), ChatwootWebhookConfig{
		BaseURL: spec.ChatwootURL, Token: spec.ChatwootToken,
		AccountID: spec.ChatwootAccountID, InboxID: spec.ChatwootInboxID,
		WebhookURL: url,
	})
}

func (h *Handler) fireMegaAPIConfig(r *http.Request, spec bridge.TenantSpec, bearer string) {
	if h.deps.ConfigWebhook == nil || h.deps.GetSetting == nil {
		return
	}
	base, _ := h.deps.GetSetting(r.Context(), settingBaseURL)
	if base == "" {
		return
	}
	url := strings.TrimRight(base, "/") + "/v1/wa/" + spec.Slug + "?token=" + bearer
	_ = h.deps.ConfigWebhook(r.Context(), MegaAPIWebhookConfig{
		Host: spec.MegaAPIHost, Instance: spec.MegaAPIInstance,
		Token: spec.MegaAPIToken, WebhookURL: url,
	})
}

func readWizardForm(r *http.Request) (bridge.TenantSpec, error) {
	if err := r.ParseForm(); err != nil {
		return bridge.TenantSpec{}, err
	}
	g := func(k string) string { return strings.TrimSpace(r.PostFormValue(k)) }
	acc, _ := strconv.ParseInt(g("chatwoot_account_id"), 10, 64)
	inb, _ := strconv.ParseInt(g("chatwoot_inbox_id"), 10, 64)
	return validateSpec(bridge.TenantSpec{
		Slug:              g("slug"),
		MegaAPIHost:       g("megaapi_host"),
		MegaAPIInstance:   g("megaapi_instance"),
		MegaAPIToken:      g("megaapi_token"),
		ChatwootURL:       g("chatwoot_url"),
		ChatwootToken:     g("chatwoot_token"),
		ChatwootAccountID: acc,
		ChatwootInboxID:   inb,
	})
}

func validateSpec(s bridge.TenantSpec) (bridge.TenantSpec, error) {
	if s.Slug == "" || s.MegaAPIHost == "" || s.MegaAPIInstance == "" || s.MegaAPIToken == "" ||
		s.ChatwootURL == "" || s.ChatwootToken == "" || s.ChatwootAccountID == 0 || s.ChatwootInboxID == 0 {
		return s, errAllFieldsRequired
	}
	return s, nil
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

var errAllFieldsRequired = wizardErr("todos os campos são obrigatórios")

type wizardErr string

func (e wizardErr) Error() string { return string(e) }
