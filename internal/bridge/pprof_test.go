package bridge

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

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
