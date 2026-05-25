package web

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestConfigureChatwootWebhook_PatchesInboxWithURL(t *testing.T) {
	var path, method, token string
	var body map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.Path
		method = r.Method
		token = r.Header.Get("api_access_token")
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &body)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()
	err := ConfigureChatwootWebhook(context.Background(), ChatwootWebhookConfig{
		BaseURL:    srv.URL,
		Token:      "cw-tok",
		AccountID:  3,
		InboxID:    7,
		WebhookURL: "https://bridge.example/v1/cw/demo",
	})
	require.NoError(t, err)
	require.Equal(t, http.MethodPatch, method)
	require.Equal(t, "/api/v1/accounts/3/inboxes/7", path)
	require.Equal(t, "cw-tok", token)
	channel, _ := body["channel"].(map[string]any)
	require.Equal(t, "https://bridge.example/v1/cw/demo", channel["webhook_url"])
}

func TestConfigureChatwootWebhook_ReturnsErrorOnNon2xx(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "boom", http.StatusUnauthorized)
	}))
	defer srv.Close()
	err := ConfigureChatwootWebhook(context.Background(), ChatwootWebhookConfig{
		BaseURL: srv.URL, Token: "x", AccountID: 1, InboxID: 1,
		WebhookURL: "https://b/v1/cw/x",
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "401")
}

func TestFetchChatwootInboxHMAC_ReturnsChannelSecret(t *testing.T) {
	var path, method, token string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.Path
		method = r.Method
		token = r.Header.Get("api_access_token")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"hmac_token":"widget-only","secret":"server-webhook-sig","name":"x"}`))
	}))
	defer srv.Close()
	got, err := FetchChatwootInboxHMAC(context.Background(), ChatwootWebhookConfig{
		BaseURL: srv.URL, Token: "cw-tok", AccountID: 3, InboxID: 7,
	})
	require.NoError(t, err)
	require.Equal(t, "server-webhook-sig", got, "must read channel.secret used for api_inbox_webhook HMAC, NOT hmac_token (widget only)")
	require.Equal(t, http.MethodGet, method)
	require.Equal(t, "/api/v1/accounts/3/inboxes/7", path)
	require.Equal(t, "cw-tok", token)
}

func TestFetchChatwootInboxHMAC_EmptyTokenReturnsEmptyNoError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"name":"x"}`))
	}))
	defer srv.Close()
	got, err := FetchChatwootInboxHMAC(context.Background(), ChatwootWebhookConfig{
		BaseURL: srv.URL, Token: "x", AccountID: 1, InboxID: 1,
	})
	require.NoError(t, err)
	require.Equal(t, "", got)
}

func TestFetchChatwootInboxHMAC_ReturnsErrorOnNon2xx(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "boom", http.StatusUnauthorized)
	}))
	defer srv.Close()
	_, err := FetchChatwootInboxHMAC(context.Background(), ChatwootWebhookConfig{
		BaseURL: srv.URL, Token: "x", AccountID: 1, InboxID: 1,
	})
	require.Error(t, err)
}

func TestConfigureChatwootWebhook_EnablesHMAC(t *testing.T) {
	var body map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &body)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()
	err := ConfigureChatwootWebhook(context.Background(), ChatwootWebhookConfig{
		BaseURL: srv.URL, Token: "x", AccountID: 1, InboxID: 1,
		WebhookURL: "https://b/v1/cw/x",
	})
	require.NoError(t, err)
	channel, _ := body["channel"].(map[string]any)
	require.Equal(t, true, channel["hmac_mandatory"])
}

func TestConfigureChatwootWebhook_TrimsBaseURLTrailingSlash(t *testing.T) {
	var path string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.Path
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()
	err := ConfigureChatwootWebhook(context.Background(), ChatwootWebhookConfig{
		BaseURL: srv.URL + "/", Token: "x", AccountID: 1, InboxID: 2,
		WebhookURL: "https://b/v1/cw/y",
	})
	require.NoError(t, err)
	require.False(t, strings.HasPrefix(path, "//"))
	require.Equal(t, "/api/v1/accounts/1/inboxes/2", path)
}
