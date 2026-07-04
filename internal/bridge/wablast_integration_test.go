//go:build integration

package bridge

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func nowTS() string { return strconv.FormatInt(time.Now().Unix(), 10) }

func decodeJSONBody(r *http.Request) map[string]any {
	var m map[string]any
	_ = json.NewDecoder(r.Body).Decode(&m)
	return m
}

func makeWablastTenant(t *testing.T, db *DB, key []byte, slug, cwURL, whsec string) Tenant {
	t.Helper()
	enc := func(s string) []byte {
		b, err := Encrypt([]byte(s), key)
		require.NoError(t, err)
		return b
	}
	id, err := db.InsertTenant(context.Background(), TenantInsert{
		Slug:                    slug,
		Provider:                providerWablast,
		ChatwootURL:             cwURL,
		ChatwootTokenEnc:        enc("cw-tok"),
		ChatwootAccountID:       1,
		ChatwootInboxID:         2,
		HMACSecretEnc:           enc("unused"),
		WebhookBearerEnc:        enc("unused"),
		WablastAPIKeyEnc:        enc("wak_testkey"),
		WablastAccountID:        "acc1",
		WablastWebhookSecretEnc: enc(whsec),
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = db.Pool.Exec(context.Background(), `DELETE FROM tenants WHERE id = $1`, id)
	})
	got, err := db.GetTenantBySlug(context.Background(), slug)
	require.NoError(t, err)
	return got
}

func TestHandleWABWebhook_ValidSignatureQueues(t *testing.T) {
	db := setupDB(t)
	key := RandomBytes(32)
	makeWablastTenant(t, db, key, "wab-ok", "https://c", swSecret)
	s := newServerWithDB(db, key, 4)

	body := []byte(`{"type":"message.received","data":{"message_id":"wamid.OK","from":"5511","type":"text","text":"oi"}}`)
	ts := nowTS()
	req := httptest.NewRequest(http.MethodPost, "/v1/wab/wab-ok", bytes.NewReader(body))
	req.Header.Set("webhook-id", "evt_1")
	req.Header.Set("webhook-timestamp", ts)
	req.Header.Set("webhook-signature", signSW(swSecret, "evt_1", ts, body))
	rec := httptest.NewRecorder()
	s.Routes().ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), "queued")
	require.Equal(t, 1, len(s.Inbox))
}

func TestHandleWABWebhook_InvalidSignatureRejected(t *testing.T) {
	db := setupDB(t)
	key := RandomBytes(32)
	makeWablastTenant(t, db, key, "wab-bad", "https://c", swSecret)
	s := newServerWithDB(db, key, 4)

	body := []byte(`{"type":"message.received","data":{"message_id":"m","from":"5511","type":"text","text":"x"}}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/wab/wab-bad", bytes.NewReader(body))
	req.Header.Set("webhook-id", "evt_1")
	req.Header.Set("webhook-timestamp", nowTS())
	req.Header.Set("webhook-signature", "v1,deadbeef")
	rec := httptest.NewRecorder()
	s.Routes().ServeHTTP(rec, req)
	require.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestHandleWABWebhook_NonReceivedIgnored(t *testing.T) {
	db := setupDB(t)
	key := RandomBytes(32)
	makeWablastTenant(t, db, key, "wab-ign", "https://c", swSecret)
	s := newServerWithDB(db, key, 4)

	body := []byte(`{"type":"message.delivered","data":{"message_id":"m"}}`)
	ts := nowTS()
	req := httptest.NewRequest(http.MethodPost, "/v1/wab/wab-ign", bytes.NewReader(body))
	req.Header.Set("webhook-id", "evt_1")
	req.Header.Set("webhook-timestamp", ts)
	req.Header.Set("webhook-signature", signSW(swSecret, "evt_1", ts, body))
	rec := httptest.NewRecorder()
	s.Routes().ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), "ignored")
	require.Equal(t, 0, len(s.Inbox))
}

func TestHandleWABWebhook_MegaapiTenantReturns404(t *testing.T) {
	db := setupDB(t)
	key := RandomBytes(32)
	makeAuthedTenant(t, db, key, "wab-wrongprov", "b", "h") // provider=megaapi
	s := newServerWithDB(db, key, 4)

	body := []byte(`{"type":"message.received","data":{"message_id":"m"}}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/wab/wab-wrongprov", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	s.Routes().ServeHTTP(rec, req)
	require.Equal(t, http.StatusNotFound, rec.Code)
}

func TestProcessInbound_Wablast_PostsTextToCW(t *testing.T) {
	db := setupDB(t)
	var msgContent string
	cwMock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/messages"):
			msgContent, _ = decodeJSONBody(r)["content"].(string)
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{}`))
		case strings.Contains(r.URL.Path, "/contacts"):
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"payload":{"contact":{"id":301}}}`))
		case strings.Contains(r.URL.Path, "/conversations"):
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"id":777}`))
		default:
			w.WriteHeader(http.StatusOK)
		}
	}))
	defer cwMock.Close()

	key := RandomBytes(32)
	tn := makeWablastTenant(t, db, key, "wab-in", cwMock.URL, swSecret)
	s := &Server{Key: key, DB: db}
	body := []byte(`{"type":"message.received","data":{"message_id":"wamid.IN","from":"5511999990000","contact_name":"Ana","type":"text","text":"ola bridge"}}`)
	require.NoError(t, s.processInbound(context.Background(), Job{TenantID: tn.ID, Payload: body}))
	require.Equal(t, "ola bridge", msgContent)

	c, err := db.GetContact(context.Background(), tn.ID, "5511999990000")
	require.NoError(t, err)
	require.Equal(t, int64(301), c.CWContactID)
}

func TestProcessOutbound_Wablast_WindowClosedPostsPrivateNote(t *testing.T) {
	db := setupDB(t)
	gw := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(409)
		_, _ = w.Write([]byte(`{"code":"WINDOW_CLOSED","error":"fechada"}`))
	}))
	defer gw.Close()
	t.Setenv("WABLAST_BASE_URL", gw.URL)

	var notePrivate bool
	var noteContent string
	cwMock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		m := decodeJSONBody(r)
		notePrivate, _ = m["private"].(bool)
		noteContent, _ = m["content"].(string)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer cwMock.Close()

	key := RandomBytes(32)
	tn := makeWablastTenant(t, db, key, "wab-win", cwMock.URL, swSecret)
	s := &Server{Key: key, DB: db}
	body := []byte(`{"event":"message_created","message_type":"outgoing","private":false,"id":9,"content":"oi","conversation":{"id":42,"contact_inbox":{"source_id":"5511"}}}`)
	err := s.processOutbound(context.Background(), Job{TenantID: tn.ID, Payload: body})
	require.Error(t, err)
	require.False(t, isRetriable(err))
	require.ErrorIs(t, err, errWindowClosed)
	require.True(t, notePrivate, "private note must be flagged private")
	require.Contains(t, noteContent, "Janela de 24h")
}
