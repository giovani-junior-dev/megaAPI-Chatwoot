package bridge

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/require"
)

func TestPprofIndex_NoCookie_Returns401(t *testing.T) {
	s := newTestServer(t, nil)
	req := httptest.NewRequest(http.MethodGet, "/debug/pprof/", nil)
	rec := httptest.NewRecorder()
	s.Routes().ServeHTTP(rec, req)
	require.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestPprofIndex_BadCookie_Returns401(t *testing.T) {
	s := newTestServer(t, nil)
	req := httptest.NewRequest(http.MethodGet, "/debug/pprof/", nil)
	req.AddCookie(&http.Cookie{Name: SessionCookieName, Value: "garbage.value"})
	rec := httptest.NewRecorder()
	s.Routes().ServeHTTP(rec, req)
	require.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestPprofIndex_ValidCookie_Returns200(t *testing.T) {
	s := newTestServer(t, nil)
	tok, err := NewSession("admin@example.com", s.Key, time.Hour)
	require.NoError(t, err)
	req := httptest.NewRequest(http.MethodGet, "/debug/pprof/", nil)
	req.AddCookie(&http.Cookie{Name: SessionCookieName, Value: tok})
	rec := httptest.NewRecorder()
	s.Routes().ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)
}

func TestPprofCmdline_ValidCookie_Returns200(t *testing.T) {
	s := newTestServer(t, nil)
	tok, err := NewSession("admin@example.com", s.Key, time.Hour)
	require.NoError(t, err)
	req := httptest.NewRequest(http.MethodGet, "/debug/pprof/cmdline", nil)
	req.AddCookie(&http.Cookie{Name: SessionCookieName, Value: tok})
	rec := httptest.NewRecorder()
	s.Routes().ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)
}

func TestPprofCmdline_NoCookie_Returns401(t *testing.T) {
	s := newTestServer(t, nil)
	req := httptest.NewRequest(http.MethodGet, "/debug/pprof/cmdline", nil)
	rec := httptest.NewRecorder()
	s.Routes().ServeHTTP(rec, req)
	require.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestPprofIndex_WithChiWebHandler_StillReachable(t *testing.T) {
	s := newTestServer(t, nil)
	webRouter := chi.NewRouter()
	webRouter.Get("/", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	s.Web = webRouter
	tok, err := NewSession("admin@example.com", s.Key, time.Hour)
	require.NoError(t, err)
	req := httptest.NewRequest(http.MethodGet, "/debug/pprof/", nil)
	req.AddCookie(&http.Cookie{Name: SessionCookieName, Value: tok})
	rec := httptest.NewRecorder()
	s.Routes().ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code, "pprof must reach pprof.Index, not the Web chi router NotFound")
}

func TestPprofProfileNames_ValidCookie_Return200(t *testing.T) {
	s := newTestServer(t, nil)
	tok, err := NewSession("admin@example.com", s.Key, time.Hour)
	require.NoError(t, err)
	for _, name := range []string{"heap", "goroutine", "allocs"} {
		req := httptest.NewRequest(http.MethodGet, "/debug/pprof/"+name, nil)
		req.AddCookie(&http.Cookie{Name: SessionCookieName, Value: tok})
		rec := httptest.NewRecorder()
		s.Routes().ServeHTTP(rec, req)
		require.Equalf(t, http.StatusOK, rec.Code, "pprof %s should be reachable", name)
	}
}
