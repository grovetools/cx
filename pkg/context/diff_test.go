package context

import (
	"os"
	"path/filepath"
	"testing"
)

func TestManager_DiffContext(t *testing.T) {
	// Create temporary directory
	tempDir := t.TempDir()
	os.Chdir(tempDir)
	defer os.Chdir("..")

	mgr := NewManager(tempDir)

	// Create .grove directory structure
	os.MkdirAll(SnapshotsDir, 0755)

	// Create test files
	os.WriteFile("file1.go", []byte("package main\n// Test file 1"), 0644)
	os.WriteFile("file2.go", []byte("package main\n// Test file 2"), 0644)
	os.WriteFile("file3.go", []byte("package main\n// Test file 3"), 0644)

	// Create current rules file
	os.WriteFile(filepath.Join(GroveDir, "rules"), []byte("file1.go\nfile2.go\n"), 0644)

	// Create a snapshot with different files
	os.WriteFile(filepath.Join(SnapshotsDir, "test-snapshot"), []byte("file2.go\nfile3.go\n"), 0644)

	// Test diff
	diff, err := mgr.DiffContext("test-snapshot")
	if err != nil {
		t.Fatalf("Failed to diff context: %v", err)
	}

	// Check results
	if len(diff.Added) != 1 || diff.Added[0].Path != "file1.go" {
		t.Errorf("Expected file1.go to be added")
	}

	if len(diff.Removed) != 1 || diff.Removed[0].Path != "file3.go" {
		t.Errorf("Expected file3.go to be removed")
	}

	if len(diff.CurrentFiles) != 2 {
		t.Errorf("Expected 2 current files, got %d", len(diff.CurrentFiles))
	}

	if len(diff.CompareFiles) != 2 {
		t.Errorf("Expected 2 compare files, got %d", len(diff.CompareFiles))
	}
}
