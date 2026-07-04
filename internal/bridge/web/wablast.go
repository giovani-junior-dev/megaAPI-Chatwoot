package web

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

type WablastWebhookConfig struct {
	APIKey     string
	AccountID  string
	WebhookURL string
}

const wablastDefaultBase = "https://api.wablastmessage.com"

func wablastBase() string {
	if v := os.Getenv("WABLAST_BASE_URL"); v != "" {
		return strings.TrimRight(v, "/")
	}
	return wablastDefaultBase
}

// RegisterWablastWebhook ensures the bridge inbound URL is registered on WaBlast
// and returns the signing secret (whsec_). WaBlast create is not upsert, so an
// existing endpoint for the same URL is rotated (rotate also returns a secret)
// instead of creating a duplicate.
func RegisterWablastWebhook(ctx context.Context, cfg WablastWebhookConfig) (string, error) {
	base := wablastBase()
	id, found, err := findWablastEndpoint(ctx, base, cfg.APIKey, cfg.WebhookURL)
	if err != nil {
		return "", err
	}
	if found {
		return wablastGetSecret(ctx, base+"/v1/webhooks/"+id+"/rotate-secret", cfg.APIKey, nil)
	}
	body := map[string]any{
		"url":           cfg.WebhookURL,
		"enabledEvents": []string{"message.received"},
	}
	if cfg.AccountID != "" {
		body["accountId"] = cfg.AccountID
	}
	return wablastGetSecret(ctx, base+"/v1/webhooks", cfg.APIKey, body)
}

func findWablastEndpoint(ctx context.Context, base, apiKey, url string) (string, bool, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/v1/webhooks", nil)
	if err != nil {
		return "", false, err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	resp, err := wablastClient().Do(req)
	if err != nil {
		return "", false, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		b, _ := io.ReadAll(resp.Body)
		return "", false, fmt.Errorf("wablast list webhooks %d: %s", resp.StatusCode, b)
	}
	var list []struct {
		ID  string `json:"id"`
		URL string `json:"url"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&list); err != nil {
		return "", false, err
	}
	for _, e := range list {
		if e.URL == url {
			return e.ID, true, nil
		}
	}
	return "", false, nil
}

func wablastGetSecret(ctx context.Context, endpoint, apiKey string, body map[string]any) (string, error) {
	var reader io.Reader
	if body != nil {
		buf, err := json.Marshal(body)
		if err != nil {
			return "", err
		}
		reader = bytes.NewReader(buf)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, reader)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := wablastClient().Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		b, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("wablast register %d: %s", resp.StatusCode, b)
	}
	var out struct {
		Secret string `json:"secret"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", err
	}
	if out.Secret == "" {
		return "", fmt.Errorf("wablast register: empty secret")
	}
	return out.Secret, nil
}

func wablastClient() *http.Client { return &http.Client{Timeout: 10 * time.Second} }
