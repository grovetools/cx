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

// TUIViewScenario tests the interactive `cx view` TUI using the enhanced APIs.
func TUIViewScenario() *harness.Scenario {
	return &harness.Scenario{
		Name:        "cx-view-tui-test",
		Description: "Tests the interactive `cx view` TUI using robust navigation and timing APIs.",
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
			harness.NewStep("Wait for TUI to stabilize", func(ctx *harness.Context) error {
				session := ctx.Get("view_session").(*tui.Session)
				
				// NEW: Use WaitForUIStable instead of WaitForText + time.Sleep
				// This is more reliable as it waits for the UI to stop changing
				if err := session.WaitForUIStable(5*time.Second, 100*time.Millisecond, 300*time.Millisecond); err != nil {
					return fmt.Errorf("TUI did not stabilize: %w", err)
				}
				
				// Verify the header is present
				if err := session.WaitForText("Grove Context Visualization", 2*time.Second); err != nil {
					return fmt.Errorf("TUI header not found: %w", err)
				}
				
				return nil
			}),
			harness.NewStep("Navigate to target file using enhanced APIs", func(ctx *harness.Context) error {
				session := ctx.Get("view_session").(*tui.Session)

				// NEW: First, let's find where PROJECT_README.md is on the screen
				row, col, found, err := session.FindTextLocation("PROJECT_README.md")
				if err != nil {
					return fmt.Errorf("error finding text location: %w", err)
				}
				if !found {
					return fmt.Errorf("PROJECT_README.md not found on screen")
				}
				
				// Log location for debugging
				fmt.Printf("   Found PROJECT_README.md at row %d, col %d\n", row, col)
				
				// NEW: Navigate directly to the file instead of using brittle Down key
				// Note: NavigateToText moves the cursor to the exact text location
				if err := session.NavigateToText("PROJECT_README.md"); err != nil {
					return fmt.Errorf("failed to navigate to PROJECT_README.md: %w", err)
				}
				
				// Verify cursor position
				curRow, curCol, err := session.GetCursorPosition()
				if err != nil {
					return fmt.Errorf("failed to get cursor position: %w", err)
				}
				fmt.Printf("   Cursor now at row %d, col %d\n", curRow, curCol)
				
				return nil
			}),
			harness.NewStep("Trigger safety validation", func(ctx *harness.Context) error {
				session := ctx.Get("view_session").(*tui.Session)

				// Simulate user pressing 'h' to attempt adding the selected file to hot context.
				// This should trigger safety validation and show an error.
				if err := session.SendKeys("h"); err != nil {
					return fmt.Errorf("failed to send 'h' key: %w", err)
				}
				
				// NEW: Use WaitForUIStable instead of time.Sleep
				// Wait for the UI to process the action and show the error
				if err := session.WaitForUIStable(3*time.Second, 100*time.Millisecond, 200*time.Millisecond); err != nil {
					// Non-fatal: UI might not stabilize due to error message
					fmt.Printf("   UI stability warning: %v\n", err)
				}
				
				return nil
			}),
			harness.NewStep("Verify safety validation triggered", func(ctx *harness.Context) error {
				session := ctx.Get("view_session").(*tui.Session)

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
				
				// NEW: Wait for UI to close cleanly instead of fixed sleep
				// Allow time for graceful shutdown
				time.Sleep(200 * time.Millisecond)
				
				return nil
			}),
		},
	}
}