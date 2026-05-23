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

type MegaAPIWebhookConfig struct {
	Host       string
	Instance   string
	Token      string
	WebhookURL string
}

func ConfigureMegaAPIWebhook(ctx context.Context, cfg MegaAPIWebhookConfig) error {
	body, err := json.Marshal(map[string]any{
		"messageData": map[string]any{
			"webhookUrl":     cfg.WebhookURL,
			"webhookEnabled": true,
		},
	})
	if err != nil {
		return err
	}
	u := strings.TrimRight(cfg.Host, "/") +
		fmt.Sprintf("/rest/webhook/%s/configWebhook", cfg.Instance)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+cfg.Token)
	req.Header.Set("Content-Type", "application/json")
	cl := &http.Client{Timeout: 10 * time.Second}
	resp, err := cl.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("megaapi %d: %s", resp.StatusCode, string(b))
	}
	return nil
}
