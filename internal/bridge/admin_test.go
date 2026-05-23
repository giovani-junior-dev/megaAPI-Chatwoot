package bridge

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAdminAuthed_EmptyTokenRejectsEverything(t *testing.T) {
	s := &Server{Cfg: Config{AdminToken: ""}}
	req := httptest.NewRequest(http.MethodGet, "/admin/failed", nil)
	req.Header.Set("Authorization", "Bearer anything")
	require.False(t, s.adminAuthed(req))
}

func TestAdminAuthed_MatchAcceptsBearerAndQuery(t *testing.T) {
	s := &Server{Cfg: Config{AdminToken: "secret-xyz"}}
	header := httptest.NewRequest(http.MethodGet, "/admin/failed", nil)
	header.Header.Set("Authorization", "Bearer secret-xyz")
	require.True(t, s.adminAuthed(header))

	query := httptest.NewRequest(http.MethodGet, "/admin/failed?token=secret-xyz", nil)
	require.True(t, s.adminAuthed(query))

	wrong := httptest.NewRequest(http.MethodGet, "/admin/failed", nil)
	wrong.Header.Set("Authorization", "Bearer wrong")
	require.False(t, s.adminAuthed(wrong))
}

func TestAdminFailed_HandlerRejectsMissingBearer(t *testing.T) {
	s := newTestServer(t, nil)
	s.Cfg.AdminToken = "k"
	req := httptest.NewRequest(http.MethodGet, "/admin/failed", nil)
	rec := httptest.NewRecorder()
	s.Routes().ServeHTTP(rec, req)
	require.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestAdminRetry_HandlerRejectsBadID(t *testing.T) {
	s := newTestServer(t, nil)
	s.Cfg.AdminToken = "k"
	req := httptest.NewRequest(http.MethodPost, "/admin/retry/not-a-uuid", nil)
	req.Header.Set("Authorization", "Bearer k")
	rec := httptest.NewRecorder()
	s.Routes().ServeHTTP(rec, req)
	require.Equal(t, http.StatusBadRequest, rec.Code)
}
