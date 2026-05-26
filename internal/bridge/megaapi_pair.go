package bridge

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type PairClient struct {
	Host     string
	Instance string
	Token    string
}

type InstanceStatus struct {
	Status   string
	JID      string
	PushName string
}

type pairStatusResp struct {
	Instance struct {
		Status string `json:"status"`
		User   struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"user"`
	} `json:"instance"`
}

const pairHTTPTimeout = 15 * time.Second

func (c PairClient) FetchQR(ctx context.Context) (string, error) {
	body, err := c.do(ctx, http.MethodGet, "/rest/instance/qrcode_base64/"+c.Instance, "")
	if err != nil {
		return "", err
	}
	var out struct {
		QRCode string `json:"qrcode"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return "", err
	}
	return out.QRCode, nil
}

func (c PairClient) FetchPairingCode(ctx context.Context, phone string) (string, error) {
	q := url.Values{"phoneNumber": []string{phone}}
	body, err := c.do(ctx, http.MethodGet,
		"/rest/instance/pairingCode/"+c.Instance+"?"+q.Encode(), "")
	if err != nil {
		return "", err
	}
	var out struct {
		PairingCode string `json:"pairingCode"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return "", err
	}
	return out.PairingCode, nil
}

func (c PairClient) FetchInstanceStatus(ctx context.Context) (InstanceStatus, error) {
	body, err := c.do(ctx, http.MethodGet, "/rest/instance/"+c.Instance, "")
	if err != nil {
		return InstanceStatus{}, err
	}
	var out pairStatusResp
	if err := json.Unmarshal(body, &out); err != nil {
		return InstanceStatus{}, err
	}
	return InstanceStatus{
		Status:   out.Instance.Status,
		JID:      out.Instance.User.ID,
		PushName: out.Instance.User.Name,
	}, nil
}

func (c PairClient) LogoutInstance(ctx context.Context) error {
	_, err := c.do(ctx, http.MethodDelete, "/rest/instance/"+c.Instance+"/logout", "")
	return err
}

func (c PairClient) RestartInstance(ctx context.Context) error {
	_, err := c.do(ctx, http.MethodDelete, "/rest/instance/"+c.Instance+"/restart", "")
	return err
}

func (c PairClient) do(ctx context.Context, method, path, body string) ([]byte, error) {
	u := strings.TrimRight(c.Host, "/") + path
	var rb io.Reader
	if body != "" {
		rb = strings.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, u, rb)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.Token)
	req.Header.Set("Content-Type", "application/json")
	cl := &http.Client{Timeout: pairHTTPTimeout}
	resp, err := cl.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	rb2, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("megaapi %d: %s", resp.StatusCode, string(rb2))
	}
	return rb2, nil
}
