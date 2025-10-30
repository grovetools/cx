package context

import (
	"os"
	"testing"
)

func TestManager_ValidateContext(t *testing.T) {
	// Create temporary directory
	tempDir := t.TempDir()
	os.Chdir(tempDir)
	defer os.Chdir("..")

	mgr := NewManager(tempDir)

	// Create .grove directory
	os.MkdirAll(GroveDir, 0755)

	// Create test files
	os.WriteFile("existing.go", []byte("package main"), 0644)

	// Create context with existing, missing, and duplicate files
	os.WriteFile(FilesListFile, []byte("existing.go\nmissing.go\nexisting.go\n"), 0644)

	// For this test, we'll read the files list directly instead of resolving from rules
	// since we want to test validation of specific files including non-existent ones
	files, err := mgr.ReadFilesList(FilesListFile)
	if err != nil {
		t.Fatalf("Failed to read files list: %v", err)
	}

	// Validate
	result, err := mgr.ValidateContext(files)
	if err != nil {
		t.Fatalf("Failed to validate context: %v", err)
	}

	// Check results
	if result.TotalFiles != 3 {
		t.Errorf("Expected 3 total files, got %d", result.TotalFiles)
	}

	if result.AccessibleFiles != 2 {
		t.Errorf("Expected 2 accessible files, got %d", result.AccessibleFiles)
	}

	if len(result.MissingFiles) != 1 {
		t.Errorf("Expected 1 missing file, got %d", len(result.MissingFiles))
	}

	if len(result.Duplicates) != 1 {
		t.Errorf("Expected 1 duplicate, got %d", len(result.Duplicates))
	}
}
