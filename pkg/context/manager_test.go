package context

import (
	"os"
	"strings"
	"testing"
)

func TestManager_ReadFilesList(t *testing.T) {
	// Create temporary directory
	tempDir := t.TempDir()
	os.Chdir(tempDir)
	defer os.Chdir("..")

	mgr := NewManager(tempDir)

	// Test with non-existent file
	_, err := mgr.ReadFilesList("nonexistent.txt")
	if err == nil {
		t.Error("Expected error for non-existent file")
	}

	// Create test file
	testFile := "test-files.txt"
	content := `# Comment line
file1.go
file2.go

# Another comment
file3.go
`
	os.WriteFile(testFile, []byte(content), 0644)

	// Test reading
	files, err := mgr.ReadFilesList(testFile)
	if err != nil {
		t.Fatalf("Failed to read files list: %v", err)
	}

	expected := []string{"file1.go", "file2.go", "file3.go"}
	if len(files) != len(expected) {
		t.Errorf("Expected %d files, got %d", len(expected), len(files))
	}

	for i, file := range files {
		if file != expected[i] {
			t.Errorf("Expected file %s, got %s", expected[i], file)
		}
	}
}

func TestManager_GenerateContext(t *testing.T) {
	// Create temporary directory
	tempDir := t.TempDir()
	os.Chdir(tempDir)
	defer os.Chdir("..")

	mgr := NewManager(tempDir)

	// Create .grove directory
	os.MkdirAll(GroveDir, 0755)

	// Create test files
	os.WriteFile("file1.go", []byte("package main\n// File 1"), 0644)
	os.WriteFile("file2.go", []byte("package main\n// File 2"), 0644)

	// Create rules file
	os.WriteFile(ActiveRulesFile, []byte("file1.go\nfile2.go\n"), 0644)

	// Generate context with XML format
	err := mgr.GenerateContext(true)
	if err != nil {
		t.Fatalf("Failed to generate context: %v", err)
	}

	// Check context file was created
	content, err := os.ReadFile(ContextFile)
	if err != nil {
		t.Fatalf("Failed to read context file: %v", err)
	}

	// Check for XML tags
	expectedTags := []string{
		`<file path="file1.go">`,
		`<file path="file2.go">`,
		`</file>`,
	}

	for _, tag := range expectedTags {
		if !strings.Contains(string(content), tag) {
			t.Errorf("Expected tag %q not found in context", tag)
		}
	}
}

func TestManager_UpdateFromRules(t *testing.T) {
	// Create temporary directory
	tempDir := t.TempDir()
	os.Chdir(tempDir)
	defer os.Chdir("..")

	mgr := NewManager(tempDir)

	// Create test files
	os.WriteFile("main.go", []byte("package main"), 0644)
	os.WriteFile("test.go", []byte("package main"), 0644)
	os.WriteFile("test_test.go", []byte("package main"), 0644)
	os.WriteFile("README.md", []byte("# Test"), 0644)

	// Create rules file with include and exclude patterns
	rules := `*.go
!*_test.go
README.md`
	os.WriteFile(RulesFile, []byte(rules), 0644)

	// Update from rules
	err := mgr.UpdateFromRules()
	if err != nil {
		t.Fatalf("Failed to update from rules: %v", err)
	}

	// Resolve files from rules
	files, err := mgr.ResolveFilesFromRules()
	if err != nil {
		t.Fatalf("Failed to resolve files from rules: %v", err)
	}

	// Should include main.go and test.go but not test_test.go
	expectedFiles := map[string]bool{
		"main.go":   true,
		"test.go":   true,
		"README.md": true,
	}

	if len(files) != len(expectedFiles) {
		t.Errorf("Expected %d files, got %d", len(expectedFiles), len(files))
	}

	for _, file := range files {
		if !expectedFiles[file] {
			t.Errorf("Unexpected file in list: %s", file)
		}
		delete(expectedFiles, file)
	}

	for file := range expectedFiles {
		t.Errorf("Expected file not found: %s", file)
	}
}

func TestManager_Snapshots(t *testing.T) {
	// Create temporary directory
	tempDir := t.TempDir()
	os.Chdir(tempDir)
	defer os.Chdir("..")

	mgr := NewManager(tempDir)

	// Create .grove directory
	os.MkdirAll(GroveDir, 0755)

	// Create initial rules file
	filesList := []string{"file1.go", "file2.go"}
	mgr.WriteFilesList(ActiveRulesFile, filesList)

	// Save snapshot
	err := mgr.SaveSnapshot("test-snapshot", "Test snapshot description")
	if err != nil {
		t.Fatalf("Failed to save snapshot: %v", err)
	}

	// Modify rules file
	newFilesList := []string{"file3.go", "file4.go", "file5.go"}
	mgr.WriteFilesList(ActiveRulesFile, newFilesList)

	// Load snapshot
	err = mgr.LoadSnapshot("test-snapshot")
	if err != nil {
		t.Fatalf("Failed to load snapshot: %v", err)
	}

	// Verify rules file was restored
	files, err := mgr.ReadFilesList(ActiveRulesFile)
	if err != nil {
		t.Fatalf("Failed to read rules file: %v", err)
	}

	if len(files) != len(filesList) {
		t.Errorf("Expected %d files, got %d", len(filesList), len(files))
	}

	// List snapshots
	snapshots, err := mgr.ListSnapshots()
	if err != nil {
		t.Fatalf("Failed to list snapshots: %v", err)
	}

	if len(snapshots) != 1 {
		t.Errorf("Expected 1 snapshot, got %d", len(snapshots))
	}

	if snapshots[0].Name != "test-snapshot" {
		t.Errorf("Expected snapshot name 'test-snapshot', got '%s'", snapshots[0].Name)
	}

	if snapshots[0].Description != "Test snapshot description" {
		t.Errorf("Expected description 'Test snapshot description', got '%s'", snapshots[0].Description)
	}
}

func TestFormatTokenCount(t *testing.T) {
	tests := []struct {
		tokens   int
		expected string
	}{
		{100, "100"},
		{999, "999"},
		{1000, "1.0k"},
		{1500, "1.5k"},
		{10000, "10.0k"},
		{999999, "1000.0k"},
		{1000000, "1.0M"},
		{1500000, "1.5M"},
	}

	for _, tt := range tests {
		result := FormatTokenCount(tt.tokens)
		if result != tt.expected {
			t.Errorf("FormatTokenCount(%d) = %s; want %s", tt.tokens, result, tt.expected)
		}
	}
}

func TestFormatBytes(t *testing.T) {
	tests := []struct {
		bytes    int
		expected string
	}{
		{100, "100 bytes"},
		{1023, "1023 bytes"},
		{1024, "1.0 KB"},
		{1536, "1.5 KB"},
		{1048576, "1.0 MB"},
		{1572864, "1.5 MB"},
		{1073741824, "1.0 GB"},
	}

	for _, tt := range tests {
		result := FormatBytes(tt.bytes)
		if result != tt.expected {
			t.Errorf("FormatBytes(%d) = %s; want %s", tt.bytes, result, tt.expected)
		}
	}
}