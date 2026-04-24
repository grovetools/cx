package context

import (
	"os"
	"path/filepath"
	"testing"
)

// TestGetStats_ExternalRulesFile reproduces the hosted BSP bug where:
//   - workDir = project directory (passed by flow)
//   - rulesFileOverride = external notebook plan rules file
//   - CWD ≠ workDir (groveterm launched from ~ or elsewhere)
//
// The manager resolves files correctly (relative to workDir), but
// GetStats calls filepath.Abs on relative paths, which resolves
// against CWD — producing wrong paths and 0 tokens.
func TestGetStats_ExternalRulesFile(t *testing.T) {
	// Reset manager cache so we get fresh instances
	ClearManagerCache()

	// Create the "project" directory with files
	projectDir := t.TempDir()
	os.WriteFile(filepath.Join(projectDir, "main.go"), []byte("package main\n\nfunc main() {\n\tfmt.Println(\"hello\")\n}\n"), 0o644)
	os.WriteFile(filepath.Join(projectDir, "lib.go"), []byte("package main\n\nfunc helper() string {\n\treturn \"help\"\n}\n"), 0o644)
	os.MkdirAll(filepath.Join(projectDir, ".grove"), 0o755)
	os.WriteFile(filepath.Join(projectDir, "grove.yml"), []byte("name: test-project\n"), 0o644)

	// Create an "external notebook" directory with the rules file
	notebookDir := t.TempDir()
	rulesDir := filepath.Join(notebookDir, "plans", "test-plan", "rules")
	os.MkdirAll(rulesDir, 0o755)
	rulesFile := filepath.Join(rulesDir, "job.rules")
	os.WriteFile(rulesFile, []byte("*.go\n"), 0o644)

	// Set CWD to a DIFFERENT directory (simulates groveterm from ~)
	otherDir := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(otherDir)
	defer os.Chdir(origDir)

	// Create manager the way the hosted BSP panel does:
	// workDir = project, rulesFile = external notebook path
	mgr := NewManagerWithOverride(projectDir, rulesFile)

	// Resolve files — this should find main.go and lib.go
	files, err := mgr.ResolveFilesFromRules()
	if err != nil {
		t.Fatalf("ResolveFilesFromRules failed: %v", err)
	}
	if len(files) != 2 {
		t.Fatalf("Expected 2 files, got %d: %v", len(files), files)
	}

	// Get stats — this is where the bug manifests.
	// Files are relative paths (e.g. "main.go") but GetFileStats
	// calls filepath.Abs which resolves against CWD (otherDir),
	// not projectDir. Result: 0 tokens.
	stats, err := mgr.GetStats("hot", files, 5)
	if err != nil {
		t.Fatalf("GetStats failed: %v", err)
	}

	if stats.TotalFiles != 2 {
		t.Errorf("Expected TotalFiles=2, got %d", stats.TotalFiles)
	}
	if stats.TotalTokens == 0 {
		t.Errorf("Expected non-zero TotalTokens, got 0 (CWD-relative path resolution bug)")
	}
	if len(stats.LargestFiles) == 0 {
		t.Errorf("Expected LargestFiles to be non-empty")
	}
}
