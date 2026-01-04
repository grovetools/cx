// File: grove-context/tests/e2e/scenarios_view_rules.go
package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/mattsolo1/grove-tend/pkg/harness"
	"github.com/mattsolo1/grove-tend/pkg/tui"
	"github.com/mattsolo1/grove-tend/pkg/verify"
)

// TUIViewRulesScenario tests the interactive features of the `cx view` rules page.
func TUIViewRulesScenario() *harness.Scenario {
	return harness.NewScenario(
		"cx-view-rules-interactive",
		"Tests starting the TUI on the rules page, verifying syntax highlighting and navigation.",
		[]string{"cx", "tui", "view", "rules"},
		[]harness.Step{
			harness.NewStep("Setup comprehensive environment", setupComprehensiveCXEnvironment),
			harness.NewStep("Launch 'cx view' on rules page", func(ctx *harness.Context) error {
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

				// Wait for UI to load
				if err := session.WaitForText("Rules File:", 5*time.Second); err != nil {
					view, _ := session.Capture()
					return fmt.Errorf("timeout waiting for rules page: %w\nView:\n%s", err, view)
				}
				if err := session.WaitStable(); err != nil {
					return err
				}

				view, _ := session.Capture()
				ctx.ShowCommandOutput("Rules Page - Initial View", view, "")
				return nil
			}),
			harness.NewStep("Verify rules content and syntax highlighting", func(ctx *harness.Context) error {
				session := ctx.Get("tui_session").(*tui.Session)

				// Verify content with clean output (ANSI stripped)
				cleanContent, err := session.Capture(tui.WithCleanedOutput())
				if err != nil {
					return err
				}

				if err := ctx.Verify(func(v *verify.Collector) {
					v.True("shows alias rule", strings.Contains(cleanContent, "@a:subproject-c::default"))
					v.True("shows exclusion rule", strings.Contains(cleanContent, "!*_test.go"))
					v.True("shows cold context separator", strings.Contains(cleanContent, "---"))
					v.True("shows cold context rule", strings.Contains(cleanContent, "README.md"))
				}); err != nil {
					return err
				}

				// Verify syntax highlighting with raw output
				rawContent, err := session.Capture(tui.WithRawOutput())
				if err != nil {
					return err
				}

				return ctx.Verify(func(v *verify.Collector) {
					v.True("contains ANSI escape codes", strings.Contains(rawContent, "\x1b["))
					v.True("contains bold formatting codes", strings.Contains(rawContent, "[1m"))
					v.True("contains RGB color codes", strings.Contains(rawContent, "[38;2;"))
				})
			}),
			harness.NewStep("Test page navigation", func(ctx *harness.Context) error {
				session := ctx.Get("tui_session").(*tui.Session)
				// Navigate to stats page
				if err := session.Type("Tab"); err != nil {
					return err
				}
				if err := session.WaitForText("File Types", 2*time.Second); err != nil {
					return err
				}

				view, _ := session.Capture()
				ctx.ShowCommandOutput("After Tab to Stats Page", view, "")
				return nil
			}),
			harness.NewStep("Quit the TUI", quitCXViewTUI),
		},
	)
}
