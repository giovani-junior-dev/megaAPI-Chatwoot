package bridge

import (
	"net/http"
	"net/http/pprof"

	"github.com/go-chi/chi/v5"
)

// SessionCookieName matches the cookie set by the web admin handler; pprof
// routes piggy-back on the same session so we don't add a second auth surface.
const SessionCookieName = "bridge_session"

func (s *Server) requireAdminCookie(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := r.Cookie(SessionCookieName)
		if err != nil || c.Value == "" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		if _, err := ParseSession(c.Value, s.Key); err != nil {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) mountPprof(r chi.Router) {
	r.Group(func(g chi.Router) {
		g.Use(s.requireAdminCookie)
		g.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
		g.HandleFunc("/debug/pprof/profile", pprof.Profile)
		g.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
		g.HandleFunc("/debug/pprof/trace", pprof.Trace)
		g.HandleFunc("/debug/pprof/*", pprof.Index)
		g.HandleFunc("/debug/pprof", pprof.Index)
	})
}
