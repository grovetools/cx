package context

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestGitIntegration(t *testing.T) {
	// Skip if git is not available
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	// Create temporary directory
	tempDir := t.TempDir()
	_ = os.Chdir(tempDir)
	defer func() { _ = os.Chdir("..") }()

	// Initialize git repo
	cmd := exec.Command("git", "init")
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to init git repo: %v", err)
	}

	// Configure git user for commits
	_ = exec.Command("git", "config", "user.email", "test@example.com").Run()
	_ = exec.Command("git", "config", "user.name", "Test User").Run()

	rulesFile := filepath.Join(tempDir, ".grove", "rules")
	mgr := NewManagerWithOverride(tempDir, rulesFile)

	// Create and commit some files
	_ = os.WriteFile("file1.go", []byte("package main\n// File 1"), 0o644)
	_ = os.WriteFile("file2.go", []byte("package main\n// File 2"), 0o644)
	_ = exec.Command("git", "add", "file1.go", "file2.go").Run()
	_ = exec.Command("git", "commit", "-m", "Initial commit").Run()

	// Create more files and stage them
	_ = os.WriteFile("file3.go", []byte("package main\n// File 3"), 0o644)
	_ = os.WriteFile("file4.go", []byte("package main\n// File 4"), 0o644)
	_ = exec.Command("git", "add", "file3.go").Run()

	t.Run("staged files", func(t *testing.T) {
		opts := GitOptions{Staged: true, Force: true}
		err := mgr.UpdateFromGit(opts)
		if err != nil {
			t.Fatalf("UpdateFromGit failed: %v", err)
		}

		// Check that only staged file is included in the rules file
		files := readRulesFileLines(t, rulesFile)
		if len(files) != 1 || files[0] != "file3.go" {
			t.Errorf("Expected only file3.go in context, got %v", files)
		}
	})

	// Commit the staged file
	_ = exec.Command("git", "commit", "-m", "Add file3").Run()

	t.Run("last N commits", func(t *testing.T) {
		opts := GitOptions{Commits: 1, Force: true}
		err := mgr.UpdateFromGit(opts)
		if err != nil {
			t.Fatalf("UpdateFromGit failed: %v", err)
		}

		// Check that only file from last commit is included in the rules file
		files := readRulesFileLines(t, rulesFile)
		if len(files) != 1 || files[0] != "file3.go" {
			t.Errorf("Expected only file3.go from last commit, got %v", files)
		}
	})

	// Create a branch
	_ = exec.Command("git", "checkout", "-b", "feature").Run()
	_ = os.WriteFile("feature.go", []byte("package main\n// Feature"), 0o644)
	_ = exec.Command("git", "add", "feature.go").Run()
	_ = exec.Command("git", "commit", "-m", "Add feature").Run()

	t.Run("branch comparison", func(t *testing.T) {
		opts := GitOptions{Branch: "HEAD~1..HEAD", Force: true}
		err := mgr.UpdateFromGit(opts)
		if err != nil {
			t.Fatalf("UpdateFromGit failed: %v", err)
		}

		// Check that only feature branch file is included in the rules file
		files := readRulesFileLines(t, rulesFile)
		if len(files) != 1 || files[0] != "feature.go" {
			t.Errorf("Expected only feature.go from branch, got %v", files)
		}
	})
}

func TestParseGitFileList(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "simple file list",
			input:    "file1.go\nfile2.go\nfile3.go\n",
			expected: []string{"file1.go", "file2.go", "file3.go"},
		},
		{
			name:     "with empty lines",
			input:    "file1.go\n\nfile2.go\n\n",
			expected: []string{"file1.go", "file2.go"},
		},
		{
			name:     "with paths",
			input:    "src/main.go\ninternal/app/handler.go\n",
			expected: []string{filepath.Clean("src/main.go"), filepath.Clean("internal/app/handler.go")},
		},
		{
			name:     "empty input",
			input:    "",
			expected: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseGitFileList([]byte(tt.input))

			if len(result) != len(tt.expected) {
				t.Errorf("Expected %d files, got %d", len(tt.expected), len(result))
				return
			}

			for i, file := range result {
				if file != tt.expected[i] {
					t.Errorf("Expected file %s, got %s", tt.expected[i], file)
				}
			}
		})
	}
}

// readRulesFileLines reads a rules file and returns non-empty lines.
func readRulesFileLines(t *testing.T, path string) []string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("Failed to read rules file %s: %v", path, err)
	}
	var lines []string
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			lines = append(lines, line)
		}
	}
	return lines
}

func TestCheckGitRepo(t *testing.T) {
	// Skip if git is not available
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	t.Run("not in git repo", func(t *testing.T) {
		tempDir := t.TempDir()
		_ = os.Chdir(tempDir)
		defer func() { _ = os.Chdir("..") }()

		err := checkGitRepo()
		if err == nil {
			t.Error("Expected error when not in git repo")
		}
	})

	t.Run("in git repo", func(t *testing.T) {
		tempDir := t.TempDir()
		_ = os.Chdir(tempDir)
		defer func() { _ = os.Chdir("..") }()

		// Initialize git repo
		cmd := exec.Command("git", "init")
		if err := cmd.Run(); err != nil {
			t.Fatalf("Failed to init git repo: %v", err)
		}

		err := checkGitRepo()
		if err != nil {
			t.Errorf("Expected no error in git repo, got %v", err)
		}
	})
}
