package context

import (
	"os"
	"path/filepath"
	"testing"
)

func TestManager_DiffContext(t *testing.T) {
	// Create temporary directory
	tempDir := t.TempDir()
	originalWd, _ := os.Getwd()
	os.Chdir(tempDir)
	defer os.Chdir(originalWd)

	mgr := NewManager(tempDir)

	// Create .cx and .grove directories
	os.MkdirAll(".cx", 0755)
	os.MkdirAll(".grove", 0755)

	// Create test files
	os.WriteFile("fileA.txt", []byte("A"), 0644)
	os.WriteFile("fileB.txt", []byte("B"), 0644)
	os.WriteFile("fileC.txt", []byte("C"), 0644)

	// Create current rules file (.grove/rules)
	os.WriteFile(".grove/rules", []byte("fileB.txt\nfileC.txt\n"), 0644)

	// Create a named rule set to compare against (.cx/compare.rules)
	os.WriteFile(".cx/compare.rules", []byte("fileA.txt\nfileB.txt\n"), 0644)

	// Test diff against the named rule set
	diff, err := mgr.DiffContext("compare")
	if err != nil {
		t.Fatalf("Failed to diff context: %v", err)
	}

	// Check results
	if len(diff.Added) != 1 || diff.Added[0].Path != filepath.Join(tempDir, "fileC.txt") {
		t.Errorf("Expected fileC.txt to be added, got: %v", diff.Added)
	}

	if len(diff.Removed) != 1 || diff.Removed[0].Path != filepath.Join(tempDir, "fileA.txt") {
		t.Errorf("Expected fileA.txt to be removed, got: %v", diff.Removed)
	}
}
