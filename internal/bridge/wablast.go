package bridge

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"strings"
)

const wablastDefaultBase = "https://api.wablastmessage.com"

// wablastProvider relays through WaBlast (official Meta WhatsApp Cloud gateway).
type wablastProvider struct{ s *Server }

func wablastBaseURL() string {
	if v := os.Getenv("WABLAST_BASE_URL"); v != "" {
		return strings.TrimRight(v, "/")
	}
	return wablastDefaultBase
}

type wablastInbound struct {
	Type string `json:"type"`
	Data struct {
		MessageID   string `json:"message_id"`
		From        string `json:"from"`
		ContactName string `json:"contact_name"`
		Type        string `json:"type"`
		Text        string `json:"text"`
		Media       struct {
			ID       string `json:"id"`
			MimeType string `json:"mime_type"`
			Caption  string `json:"caption"`
		} `json:"media"`
	} `json:"data"`
}

// wablastInboundID returns the message id and whether the event is a
// message.received (the only inbound type the bridge relays).
func wablastInboundID(body []byte) (id string, received bool) {
	var e wablastInbound
	if err := json.Unmarshal(body, &e); err != nil {
		return "", false
	}
	if e.Type != "message.received" {
		return "", false
	}
	return e.Data.MessageID, true
}

func (w wablastProvider) ParseInbound(body []byte) (InboundMessage, error) {
	var e wablastInbound
	if err := json.Unmarshal(body, &e); err != nil {
		return InboundMessage{}, err
	}
	if e.Type != "message.received" {
		return InboundMessage{}, fmt.Errorf("wablast: ignored event %q", e.Type)
	}
	msg := InboundMessage{
		ContactID:  strings.TrimPrefix(e.Data.From, "+"),
		Name:       e.Data.ContactName,
		Text:       e.Data.Text,
		ExternalID: e.Data.MessageID,
	}
	if e.Data.Media.ID != "" {
		msg.Attachment = &Attachment{
			MediaKey: e.Data.Media.ID,
			MimeType: e.Data.Media.MimeType,
			Caption:  e.Data.Media.Caption,
			Kind:     e.Data.Type,
		}
	}
	return msg, nil
}

func (w wablastProvider) SendText(ctx context.Context, t Tenant, to, text string) error {
	key, err := Decrypt(t.WablastAPIKeyEnc, w.s.Key)
	if err != nil {
		return notRetriable(err)
	}
	body := wablastMessageBody(t, to, "text")
	body["text"] = map[string]any{"body": text}
	_, err = w.postJSON(ctx, string(key), wablastBaseURL()+"/v1/messages", body)
	return err
}

func (w wablastProvider) SendMedia(ctx context.Context, t Tenant, to string, a Attachment) error {
	key, err := Decrypt(t.WablastAPIKeyEnc, w.s.Key)
	if err != nil {
		return notRetriable(err)
	}
	data, mime, err := downloadPublicURL(ctx, a)
	if err != nil {
		return err
	}
	mediaID, err := w.uploadMedia(ctx, string(key), t.WablastAccountID, data, chooseFileName(a, mime), mime)
	if err != nil {
		return err
	}
	body := wablastMessageBody(t, to, a.Kind)
	media := map[string]any{"id": mediaID, "caption": a.Caption}
	if a.Kind == "document" {
		media["filename"] = chooseFileName(a, mime)
	}
	body["media"] = media
	_, err = w.postJSON(ctx, string(key), wablastBaseURL()+"/v1/messages", body)
	return err
}

func (w wablastProvider) DownloadMedia(ctx context.Context, t Tenant, a Attachment) ([]byte, string, error) {
	key, err := Decrypt(t.WablastAPIKeyEnc, w.s.Key)
	if err != nil {
		return nil, "", notRetriable(err)
	}
	endpoint := wablastBaseURL() + "/v1/media/" + url.PathEscape(a.MediaKey)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, "", notRetriable(err)
	}
	req.Header.Set("Authorization", "Bearer "+string(key))
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, "", retriable(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		b, _ := io.ReadAll(resp.Body)
		return nil, "", classifyWablast(resp.StatusCode, b, "wablast media")
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, cwUploadLimit+1))
	if err != nil {
		return nil, "", retriable(err)
	}
	mime := resp.Header.Get("Content-Type")
	if mime == "" {
		mime = firstNonEmpty(a.MimeType, defaultMime(a.Kind))
	}
	return data, mime, nil
}

func wablastMessageBody(t Tenant, to, typ string) map[string]any {
	m := map[string]any{"to": to, "type": typ}
	if t.WablastAccountID != "" {
		m["account_id"] = t.WablastAccountID
	}
	return m
}

func (w wablastProvider) postJSON(ctx context.Context, key, endpoint string, in any) ([]byte, error) {
	resp, err := bearerPost(ctx, endpoint, key, in)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return nil, classifyWablast(resp.StatusCode, b, "wablast "+endpoint)
	}
	return b, nil
}

func (w wablastProvider) uploadMedia(ctx context.Context, key, accountID string, data []byte, filename, mime string) (string, error) {
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	part, err := newFilePart(mw, "file", filename, mime)
	if err != nil {
		return "", retriable(err)
	}
	if _, err := part.Write(data); err != nil {
		return "", retriable(err)
	}
	if err := mw.Close(); err != nil {
		return "", retriable(err)
	}
	endpoint := wablastBaseURL() + "/v1/media"
	if accountID != "" {
		endpoint += "?account_id=" + url.QueryEscape(accountID)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, &buf)
	if err != nil {
		return "", notRetriable(err)
	}
	req.Header.Set("Authorization", "Bearer "+key)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	resp, err := httpClient.Do(req)
	if err != nil {
		return "", retriable(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return "", classifyWablast(resp.StatusCode, b, "wablast media upload")
	}
	var out struct {
		MediaID string `json:"media_id"`
	}
	if err := json.Unmarshal(b, &out); err != nil || out.MediaID == "" {
		return "", retriable(fmt.Errorf("wablast media upload: no media_id in %s", b))
	}
	return out.MediaID, nil
}

// classifyWablast maps a WaBlast error envelope {error, code} to the bridge retry
// policy. WINDOW_CLOSED is surfaced as errWindowClosed so the caller can leave a
// private note; transient codes (rate limit, tier cap, token) and 5xx/429 retry.
func classifyWablast(status int, body []byte, label string) error {
	var e struct {
		Error string `json:"error"`
		Code  string `json:"code"`
	}
	_ = json.Unmarshal(body, &e)
	switch e.Code {
	case "WINDOW_CLOSED":
		return errWindowClosed
	case "META_RATE_LIMIT", "TIER_DAILY_CAP_REACHED", "ACCOUNT_TOKEN_INVALID":
		return retriable(fmt.Errorf("%s %d: %s", label, status, body))
	}
	if status >= 500 || status == http.StatusTooManyRequests {
		return retriable(fmt.Errorf("%s %d: %s", label, status, body))
	}
	return notRetriable(fmt.Errorf("%s %d: %s", label, status, body))
}
