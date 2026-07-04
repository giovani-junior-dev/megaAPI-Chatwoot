package bridge

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

const swSecret = "whsec_MfKQ9r8GKYqrTwjUPD8ILPZIo2LaLaSw" // 24 random bytes, base64

func signSW(secret, id, ts string, body []byte) string {
	sb, _ := base64.StdEncoding.DecodeString(secret[len("whsec_"):])
	mac := hmac.New(sha256.New, sb)
	mac.Write([]byte(id + "." + ts + "." + string(body)))
	return "v1," + base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

func TestVerifyStandardWebhook_ValidAccepted(t *testing.T) {
	body := []byte(`{"type":"message.received"}`)
	ts := "1700000000"
	now := time.Unix(1700000000, 0)
	sig := signSW(swSecret, "evt_1", ts, body)
	require.True(t, VerifyStandardWebhook(swSecret, "evt_1", ts, sig, body, now))
}

func TestVerifyStandardWebhook_TamperedRejected(t *testing.T) {
	body := []byte(`{"type":"message.received"}`)
	ts := "1700000000"
	now := time.Unix(1700000000, 0)
	sig := signSW(swSecret, "evt_1", ts, body)
	require.False(t, VerifyStandardWebhook(swSecret, "evt_1", ts, sig, []byte(`{"type":"x"}`), now))
}

func TestVerifyStandardWebhook_ExpiredTimestampRejected(t *testing.T) {
	body := []byte(`{}`)
	ts := "1700000000"
	now := time.Unix(1700000000+400, 0) // >300s skew
	sig := signSW(swSecret, "evt_1", ts, body)
	require.False(t, VerifyStandardWebhook(swSecret, "evt_1", ts, sig, body, now))
}

func TestVerifyStandardWebhook_MultiSigAcceptsMatch(t *testing.T) {
	body := []byte(`{}`)
	ts := "1700000000"
	now := time.Unix(1700000000, 0)
	good := signSW(swSecret, "evt_1", ts, body)
	header := "v1,AAAAbad " + good // space-separated list, first is wrong
	require.True(t, VerifyStandardWebhook(swSecret, "evt_1", ts, header, body, now))
}

func TestVerifyStandardWebhook_BadSecretPrefixRejected(t *testing.T) {
	require.False(t, VerifyStandardWebhook("nope_abc", "evt_1", "1700000000", "v1,x", []byte(`{}`), time.Unix(1700000000, 0)))
}

func TestEncrypt_RoundtripWithSameKey(t *testing.T) {
	key := RandomBytes(32)
	plain := []byte("hello secret world")
	ct, err := Encrypt(plain, key)
	require.NoError(t, err)
	require.NotEqual(t, plain, ct)
	got, err := Decrypt(ct, key)
	require.NoError(t, err)
	require.Equal(t, plain, got)
}

func TestDecrypt_WrongKeyFails(t *testing.T) {
	k1, k2 := RandomBytes(32), RandomBytes(32)
	ct, err := Encrypt([]byte("secret"), k1)
	require.NoError(t, err)
	_, err = Decrypt(ct, k2)
	require.Error(t, err)
}

func TestDecrypt_TamperedFails(t *testing.T) {
	key := RandomBytes(32)
	ct, err := Encrypt([]byte("secret"), key)
	require.NoError(t, err)
	ct[len(ct)-1] ^= 0x01
	_, err = Decrypt(ct, key)
	require.Error(t, err)
}

func TestDecrypt_TruncatedFails(t *testing.T) {
	key := RandomBytes(32)
	_, err := Decrypt([]byte("short"), key)
	require.Error(t, err)
}

func TestEncrypt_RejectsBadKey(t *testing.T) {
	_, err := Encrypt([]byte("x"), []byte("too-short"))
	require.Error(t, err)
}

func TestVerifyHMAC_ValidAccepted(t *testing.T) {
	body := []byte(`{"event":"x"}`)
	secret := "super-secret"
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	sig := hex.EncodeToString(mac.Sum(nil))
	require.True(t, VerifyHMAC(body, sig, secret))
}

func TestVerifyHMAC_InvalidRejected(t *testing.T) {
	require.False(t, VerifyHMAC([]byte("x"), "deadbeef", "secret"))
	require.False(t, VerifyHMAC([]byte("x"), "not-hex", "secret"))
}

func TestRandomBytes_Length(t *testing.T) {
	a := RandomBytes(32)
	b := RandomBytes(32)
	require.Len(t, a, 32)
	require.Len(t, b, 32)
	require.NotEqual(t, a, b)
}
