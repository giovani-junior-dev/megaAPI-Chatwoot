package web

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type ChatwootWebhookConfig struct {
	BaseURL    string
	Token      string
	AccountID  int64
	InboxID    int64
	WebhookURL string
}

func ConfigureChatwootWebhook(ctx context.Context, cfg ChatwootWebhookConfig) error {
	body, err := json.Marshal(map[string]any{
		"channel": map[string]any{
			"webhook_url":    cfg.WebhookURL,
			"hmac_mandatory": true,
		},
	})
	if err != nil {
		return err
	}
	u := strings.TrimRight(cfg.BaseURL, "/") +
		fmt.Sprintf("/api/v1/accounts/%d/inboxes/%d", cfg.AccountID, cfg.InboxID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPatch, u, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("api_access_token", cfg.Token)
	req.Header.Set("Content-Type", "application/json")
	cl := &http.Client{Timeout: 10 * time.Second}
	resp, err := cl.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("chatwoot %d: %s", resp.StatusCode, string(b))
	}
	return nil
}

func FetchChatwootInboxHMAC(ctx context.Context, cfg ChatwootWebhookConfig) (string, error) {
	u := strings.TrimRight(cfg.BaseURL, "/") +
		fmt.Sprintf("/api/v1/accounts/%d/inboxes/%d", cfg.AccountID, cfg.InboxID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("api_access_token", cfg.Token)
	cl := &http.Client{Timeout: 10 * time.Second}
	resp, err := cl.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		b, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("chatwoot %d: %s", resp.StatusCode, string(b))
	}
	var out struct {
		HMACToken string `json:"hmac_token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", err
	}
	return out.HMACToken, nil
}
