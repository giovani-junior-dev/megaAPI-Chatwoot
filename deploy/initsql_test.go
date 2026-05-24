package deploy_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInitSQLCreatesBothDatabasesAndUsers(t *testing.T) {
	p := filepath.Join(".", "init.sql")
	if _, err := os.Stat(p); err != nil {
		p = filepath.Join("deploy", "init.sql")
	}
	raw, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("read init.sql: %v", err)
	}
	body := string(raw)
	for _, want := range []string{
		"CREATE USER chatwoot",
		"CREATE USER bridge",
		"CREATE DATABASE chatwoot",
		"CREATE DATABASE bridge",
		"GRANT ALL PRIVILEGES ON DATABASE chatwoot TO chatwoot",
		"GRANT ALL PRIVILEGES ON DATABASE bridge   TO bridge",
		"CREATE EXTENSION IF NOT EXISTS vector",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("init.sql missing %q", want)
		}
	}
}
