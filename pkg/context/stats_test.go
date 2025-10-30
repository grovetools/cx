package context

import (
	"os"
	"path/filepath"
	"testing"
)

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
