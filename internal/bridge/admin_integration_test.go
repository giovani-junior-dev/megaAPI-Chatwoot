//go:build integration

package bridge

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func seedFailedMessage(t *testing.T, db *DB, tenantID uuid.UUID, extID string) uuid.UUID {
	t.Helper()
	id, _, err := db.InsertMessage(context.Background(), Message{
		TenantID: tenantID, Direction: directionIn, ExternalID: extID,
		Payload: []byte(`{"k":1}`),
	})
	require.NoError(t, err)
	require.NoError(t, db.MarkStatus(context.Background(), id, "failed", "boom"))
	return id
}

func TestAdminFailed_ReturnsFailedRowsWithCount(t *testing.T) {
	db := setupDB(t)
	key := RandomBytes(32)
	tn := makeAuthedTenant(t, db, key, "demo-admin-list", "b", "h")
	s := newServerWithDB(db, key, 4)
	s.Cfg.AdminToken = "tok-admin"

	_ = seedFailedMessage(t, db, tn.ID, "fail-1")
	_ = seedFailedMessage(t, db, tn.ID, "fail-2")

	req := httptest.NewRequest(http.MethodGet, "/admin/failed?limit=10", nil)
	req.Header.Set("Authorization", "Bearer tok-admin")
	rec := httptest.NewRecorder()
	s.Routes().ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	var resp struct {
		Count int          `json:"count"`
		Items []failedRow `json:"items"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Equal(t, 2, resp.Count)
	require.Len(t, resp.Items, 2)
	require.Equal(t, "boom", resp.Items[0].LastError)
}

func TestAdminRetry_RequeuesAndResetsAttempts(t *testing.T) {
	db := setupDB(t)
	key := RandomBytes(32)
	tn := makeAuthedTenant(t, db, key, "demo-admin-retry", "b", "h")
	s := newServerWithDB(db, key, 4)
	s.Cfg.AdminToken = "tok-r"

	msgID := seedFailedMessage(t, db, tn.ID, "retry-1")
	_, _ = db.Pool.Exec(context.Background(),
		`UPDATE messages SET attempts = 4 WHERE id = $1`, msgID)

	req := httptest.NewRequest(http.MethodPost, "/admin/retry/"+msgID.String(), nil)
	req.Header.Set("Authorization", "Bearer tok-r")
	rec := httptest.NewRecorder()
	s.Routes().ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), "requeued")
	require.Equal(t, 1, len(s.Inbox), "requeued job must land in inbox channel")

	var status string
	var attempts int
	require.NoError(t, db.Pool.QueryRow(context.Background(),
		`SELECT status, attempts FROM messages WHERE id = $1`, msgID).
		Scan(&status, &attempts))
	require.Equal(t, "pending", status)
	require.Equal(t, 0, attempts, "retry must reset attempts counter")
}

func TestAdminRetry_NotFoundOrAlreadyDoneReturns404(t *testing.T) {
	db := setupDB(t)
	key := RandomBytes(32)
	s := newServerWithDB(db, key, 4)
	s.Cfg.AdminToken = "tok-nf"

	req := httptest.NewRequest(http.MethodPost, "/admin/retry/"+uuid.NewString(), nil)
	req.Header.Set("Authorization", "Bearer tok-nf")
	rec := httptest.NewRecorder()
	s.Routes().ServeHTTP(rec, req)
	require.Equal(t, http.StatusNotFound, rec.Code)
}

func TestAdminFailed_AcceptsTokenInQueryParam(t *testing.T) {
	db := setupDB(t)
	key := RandomBytes(32)
	s := newServerWithDB(db, key, 4)
	s.Cfg.AdminToken = "qtoken"

	req := httptest.NewRequest(http.MethodGet, "/admin/failed?token=qtoken", nil)
	rec := httptest.NewRecorder()
	s.Routes().ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)
}

func TestAdminFailed_NoTokenConfiguredRejectsAll(t *testing.T) {
	db := setupDB(t)
	key := RandomBytes(32)
	s := newServerWithDB(db, key, 4)
	s.Cfg.AdminToken = ""

	req := httptest.NewRequest(http.MethodGet, "/admin/failed", nil)
	req.Header.Set("Authorization", "Bearer anything")
	rec := httptest.NewRecorder()
	s.Routes().ServeHTTP(rec, req)
	require.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestAdminFailed_MissingAuthorizationReturns401(t *testing.T) {
	db := setupDB(t)
	key := RandomBytes(32)
	s := newServerWithDB(db, key, 4)
	s.Cfg.AdminToken = "x"

	req := httptest.NewRequest(http.MethodGet, "/admin/failed", nil)
	rec := httptest.NewRecorder()
	s.Routes().ServeHTTP(rec, req)
	require.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestAdminRetry_InvalidUUIDReturns400(t *testing.T) {
	db := setupDB(t)
	key := RandomBytes(32)
	s := newServerWithDB(db, key, 4)
	s.Cfg.AdminToken = "tok-badid"

	req := httptest.NewRequest(http.MethodPost, "/admin/retry/not-a-uuid", nil)
	req.Header.Set("Authorization", "Bearer tok-badid")
	rec := httptest.NewRecorder()
	s.Routes().ServeHTTP(rec, req)
	require.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestAdminFailed_LimitDefaultsTo50(t *testing.T) {
	db := setupDB(t)
	key := RandomBytes(32)
	s := newServerWithDB(db, key, 4)
	s.Cfg.AdminToken = "tok-lim"

	req := httptest.NewRequest(http.MethodGet, "/admin/failed?limit=0", nil)
	req.Header.Set("Authorization", "Bearer tok-lim")
	rec := httptest.NewRecorder()
	s.Routes().ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)
}

func TestAdminFailed_RejectsBearerMismatch(t *testing.T) {
	db := setupDB(t)
	key := RandomBytes(32)
	s := newServerWithDB(db, key, 4)
	s.Cfg.AdminToken = "right-tok"

	req := httptest.NewRequest(http.MethodGet, "/admin/failed", nil)
	req.Header.Set("Authorization", "Bearer wrong-tok")
	rec := httptest.NewRecorder()
	s.Routes().ServeHTTP(rec, req)
	require.Equal(t, http.StatusUnauthorized, rec.Code)
}
