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

func TestManager_GetStats(t *testing.T) {
	// Create temporary directory
	tempDir := t.TempDir()
	os.Chdir(tempDir)
	defer os.Chdir("..")

	mgr := NewManager(tempDir)

	// Create .grove directory
	os.MkdirAll(GroveDir, 0755)

	// Create test files of different types
	os.WriteFile("main.go", []byte("package main\n\nfunc main() {\n\t// Main function\n}"), 0644)
	os.WriteFile("README.md", []byte("# Test Project\n\nThis is a test project."), 0644)
	os.WriteFile("config.yaml", []byte("version: 1.0\nname: test"), 0644)

	// Create rules file
	os.WriteFile(filepath.Join(GroveDir, "rules"), []byte("main.go\nREADME.md\nconfig.yaml\n"), 0644)

	// Resolve files from rules
	files, err := mgr.ResolveFilesFromRules()
	if err != nil {
		t.Fatalf("Failed to resolve files from rules: %v", err)
	}

	// Get stats
	stats, err := mgr.GetStats("hot", files, 5)
	if err != nil {
		t.Fatalf("Failed to get stats: %v", err)
	}

	// Check results
	if stats.TotalFiles != 3 {
		t.Errorf("Expected 3 total files, got %d", stats.TotalFiles)
	}

	if len(stats.Languages) != 3 {
		t.Errorf("Expected 3 languages, got %d", len(stats.Languages))
	}

	// Check language detection
	if _, ok := stats.Languages["Go"]; !ok {
		t.Error("Expected Go language to be detected")
	}

	if _, ok := stats.Languages["Markdown"]; !ok {
		t.Error("Expected Markdown language to be detected")
	}

	if _, ok := stats.Languages["YAML"]; !ok {
		t.Error("Expected YAML language to be detected")
	}

	if len(stats.LargestFiles) != 3 {
		t.Errorf("Expected 3 files in largest files, got %d", len(stats.LargestFiles))
	}

	if len(stats.Distribution) != 4 {
		t.Errorf("Expected 4 distribution ranges, got %d", len(stats.Distribution))
	}
}

func TestManager_FixContext(t *testing.T) {
	// Create temporary directory
	tempDir := t.TempDir()
	os.Chdir(tempDir)
	defer os.Chdir("..")

	mgr := NewManager(tempDir)

	// Create .grove directory
	os.MkdirAll(GroveDir, 0755)

	// FixContext is deprecated and just prints a message
	err := mgr.FixContext()
	if err != nil {
		t.Fatalf("Failed to call FixContext: %v", err)
	}

	// The function should succeed but not do anything
	// Just verify it doesn't return an error
}
