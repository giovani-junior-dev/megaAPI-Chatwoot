package web

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestConfigureMegaAPIWebhookPostsExpectedBody(t *testing.T) {
	var (
		gotPath  string
		gotAuth  string
		gotCT    string
		gotBody  map[string]any
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		gotCT = r.Header.Get("Content-Type")
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &gotBody)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	err := ConfigureMegaAPIWebhook(context.Background(), MegaAPIWebhookConfig{
		Host: srv.URL, Instance: "inst1", Token: "tok",
		WebhookURL: "https://bridge.example/v1/wa/slug?token=abc",
	})
	if err != nil {
		t.Fatalf("configure: %v", err)
	}
	if gotPath != "/rest/webhook/inst1/configWebhook" {
		t.Errorf("path=%q", gotPath)
	}
	if gotAuth != "Bearer tok" {
		t.Errorf("auth=%q", gotAuth)
	}
	if !strings.Contains(gotCT, "application/json") {
		t.Errorf("content-type=%q", gotCT)
	}
	md, _ := gotBody["messageData"].(map[string]any)
	if md == nil {
		t.Fatalf("missing messageData: %+v", gotBody)
	}
	if md["webhookUrl"] != "https://bridge.example/v1/wa/slug?token=abc" {
		t.Errorf("webhookUrl=%v", md["webhookUrl"])
	}
	if md["webhookEnabled"] != true {
		t.Errorf("webhookEnabled=%v", md["webhookEnabled"])
	}
}

func TestConfigureMegaAPIWebhookErrorOn500(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("boom"))
	}))
	defer srv.Close()
	err := ConfigureMegaAPIWebhook(context.Background(), MegaAPIWebhookConfig{
		Host: srv.URL, Instance: "x", Token: "y", WebhookURL: "z",
	})
	if err == nil || !strings.Contains(err.Error(), "500") {
		t.Fatalf("err=%v", err)
	}
}

func TestConfigureMegaAPIWebhookErrorOnDialFailure(t *testing.T) {
	err := ConfigureMegaAPIWebhook(context.Background(), MegaAPIWebhookConfig{
		Host: "http://127.0.0.1:1", Instance: "x", Token: "y", WebhookURL: "z",
	})
	if err == nil {
		t.Fatalf("expected dial error")
	}
}
