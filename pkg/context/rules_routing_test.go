package context

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGetRulesFileFromJob(t *testing.T) {
	testDir := t.TempDir()

	// Create a job file with rules_file in frontmatter
	jobContent := `---
rules_file: rules/02-spec.md.rules
---
# Job description
Some content here
`
	jobPath := filepath.Join(testDir, "02-spec.md")
	if err := os.WriteFile(jobPath, []byte(jobContent), 0o644); err != nil {
		t.Fatalf("Failed to write job file: %v", err)
	}

	mgr := NewManager(testDir)

	rulesPath, err := mgr.GetRulesFileFromJob(jobPath)
	if err != nil {
		t.Fatalf("GetRulesFileFromJob returned error: %v", err)
	}

	// Should resolve relative to the job file's directory
	expectedPath := filepath.Join(testDir, "rules/02-spec.md.rules")
	if rulesPath != expectedPath {
		t.Errorf("Expected rules path %s, got %s", expectedPath, rulesPath)
	}
}

func TestGetRulesFileFromJob_AbsolutePath(t *testing.T) {
	testDir := t.TempDir()

	absRulesPath := "/absolute/path/to/rules.rules"
	jobContent := "---\nrules_file: " + absRulesPath + "\n---\n"
	jobPath := filepath.Join(testDir, "job.md")
	if err := os.WriteFile(jobPath, []byte(jobContent), 0o644); err != nil {
		t.Fatalf("Failed to write job file: %v", err)
	}

	mgr := NewManager(testDir)

	rulesPath, err := mgr.GetRulesFileFromJob(jobPath)
	if err != nil {
		t.Fatalf("GetRulesFileFromJob returned error: %v", err)
	}

	if rulesPath != absRulesPath {
		t.Errorf("Expected absolute rules path %s, got %s", absRulesPath, rulesPath)
	}
}

func TestGetRulesFileFromJob_MissingKey(t *testing.T) {
	testDir := t.TempDir()

	jobContent := "---\ntitle: My Job\n---\n# No rules_file key\n"
	jobPath := filepath.Join(testDir, "job.md")
	if err := os.WriteFile(jobPath, []byte(jobContent), 0o644); err != nil {
		t.Fatalf("Failed to write job file: %v", err)
	}

	mgr := NewManager(testDir)

	_, err := mgr.GetRulesFileFromJob(jobPath)
	if err == nil {
		t.Error("Expected error for missing rules_file key, got nil")
	}
	if !strings.Contains(err.Error(), "rules_file key not found") {
		t.Errorf("Expected 'rules_file key not found' error, got: %v", err)
	}
}

func TestSetActiveRules_WritesToCorrectPath(t *testing.T) {
	testDir := t.TempDir()

	// Create .grove directory and a source rules file
	groveDir := filepath.Join(testDir, ".grove")
	if err := os.MkdirAll(groveDir, 0o755); err != nil {
		t.Fatalf("Failed to create .grove dir: %v", err)
	}

	sourceContent := "**/*.go\n---\n**/*.md\n"
	sourcePath := filepath.Join(testDir, "source.rules")
	if err := os.WriteFile(sourcePath, []byte(sourceContent), 0o644); err != nil {
		t.Fatalf("Failed to write source rules: %v", err)
	}

	mgr := NewManager(testDir)

	if err := mgr.SetActiveRules(sourcePath); err != nil {
		t.Fatalf("SetActiveRules returned error: %v", err)
	}

	// Verify the file was written (the exact path depends on ResolveRulesWritePath,
	// but in a non-plan context it should go to .grove/rules)
	writtenPath := mgr.ResolveRulesWritePath()
	content, err := os.ReadFile(writtenPath)
	if err != nil {
		t.Fatalf("Failed to read written rules file at %s: %v", writtenPath, err)
	}
	if string(content) != sourceContent {
		t.Errorf("Written content doesn't match. Expected %q, got %q", sourceContent, string(content))
	}
}

func TestAppendRule_CreatesFileAtCorrectPath(t *testing.T) {
	testDir := t.TempDir()

	// Create .grove directory (needed for NewManager)
	groveDir := filepath.Join(testDir, ".grove")
	if err := os.MkdirAll(groveDir, 0o755); err != nil {
		t.Fatalf("Failed to create .grove dir: %v", err)
	}

	// Create a test file to append as a rule
	testFile := filepath.Join(testDir, "test.go")
	if err := os.WriteFile(testFile, []byte("package main"), 0o644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	mgr := NewManager(testDir)

	if err := mgr.AppendRule("test.go", "hot"); err != nil {
		t.Fatalf("AppendRule returned error: %v", err)
	}

	// Verify the rules file was created at the correct path
	writtenPath := mgr.ResolveRulesWritePath()
	content, err := os.ReadFile(writtenPath)
	if err != nil {
		t.Fatalf("Failed to read written rules file at %s: %v", writtenPath, err)
	}
	if !strings.Contains(string(content), "test.go") {
		t.Errorf("Rules file at %s doesn't contain 'test.go'. Content: %q", writtenPath, string(content))
	}
}

func TestToggleViewDirective_CreatesFileAtCorrectPath(t *testing.T) {
	testDir := t.TempDir()

	// Create .grove directory
	groveDir := filepath.Join(testDir, ".grove")
	if err := os.MkdirAll(groveDir, 0o755); err != nil {
		t.Fatalf("Failed to create .grove dir: %v", err)
	}

	mgr := NewManager(testDir)

	if err := mgr.ToggleViewDirective("src/"); err != nil {
		t.Fatalf("ToggleViewDirective returned error: %v", err)
	}

	// Verify the rules file was created at the correct path
	writtenPath := mgr.ResolveRulesWritePath()
	content, err := os.ReadFile(writtenPath)
	if err != nil {
		t.Fatalf("Failed to read written rules file at %s: %v", writtenPath, err)
	}
	if !strings.Contains(string(content), "@view: src/") {
		t.Errorf("Rules file at %s doesn't contain '@view: src/'. Content: %q", writtenPath, string(content))
	}
}
