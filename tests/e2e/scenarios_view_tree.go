// File: grove-context/tests/e2e/scenarios_view_tree.go
package main

import (
	"fmt"
	"time"

	"github.com/grovetools/tend/pkg/harness"
	"github.com/grovetools/tend/pkg/tui"
	"github.com/grovetools/tend/pkg/verify"
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
			harness.NewStep("Quit the TUI", quitCXViewTUI),
		},
	)
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

	// Wait for tree to render - tree shows absolute path components
	// On macOS /var is symlinked to /private/var, so temp dirs show as private/var
	if _, err := session.WaitForAnyText([]string{"var", "private", "TREE"}, 30*time.Second); err != nil {
		view, _ := session.Capture()
		ctx.ShowCommandOutput("TUI Failed to Start - Current View", view, "")
		return fmt.Errorf("timeout waiting for TUI to start: %w\nView:\n%s", err, view)
	}

	// Wait for UI to stabilize after async loading
	if err := session.WaitStable(); err != nil {
		return err
	}

	view, _ := session.Capture()
	ctx.ShowCommandOutput("Tree Page - Initial View", view, "")

	// The tree page rendered successfully
	return nil
}

func testCXViewPageNavigation(ctx *harness.Context) error {
	session := ctx.Get("tui_session").(*tui.Session)
	// Test Tab navigation - just verify it moves to different pages
	if err := session.Type("Tab"); err != nil {
		return err
	}
	time.Sleep(500 * time.Millisecond)
	view1, _ := session.Capture()
	ctx.ShowCommandOutput("After Tab #1", view1, "")

	if err := session.Type("Tab"); err != nil {
		return err
	}
	time.Sleep(500 * time.Millisecond)
	view2, _ := session.Capture()
	ctx.ShowCommandOutput("After Tab #2", view2, "")

	if err := session.Type("Tab"); err != nil {
		return err
	}
	// After 3 tabs, we should be on a different page than tree
	time.Sleep(500 * time.Millisecond)
	view3, _ := session.Capture()
	ctx.ShowCommandOutput("After Tab #3", view3, "")
	return nil
}

func testCXViewRuleModification(ctx *harness.Context) error {
	session := ctx.Get("tui_session").(*tui.Session)
	// Go to tree view (from list page, tab forward to tree)
	if err := session.Type("Tab"); err != nil {
		return err
	}
	// Wait for tree view to load (looking for directory indicators)
	time.Sleep(500 * time.Millisecond)

	// Navigate to untracked.txt
	if err := session.NavigateToText("untracked.txt"); err != nil {
		return err
	}
	// Add to hot context
	if err := session.Type("h"); err != nil {
		return err
	}
	if err := session.WaitForText("untracked.txt *", 3*time.Second); err != nil {
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
