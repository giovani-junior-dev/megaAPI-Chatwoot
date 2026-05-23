package main

import (
	"testing"
)

func TestParseAdminFlagsRequiresEmailAndPassword(t *testing.T) {
	if _, _, err := parseAdminFlags([]string{}); err == nil {
		t.Fatalf("expected error for empty flags")
	}
	if _, _, err := parseAdminFlags([]string{"--email", "a@b"}); err == nil {
		t.Fatalf("expected error for missing password")
	}
	if _, _, err := parseAdminFlags([]string{"--password", "p"}); err == nil {
		t.Fatalf("expected error for missing email")
	}
}

func TestParseAdminFlagsParsesValues(t *testing.T) {
	email, pwd, err := parseAdminFlags([]string{"--email", "a@b", "--password", "p"})
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if email != "a@b" || pwd != "p" {
		t.Fatalf("got email=%q pwd=%q", email, pwd)
	}
}
