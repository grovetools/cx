package context

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveRulesSpec(t *testing.T) {
	repo := t.TempDir()
	mgr := NewManager(repo)

	write := func(path string) string {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("**/*.go\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		return path
	}

	preset := write(filepath.Join(repo, ".cx", "doc.rules"))
	got, err := ResolveRulesSpec(mgr, repo, "doc")
	if err != nil || got != preset {
		t.Fatalf("named preset = %q, %v; want %q", got, err, preset)
	}

	legacy := write(filepath.Join(repo, "docs", "legacy.rules"))
	got, err = ResolveRulesSpec(mgr, repo, "legacy.rules")
	if err != nil || got != legacy {
		t.Fatalf("legacy filename = %q, %v; want %q", got, err, legacy)
	}
	got, err = ResolveRulesSpec(mgr, repo, legacy)
	if err != nil || got != legacy {
		t.Fatalf("absolute legacy path = %q, %v; want %q", got, err, legacy)
	}

	if _, err := ResolveRulesSpec(mgr, repo, "missing"); err == nil || !strings.Contains(err.Error(), "searched:") {
		t.Fatalf("missing preset error = %v; want searched paths", err)
	}
}

func TestIsLegacyRulesSpec(t *testing.T) {
	for _, tc := range []struct {
		spec string
		want bool
	}{
		{"doc", false},
		{"doc.rules", true},
		{".cx/doc.rules", true},
		{"/tmp/doc.rules", true},
	} {
		if got := IsLegacyRulesSpec(tc.spec); got != tc.want {
			t.Errorf("IsLegacyRulesSpec(%q) = %v, want %v", tc.spec, got, tc.want)
		}
	}
}
