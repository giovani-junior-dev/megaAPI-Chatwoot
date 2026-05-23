package deploy_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCaddyfileContainsSecurityHeaders(t *testing.T) {
	path := filepath.Join(".", "Caddyfile")
	if _, err := os.Stat(path); err != nil {
		path = filepath.Join("deploy", "Caddyfile")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read Caddyfile: %v", err)
	}
	body := string(raw)
	wants := []string{
		"Strict-Transport-Security",
		"Content-Security-Policy",
		`X-Frame-Options "DENY"`,
		"X-Content-Type-Options",
		"reverse_proxy",
	}
	for _, w := range wants {
		if !strings.Contains(body, w) {
			t.Errorf("Caddyfile missing %q", w)
		}
	}
}
