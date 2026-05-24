package deploy_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestRenderTemplatesProducesCaddyfileWithDomain(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("bash/envsubst not available on Windows CI")
	}
	if _, err := exec.LookPath("envsubst"); err != nil {
		t.Skip("envsubst not installed")
	}
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("bash not installed")
	}

	root, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if filepath.Base(root) != "deploy" {
		root = filepath.Join(root, "deploy")
	}

	tmp := t.TempDir()
	out := filepath.Join(tmp, "deploy-test")
	if err := os.CopyFS(out, os.DirFS(root)); err != nil {
		t.Fatalf("copy deploy: %v", err)
	}

	cmd := exec.Command("bash", filepath.Join(out, "render-templates.sh"))
	cmd.Env = append(os.Environ(),
		"DOMAIN=example.test",
		"EMAIL=ops@example.test",
		"MASTER_KEY=k1",
		"POSTGRES_PASSWORD=p1",
		"REDIS_PASSWORD=r1",
		"CHATWOOT_SECRET_KEY_BASE=s1",
	)
	stdout, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("render: %v\n%s", err, stdout)
	}

	caddy, err := os.ReadFile(filepath.Join(out, "Caddyfile"))
	if err != nil {
		t.Fatalf("read Caddyfile: %v", err)
	}
	body := string(caddy)
	if !strings.Contains(body, "example.test {") {
		t.Errorf("Caddyfile missing rendered domain. body:\n%s", body)
	}
	if !strings.Contains(body, "email ops@example.test") {
		t.Errorf("Caddyfile missing rendered email")
	}
	if strings.Contains(body, "${DOMAIN}") || strings.Contains(body, "${EMAIL}") {
		t.Errorf("Caddyfile still has unrendered placeholders")
	}
}
