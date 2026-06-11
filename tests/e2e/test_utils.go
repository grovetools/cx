// File: grove-context/tests/e2e/test_utils.go
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/grovetools/core/pkg/repo"
	"github.com/grovetools/core/pkg/tmux"
	"github.com/grovetools/core/pkg/workspace"
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
	_ = filepath.Walk(rootDir, func(path string, info os.FileInfo, err error) error {
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

// CleanupTestRepos removes any cx-managed bare clones whose URLs reference
// the active test sandbox from the user's real ~/.local/share/grove/cx/repos.
// Without this, e2e runs leak entries into `cx alias list` / `cx repo list`.
func CleanupTestRepos(ctx *harness.Context) error {
	mgr, err := repo.NewManager()
	if err != nil {
		return nil
	}
	manifest, err := mgr.LoadManifest()
	if err != nil || manifest == nil {
		return nil
	}
	rootDir := ctx.RootDir
	for url, info := range manifest.Repositories {
		if !looksLikeTestRepo(url, rootDir) {
			continue
		}
		if info.BarePath != "" {
			os.RemoveAll(info.BarePath)
		}
		delete(manifest.Repositories, url)
	}
	cxPath, err := repo.GetCxEcosystemPath()
	if err != nil {
		return nil
	}
	manifestPath := filepath.Join(cxPath, "manifest.json")
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return nil
	}
	_ = os.WriteFile(manifestPath, data, 0o644) //nolint:gosec // test manifest file
	return nil
}

func looksLikeTestRepo(url, rootDir string) bool {
	if rootDir != "" && strings.Contains(url, rootDir) {
		return true
	}
	return strings.Contains(url, "grove-tend") ||
		strings.Contains(url, "source-repo") ||
		strings.Contains(url, "/var/folders/") ||
		strings.Contains(url, "10.255.255.1")
}

// XDGEcosystemWorktreeFixture holds the paths produced by
// setupXDGEcosystemWorktreeFixture so scenarios can drive cx from the XDG
// worktree and assert against the original checkouts.
type XDGEcosystemWorktreeFixture struct {
	EcoDir           string // original ecosystem checkout
	WorktreeDir      string // XDG ecosystem worktree (WorktreesDir()/<id>/feature-x)
	RepoAWorktreeDir string // repo-a inside the XDG worktree
	RepoBWorktreeDir string // repo-b inside the XDG worktree
}

// setupXDGEcosystemWorktreeFixture builds the XDG-layout sibling of the legacy
// AliasSiblingResolutionScenario fixture: an ecosystem with two main repos and
// an ecosystem worktree that lives under the sandboxed XDG data dir
// (paths.WorktreesDir()/<DirIdentifier(eco)>/feature-x) instead of
// <eco>/.grove-worktrees/feature-x. The worktree's .git pointer names the
// original ecosystem checkout as owner so cx resolves identity and siblings
// against the original repos. Only repo-b's worktree carries worktree_only.go,
// the marker the sibling-resolution assertions key on.
func setupXDGEcosystemWorktreeFixture(ctx *harness.Context) (*XDGEcosystemWorktreeFixture, error) {
	grovesDir := filepath.Join(ctx.RootDir, "mock-groves")
	groveConfigDir := filepath.Join(ctx.ConfigDir(), "grove")

	groveConfig := fmt.Sprintf("groves:\n  test:\n    path: %s\n    enabled: true\ncontext:\n  repos_dir: \"\"\n", grovesDir)
	if err := fs.WriteString(filepath.Join(groveConfigDir, "grove.yml"), groveConfig); err != nil {
		return nil, err
	}

	gitInit := func(dir string) error {
		cmd := exec.Command("git", "init")
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("git init %s: %w (%s)", dir, err, out)
		}
		return nil
	}

	ecoConfig := "name: my-ecosystem\nworkspaces:\n  - \"*\"\n"

	// Original ecosystem checkout + two main repos.
	ecoDir := filepath.Join(grovesDir, "my-ecosystem")
	if err := fs.WriteString(filepath.Join(ecoDir, ".gitmodules"), "# ecosystem"); err != nil {
		return nil, err
	}
	if err := fs.WriteString(filepath.Join(ecoDir, "grove.yml"), ecoConfig); err != nil {
		return nil, err
	}
	if err := gitInit(ecoDir); err != nil {
		return nil, err
	}
	for _, r := range []string{"repo-a", "repo-b"} {
		d := filepath.Join(ecoDir, r)
		if err := fs.WriteString(filepath.Join(d, "grove.yml"), "name: "+r); err != nil {
			return nil, err
		}
		if err := fs.WriteString(filepath.Join(d, "main.go"), "package main // main version"); err != nil {
			return nil, err
		}
		if err := gitInit(d); err != nil {
			return nil, err
		}
	}

	// XDG ecosystem worktree under the sandboxed data dir. The identifier
	// matches what cx computes for ecoDir, so discovery scans the same base.
	id := workspace.DirIdentifier(ecoDir)
	worktreeDir := filepath.Join(ctx.DataDir(), "grove", "worktrees", id, "feature-x")
	if err := fs.WriteString(filepath.Join(worktreeDir, ".gitmodules"), "# ecosystem worktree"); err != nil {
		return nil, err
	}
	if err := fs.WriteString(filepath.Join(worktreeDir, "grove.yml"), ecoConfig); err != nil {
		return nil, err
	}
	// .git FILE naming the original checkout as owner (absolute gitdir, since
	// the legacy relative ../../ pointer is meaningless from the XDG base).
	if err := fs.WriteString(filepath.Join(worktreeDir, ".git"),
		"gitdir: "+filepath.Join(ecoDir, ".git", "worktrees", "feature-x")); err != nil {
		return nil, err
	}

	repoAWorktreeDir := filepath.Join(worktreeDir, "repo-a")
	if err := fs.WriteString(filepath.Join(repoAWorktreeDir, "grove.yml"), "name: repo-a"); err != nil {
		return nil, err
	}
	if err := fs.WriteString(filepath.Join(repoAWorktreeDir, "main.go"), "package main // worktree version"); err != nil {
		return nil, err
	}
	if err := gitInit(repoAWorktreeDir); err != nil {
		return nil, err
	}

	repoBWorktreeDir := filepath.Join(worktreeDir, "repo-b")
	if err := fs.WriteString(filepath.Join(repoBWorktreeDir, "grove.yml"), "name: repo-b"); err != nil {
		return nil, err
	}
	if err := fs.WriteString(filepath.Join(repoBWorktreeDir, "main.go"), "package main // worktree version"); err != nil {
		return nil, err
	}
	if err := fs.WriteString(filepath.Join(repoBWorktreeDir, "worktree_only.go"), "package main // ONLY in worktree"); err != nil {
		return nil, err
	}
	if err := gitInit(repoBWorktreeDir); err != nil {
		return nil, err
	}

	f := &XDGEcosystemWorktreeFixture{
		EcoDir:           ecoDir,
		WorktreeDir:      worktreeDir,
		RepoAWorktreeDir: repoAWorktreeDir,
		RepoBWorktreeDir: repoBWorktreeDir,
	}
	ctx.Set("xdgFixture", f)
	return f, nil
}
