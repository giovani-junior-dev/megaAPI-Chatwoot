package bridge

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

func NewSession(email string, key []byte, ttl time.Duration) (string, error) {
	if email == "" {
		return "", errors.New("session: empty email")
	}
	exp := time.Now().Add(ttl).Unix()
	body := fmt.Sprintf("%s|%d", email, exp)
	sig := hmacB64([]byte(body), key)
	return base64.RawURLEncoding.EncodeToString([]byte(body)) + "." + sig, nil
}

func ParseSession(token string, key []byte) (string, error) {
	parts := strings.SplitN(token, ".", 2)
	if len(parts) != 2 {
		return "", errors.New("session: malformed")
	}
	body, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return "", err
	}
	want := hmacB64(body, key)
	if subtle.ConstantTimeCompare([]byte(want), []byte(parts[1])) != 1 {
		return "", errors.New("session: bad signature")
	}
	fields := strings.SplitN(string(body), "|", 2)
	if len(fields) != 2 {
		return "", errors.New("session: bad body")
	}
	exp, err := strconv.ParseInt(fields[1], 10, 64)
	if err != nil {
		return "", err
	}
	if time.Now().Unix() > exp {
		return "", errors.New("session: expired")
	}
	return fields[0], nil
}

func hmacB64(body, key []byte) string {
	m := hmac.New(sha256.New, key)
	_, _ = m.Write(body)
	return base64.RawURLEncoding.EncodeToString(m.Sum(nil))
}
