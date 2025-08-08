package context

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestGitIntegration(t *testing.T) {
	// Skip if git is not available
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	// Create temporary directory
	tempDir := t.TempDir()
	os.Chdir(tempDir)
	defer os.Chdir("..")

	// Initialize git repo
	cmd := exec.Command("git", "init")
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to init git repo: %v", err)
	}

	// Configure git user for commits
	exec.Command("git", "config", "user.email", "test@example.com").Run()
	exec.Command("git", "config", "user.name", "Test User").Run()

	mgr := NewManager(tempDir)

	// Create and commit some files
	os.WriteFile("file1.go", []byte("package main\n// File 1"), 0644)
	os.WriteFile("file2.go", []byte("package main\n// File 2"), 0644)
	exec.Command("git", "add", "file1.go", "file2.go").Run()
	exec.Command("git", "commit", "-m", "Initial commit").Run()

	// Create more files and stage them
	os.WriteFile("file3.go", []byte("package main\n// File 3"), 0644)
	os.WriteFile("file4.go", []byte("package main\n// File 4"), 0644)
	exec.Command("git", "add", "file3.go").Run()

	t.Run("staged files", func(t *testing.T) {
		opts := GitOptions{Staged: true}
		err := mgr.UpdateFromGit(opts)
		if err != nil {
			t.Fatalf("UpdateFromGit failed: %v", err)
		}

		// Check that only staged file is included
		files, err := mgr.ResolveFilesFromRules()
		if err != nil {
			t.Fatalf("Failed to resolve files from rules: %v", err)
		}

		if len(files) != 1 || files[0] != "file3.go" {
			t.Errorf("Expected only file3.go in context, got %v", files)
		}
	})

	// Commit the staged file
	exec.Command("git", "commit", "-m", "Add file3").Run()

	t.Run("last N commits", func(t *testing.T) {
		opts := GitOptions{Commits: 1}
		err := mgr.UpdateFromGit(opts)
		if err != nil {
			t.Fatalf("UpdateFromGit failed: %v", err)
		}

		// Check that only file from last commit is included
		files, err := mgr.ResolveFilesFromRules()
		if err != nil {
			t.Fatalf("Failed to resolve files from rules: %v", err)
		}

		if len(files) != 1 || files[0] != "file3.go" {
			t.Errorf("Expected only file3.go from last commit, got %v", files)
		}
	})

	// Create a branch
	exec.Command("git", "checkout", "-b", "feature").Run()
	os.WriteFile("feature.go", []byte("package main\n// Feature"), 0644)
	exec.Command("git", "add", "feature.go").Run()
	exec.Command("git", "commit", "-m", "Add feature").Run()

	t.Run("branch comparison", func(t *testing.T) {
		// Use HEAD~1..HEAD to compare last commit instead of branch names
		opts := GitOptions{Branch: "HEAD~1..HEAD"}
		err := mgr.UpdateFromGit(opts)
		if err != nil {
			t.Fatalf("UpdateFromGit failed: %v", err)
		}

		// Check that only feature branch file is included
		files, err := mgr.ResolveFilesFromRules()
		if err != nil {
			t.Fatalf("Failed to resolve files from rules: %v", err)
		}

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

func TestCheckGitRepo(t *testing.T) {
	// Skip if git is not available
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	t.Run("not in git repo", func(t *testing.T) {
		tempDir := t.TempDir()
		os.Chdir(tempDir)
		defer os.Chdir("..")

		err := checkGitRepo()
		if err == nil {
			t.Error("Expected error when not in git repo")
		}
	})

	t.Run("in git repo", func(t *testing.T) {
		tempDir := t.TempDir()
		os.Chdir(tempDir)
		defer os.Chdir("..")

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