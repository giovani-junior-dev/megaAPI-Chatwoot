package web

import (
	"bytes"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func renderLayout(t *testing.T, p page) string {
	t.Helper()
	h := newTestHandler(t, nil)
	tpl, err := h.tpl.Clone()
	if err != nil {
		t.Fatalf("clone: %v", err)
	}
	if _, err := tpl.New("stub").Parse(`{{define "content"}}<div data-test="content"></div>{{end}}`); err != nil {
		t.Fatalf("stub parse: %v", err)
	}
	var buf bytes.Buffer
	if err := tpl.ExecuteTemplate(&buf, "layout", p); err != nil {
		t.Fatalf("execute layout: %v", err)
	}
	return buf.String()
}

func TestLayoutIncludesInterFont(t *testing.T) {
	out := renderLayout(t, page{Title: "X"})
	if !strings.Contains(out, "fonts.googleapis.com") {
		t.Fatalf("layout missing Google Fonts link (Inter): %s", out)
	}
	if !strings.Contains(out, "display=swap") {
		t.Fatalf("layout missing font-display=swap")
	}
}

func TestLayoutDeclaresDesignTokens(t *testing.T) {
	out := renderLayout(t, page{Title: "X"})
	required := []string{"--bg-app", "--bg-sidebar", "--fg-strong", "--accent", "--accent-hover", "--border-subtle", "--radius-md", "--space-8", "--dur-base", "--ease-out"}
	for _, tok := range required {
		if !strings.Contains(out, tok) {
			t.Fatalf("layout missing design token %s", tok)
		}
	}
	if !strings.Contains(out, "prefers-reduced-motion") {
		t.Fatalf("layout missing prefers-reduced-motion media query")
	}
}

func TestLayoutHidesShellOnLogin(t *testing.T) {
	out := renderLayout(t, page{Title: "Login", HideShell: true})
	if strings.Contains(out, "<aside") {
		t.Fatalf("expected no <aside when HideShell=true; got: %s", out)
	}
	if strings.Contains(out, `id="sidebar"`) {
		t.Fatalf("expected no sidebar element when HideShell=true")
	}
}

func TestSidebarActiveItem(t *testing.T) {
	out := renderLayout(t, page{Title: "Mensagens", ActivePath: "/messages", Email: "admin@example.com"})
	if !strings.Contains(out, `aria-current="page"`) {
		t.Fatalf("expected aria-current=page on active item; got: %s", out)
	}
	idxAria := strings.Index(out, `aria-current="page"`)
	if idxAria < 0 {
		t.Fatalf("aria-current absent")
	}
	window := out[max(0, idxAria-200):idxAria]
	if !strings.Contains(window, "/messages") {
		t.Fatalf("aria-current is not on /messages link; window=%s", window)
	}
	if !strings.Contains(out, "admin@example.com") {
		t.Fatalf("expected admin email in sidebar user card")
	}
}

func TestLayoutSnapshot(t *testing.T) {
	out := renderLayout(t, page{Title: "Painel", ActivePath: "/", Email: "admin@example.com"})
	goldenPath := filepath.Join("testdata", "layout.golden.html")
	if os.Getenv("UPDATE_GOLDEN") == "1" {
		if err := os.MkdirAll("testdata", 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.WriteFile(goldenPath, []byte(out), 0o644); err != nil {
			t.Fatalf("write golden: %v", err)
		}
		return
	}
	want, err := os.ReadFile(goldenPath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			t.Fatalf("golden file missing at %s; run with UPDATE_GOLDEN=1 to create", goldenPath)
		}
		t.Fatalf("read golden: %v", err)
	}
	if string(want) != out {
		t.Fatalf("layout snapshot drift.\nwant:\n%s\n\ngot:\n%s", want, out)
	}
}
