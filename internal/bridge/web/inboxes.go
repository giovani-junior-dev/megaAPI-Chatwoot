package web

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type Inbox struct {
	ID           int64  `json:"id"`
	Name         string `json:"name"`
	ChannelType  string `json:"channel_type"`
	WebhookURL   string `json:"webhook_url,omitempty"`
}

type discoverReq struct {
	ChatwootURL string `json:"chatwoot_url"`
	Token       string `json:"chatwoot_token"`
	AccountID   int64  `json:"account_id"`
}

func (h *Handler) handleDiscoverInboxes(w http.ResponseWriter, r *http.Request) {
	req, err := readDiscoverReq(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	inboxes, err := h.fetchInboxes(r.Context(), req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(inboxes)
}

func readDiscoverReq(r *http.Request) (discoverReq, error) {
	var req discoverReq
	if strings.Contains(r.Header.Get("Content-Type"), "application/json") {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			return req, fmt.Errorf("bad json: %w", err)
		}
	} else {
		if err := r.ParseForm(); err != nil {
			return req, err
		}
		req.ChatwootURL = r.PostFormValue("chatwoot_url")
		req.Token = r.PostFormValue("chatwoot_token")
		req.AccountID, _ = strconv.ParseInt(r.PostFormValue("account_id"), 10, 64)
	}
	if req.ChatwootURL == "" || req.Token == "" || req.AccountID == 0 {
		return req, errors.New("chatwoot_url, chatwoot_token, account_id required")
	}
	return req, nil
}

func (h *Handler) fetchInboxes(ctx context.Context, req discoverReq) ([]Inbox, error) {
	fetcher := h.deps.FetchInboxes
	if fetcher == nil {
		fetcher = fetchInboxesHTTP
	}
	return fetcher(ctx, req)
}

func fetchInboxesHTTP(ctx context.Context, req discoverReq) ([]Inbox, error) {
	u := strings.TrimRight(req.ChatwootURL, "/") +
		fmt.Sprintf("/api/v1/accounts/%d/inboxes", req.AccountID)
	hr, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	hr.Header.Set("api_access_token", req.Token)
	cl := &http.Client{Timeout: 10 * time.Second}
	resp, err := cl.Do(hr)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("chatwoot %d: %s", resp.StatusCode, string(body))
	}
	return decodeInboxes(resp.Body)
}

func decodeInboxes(r io.Reader) ([]Inbox, error) {
	var wrap struct {
		Payload []Inbox `json:"payload"`
	}
	if err := json.NewDecoder(r).Decode(&wrap); err != nil {
		return nil, err
	}
	return wrap.Payload, nil
}
