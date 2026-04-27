// File: grove-context/tests/e2e/scenarios_view_watcher.go
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/grovetools/tend/pkg/harness"
	"github.com/grovetools/tend/pkg/tui"
	"github.com/grovetools/tend/pkg/verify"
)

// TUIViewRulesWatcherScenario tests that editing a rules file on disk causes the
// cx TUI to auto-refresh via the file watcher.
func TUIViewRulesWatcherScenario() *harness.Scenario {
	return harness.NewScenario(
		"cx-view-rules-watcher",
		"Tests that external rules file edits trigger automatic TUI refresh via file watcher.",
		[]string{"cx", "tui", "view", "rules", "watcher"},
		[]harness.Step{
			harness.NewStep("Setup environment with rules file", setupComprehensiveCXEnvironment),
			harness.NewStep("Launch cx view on rules page", launchCXViewForWatcher),
			harness.NewStep("Verify initial rules content", verifyInitialRulesContent),
			harness.NewStep("Externally modify the rules file", modifyRulesFileExternally),
			harness.NewStep("Verify TUI refreshes with updated content", verifyWatcherRefresh),
			harness.NewStep("Quit the TUI", quitCXViewTUI),
		},
	)
}

func launchCXViewForWatcher(ctx *harness.Context) error {
	cxBinary, err := FindProjectBinary()
	if err != nil {
		return err
	}
	projectADir := ctx.GetString("project_a_dir")

	session, err := ctx.StartTUI(cxBinary, []string{"view", "--page", "rules"},
		tui.WithCwd(projectADir),
		tui.WithEnv("CLICOLOR_FORCE=1"),
	)
	if err != nil {
		return fmt.Errorf("failed to start 'cx view' TUI: %w", err)
	}
	ctx.Set("tui_session", session)

	if err := session.WaitForText("Rules File:", 15*time.Second); err != nil {
		view, _ := session.Capture()
		return fmt.Errorf("timeout waiting for rules page: %w\nView:\n%s", err, view)
	}
	if err := session.WaitStable(); err != nil {
		return err
	}

	session.LogDiagnostic(ctx, "Watcher test: initial rules page loaded")
	return nil
}

func verifyInitialRulesContent(ctx *harness.Context) error {
	session := ctx.Get("tui_session").(*tui.Session)

	cleanContent, err := session.Capture(tui.WithCleanedOutput())
	if err != nil {
		return err
	}

	session.LogDiagnostic(ctx, "Watcher test: verifying initial content")

	return ctx.Verify(func(v *verify.Collector) {
		v.True("initial: shows *.go pattern", strings.Contains(cleanContent, "*.go"))
		v.True("initial: shows exclusion rule", strings.Contains(cleanContent, "!*_test.go"))
		v.True("initial: shows alias rule", strings.Contains(cleanContent, "@a:subproject-c::default"))
		v.True("initial: does NOT contain watcher-sentinel pattern", !strings.Contains(cleanContent, "docs/**/*.md"))
	})
}

func modifyRulesFileExternally(ctx *harness.Context) error {
	projectADir := ctx.GetString("project_a_dir")
	rulesPath := filepath.Join(projectADir, ".grove", "rules")

	existing, err := os.ReadFile(rulesPath)
	if err != nil {
		return fmt.Errorf("failed to read rules file: %w", err)
	}

	// Append a new, distinctive glob pattern that we can search for after refresh.
	updated := string(existing) + "docs/**/*.md\n"
	if err := os.WriteFile(rulesPath, []byte(updated), 0o644); err != nil { //nolint:gosec // test rules file
		return fmt.Errorf("failed to write updated rules file: %w", err)
	}

	ctx.ShowCommandOutput("Watcher test: wrote updated rules file", updated, "")
	return nil
}

func verifyWatcherRefresh(ctx *harness.Context) error {
	session := ctx.Get("tui_session").(*tui.Session)

	// Wait for the new pattern to appear. The watcher debounce is 200ms, plus
	// the state refresh takes time, so use a generous timeout.
	if err := session.WaitForText("docs/**/*.md", 10*time.Second); err != nil {
		// Capture for diagnostics before failing.
		session.LogDiagnostic(ctx, "Watcher test: FAILED - new pattern not found")
		view, _ := session.Capture(tui.WithCleanedOutput())
		return fmt.Errorf("rules file watcher did not refresh TUI after external edit: %w\nView:\n%s", err, view)
	}

	if err := session.WaitStable(); err != nil {
		return err
	}

	session.LogDiagnostic(ctx, "Watcher test: refresh detected")

	cleanContent, err := session.Capture(tui.WithCleanedOutput())
	if err != nil {
		return err
	}

	return ctx.Verify(func(v *verify.Collector) {
		// Original content still present.
		v.True("after refresh: still shows *.go pattern", strings.Contains(cleanContent, "*.go"))
		v.True("after refresh: still shows exclusion rule", strings.Contains(cleanContent, "!*_test.go"))
		// New content visible.
		v.True("after refresh: shows new docs/**/*.md pattern", strings.Contains(cleanContent, "docs/**/*.md"))
	})
}
