package web

import (
	"net/http"

	"github.com/madeinlowcode/chatwoot-megaapi-bridge/internal/bridge"
)

func (h *Handler) handleLoginForm(w http.ResponseWriter, _ *http.Request) {
	h.render(w, "login.html", page{Title: "Login"})
}

func (h *Handler) handleLoginSubmit(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}
	email := r.PostFormValue("email")
	pwd := r.PostFormValue("password")
	ok := h.validateLogin(r.Context(), email, pwd)
	if !ok {
		w.WriteHeader(http.StatusUnauthorized)
		h.render(w, "login.html", page{Title: "Login", Error: "Credenciais inválidas"})
		return
	}
	tok, err := bridge.NewSession(email, h.deps.Key, sessionTTL)
	if err != nil {
		http.Error(w, "session error", http.StatusInternalServerError)
		return
	}
	setSessionCookie(w, tok)
	http.Redirect(w, r, "/", http.StatusFound)
}

func (h *Handler) validateLogin(ctx httpCtx, email, pwd string) bool {
	if email == "" || pwd == "" || h.deps.GetAdmin == nil {
		return false
	}
	a, err := h.deps.GetAdmin(ctx, email)
	if err != nil {
		return false
	}
	ok, err := bridge.VerifyPassword(pwd, a.PasswordHash)
	if err != nil {
		return false
	}
	return ok
}

func (h *Handler) handleLogout(w http.ResponseWriter, r *http.Request) {
	clearSessionCookie(w)
	http.Redirect(w, r, "/login", http.StatusFound)
}

func setSessionCookie(w http.ResponseWriter, value string) {
	http.SetCookie(w, &http.Cookie{
		Name:     cookieName,
		Value:    value,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(sessionTTL.Seconds()),
	})
}

func clearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     cookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})
}
