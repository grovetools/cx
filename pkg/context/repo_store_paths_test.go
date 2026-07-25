package context

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// newRepoStoreWorktree builds a git checkout at the layout `cx repo add/audit`
// produces — <data>/cx/repos/<slug>/.grove-worktrees/<commit-prefix> — and
// returns its path. GROVE_HOME is pointed at a temp dir so the store is the
// test's own.
func newRepoStoreWorktree(t *testing.T) string {
	t.Helper()

	groveHome := t.TempDir()
	t.Setenv("GROVE_HOME", groveHome)
	ClearManagerCache()
	t.Cleanup(ClearManagerCache)

	worktree := filepath.Join(groveHome, "data", "grove", "cx", "repos",
		"github.com_acme_widget_deadbeef", ".grove-worktrees", "2671cf114428")
	if err := os.MkdirAll(worktree, 0o755); err != nil {
		t.Fatalf("creating repo-store worktree: %v", err)
	}
	// A real checkout — workspace classification only treats git repos as
	// workspaces, which is what routed these paths into the notebook.
	if out, err := exec.Command("git", "-C", worktree, "init", "-q").CombinedOutput(); err != nil {
		t.Fatalf("git init: %v: %s", err, out)
	}
	if err := os.WriteFile(filepath.Join(worktree, "main.go"), []byte("package main\n"), 0o644); err != nil {
		t.Fatalf("writing repo file: %v", err)
	}
	return worktree
}

// A cx-managed external repo must keep its rules and generated context inside
// the checkout. Routing them through the notebook created a workspace named
// after the worktree's commit-hash directory, so the audit wrote its rules
// somewhere no other tool (or user) would look.
func TestRepoStoreWorktreeResolvesArtifactsLocally(t *testing.T) {
	worktree := newRepoStoreWorktree(t)
	mgr := NewManager(worktree)

	cases := []struct {
		name string
		got  string
		want string
	}{
		{"rules", mgr.ResolveRulesPath(), filepath.Join(worktree, ActiveRulesFile)},
		{"rules write", mgr.ResolveRulesWritePath(), filepath.Join(worktree, ActiveRulesFile)},
		{"context", mgr.ResolveContextPath(), filepath.Join(worktree, ContextFile)},
		{"context write", mgr.ResolveContextWritePath(), filepath.Join(worktree, ContextFile)},
		{"cached context", mgr.ResolveCachedContextPath(), filepath.Join(worktree, CachedContextFile)},
		{"cached context write", mgr.ResolveCachedContextWritePath(), filepath.Join(worktree, CachedContextFile)},
		{"cached list", mgr.ResolveCachedContextFilesListPath(), filepath.Join(worktree, CachedContextFilesListFile)},
	}
	for _, tc := range cases {
		if tc.got != tc.want {
			t.Errorf("%s path = %s, want %s", tc.name, tc.got, tc.want)
		}
	}
}

// The rules cascade must read back what the audit seeds in the checkout rather
// than falling through to the invoking workspace's rules.
func TestRepoStoreWorktreeLoadsLocalRules(t *testing.T) {
	worktree := newRepoStoreWorktree(t)

	rulesPath := filepath.Join(worktree, ActiveRulesFile)
	if err := os.MkdirAll(filepath.Dir(rulesPath), 0o755); err != nil {
		t.Fatalf("creating .grove dir: %v", err)
	}
	if err := os.WriteFile(rulesPath, []byte("*\n"), 0o644); err != nil {
		t.Fatalf("writing rules: %v", err)
	}

	content, path, err := NewManager(worktree).LoadRulesContent()
	if err != nil {
		t.Fatalf("LoadRulesContent: %v", err)
	}
	if path != rulesPath {
		t.Errorf("rules loaded from %s, want %s", path, rulesPath)
	}
	if string(content) != "*\n" {
		t.Errorf("rules content = %q, want %q", string(content), "*\n")
	}
}
