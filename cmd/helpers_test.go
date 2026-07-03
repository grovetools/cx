package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/grovetools/cx/pkg/context"
)

// A minimal job .md: YAML frontmatter (two '---' fences) plus a body. Passing
// this to the rules parser triggers "multiple '---' separators found".
const sampleJobMarkdown = `---
id: sample-job-1234
type: chat
rules_file: sample.rules
---

Some chat prompt body.
`

func TestResolveRulesFileFlag_RejectsJobMarkdown(t *testing.T) {
	dir := t.TempDir()
	mdPath := filepath.Join(dir, "20-my-job.md")
	if err := os.WriteFile(mdPath, []byte(sampleJobMarkdown), 0o600); err != nil {
		t.Fatal(err)
	}

	mgr := context.NewManager(dir)
	_, err := ResolveRulesFileFlag(mgr, "", mdPath)
	if err == nil {
		t.Fatalf("expected an error for a .md passed to --rules-file, got nil")
	}
	if !strings.Contains(err.Error(), "--job") {
		t.Fatalf("error should point the user at --job, got: %v", err)
	}
}

func TestResolveRulesFileFlag_AllowsRulesFile(t *testing.T) {
	dir := t.TempDir()
	rulesPath := filepath.Join(dir, "custom.rules")
	// A .rules file may legitimately open with a '---' hot/cold separator; it
	// must never be mistaken for a job file.
	if err := os.WriteFile(rulesPath, []byte("---\n**/*.go\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	mgr := context.NewManager(dir)
	got, err := ResolveRulesFileFlag(mgr, "", rulesPath)
	if err != nil {
		t.Fatalf("unexpected error for a .rules file: %v", err)
	}
	if got != rulesPath {
		t.Fatalf("expected rules path %q returned unchanged, got %q", rulesPath, got)
	}
}

func TestResolveRulesFileFlag_EmptyIsNoop(t *testing.T) {
	mgr := context.NewManager(t.TempDir())
	got, err := ResolveRulesFileFlag(mgr, "", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "" {
		t.Fatalf("expected empty result, got %q", got)
	}
}

func TestIsJobFile(t *testing.T) {
	cases := []struct {
		name    string
		path    string
		content string
		want    bool
	}{
		{"md extension", "job.md", "no frontmatter here", true},
		{"markdown extension", "job.markdown", "", true},
		{"rules extension with separator", "custom.rules", "---\n**/*.go\n", false},
		{"unknown ext with frontmatter fence", "job.txt", "---\nid: x\n---\n", true},
		{"unknown ext with leading blank then fence", "job.txt", "\n\n---\nid: x\n", true},
		{"unknown ext plain rules content", "job.txt", "**/*.go\nsrc/**\n", false},
		{"unknown ext with comment first", "job.txt", "# a comment\n**/*.go\n", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := context.IsJobFile(tc.path, []byte(tc.content)); got != tc.want {
				t.Fatalf("IsJobFile(%q, %q) = %v, want %v", tc.path, tc.content, got, tc.want)
			}
		})
	}
}
