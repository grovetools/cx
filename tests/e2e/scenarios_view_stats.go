// File: grove-context/tests/e2e/scenarios_view_stats.go
package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/mattsolo1/grove-tend/pkg/harness"
	"github.com/mattsolo1/grove-tend/pkg/tui"
	"github.com/mattsolo1/grove-tend/pkg/verify"
)

// TUIViewStatsScenario tests the `cx view` stats page.
func TUIViewStatsScenario() *harness.Scenario {
	return harness.NewScenario(
		"cx-view-stats-interactive",
		"Tests the `cx view` stats page display and basic content.",
		[]string{"cx", "tui", "view", "stats"},
		[]harness.Step{
			harness.NewStep("Setup comprehensive environment", setupComprehensiveCXEnvironment),
			harness.NewStep("Launch 'cx view' on stats page", func(ctx *harness.Context) error {
				cxBinary, err := FindProjectBinary()
				if err != nil {
					return err
				}
				projectADir := ctx.GetString("project_a_dir")

				session, err := ctx.StartTUI(cxBinary, []string{"view", "--page", "stats"},
					tui.WithCwd(projectADir),
					tui.WithEnv("CLICOLOR_FORCE=1"),
				)
				if err != nil {
					return fmt.Errorf("failed to start 'cx view' TUI: %w", err)
				}
				ctx.Set("tui_session", session)

				if err := session.WaitForText("File Types", 5*time.Second); err != nil {
					view, _ := session.Capture()
					return fmt.Errorf("timeout waiting for stats page: %w\nView:\n%s", err, view)
				}
				if err := session.WaitStable(); err != nil {
					return err
				}

				view, _ := session.Capture()
				ctx.ShowCommandOutput("Stats Page - Initial View", view, "")
				return nil
			}),
			harness.NewStep("Verify stats content", func(ctx *harness.Context) error {
				session := ctx.Get("tui_session").(*tui.Session)
				content, err := session.Capture(tui.WithCleanedOutput())
				if err != nil {
					return err
				}

				return ctx.Verify(func(v *verify.Collector) {
					v.True("shows File Types header", strings.Contains(content, "File Types"))
					v.True("shows Largest Files header", strings.Contains(content, "Largest Files"))
					v.True("shows .go files", strings.Contains(content, ".go"))
					v.True("does not show excluded test files", !strings.Contains(content, "_test.go"))
				})
			}),
			harness.NewStep("Test page navigation", func(ctx *harness.Context) error {
				session := ctx.Get("tui_session").(*tui.Session)
				// Navigate to list page
				if err := session.Type("Tab"); err != nil {
					return err
				}
				// Wait for list page to load
				if err := session.WaitForText("main.go", 2*time.Second); err != nil {
					return err
				}

				view, _ := session.Capture()
				ctx.ShowCommandOutput("After Tab to List Page", view, "")
				return nil
			}),
			harness.NewStep("Quit the TUI", quitCXViewTUI),
		},
	)
}
