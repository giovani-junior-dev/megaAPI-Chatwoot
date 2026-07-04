package bridge

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"strconv"
	"strings"
	"time"
)

const (
	stdWebhookPrefix = "whsec_"
	stdWebhookSkew   = 300 // seconds tolerated between webhook-timestamp and now (anti-replay)
)

// VerifyStandardWebhook checks a Standard Webhooks signature (standardwebhooks.com):
// signed = "{id}.{ts}.{body}", sig = base64(HMAC-SHA256(base64decode(secret[6:]), signed)),
// header "webhook-signature" carries one or more space-separated "v1,<sig>" tokens.
func VerifyStandardWebhook(secret, id, ts, sigHeader string, body []byte, now time.Time) bool {
	if !strings.HasPrefix(secret, stdWebhookPrefix) {
		return false
	}
	secretBytes, err := base64.StdEncoding.DecodeString(secret[len(stdWebhookPrefix):])
	if err != nil {
		return false
	}
	tsInt, err := strconv.ParseInt(ts, 10, 64)
	if err != nil {
		return false
	}
	if skew := now.Unix() - tsInt; skew > stdWebhookSkew || skew < -stdWebhookSkew {
		return false
	}
	mac := hmac.New(sha256.New, secretBytes)
	mac.Write([]byte(id + "." + ts + "." + string(body)))
	expected := base64.StdEncoding.EncodeToString(mac.Sum(nil))
	for _, part := range strings.Fields(sigHeader) {
		version, sig, ok := strings.Cut(part, ",")
		if !ok || version != "v1" {
			continue
		}
		if subtle.ConstantTimeCompare([]byte(sig), []byte(expected)) == 1 {
			return true
		}
	}
	return false
}

const nonceLen = 12

var (
	errKeyLen        = errors.New("bridge: master key must be 32 bytes")
	errCiphertextLen = errors.New("bridge: ciphertext too short")
)

func Encrypt(plaintext, key []byte) ([]byte, error) {
	gcm, err := newGCM(key)
	if err != nil {
		return nil, err
	}
	nonce := RandomBytes(nonceLen)
	out := make([]byte, 0, nonceLen+len(plaintext)+gcm.Overhead())
	out = append(out, nonce...)
	return gcm.Seal(out, nonce, plaintext, nil), nil
}

func Decrypt(ciphertext, key []byte) ([]byte, error) {
	gcm, err := newGCM(key)
	if err != nil {
		return nil, err
	}
	if len(ciphertext) < nonceLen+gcm.Overhead() {
		return nil, errCiphertextLen
	}
	nonce, body := ciphertext[:nonceLen], ciphertext[nonceLen:]
	return gcm.Open(nil, nonce, body, nil)
}

func VerifyHMAC(body []byte, signatureHex, secret string) bool {
	want, err := hex.DecodeString(signatureHex)
	if err != nil {
		return false
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	return subtle.ConstantTimeCompare(want, mac.Sum(nil)) == 1
}

func RandomBytes(n int) []byte {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return b
}

func newGCM(key []byte) (cipher.AEAD, error) {
	if len(key) != 32 {
		return nil, errKeyLen
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}
