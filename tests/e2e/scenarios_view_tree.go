// File: grove-context/tests/e2e/scenarios_view_tree.go
package main

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/mattsolo1/grove-tend/pkg/fs"
	"github.com/mattsolo1/grove-tend/pkg/git"
	"github.com/mattsolo1/grove-tend/pkg/harness"
	"github.com/mattsolo1/grove-tend/pkg/tui"
	"github.com/mattsolo1/grove-tend/pkg/verify"
)

// TUIViewTreeScenario tests the primary features of `cx view`.
func TUIViewTreeScenario() *harness.Scenario {
	return harness.NewScenario(
		"cx-view-tui-comprehensive",
		"Verifies core features of the `cx view` command in a comprehensive environment.",
		[]string{"cx", "tui", "view", "tree"},
		[]harness.Step{
			harness.NewStep("Setup comprehensive TUI environment", setupComprehensiveCXEnvironment),
			harness.NewStep("Launch TUI and test initial navigation", launchAndTestInitialCXView),
			harness.NewStep("Test page navigation", testCXViewPageNavigation),
			harness.NewStep("Test interactive rule modification", testCXViewRuleModification),
			harness.NewStep("Test search functionality", testCXViewSearch),
			harness.NewStep("Quit the TUI", quitCXViewTUI),
		},
	)
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

func launchAndTestInitialCXView(ctx *harness.Context) error {
	cxBin, err := FindProjectBinary()
	if err != nil {
		return err
	}
	projectADir := ctx.GetString("project_a_dir")

	session, err := ctx.StartTUI(cxBin, []string{"view", "--page", "tree"},
		tui.WithCwd(projectADir),
		tui.WithEnv("CLICOLOR_FORCE=1"),
	)
	if err != nil {
		return fmt.Errorf("failed to start TUI session: %w", err)
	}
	ctx.Set("tui_session", session)

	// Wait for tree to render - look for any top-level directory names
	// The tree starts collapsed, so we won't see files immediately
	if _, err := session.WaitForAnyText([]string{"var", "private", "Users"}, 10*time.Second); err != nil {
		view, _ := session.Capture()
		return fmt.Errorf("timeout waiting for TUI to start: %w\nView:\n%s", err, view)
	}
	if err := session.WaitStable(); err != nil {
		return err
	}

	// Verify initial state - tree starts collapsed showing only top-level directories
	content, _ := session.Capture(tui.WithCleanedOutput())
	return ctx.Verify(func(v *verify.Collector) {
		// Check that the tree is showing top-level directories
		v.True("tree shows top-level directory",
			strings.Contains(content, "var") || strings.Contains(content, "private") || strings.Contains(content, "Users"))
		// The tree is collapsed, so individual files aren't visible yet
		// We'll expand and test them in the navigation step
	})
}

func testCXViewPageNavigation(ctx *harness.Context) error {
	session := ctx.Get("tui_session").(*tui.Session)
	if err := session.Type("Tab"); err != nil { // to rules page
		return err
	}
	if err := session.WaitForText("Rules File:", 2*time.Second); err != nil {
		return err
	}
	if err := session.Type("Tab"); err != nil { // to stats page
		return err
	}
	if err := session.WaitForText("File Types", 2*time.Second); err != nil {
		return err
	}
	if err := session.Type("Tab"); err != nil { // to list page
		return err
	}
	if err := session.WaitForText("Files in Hot Context", 2*time.Second); err != nil {
		return err
	}
	if err := session.Type("Shift+Tab"); err != nil { // back to stats page
		return err
	}
	return session.WaitForText("File Types", 2*time.Second)
}

func testCXViewRuleModification(ctx *harness.Context) error {
	session := ctx.Get("tui_session").(*tui.Session)
	// Go back to tree view
	if err := session.Type("Shift+Tab", "Shift+Tab"); err != nil {
		return err
	}
	if err := session.WaitForText("main.go", 2*time.Second); err != nil {
		return err
	}

	// Navigate to untracked.txt
	if err := session.NavigateToText("untracked.txt"); err != nil {
		return err
	}
	// Add to hot context
	if err := session.Type("h"); err != nil {
		return err
	}
	if err := session.WaitForText("untracked.txt ✓", 3*time.Second); err != nil {
		return err
	}
	// Move to cold context
	if err := session.Type("c"); err != nil {
		return err
	}
	if err := session.WaitForText("untracked.txt ❄️", 3*time.Second); err != nil {
		return err
	}
	// Exclude it
	if err := session.Type("x"); err != nil {
		return err
	}
	return session.WaitForText("untracked.txt 🚫", 3*time.Second)
}

func testCXViewSearch(ctx *harness.Context) error {
	session := ctx.Get("tui_session").(*tui.Session)
	if err := session.Type("/"); err != nil { // Start search
		return err
	}
	if err := session.Type("lib.go"); err != nil { // Type search term
		return err
	}
	if err := session.Type("Enter"); err != nil { // Apply search
		return err
	}

	if err := ctx.Verify(func(v *verify.Collector) {
		v.Equal("lib.go is visible after search", nil,
			session.WaitForText("lib.go", 3*time.Second))
		v.Equal("main.go is not visible", nil,
			session.AssertNotContains("main.go"))
	}); err != nil {
		return err
	}

	// Clear search
	return session.Type("Escape")
}

func quitCXViewTUI(ctx *harness.Context) error {
	session := ctx.Get("tui_session").(*tui.Session)
	return session.SendKeys("q")
}
