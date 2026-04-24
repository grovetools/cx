// File: grove-context/tests/e2e/test_utils.go
package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/grovetools/core/pkg/tmux"
	"github.com/grovetools/tend/pkg/fs"
	"github.com/grovetools/tend/pkg/git"
	"github.com/grovetools/tend/pkg/harness"
	"github.com/grovetools/tend/pkg/project"
)

// FindProjectBinary finds the project's main binary path by reading grove.yml.
// This provides a single source of truth for locating the binary under test.
func FindProjectBinary() (string, error) {
	// The test runner is executed from the project root, so we start the search here.
	wd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("could not get working directory: %w", err)
	}

	binaryPath, err := project.GetBinaryPath(wd)
	if err != nil {
		return "", fmt.Errorf("failed to find project binary via grove.yml: %w", err)
	}

	return binaryPath, nil
}

// findGeneratedFile looks for a generated file across all possible output locations.
// The cx binary may write to different paths depending on notebook config, plan state, etc.
// When run via ctx.Command(), the binary has a sandboxed HOME under <rootDir>/home/,
// so files may end up under home/.grove/notebooks/... within the test root.
// This helper searches all known locations using glob patterns.
func findGeneratedFile(rootDir, filename string) string {
	// Check direct paths first (fastest)
	directPaths := []string{
		filepath.Join(rootDir, ".notebook", "context", "generated", filename),
		filepath.Join(rootDir, ".notebook", "context", "cache", filename),
		filepath.Join(rootDir, ".grove", filename),
	}
	for _, p := range directPaths {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}

	// Check under sandboxed home (ctx.Command sets HOME=<rootDir>/home)
	homeGlob := filepath.Join(rootDir, "home", ".grove", "notebooks", "*", "workspaces", "*", "context", "**", filename)
	matches, _ := filepath.Glob(homeGlob)
	if len(matches) > 0 {
		return matches[0]
	}

	// Broader search: find the file anywhere under rootDir
	var found string
	filepath.Walk(rootDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		if filepath.Base(path) == filename && found == "" {
			found = path
		}
		return nil
	})
	if found != "" {
		return found
	}

	// Return fallback path for better error messages
	return filepath.Join(rootDir, ".grove", filename)
}

// findContextFileOrFallback finds the generated context file across all possible output locations.
func findContextFileOrFallback(rootDir string) string {
	return findGeneratedFile(rootDir, "context")
}

// findCachedContextFileOrFallback finds the cached context file across all possible output locations.
func findCachedContextFileOrFallback(rootDir string) string {
	return findGeneratedFile(rootDir, "cached-context")
}

// findCachedContextFilesListOrFallback finds the cached context files list across all possible output locations.
func findCachedContextFilesListOrFallback(rootDir string) string {
	return findGeneratedFile(rootDir, "cached-context-files")
}

// findRulesFileOrFallback finds the active rules file across all possible locations.
func findRulesFileOrFallback(rootDir string) string {
	return findGeneratedFile(rootDir, "rules")
}

// CleanupExistingTestSessions kills any existing tmux sessions that match tend test patterns.
// This helps ensure a clean test environment and avoids port conflicts or session collisions.
func CleanupExistingTestSessions() error {
	// List all tmux sessions
	cmd := tmux.Command("list-sessions", "-F", "#{session_name}")
	output, err := cmd.Output()
	if err != nil {
		// If tmux returns an error, it might mean no server is running
		// This is fine - nothing to clean up
		if exitErr, ok := err.(*exec.ExitError); ok {
			if strings.Contains(string(exitErr.Stderr), "no server running") {
				return nil
			}
		}
		return fmt.Errorf("failed to list tmux sessions: %w", err)
	}

	sessions := strings.Split(strings.TrimSpace(string(output)), "\n")
	cleanedCount := 0

	for _, session := range sessions {
		session = strings.TrimSpace(session)
		if session == "" {
			continue
		}

		// Check if this looks like a tend test session
		// Tend test sessions typically have patterns like "tend-test-*" or contain "cx-view"
		if strings.Contains(session, "tend-test") ||
			strings.Contains(session, "cx-view") ||
			strings.Contains(session, "grove-tend") {
			// Kill the session
			killCmd := tmux.Command("kill-session", "-t", session)
			if err := killCmd.Run(); err != nil {
				// Log but don't fail - session might have already ended
				fmt.Printf("   Note: Could not kill session %s: %v\n", session, err)
			} else {
				cleanedCount++
			}
		}
	}

	if cleanedCount > 0 {
		fmt.Printf("   Cleaned %d existing test session(s)\n", cleanedCount)
	}

	return nil
}

// setupComprehensiveCXEnvironment creates a rich, multi-project environment for testing `cx view`.
func setupComprehensiveCXEnvironment(ctx *harness.Context) error {
	// 1. Configure a sandboxed global environment.
	grovesDir := filepath.Join(ctx.RootDir, "projects")
	globalYAML := fmt.Sprintf(`
version: "1.0"
groves:
  e2e-projects:
    path: "%s"
`, grovesDir)
	globalConfigDir := filepath.Join(ctx.ConfigDir(), "grove")
	if err := fs.CreateDir(globalConfigDir); err != nil {
		return err
	}
	if err := fs.WriteString(filepath.Join(globalConfigDir, "grove.yml"), globalYAML); err != nil {
		return err
	}

	// 2. Create multiple projects.
	projectADir := filepath.Join(grovesDir, "project-a")
	ecosystemBDir := filepath.Join(grovesDir, "ecosystem-b")
	subprojectCDir := filepath.Join(grovesDir, "subproject-c")

	// -- Project A (Standalone) --
	if err := fs.WriteString(filepath.Join(projectADir, "grove.yml"), "name: project-a"); err != nil {
		return err
	}
	if err := fs.WriteString(filepath.Join(projectADir, ".gitignore"), "*.log\n"); err != nil {
		return err
	}
	if err := fs.WriteString(filepath.Join(projectADir, "main.go"), "package main // hot"); err != nil {
		return err
	}
	if err := fs.WriteString(filepath.Join(projectADir, "main_test.go"), "package main // excluded"); err != nil {
		return err
	}
	if err := fs.WriteString(filepath.Join(projectADir, "README.md"), "# Project A // cold"); err != nil {
		return err
	}
	if err := fs.WriteString(filepath.Join(projectADir, "test.log"), "log content"); err != nil {
		return err
	}
	if err := fs.WriteString(filepath.Join(projectADir, "untracked.txt"), "omitted file"); err != nil {
		return err
	}

	repoA, err := git.SetupTestRepo(projectADir)
	if err != nil {
		return err
	}
	if err := repoA.AddCommit("initial commit for project A"); err != nil {
		return err
	}
	if err := repoA.CreateWorktree(filepath.Join(projectADir, ".grove-worktrees", "feature-branch"), "feature-branch"); err != nil {
		return err
	}
	if err := fs.WriteString(filepath.Join(projectADir, ".grove-worktrees", "feature-branch", "feature.go"), "package main // feature file"); err != nil {
		return err
	}

	// -- Ecosystem B --
	if err := fs.WriteString(filepath.Join(ecosystemBDir, "grove.yml"), "name: ecosystem-b"); err != nil {
		return err
	}
	if err := fs.WriteString(filepath.Join(ecosystemBDir, "helper.go"), "package helper"); err != nil {
		return err
	}
	repoB, err := git.SetupTestRepo(ecosystemBDir)
	if err != nil {
		return err
	}
	if err := repoB.AddCommit("initial ecosystem commit"); err != nil {
		return err
	}

	// -- Subproject C (Standalone) --
	if err := fs.WriteString(filepath.Join(subprojectCDir, "grove.yml"), "name: subproject-c"); err != nil {
		return err
	}
	if err := fs.WriteString(filepath.Join(subprojectCDir, "lib.go"), "package lib // from subproject"); err != nil {
		return err
	}
	if err := fs.WriteString(filepath.Join(subprojectCDir, "lib_test.go"), "package lib_test"); err != nil {
		return err
	}
	if err := fs.WriteString(filepath.Join(subprojectCDir, ".cx", "default.rules"), "lib.go"); err != nil {
		return err
	}
	repoC, err := git.SetupTestRepo(subprojectCDir)
	if err != nil {
		return err
	}
	if err := repoC.AddCommit("initial subproject commit"); err != nil {
		return err
	}

	// -- Main rules file in project-a --
	rules := `*.go
!*_test.go
@a:subproject-c::default
---
README.md
`
	if err := fs.WriteString(filepath.Join(projectADir, ".grove", "rules"), rules); err != nil {
		return err
	}

	ctx.Set("project_a_dir", projectADir)
	return nil
}
