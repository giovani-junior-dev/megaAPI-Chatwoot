package web

import (
	"net/http"
	"strings"
)

const settingBaseURL = "base_url"

type settingsView struct {
	BaseURL string
	Saved   bool
}

func (h *Handler) handleSettings(w http.ResponseWriter, r *http.Request) {
	v := settingsView{Saved: r.URL.Query().Get("saved") == "1"}
	if h.deps.GetSetting != nil {
		v.BaseURL, _ = h.deps.GetSetting(r.Context(), settingBaseURL)
	}
	h.render(w, "settings.html", page{Title: "Configurações", Data: v})
}

func (h *Handler) handleSettingsBaseURL(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}
	val := strings.TrimSpace(r.PostFormValue("base_url"))
	if val == "" {
		http.Error(w, "base_url required", http.StatusBadRequest)
		return
	}
	if h.deps.SetSetting == nil {
		http.Error(w, "settings disabled", http.StatusServiceUnavailable)
		return
	}
	if err := h.deps.SetSetting(r.Context(), settingBaseURL, val); err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/settings?saved=1", http.StatusFound)
}
