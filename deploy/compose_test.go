package deploy_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDockerComposeHasBackupSidecar(t *testing.T) {
	p := filepath.Join(".", "docker-compose.yml")
	if _, err := os.Stat(p); err != nil {
		p = filepath.Join("deploy", "docker-compose.yml")
	}
	raw, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("read docker-compose.yml: %v", err)
	}
	body := string(raw)
	for _, want := range []string{
		"prodrigestivill/postgres-backup-local",
		"BACKUP_KEEP_DAYS: 14",
		"/backups",
		"profiles: [\"tunnel\"]",
		"profiles: [\"tls\"]",
		"caddy:2-alpine",
		"cloudflare/cloudflared:latest",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("docker-compose.yml missing %q", want)
		}
	}
}
