package web

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
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
	h.render(w, "wizard.html", h.shellPage(r, "Novo tenant", "/tenants/new", v))
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
	if h.deps.GetSetting != nil {
		base, _ := h.deps.GetSetting(r.Context(), settingBaseURL)
		if strings.TrimSpace(base) == "" {
			http.Error(w, "Base URL pública não configurada. Acesse /settings antes de criar tenants.", http.StatusBadRequest)
			return
		}
	}
	// Valida a inbox do Chatwoot e obtem o secret ANTES de criar o tenant, para
	// nao deixar um tenant meio-configurado (o outbound falharia em silencio se
	// a inbox nao existir no momento da criacao).
	cwSecret := ""
	if h.deps.FetchCwHMAC != nil {
		s, ferr := h.deps.FetchCwHMAC(r.Context(), ChatwootWebhookConfig{
			BaseURL: spec.ChatwootURL, Token: spec.ChatwootToken,
			AccountID: spec.ChatwootAccountID, InboxID: spec.ChatwootInboxID,
		})
		if ferr != nil || s == "" {
			http.Error(w, "Não foi possível acessar a inbox do Chatwoot. Crie a inbox do tipo API primeiro e confira Account ID, Inbox ID e o token de acesso.", http.StatusBadRequest)
			return
		}
		cwSecret = s
	}
	bearer, _, ti, err := bridge.BuildTenantInsert(h.deps.Key, spec)
	if err != nil {
		http.Error(w, "crypto error", http.StatusInternalServerError)
		return
	}
	id, err := h.deps.InsertTenant(r.Context(), ti)
	if err != nil {
		if isUniqueViolation(err) {
			http.Error(w, fmt.Sprintf("Slug %q já existe — escolha outro identificador.", spec.Slug), http.StatusConflict)
			return
		}
		http.Error(w, "erro ao salvar tenant", http.StatusInternalServerError)
		return
	}
	if cwSecret != "" && h.deps.UpdateTenantHMAC != nil {
		if enc, encErr := bridge.Encrypt([]byte(cwSecret), h.deps.Key); encErr == nil {
			_ = h.deps.UpdateTenantHMAC(r.Context(), id, enc)
		}
	}
	h.fireMegaAPIConfig(r, spec, bearer)
	h.fireChatwootConfig(r, spec, cwSecret)
	http.Redirect(w, r, "/", http.StatusFound)
}

func (h *Handler) fireChatwootConfig(r *http.Request, spec bridge.TenantSpec, token string) {
	if h.deps.ConfigCwWebhook == nil || h.deps.GetSetting == nil {
		return
	}
	base, _ := h.deps.GetSetting(r.Context(), settingBaseURL)
	if base == "" {
		return
	}
	// The Chatwoot v4 API-channel outgoing webhook is not HMAC-signed, so the
	// bridge authenticates it by a token in the URL (matched against the tenant
	// secret). Omit it when unknown to preserve the no-secret behaviour.
	webhookURL := strings.TrimRight(base, "/") + "/v1/cw/" + spec.Slug
	if token != "" {
		webhookURL += "?token=" + url.QueryEscape(token)
	}
	_ = h.deps.ConfigCwWebhook(r.Context(), ChatwootWebhookConfig{
		BaseURL: spec.ChatwootURL, Token: spec.ChatwootToken,
		AccountID: spec.ChatwootAccountID, InboxID: spec.ChatwootInboxID,
		WebhookURL: webhookURL,
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

// slugPattern mirrors the tenants_slug_check DB constraint so the wizard can
// reject a bad slug with a friendly message instead of a generic 500.
var slugPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{2,63}$`)

func validateSpec(s bridge.TenantSpec) (bridge.TenantSpec, error) {
	if s.Slug == "" || s.MegaAPIHost == "" || s.MegaAPIInstance == "" || s.MegaAPIToken == "" ||
		s.ChatwootURL == "" || s.ChatwootToken == "" || s.ChatwootAccountID == 0 || s.ChatwootInboxID == 0 {
		return s, errAllFieldsRequired
	}
	if !slugPattern.MatchString(s.Slug) {
		return s, errInvalidSlug
	}
	return s, nil
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

var errAllFieldsRequired = wizardErr("todos os campos são obrigatórios")

var errInvalidSlug = wizardErr("slug inválido: use apenas minúsculas, números e hífen (ex: empresa-x), de 3 a 64 caracteres")

type wizardErr string

func (e wizardErr) Error() string { return string(e) }
