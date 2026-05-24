package deploy_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func templatePath(t *testing.T, name string) string {
	t.Helper()
	p := filepath.Join(".", "templates", name)
	if _, err := os.Stat(p); err == nil {
		return p
	}
	return filepath.Join("deploy", "templates", name)
}

func TestEnvBridgeTemplatePlaceholders(t *testing.T) {
	raw, err := os.ReadFile(templatePath(t, ".env.bridge.tpl"))
	if err != nil {
		t.Fatalf("read .env.bridge.tpl: %v", err)
	}
	body := string(raw)
	for _, want := range []string{"${MASTER_KEY}", "${POSTGRES_PASSWORD}", "${DOMAIN}", "${EMAIL}"} {
		if !strings.Contains(body, want) {
			t.Errorf(".env.bridge.tpl missing %q", want)
		}
	}
}

func TestEnvChatwootTemplatePlaceholders(t *testing.T) {
	raw, err := os.ReadFile(templatePath(t, ".env.chatwoot.tpl"))
	if err != nil {
		t.Fatalf("read .env.chatwoot.tpl: %v", err)
	}
	body := string(raw)
	for _, want := range []string{"${CHATWOOT_SECRET_KEY_BASE}", "${POSTGRES_PASSWORD}", "${REDIS_PASSWORD}", "${DOMAIN}", "${EMAIL}"} {
		if !strings.Contains(body, want) {
			t.Errorf(".env.chatwoot.tpl missing %q", want)
		}
	}
}

func TestCaddyfileTemplatePlaceholders(t *testing.T) {
	raw, err := os.ReadFile(templatePath(t, "Caddyfile.tpl"))
	if err != nil {
		t.Fatalf("read Caddyfile.tpl: %v", err)
	}
	body := string(raw)
	for _, want := range []string{"${DOMAIN}", "${EMAIL}", "Strict-Transport-Security", "reverse_proxy"} {
		if !strings.Contains(body, want) {
			t.Errorf("Caddyfile.tpl missing %q", want)
		}
	}
}
