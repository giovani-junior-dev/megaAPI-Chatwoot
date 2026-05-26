package web

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/madeinlowcode/chatwoot-megaapi-bridge/internal/bridge"
)

const defaultPairTTL = 24 * time.Hour

type PairClaim struct {
	Slug  string
	Exp   int64
	Token string
}

type PairLinkParams struct {
	BaseURL    string
	Slug       string
	TTLSeconds int64
}

func SignPairToken(c PairClaim, key []byte) string {
	msg := c.Slug + "|" + strconv.FormatInt(c.Exp, 10)
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(msg))
	return hex.EncodeToString(mac.Sum(nil))
}

func VerifyPairToken(c PairClaim, key []byte) bool {
	if c.Token == "" || c.Slug == "" {
		return false
	}
	if time.Now().Unix() > c.Exp {
		return false
	}
	want := SignPairToken(PairClaim{Slug: c.Slug, Exp: c.Exp}, key)
	return hmac.Equal([]byte(want), []byte(c.Token))
}

func BuildPairLink(p PairLinkParams, key []byte) string {
	ttl := p.TTLSeconds
	if ttl <= 0 {
		ttl = int64(defaultPairTTL.Seconds())
	}
	exp := time.Now().Add(time.Duration(ttl) * time.Second).Unix()
	tok := SignPairToken(PairClaim{Slug: p.Slug, Exp: exp}, key)
	return strings.TrimRight(p.BaseURL, "/") + "/pair/" + p.Slug +
		"?t=" + tok + "&exp=" + strconv.FormatInt(exp, 10)
}

func claimFromRequest(r *http.Request) PairClaim {
	exp, _ := strconv.ParseInt(r.URL.Query().Get("exp"), 10, 64)
	return PairClaim{
		Slug:  chi.URLParam(r, "slug"),
		Exp:   exp,
		Token: r.URL.Query().Get("t"),
	}
}

func (h *Handler) authorizePair(w http.ResponseWriter, r *http.Request) (bridge.Tenant, bool) {
	c := claimFromRequest(r)
	if !VerifyPairToken(c, h.deps.Key) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return bridge.Tenant{}, false
	}
	t, err := h.deps.GetTenant(r.Context(), c.Slug)
	if errors.Is(err, bridge.ErrNotFound) {
		http.Error(w, "tenant not found", http.StatusNotFound)
		return bridge.Tenant{}, false
	}
	if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return bridge.Tenant{}, false
	}
	return t, true
}

func (h *Handler) pairClient(t bridge.Tenant) (bridge.PairClient, error) {
	tok, err := bridge.Decrypt(t.MegaAPITokenEnc, h.deps.Key)
	if err != nil {
		return bridge.PairClient{}, err
	}
	return bridge.PairClient{
		Host:     t.MegaAPIHost,
		Instance: t.MegaAPIInstance,
		Token:    string(tok),
	}, nil
}

func (h *Handler) handlePairLanding(w http.ResponseWriter, r *http.Request) {
	t, ok := h.authorizePair(w, r)
	if !ok {
		return
	}
	data := pairView{
		Slug:  t.Slug,
		Token: r.URL.Query().Get("t"),
		Exp:   r.URL.Query().Get("exp"),
	}
	h.render(w, "pair.html", page{Title: "Pareamento WhatsApp", Data: data})
}

type pairView struct {
	Slug  string
	Token string
	Exp   string
}

func (h *Handler) handlePairQR(w http.ResponseWriter, r *http.Request) {
	t, ok := h.authorizePair(w, r)
	if !ok {
		return
	}
	cl, err := h.pairClient(t)
	if err != nil {
		http.Error(w, "crypto error", http.StatusInternalServerError)
		return
	}
	qr, err := cl.FetchQR(r.Context())
	if err != nil {
		writePairJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}
	writePairJSON(w, http.StatusOK, map[string]string{"qrcode": qr})
}

func (h *Handler) handlePairCode(w http.ResponseWriter, r *http.Request) {
	t, ok := h.authorizePair(w, r)
	if !ok {
		return
	}
	phone := readPhone(r)
	if phone == "" {
		http.Error(w, "phone required", http.StatusBadRequest)
		return
	}
	cl, err := h.pairClient(t)
	if err != nil {
		http.Error(w, "crypto error", http.StatusInternalServerError)
		return
	}
	code, err := cl.FetchPairingCode(r.Context(), phone)
	if err != nil {
		writePairJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}
	writePairJSON(w, http.StatusOK, map[string]string{"pairingCode": code})
}

func (h *Handler) handlePairLogout(w http.ResponseWriter, r *http.Request) {
	t, ok := h.authorizePair(w, r)
	if !ok {
		return
	}
	cl, err := h.pairClient(t)
	if err != nil {
		http.Error(w, "crypto error", http.StatusInternalServerError)
		return
	}
	if err := cl.LogoutInstance(r.Context()); err != nil {
		writePairJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}
	writePairJSON(w, http.StatusOK, map[string]string{"status": "logged_out"})
}

func (h *Handler) handlePairStatus(w http.ResponseWriter, r *http.Request) {
	t, ok := h.authorizePair(w, r)
	if !ok {
		return
	}
	cl, err := h.pairClient(t)
	if err != nil {
		http.Error(w, "crypto error", http.StatusInternalServerError)
		return
	}
	st, err := cl.FetchInstanceStatus(r.Context())
	if err != nil {
		writePairJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}
	writePairJSON(w, http.StatusOK, map[string]any{
		"status":   st.Status,
		"jid":      st.JID,
		"pushName": st.PushName,
	})
}

func readPhone(r *http.Request) string {
	ct := r.Header.Get("Content-Type")
	if strings.Contains(ct, "application/json") {
		var body struct {
			Phone string `json:"phone"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		return strings.TrimSpace(body.Phone)
	}
	_ = r.ParseForm()
	return strings.TrimSpace(r.PostFormValue("phone"))
}

func writePairJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

