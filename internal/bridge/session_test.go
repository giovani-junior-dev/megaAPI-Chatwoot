package bridge

import (
	"testing"
	"time"
)

func TestSessionRoundtrip(t *testing.T) {
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}
	tok, err := NewSession("admin@example.com", key, time.Hour)
	if err != nil {
		t.Fatalf("NewSession: %v", err)
	}
	email, err := ParseSession(tok, key)
	if err != nil {
		t.Fatalf("ParseSession: %v", err)
	}
	if email != "admin@example.com" {
		t.Fatalf("email=%q", email)
	}
}

func TestSessionTamperRejected(t *testing.T) {
	key := make([]byte, 32)
	tok, _ := NewSession("admin@example.com", key, time.Hour)
	bad := tok[:len(tok)-2] + "AA"
	if _, err := ParseSession(bad, key); err == nil {
		t.Fatalf("expected error on tamper")
	}
}

func TestSessionExpired(t *testing.T) {
	key := make([]byte, 32)
	tok, _ := NewSession("a@b", key, -time.Second)
	if _, err := ParseSession(tok, key); err == nil {
		t.Fatalf("expected expiry error")
	}
}
