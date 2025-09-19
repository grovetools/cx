// File: grove-context/tests/e2e/scenarios_tui.go
package main

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/mattsolo1/grove-tend/pkg/fs"
	"github.com/mattsolo1/grove-tend/pkg/harness"
	"github.com/mattsolo1/grove-tend/pkg/tui"
)

// TUIViewScenario tests the interactive `cx view` TUI.
func TUIViewScenario() *harness.Scenario {
	return &harness.Scenario{
		Name:        "cx-view-tui-test",
		Description: "Tests the interactive `cx view` TUI by launching it in tmux and validating safety features.",
		Tags:        []string{"cx", "tui", "view", "interactive"},
		Steps: []harness.Step{
			harness.NewStep("Setup project for TUI view", func(ctx *harness.Context) error {
				// Create a predictable file structure for the TUI to display.
				if err := fs.WriteString(filepath.Join(ctx.RootDir, "main.go"), "package main // in hot context"); err != nil {
					return err
				}
				// Create a specific readme file that should be safe to add
				if err := fs.WriteString(filepath.Join(ctx.RootDir, "PROJECT_README.md"), "# Project README"); err != nil {
					return err
				}
				if err := fs.WriteString(filepath.Join(ctx.RootDir, "utils_test.go"), "package main // excluded"); err != nil {
					return err
				}
				// Create rules that will put main.go in hot context, exclude the test, and leave PROJECT_README.md omitted.
				rules := "**/*.go\n!**/*_test.go"
				return fs.WriteString(filepath.Join(ctx.RootDir, ".grove", "rules"), rules)
			}),
			harness.NewStep("Launch 'cx view' in tmux", func(ctx *harness.Context) error {
				cxBinary, err := FindProjectBinary()
				if err != nil {
					return err
				}

				// Start the TUI in an isolated tmux session. The harness manages its lifecycle.
				session, err := ctx.StartTUI(cxBinary, "view")
				if err != nil {
					return fmt.Errorf("failed to start 'cx view' TUI: %w", err)
				}

				// Store the session handle in the context to interact with it in later steps.
				ctx.Set("view_session", session)
				return nil
			}),
			harness.NewStep("Wait for TUI to initialize", func(ctx *harness.Context) error {
				session := ctx.Get("view_session").(*tui.Session)
				// Wait for a known header text to appear, confirming the TUI is ready.
				return session.WaitForText("Grove Context Visualization", 5*time.Second)
			}),
			harness.NewStep("Interact with TUI to trigger safety validation", func(ctx *harness.Context) error {
				session := ctx.Get("view_session").(*tui.Session)

				// Navigate to PROJECT_README.md. Based on alphabetical order, we expect:
				// main.go, PROJECT_README.md, utils_test.go
				// So navigate down once to get to PROJECT_README.md
				if err := session.SendKeys("Down"); err != nil {
					return fmt.Errorf("failed to send 'Down' key: %w", err)
				}

				// Simulate user pressing 'h' to attempt adding the selected file to hot context.
				// This should trigger safety validation and show an error.
				if err := session.SendKeys("h"); err != nil {
					return fmt.Errorf("failed to send 'h' key: %w", err)
				}
				return nil
			}),
			harness.NewStep("Verify safety validation triggered", func(ctx *harness.Context) error {
				session := ctx.Get("view_session").(*tui.Session)

				// Wait a moment for the action to complete
				time.Sleep(1 * time.Second)

				// Capture the entire screen content to verify the safety validation.
				content, err := session.Capture()
				if err != nil {
					return fmt.Errorf("failed to capture TUI screen: %w", err)
				}
				ctx.ShowCommandOutput("TUI Capture", content, "")

				// Verify that safety validation was triggered as expected
				if strings.Contains(content, "safety validation failed") && 
				   strings.Contains(content, "PROJECT_README.md") &&
				   strings.Contains(content, "would include system directory") {
					return nil // This is the expected behavior - safety validation working correctly
				}

				// If the file was somehow added (unexpected), that would be an error
				if strings.Contains(content, "✓ PROJECT_README.md") {
					return fmt.Errorf("unexpected: PROJECT_README.md was added despite safety concerns")
				}

				return fmt.Errorf("expected safety validation error but did not find it. Screen content:\n%s", content)
			}),
			harness.NewStep("Quit the TUI", func(ctx *harness.Context) error {
				session := ctx.Get("view_session").(*tui.Session)
				// Send 'q' to gracefully exit the application.
				if err := session.SendKeys("q"); err != nil {
					return fmt.Errorf("failed to send 'q' key: %w", err)
				}
				// Give the process a moment to exit before the harness cleans up the session.
				time.Sleep(500 * time.Millisecond)
				return nil
			}),
		},
	}
}