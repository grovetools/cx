package context

import (
	"os"
	"testing"
)

func TestManager_DiffContext(t *testing.T) {
	// Create temporary directory
	tempDir := t.TempDir()
	originalWd, _ := os.Getwd()
	_ = os.Chdir(tempDir)
	defer func() { _ = os.Chdir(originalWd) }()

	mgr := NewManager(tempDir)

	// Create .cx and .grove directories
	_ = os.MkdirAll(".cx", 0o755)
	_ = os.MkdirAll(".grove", 0o755)

	// Create test files
	_ = os.WriteFile("fileA.txt", []byte("A"), 0o644)
	_ = os.WriteFile("fileB.txt", []byte("B"), 0o644)
	_ = os.WriteFile("fileC.txt", []byte("C"), 0o644)

	// Create current rules file (.grove/rules)
	_ = os.WriteFile(".grove/rules", []byte("fileB.txt\nfileC.txt\n"), 0o644)

	// Create a named rule set to compare against (.cx/compare.rules)
	_ = os.WriteFile(".cx/compare.rules", []byte("fileA.txt\nfileB.txt\n"), 0o644)

	// Test diff against the named rule set
	diff, err := mgr.DiffContext("compare")
	if err != nil {
		t.Fatalf("Failed to diff context: %v", err)
	}

	// Check results — ResolveFilesFromRules returns relative paths, so
	// diff entries carry relative paths (not absolute).
	if len(diff.Added) != 1 || diff.Added[0].Path != "fileC.txt" {
		t.Errorf("Expected fileC.txt to be added, got: %v", diff.Added)
	}

	if len(diff.Removed) != 1 || diff.Removed[0].Path != "fileA.txt" {
		t.Errorf("Expected fileA.txt to be removed, got: %v", diff.Removed)
	}
}
