package context

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestResolveFilesWithAttribution_RespectsGitignore(t *testing.T) {
	// Skip if git is not available
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	// Create test directory structure
	testDir := t.TempDir()

	// Initialize git repo
	cmd := exec.Command("git", "init")
	cmd.Dir = testDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to init git repo: %v", err)
	}

	// Create .gitignore
	gitignoreContent := `node_modules/
dist/
`
	if err := os.WriteFile(filepath.Join(testDir, ".gitignore"), []byte(gitignoreContent), 0644); err != nil {
		t.Fatalf("Failed to write .gitignore: %v", err)
	}

	// Create test files
	if err := os.WriteFile(filepath.Join(testDir, "main.go"), []byte("package main"), 0644); err != nil {
		t.Fatalf("Failed to write main.go: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(testDir, "node_modules", "dep"), 0755); err != nil {
		t.Fatalf("Failed to create node_modules dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(testDir, "node_modules", "dep", "index.js"), []byte("console.log('ignored')"), 0644); err != nil {
		t.Fatalf("Failed to write ignored file: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(testDir, "dist"), 0755); err != nil {
		t.Fatalf("Failed to create dist dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(testDir, "dist", "bundle.js"), []byte("var app;"), 0644); err != nil {
		t.Fatalf("Failed to write ignored file in dist: %v", err)
	}

	// Create manager
	mgr := NewManager(testDir)

	// Broad rule that should be filtered by gitignore
	rulesContent := `**/*`

	attribution, _, exclusions, _, err := mgr.ResolveFilesWithAttribution(rulesContent)
	if err != nil {
		t.Fatalf("ResolveFilesWithAttribution failed: %v", err)
	}

	// Verify attribution
	// Only main.go and .gitignore should be included. `resolveFilesFromPatterns` doesn't ignore .gitignore itself.
	assert.NotNil(t, attribution[1], "Attribution for line 1 should not be nil")

	includedFiles := attribution[1]

	// Check that main.go is included
	foundMainGo := false
	for _, file := range includedFiles {
		if filepath.Base(file) == "main.go" {
			foundMainGo = true
			break
		}
	}
	assert.True(t, foundMainGo, "Expected main.go to be included")

	// Check that gitignored files are NOT included
	for _, file := range includedFiles {
		if strings.Contains(file, "node_modules") {
			t.Errorf("Gitignored file from node_modules was found in attribution: %s", file)
		}
		if strings.Contains(file, "dist") {
			t.Errorf("Gitignored file from dist was found in attribution: %s", file)
		}
	}

	// .gitignore itself is usually not ignored by git ls-files, so it might be included by **/*
	// We expect 2 files: main.go and .gitignore
	assert.Len(t, includedFiles, 2, "Expected 2 files in attribution (main.go, .gitignore)")

	// Verify exclusions are empty because gitignored files should never be considered "potential"
	assert.Empty(t, exclusions, "Exclusions map should be empty for gitignored files")
}

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

	attribution, rules, exclusions, _, err := mgr.ResolveFilesWithAttribution(rulesContent)
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

	attribution, rules, exclusions, _, err := mgr.ResolveFilesWithAttribution(rulesContent)
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

	attribution, rules, exclusions, _, err := mgr.ResolveFilesWithAttribution(rulesContent)
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

func TestResolveFilesWithAttribution_StarPatternRespectsGitignore(t *testing.T) {
	// Skip if git is not available
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	// Create test directory structure
	testDir := t.TempDir()

	// Initialize git repo
	cmd := exec.Command("git", "init")
	cmd.Dir = testDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to init git repo: %v", err)
	}

	// Create .gitignore with common directories
	gitignoreContent := `node_modules
dist
coverage
`
	if err := os.WriteFile(filepath.Join(testDir, ".gitignore"), []byte(gitignoreContent), 0644); err != nil {
		t.Fatalf("Failed to write .gitignore: %v", err)
	}

	// Create test files
	if err := os.WriteFile(filepath.Join(testDir, "main.go"), []byte("package main"), 0644); err != nil {
		t.Fatalf("Failed to write main.go: %v", err)
	}

	// Create gitignored directories
	if err := os.MkdirAll(filepath.Join(testDir, "node_modules"), 0755); err != nil {
		t.Fatalf("Failed to create node_modules dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(testDir, "node_modules", "package.json"), []byte("{}"), 0644); err != nil {
		t.Fatalf("Failed to write file in node_modules: %v", err)
	}

	if err := os.MkdirAll(filepath.Join(testDir, "dist"), 0755); err != nil {
		t.Fatalf("Failed to create dist dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(testDir, "dist", "bundle.js"), []byte("var app;"), 0644); err != nil {
		t.Fatalf("Failed to write file in dist: %v", err)
	}

	if err := os.MkdirAll(filepath.Join(testDir, "coverage"), 0755); err != nil {
		t.Fatalf("Failed to create coverage dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(testDir, "coverage", "lcov.info"), []byte("data"), 0644); err != nil {
		t.Fatalf("Failed to write file in coverage: %v", err)
	}

	// Create manager
	mgr := NewManager(testDir)

	// Use * pattern which should respect .gitignore
	rulesContent := `*`

	attribution, _, _, _, err := mgr.ResolveFilesWithAttribution(rulesContent)
	if err != nil {
		t.Fatalf("ResolveFilesWithAttribution failed: %v", err)
	}

	// Verify attribution - * should match files but respect gitignore
	assert.NotNil(t, attribution[1], "Attribution for line 1 should not be nil")
	includedFiles := attribution[1]

	// Check that main.go is included
	foundMainGo := false
	for _, file := range includedFiles {
		if filepath.Base(file) == "main.go" {
			foundMainGo = true
		}
	}
	assert.True(t, foundMainGo, "Expected main.go to be included by * pattern")

	// Verify gitignored directories are NOT included
	for _, file := range includedFiles {
		if strings.Contains(file, "node_modules") {
			t.Errorf("* pattern should not include gitignored node_modules: %s", file)
		}
		if strings.Contains(file, "dist") {
			t.Errorf("* pattern should not include gitignored dist: %s", file)
		}
		if strings.Contains(file, "coverage") {
			t.Errorf("* pattern should not include gitignored coverage: %s", file)
		}
	}
}
