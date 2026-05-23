package bridge

import (
	"strings"
	"testing"
)

func TestHashPasswordProducesArgon2idPHC(t *testing.T) {
	h, err := HashPassword("hunter2")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	if !strings.HasPrefix(h, "$argon2id$") {
		t.Fatalf("hash prefix: %q", h)
	}
	parts := strings.Split(h, "$")
	if len(parts) != 6 {
		t.Fatalf("PHC must have 6 parts, got %d: %q", len(parts), h)
	}
}

func TestHashPasswordRandomSalt(t *testing.T) {
	a, _ := HashPassword("same")
	b, _ := HashPassword("same")
	if a == b {
		t.Fatalf("equal hashes for equal pwd — salt not random")
	}
}

func TestVerifyPasswordCorrect(t *testing.T) {
	h, _ := HashPassword("correctHorse")
	ok, err := VerifyPassword("correctHorse", h)
	if err != nil {
		t.Fatalf("VerifyPassword: %v", err)
	}
	if !ok {
		t.Fatalf("expected match")
	}
}

func TestVerifyPasswordWrong(t *testing.T) {
	h, _ := HashPassword("correctHorse")
	ok, _ := VerifyPassword("battery", h)
	if ok {
		t.Fatalf("expected mismatch")
	}
}

func TestVerifyPasswordRejectsNonArgon2id(t *testing.T) {
	_, err := VerifyPassword("x", "$2a$10$abc")
	if err == nil {
		t.Fatalf("expected error for non-argon2id hash")
	}
}
