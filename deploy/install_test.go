package deploy_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInstallScriptHasRequiredFeatures(t *testing.T) {
	p := filepath.Join(".", "install.sh")
	if _, err := os.Stat(p); err != nil {
		p = filepath.Join("deploy", "install.sh")
	}
	raw, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("read install.sh: %v", err)
	}
	body := string(raw)
	for _, want := range []string{
		"set -euo pipefail",
		"openssl rand -base64 32",
		"openssl rand -hex 64",
		"--tls",
		"--tunnel",
		"render-templates.sh",
		"bootstrap-chatwoot.sh",
		"bootstrap-bridge.sh",
		"setup-tunnel.sh",
		"docker compose",
		"/healthz",
		"chmod 600",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("install.sh missing %q", want)
		}
	}
}
