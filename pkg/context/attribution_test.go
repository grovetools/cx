package context

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveFilesWithAttribution_Exclusions(t *testing.T) {
	// Create test directory structure
	testDir := t.TempDir()

	// Create test files
	testFiles := map[string]string{
		"main.go":      "package main",
		"main_test.go": "package main",
		"helper.go":    "package main",
		"util_test.go": "package main",
		"README.md":    "# README",
	}

	for relPath, content := range testFiles {
		fullPath := filepath.Join(testDir, relPath)
		if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to write file %s: %v", relPath, err)
		}
	}

	// Create manager
	mgr := NewManager(testDir)

	// Test case: include all .go files, then exclude *_test.go
	rulesContent := `**/*.go
!*_test.go`

	attribution, rules, exclusions, err := mgr.ResolveFilesWithAttribution(rulesContent)
	if err != nil {
		t.Fatalf("ResolveFilesWithAttribution failed: %v", err)
	}

	// Verify we got the expected rules
	if len(rules) != 2 {
		t.Fatalf("Expected 2 rules, got %d", len(rules))
	}

	// First rule should be the inclusion pattern
	if rules[0].Pattern != "**/*.go" || rules[0].IsExclude {
		t.Errorf("Expected first rule to be '**/*.go' (inclusion), got '%s' (exclude=%v)", rules[0].Pattern, rules[0].IsExclude)
	}

	// Second rule should be the exclusion pattern
	if rules[1].Pattern != "*_test.go" || !rules[1].IsExclude {
		t.Errorf("Expected second rule to be '*_test.go' (exclusion), got '%s' (exclude=%v)", rules[1].Pattern, rules[1].IsExclude)
	}

	// Verify exclusions - should have 2 files excluded by line 2
	expectedExclusionCount := 2 // main_test.go and util_test.go
	if excludedFiles, ok := exclusions[2]; !ok || len(excludedFiles) != expectedExclusionCount {
		t.Errorf("Expected %d files excluded by line 2, got %d (ok=%v)", expectedExclusionCount, len(excludedFiles), ok)
	}

	// Verify attribution - should have 2 files included by line 1
	expectedInclusionCount := 2 // main.go and helper.go
	if files, ok := attribution[1]; !ok || len(files) != expectedInclusionCount {
		t.Errorf("Expected %d files attributed to line 1, got %d (ok=%v)", expectedInclusionCount, len(files), ok)
	}

	// Verify the included files are correct
	if files, ok := attribution[1]; ok {
		foundMain := false
		foundHelper := false
		for _, file := range files {
			base := filepath.Base(file)
			if base == "main.go" {
				foundMain = true
			}
			if base == "helper.go" {
				foundHelper = true
			}
		}
		if !foundMain || !foundHelper {
			t.Errorf("Expected to find main.go and helper.go in attribution, found: %v", files)
		}
	}
}

func TestResolveFilesWithAttribution_NoExclusions(t *testing.T) {
	// Create test directory structure
	testDir := t.TempDir()

	// Create test files
	testFiles := map[string]string{
		"main.go":   "package main",
		"helper.go": "package main",
	}

	for relPath, content := range testFiles {
		fullPath := filepath.Join(testDir, relPath)
		if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to write file %s: %v", relPath, err)
		}
	}

	// Create manager
	mgr := NewManager(testDir)

	// Test case: only inclusion pattern, no exclusions
	rulesContent := `**/*.go`

	attribution, rules, exclusions, err := mgr.ResolveFilesWithAttribution(rulesContent)
	if err != nil {
		t.Fatalf("ResolveFilesWithAttribution failed: %v", err)
	}

	// Verify we got the expected rules
	if len(rules) != 1 {
		t.Fatalf("Expected 1 rule, got %d", len(rules))
	}

	// Verify no exclusions
	if len(exclusions) != 0 {
		t.Errorf("Expected 0 exclusions, got %d", len(exclusions))
	}

	// Verify attribution - should have 2 files included
	if files, ok := attribution[1]; !ok || len(files) != 2 {
		t.Errorf("Expected 2 files attributed to line 1, got %d (ok=%v)", len(files), ok)
	}
}

func TestResolveFilesWithAttribution_OnlyExclusions(t *testing.T) {
	// Create test directory structure
	testDir := t.TempDir()

	// Create test files
	testFiles := map[string]string{
		"main.go":      "package main",
		"main_test.go": "package main",
	}

	for relPath, content := range testFiles {
		fullPath := filepath.Join(testDir, relPath)
		if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to write file %s: %v", relPath, err)
		}
	}

	// Create manager
	mgr := NewManager(testDir)

	// Test case: only exclusion pattern (should result in no files)
	rulesContent := `!*_test.go`

	attribution, rules, exclusions, err := mgr.ResolveFilesWithAttribution(rulesContent)
	if err != nil {
		t.Fatalf("ResolveFilesWithAttribution failed: %v", err)
	}

	// Verify we got the expected rules
	if len(rules) != 1 {
		t.Fatalf("Expected 1 rule, got %d", len(rules))
	}

	// When there are no inclusion patterns, potentialFiles should be empty
	// so exclusions should be 0
	if len(exclusions) != 0 {
		t.Errorf("Expected 0 exclusions with no inclusion patterns, got %d", len(exclusions))
	}

	// Verify no files attributed
	if len(attribution) != 0 {
		t.Errorf("Expected 0 files in attribution, got %d", len(attribution))
	}
}
