package context

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"
)

func TestManager_ExclusionPatterns(t *testing.T) {
	// Create test directory structure
	testDir := t.TempDir()
	
	// Create files in various locations including test directories
	testFiles := map[string]string{
		"main.go":                          "package main",
		"cmd/app.go":                       "package cmd",
		"cmd/app_test.go":                  "package cmd",
		"tests/unit_test.go":               "package tests",
		"tests/integration/api_test.go":    "package tests",
		"pkg/util/helper.go":               "package util",
		"pkg/tests/helper_test.go":         "package tests",
		"internal/tests/fixtures/data.go":  "package fixtures",
	}
	
	for relPath, content := range testFiles {
		fullPath := filepath.Join(testDir, relPath)
		if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
			t.Fatalf("Failed to create directory: %v", err)
		}
		if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to write file %s: %v", relPath, err)
		}
	}
	
	// Create .grove directory
	groveDir := filepath.Join(testDir, ".grove")
	if err := os.MkdirAll(groveDir, 0755); err != nil {
		t.Fatalf("Failed to create .grove directory: %v", err)
	}
	
	tests := []struct {
		name     string
		rules    string
		expected []string
	}{
		{
			name: "exclude with !tests pattern (gitignore compatible)",
			rules: `*.go
!tests`,
			expected: []string{
				"main.go",
				"cmd/app.go",
				"cmd/app_test.go",
				"pkg/util/helper.go",
			},
		},
		{
			name: "exclude with !**/tests/** pattern",
			rules: `**/*.go
!**/tests/**`,
			expected: []string{
				"main.go",
				"cmd/app.go",
				"cmd/app_test.go",
				"pkg/util/helper.go",
			},
		},
		{
			name: "exclude test files with !*_test.go",
			rules: `**/*.go
!*_test.go`,
			expected: []string{
				"main.go",
				"cmd/app.go",
				"pkg/util/helper.go",
				"internal/tests/fixtures/data.go",
			},
		},
		{
			name: "multiple exclusion patterns",
			rules: `**/*.go
!tests
!*_test.go`,
			expected: []string{
				"main.go",
				"cmd/app.go",
				"pkg/util/helper.go",
			},
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Write rules file
			rulesPath := filepath.Join(groveDir, "rules")
			if err := os.WriteFile(rulesPath, []byte(tt.rules), 0644); err != nil {
				t.Fatalf("Failed to write rules file: %v", err)
			}
			
			// Create manager and resolve files
			mgr := NewManager(testDir)
			files, err := mgr.ResolveFilesFromRules()
			if err != nil {
				t.Fatalf("Failed to resolve files: %v", err)
			}
			
			// Sort for consistent comparison
			sort.Strings(files)
			sort.Strings(tt.expected)
			
			// Compare results
			if len(files) != len(tt.expected) {
				t.Errorf("Expected %d files, got %d\nExpected: %v\nGot: %v",
					len(tt.expected), len(files), tt.expected, files)
				return
			}
			
			for i, expected := range tt.expected {
				if files[i] != expected {
					t.Errorf("File mismatch at index %d: expected %s, got %s",
						i, expected, files[i])
				}
			}
		})
	}
}

func TestManager_CrossDirectoryExclusions(t *testing.T) {
	// This tests the specific case where we have patterns like ../other-project/**/*.go
	// with exclusions that should apply to those external paths
	
	// Create two sibling directories
	tempParent := t.TempDir()
	projectDir := filepath.Join(tempParent, "main-project")
	siblingDir := filepath.Join(tempParent, "sibling-project")
	
	// Create directories
	for _, dir := range []string{projectDir, siblingDir} {
		if err := os.MkdirAll(filepath.Join(dir, ".grove"), 0755); err != nil {
			t.Fatalf("Failed to create directory: %v", err)
		}
	}
	
	// Create files in sibling project
	siblingFiles := map[string]string{
		"main.go":                          "package main",
		"cmd/app.go":                       "package cmd",
		"tests/unit_test.go":               "package tests",
		"tests/e2e/api_test.go":            "package e2e",
		"pkg/core/logic.go":                "package core",
		"pkg/core/logic_test.go":           "package core",
		"internal/tests/fixtures/data.go":  "package fixtures",
	}
	
	for relPath, content := range siblingFiles {
		fullPath := filepath.Join(siblingDir, relPath)
		if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
			t.Fatalf("Failed to create directory: %v", err)
		}
		if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to write file %s: %v", relPath, err)
		}
	}
	
	// Test cases
	tests := []struct {
		name     string
		rules    string
		expected []string
	}{
		{
			name: "cross-directory with !tests exclusion",
			rules: `../sibling-project/**/*.go
!tests`,
			expected: []string{
				"../sibling-project/main.go",
				"../sibling-project/cmd/app.go",
				"../sibling-project/pkg/core/logic.go",
				"../sibling-project/pkg/core/logic_test.go",
			},
		},
		{
			name: "cross-directory with !**/tests/** exclusion",
			rules: `../sibling-project/**/*.go
!**/tests/**`,
			expected: []string{
				"../sibling-project/main.go",
				"../sibling-project/cmd/app.go",
				"../sibling-project/pkg/core/logic.go",
				"../sibling-project/pkg/core/logic_test.go",
			},
		},
		{
			name: "cross-directory with multiple exclusions",
			rules: `../sibling-project/**/*.go
!**/tests/**
!*_test.go`,
			expected: []string{
				"../sibling-project/main.go",
				"../sibling-project/cmd/app.go",
				"../sibling-project/pkg/core/logic.go",
			},
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Write rules file
			rulesPath := filepath.Join(projectDir, ".grove", "rules")
			if err := os.WriteFile(rulesPath, []byte(tt.rules), 0644); err != nil {
				t.Fatalf("Failed to write rules file: %v", err)
			}
			
			// Create manager and resolve files
			mgr := NewManager(projectDir)
			files, err := mgr.ResolveFilesFromRules()
			if err != nil {
				t.Fatalf("Failed to resolve files: %v", err)
			}
			
			// Sort for consistent comparison
			sort.Strings(files)
			sort.Strings(tt.expected)
			
			// Compare results - need to handle both relative and absolute paths
			if len(files) != len(tt.expected) {
				t.Errorf("Expected %d files, got %d", len(tt.expected), len(files))
				t.Errorf("Expected: %v", tt.expected)
				t.Errorf("Got: %v", files)
				return
			}
			
			// Check that all expected files are present (handling path differences)
			found := make(map[string]bool)
			for _, expected := range tt.expected {
				for _, got := range files {
					// Check various ways the paths might match
					if got == expected || 
					   strings.HasSuffix(got, expected) ||
					   strings.HasSuffix(got, strings.TrimPrefix(expected, "../")) {
						found[expected] = true
						break
					}
				}
				if !found[expected] {
					t.Errorf("Expected file not found: %s (got files: %v)", expected, files)
				}
			}
		})
	}
}

func TestMatchDoubleStarPattern(t *testing.T) {
	tests := []struct {
		pattern string
		path    string
		want    bool
	}{
		// Basic ** patterns
		{"**/*.go", "main.go", true},
		{"**/*.go", "cmd/app.go", true},
		{"**/*.go", "deep/nested/path/file.go", true},
		{"**/*.go", "file.txt", false},
		
		// Patterns with prefix
		{"src/**/*.go", "src/main.go", true},
		{"src/**/*.go", "src/cmd/app.go", true},
		{"src/**/*.go", "main.go", false},
		{"src/**/*.go", "other/main.go", false},
		
		// Patterns with complex suffix
		{"**/tests/*.go", "tests/unit.go", true},
		{"**/tests/*.go", "src/tests/unit.go", true},
		{"**/tests/*.go", "src/tests/e2e/api.go", false}, // Too deep
		
		// Special case: **/dir/** patterns
		{"**/tests/**", "tests/unit.go", true},
		{"**/tests/**", "src/tests/unit.go", true},
		{"**/tests/**", "src/tests/e2e/api.go", true},
		{"**/tests/**", "src/testing/api.go", false},
		{"**/tests/**", "../project/tests/unit.go", true},
		
		// Edge cases
		{"**", "anything", true},
		{"**", "deep/nested/path", true},
		{"**/", "dir/", true},
		{"**.go", "file.go", false}, // Invalid pattern, falls back to literal match
	}
	
	for _, tt := range tests {
		t.Run(tt.pattern+"_"+tt.path, func(t *testing.T) {
			got := matchDoubleStarPattern(tt.pattern, tt.path)
			if got != tt.want {
				t.Errorf("matchDoubleStarPattern(%q, %q) = %v, want %v",
					tt.pattern, tt.path, got, tt.want)
			}
		})
	}
}

// Add this test to the existing test file
func TestManager_GitignoreCompatibility(t *testing.T) {
	// Test that our patterns behave like gitignore
	testDir := t.TempDir()
	
	// Create test structure
	files := []string{
		"main.go",
		"test.go",
		"tests/unit.go",
		"src/tests/integration.go",
		"testdata/sample.go",
		"contest/solution.go",
	}
	
	for _, relPath := range files {
		fullPath := filepath.Join(testDir, relPath)
		if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
			t.Fatalf("Failed to create directory: %v", err)
		}
		if err := os.WriteFile(fullPath, []byte("package main"), 0644); err != nil {
			t.Fatalf("Failed to write file: %v", err)
		}
	}
	
	// Create .grove directory
	groveDir := filepath.Join(testDir, ".grove")
	if err := os.MkdirAll(groveDir, 0755); err != nil {
		t.Fatalf("Failed to create .grove directory: %v", err)
	}
	
	tests := []struct {
		name     string
		rules    string
		expected []string
	}{
		{
			name: "pattern without slash matches at any level",
			rules: `**/*.go
!test`,
			expected: []string{
				"main.go",
				"test.go", // NOT excluded - filename is "test.go" not "test"
				"tests/unit.go",
				"src/tests/integration.go",
				"testdata/sample.go",
				"contest/solution.go",
			},
		},
		{
			name: "pattern without slash matches directories",
			rules: `**/*.go
!tests`,
			expected: []string{
				"main.go",
				"test.go",
				"testdata/sample.go",
				"contest/solution.go",
			},
		},
		{
			name: "wildcard patterns",
			rules: `**/*.go
!test*`,
			expected: []string{
				"main.go",
				"contest/solution.go",
			},
		},
		{
			name: "partial name matching",
			rules: `**/*.go
!*test*`,
			expected: []string{
				"main.go",
			},
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Write rules file
			rulesPath := filepath.Join(groveDir, "rules")
			if err := os.WriteFile(rulesPath, []byte(tt.rules), 0644); err != nil {
				t.Fatalf("Failed to write rules file: %v", err)
			}
			
			// Create manager and resolve files
			mgr := NewManager(testDir)
			files, err := mgr.ResolveFilesFromRules()
			if err != nil {
				t.Fatalf("Failed to resolve files: %v", err)
			}
			
			// Sort for comparison
			sort.Strings(files)
			sort.Strings(tt.expected)
			
			// Compare
			if !slicesEqual(files, tt.expected) {
				t.Errorf("Pattern %q: expected %v, got %v", 
					strings.Split(tt.rules, "\n")[1], tt.expected, files)
			}
		})
	}
}

func slicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestManager_ParseDirectives(t *testing.T) {
	// Create test directory
	testDir := t.TempDir()
	groveDir := filepath.Join(testDir, ".grove")
	if err := os.MkdirAll(groveDir, 0755); err != nil {
		t.Fatalf("Failed to create .grove directory: %v", err)
	}
	
	tests := []struct {
		name              string
		rules             string
		expectFreeze      bool
		expectNoExpire    bool
		expectDisableCache bool
		expectExpireTime  time.Duration
		expectError       bool
	}{
		{
			name: "no directives",
			rules: `*.go
pkg/**/*.go`,
			expectFreeze:      false,
			expectNoExpire:    false,
			expectDisableCache: false,
			expectExpireTime:  0,
		},
		{
			name: "only @freeze-cache",
			rules: `@freeze-cache
*.go
pkg/**/*.go`,
			expectFreeze:      true,
			expectNoExpire:    false,
			expectDisableCache: false,
			expectExpireTime:  0,
		},
		{
			name: "only @no-expire",
			rules: `@no-expire
*.go
pkg/**/*.go`,
			expectFreeze:      false,
			expectNoExpire:    true,
			expectDisableCache: false,
			expectExpireTime:  0,
		},
		{
			name: "only @expire-time with valid duration",
			rules: `@expire-time 24h
*.go
pkg/**/*.go`,
			expectFreeze:      false,
			expectNoExpire:    false,
			expectDisableCache: false,
			expectExpireTime:  24 * time.Hour,
		},
		{
			name: "multiple time formats",
			rules: `@expire-time 1h30m
*.go
pkg/**/*.go`,
			expectFreeze:      false,
			expectNoExpire:    false,
			expectDisableCache: false,
			expectExpireTime:  90 * time.Minute,
		},
		{
			name: "@expire-time with seconds",
			rules: `@expire-time 300s
*.go
pkg/**/*.go`,
			expectFreeze:      false,
			expectNoExpire:    false,
			expectDisableCache: false,
			expectExpireTime:  300 * time.Second,
		},
		{
			name: "all directives combined",
			rules: `@freeze-cache
@no-expire
@expire-time 48h
*.go
pkg/**/*.go`,
			expectFreeze:      true,
			expectNoExpire:    true,
			expectDisableCache: false,
			expectExpireTime:  48 * time.Hour,
		},
		{
			name: "directives with cold section",
			rules: `@freeze-cache
@no-expire
@expire-time 12h
*.go
---
pkg/**/*.go`,
			expectFreeze:      true,
			expectNoExpire:    true,
			expectDisableCache: false,
			expectExpireTime:  12 * time.Hour,
		},
		{
			name: "@expire-time with invalid duration",
			rules: `@expire-time invalid
*.go
pkg/**/*.go`,
			expectError: true,
		},
		{
			name: "@expire-time with no argument",
			rules: `@expire-time
*.go
pkg/**/*.go`,
			expectFreeze:      false,
			expectNoExpire:    false,
			expectDisableCache: false,
			expectExpireTime:  0,
		},
		{
			name: "only @disable-cache",
			rules: `@disable-cache
*.go
pkg/**/*.go`,
			expectFreeze:      false,
			expectNoExpire:    false,
			expectDisableCache: true,
			expectExpireTime:  0,
		},
		{
			name: "@disable-cache with other directives",
			rules: `@freeze-cache
@no-expire
@disable-cache
@expire-time 6h
*.go
pkg/**/*.go`,
			expectFreeze:      true,
			expectNoExpire:    true,
			expectDisableCache: true,
			expectExpireTime:  6 * time.Hour,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Write rules file
			rulesPath := filepath.Join(groveDir, "rules")
			if err := os.WriteFile(rulesPath, []byte(tt.rules), 0644); err != nil {
				t.Fatalf("Failed to write rules file: %v", err)
			}
			
			// Create manager and check directives
			mgr := NewManager(testDir)
			
			// Check if we expect an error
			if tt.expectError {
				_, err := mgr.GetExpireTime()
				if err == nil {
					t.Errorf("Expected error for invalid duration, but got none")
				}
				return
			}
			
			freezeCache, err := mgr.ShouldFreezeCache()
			if err != nil {
				t.Fatalf("Failed to check freeze cache directive: %v", err)
			}
			if freezeCache != tt.expectFreeze {
				t.Errorf("Expected ShouldFreezeCache to return %v, got %v", tt.expectFreeze, freezeCache)
			}
			
			disableExpiration, err := mgr.ShouldDisableExpiration()
			if err != nil {
				t.Fatalf("Failed to check no-expire directive: %v", err)
			}
			if disableExpiration != tt.expectNoExpire {
				t.Errorf("Expected ShouldDisableExpiration to return %v, got %v", tt.expectNoExpire, disableExpiration)
			}
			
			disableCache, err := mgr.ShouldDisableCache()
			if err != nil {
				t.Fatalf("Failed to check disable-cache directive: %v", err)
			}
			if disableCache != tt.expectDisableCache {
				t.Errorf("Expected ShouldDisableCache to return %v, got %v", tt.expectDisableCache, disableCache)
			}
			
			expireTime, err := mgr.GetExpireTime()
			if err != nil {
				t.Fatalf("Failed to get expire time: %v", err)
			}
			if expireTime != tt.expectExpireTime {
				t.Errorf("Expected GetExpireTime to return %v, got %v", tt.expectExpireTime, expireTime)
			}
		})
	}
}
