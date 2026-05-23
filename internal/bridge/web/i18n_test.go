package web

import "testing"

func TestPTBRHasAtLeast20Strings(t *testing.T) {
	m, err := loadLocale("pt-BR")
	if err != nil {
		t.Fatalf("loadLocale: %v", err)
	}
	if len(m) < 20 {
		t.Fatalf("pt-BR has %d strings, expected >=20", len(m))
	}
}

func TestPTBRRequiredKeysPresent(t *testing.T) {
	m, err := loadLocale("pt-BR")
	if err != nil {
		t.Fatalf("loadLocale: %v", err)
	}
	required := []string{
		"app.title", "nav.dashboard", "nav.new_tenant", "nav.messages",
		"nav.dlq", "nav.settings", "login.title", "login.invalid",
		"dashboard.title", "wizard.title", "settings.title",
		"messages.title", "dlq.title",
	}
	for _, k := range required {
		if v, ok := m[k]; !ok || v == "" {
			t.Errorf("missing/empty key %q", k)
		}
	}
}

func TestTFuncResolvesAndFallsBack(t *testing.T) {
	tf := newTFunc(map[string]string{"a": "A"})
	if tf("a") != "A" {
		t.Errorf("resolve a")
	}
	if tf("missing") != "missing" {
		t.Errorf("fallback should echo key")
	}
}
