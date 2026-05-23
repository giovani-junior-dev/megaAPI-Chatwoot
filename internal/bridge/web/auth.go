package web

import (
	"net/http"

	"github.com/madeinlowcode/chatwoot-megaapi-bridge/internal/bridge"
)

func (h *Handler) requireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if isPublicPath(r.URL.Path) {
			next.ServeHTTP(w, r)
			return
		}
		if h.currentAdmin(r) == "" {
			http.Redirect(w, r, "/login", http.StatusFound)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func isPublicPath(p string) bool {
	return p == "/login" || p == "/logout"
}

func (h *Handler) currentAdmin(r *http.Request) string {
	c, err := r.Cookie(cookieName)
	if err != nil || c.Value == "" {
		return ""
	}
	email, err := bridge.ParseSession(c.Value, h.deps.Key)
	if err != nil {
		return ""
	}
	return email
}
