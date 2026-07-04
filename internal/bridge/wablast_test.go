package bridge

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func wablastTenant(t *testing.T, key []byte, accountID string) Tenant {
	t.Helper()
	enc, err := Encrypt([]byte("wak_testkey"), key)
	require.NoError(t, err)
	return Tenant{Provider: providerWablast, WablastAPIKeyEnc: enc, WablastAccountID: accountID}
}

func TestWablastParseInbound_Text(t *testing.T) {
	body := []byte(`{"type":"message.received","data":{"message_id":"wamid.1","from":"5511999990000","contact_name":"Ana","type":"text","text":"oi"}}`)
	msg, err := wablastProvider{}.ParseInbound(body)
	require.NoError(t, err)
	require.Equal(t, InboundMessage{ContactID: "5511999990000", Name: "Ana", Text: "oi", ExternalID: "wamid.1"}, msg)
}

func TestWablastParseInbound_StripsLeadingPlus(t *testing.T) {
	body := []byte(`{"type":"message.received","data":{"message_id":"m","from":"+5511999990000","type":"text","text":"x"}}`)
	msg, err := wablastProvider{}.ParseInbound(body)
	require.NoError(t, err)
	require.Equal(t, "5511999990000", msg.ContactID)
}

func TestWablastParseInbound_Media(t *testing.T) {
	body := []byte(`{"type":"message.received","data":{"message_id":"m","from":"5511","type":"image","media":{"id":"MID","mime_type":"image/jpeg","caption":"veja"}}}`)
	msg, err := wablastProvider{}.ParseInbound(body)
	require.NoError(t, err)
	require.NotNil(t, msg.Attachment)
	require.Equal(t, Attachment{MediaKey: "MID", MimeType: "image/jpeg", Caption: "veja", Kind: "image"}, *msg.Attachment)
}

func TestWablastParseInbound_IgnoresNonReceived(t *testing.T) {
	_, err := wablastProvider{}.ParseInbound([]byte(`{"type":"message.delivered","data":{}}`))
	require.Error(t, err)
}

func TestClassifyWablast_WindowClosedMapsSentinel(t *testing.T) {
	err := classifyWablast(409, []byte(`{"code":"WINDOW_CLOSED","error":"fechada"}`), "x")
	require.ErrorIs(t, err, errWindowClosed)
}

func TestClassifyWablast_RateLimitRetriable(t *testing.T) {
	require.True(t, isRetriable(classifyWablast(429, []byte(`{"code":"META_RATE_LIMIT"}`), "x")))
}

func TestClassifyWablast_ClientErrorNotRetriable(t *testing.T) {
	require.False(t, isRetriable(classifyWablast(400, []byte(`{"code":"INVALID_BODY"}`), "x")))
}

func TestWablastSendText_PostsMessage(t *testing.T) {
	var got map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/v1/messages", r.URL.Path)
		require.Equal(t, "Bearer wak_testkey", r.Header.Get("Authorization"))
		_ = json.NewDecoder(r.Body).Decode(&got)
		w.WriteHeader(201)
		_, _ = w.Write([]byte(`{"id":"msg_1","status":"sent"}`))
	}))
	defer srv.Close()
	t.Setenv("WABLAST_BASE_URL", srv.URL)
	key := RandomBytes(32)
	err := wablastProvider{s: &Server{Key: key}}.SendText(context.Background(), wablastTenant(t, key, "acc1"), "5511", "ola")
	require.NoError(t, err)
	require.Equal(t, "text", got["type"])
	require.Equal(t, "acc1", got["account_id"])
	require.Equal(t, map[string]any{"body": "ola"}, got["text"])
}

func TestWablastSendText_WindowClosedSurfacesSentinel(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(409)
		_, _ = w.Write([]byte(`{"code":"WINDOW_CLOSED","error":"fechada"}`))
	}))
	defer srv.Close()
	t.Setenv("WABLAST_BASE_URL", srv.URL)
	key := RandomBytes(32)
	err := wablastProvider{s: &Server{Key: key}}.SendText(context.Background(), wablastTenant(t, key, ""), "5511", "ola")
	require.ErrorIs(t, err, errWindowClosed)
}

func TestWablastSendMedia_UploadsThenSends(t *testing.T) {
	media := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write([]byte("PNGBYTES"))
	}))
	defer media.Close()

	var uploaded bool
	var sent map[string]any
	gw := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/media":
			uploaded = true
			require.Equal(t, "acc1", r.URL.Query().Get("account_id"))
			require.Contains(t, r.Header.Get("Content-Type"), "multipart/form-data")
			w.WriteHeader(201)
			_, _ = w.Write([]byte(`{"media_id":"MID99"}`))
		case "/v1/messages":
			_ = json.NewDecoder(r.Body).Decode(&sent)
			w.WriteHeader(201)
			_, _ = w.Write([]byte(`{"id":"m","status":"sent"}`))
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer gw.Close()
	t.Setenv("WABLAST_BASE_URL", gw.URL)

	key := RandomBytes(32)
	att := Attachment{URL: media.URL, Kind: "image", Caption: "legenda"}
	err := wablastProvider{s: &Server{Key: key}}.SendMedia(context.Background(), wablastTenant(t, key, "acc1"), "5511", att)
	require.NoError(t, err)
	require.True(t, uploaded)
	require.Equal(t, "image", sent["type"])
	require.Equal(t, map[string]any{"id": "MID99", "caption": "legenda"}, sent["media"])
}

func TestWablastDownloadMedia_ReturnsBytes(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/v1/media/MID", r.URL.Path)
		w.Header().Set("Content-Type", "audio/ogg")
		_, _ = w.Write([]byte("OGG"))
	}))
	defer srv.Close()
	t.Setenv("WABLAST_BASE_URL", srv.URL)
	key := RandomBytes(32)
	data, mime, err := wablastProvider{s: &Server{Key: key}}.DownloadMedia(
		context.Background(), wablastTenant(t, key, ""), Attachment{MediaKey: "MID", Kind: "audio"})
	require.NoError(t, err)
	require.Equal(t, "OGG", string(data))
	require.Equal(t, "audio/ogg", mime)
}

func TestWablastDownloadMedia_ExpiredNotRetriable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(410)
		_, _ = w.Write([]byte(`{"code":"MEDIA_EXPIRED"}`))
	}))
	defer srv.Close()
	t.Setenv("WABLAST_BASE_URL", srv.URL)
	key := RandomBytes(32)
	_, _, err := wablastProvider{s: &Server{Key: key}}.DownloadMedia(
		context.Background(), wablastTenant(t, key, ""), Attachment{MediaKey: "X", Kind: "image"})
	require.Error(t, err)
	require.False(t, isRetriable(err))
}
