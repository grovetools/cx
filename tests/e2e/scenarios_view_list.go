// File: grove-context/tests/e2e/scenarios_view_list.go
package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/grovetools/tend/pkg/harness"
	"github.com/grovetools/tend/pkg/tui"
	"github.com/grovetools/tend/pkg/verify"
)

// TUIViewListScenario tests the `cx view` list page.
func TUIViewListScenario() *harness.Scenario {
	return harness.NewScenario(
		"cx-view-list-interactive",
		"Tests the `cx view` list page display and basic navigation.",
		[]string{"cx", "tui", "view", "list"},
		[]harness.Step{
			harness.NewStep("Setup comprehensive environment", setupComprehensiveCXEnvironment),
			harness.NewStep("Launch TUI and verify list page", func(ctx *harness.Context) error {
				cxBin, err := FindProjectBinary()
				if err != nil {
					return err
				}
				projectADir := ctx.GetString("project_a_dir")

				session, err := ctx.StartTUI(cxBin, []string{"view", "--page", "list"},
					tui.WithCwd(projectADir),
					tui.WithEnv("CLICOLOR_FORCE=1"),
				)
				if err != nil {
					return fmt.Errorf("failed to start TUI session: %w", err)
				}
				ctx.Set("tui_session", session)

				// Wait for hot context files to appear
				if _, err := session.WaitForAnyText([]string{"main.go", "lib.go"}, 5*time.Second); err != nil {
					view, _ := session.Capture()
					return fmt.Errorf("timeout waiting for list content: %w\nView:\n%s", err, view)
				}

				view, _ := session.Capture()
				ctx.ShowCommandOutput("List Page - Initial View", view, "")

				content, _ := session.Capture(tui.WithCleanedOutput())
				return ctx.Verify(func(v *verify.Collector) {
					v.True("shows hot file from main project", strings.Contains(content, "main.go"))
					v.True("shows hot file from aliased project", strings.Contains(content, "lib.go"))
					v.True("does not show cold file", !strings.Contains(content, "README.md"))
					v.True("does not show excluded file", !strings.Contains(content, "main_test.go"))
				})
			}),
			harness.NewStep("Test page navigation", func(ctx *harness.Context) error {
				session := ctx.Get("tui_session").(*tui.Session)
				// Navigate to tree page
				if err := session.Type("Tab"); err != nil {
					return err
				}
				// Wait for tree view (directory indicators)
				time.Sleep(500 * time.Millisecond)

				view, _ := session.Capture()
				ctx.ShowCommandOutput("After Tab to Tree Page", view, "")

				content, _ := session.Capture(tui.WithCleanedOutput())
				// Tree view shows directory icons
				return ctx.Verify(func(v *verify.Collector) {
					v.True("navigated to tree page", strings.Contains(content, "var") || strings.Contains(content, "private"))
				})
			}),
			harness.NewStep("Quit the TUI", quitCXViewTUI),
		},
	)
}
