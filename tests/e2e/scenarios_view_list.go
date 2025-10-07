// File: grove-context/tests/e2e/scenarios_view_list.go
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

// TUIViewListScenario tests launching `cx view` directly on the list page.
func TUIViewListScenario() *harness.Scenario {
	return &harness.Scenario{
		Name:        "cx-view-list-tui",
		Description: "Tests starting the 'cx view' TUI on the list page and verifying its content.",
		Tags:        []string{"cx", "tui", "view", "list"},
		Steps: []harness.Step{
			harness.NewStep("Setup project with hybrid rules", func(ctx *harness.Context) error {
				fs.WriteString(filepath.Join(ctx.RootDir, "hot.go"), "package main // hot")
				fs.WriteString(filepath.Join(ctx.RootDir, "cold.md"), "# Cold")
				rules := `*.go
---
*.md`
				return fs.WriteString(filepath.Join(ctx.RootDir, ".grove", "rules"), rules)
			}),
			harness.NewStep("Launch 'cx view' on list page", func(ctx *harness.Context) error {
				cxBinary, err := FindProjectBinary()
				if err != nil {
					return err
				}

				session, err := ctx.StartTUI(cxBinary, []string{"view", "--page", "list"},
					tui.WithEnv("CLICOLOR_FORCE=1"),
				)
				if err != nil {
					return fmt.Errorf("failed to start 'cx view' TUI: %w", err)
				}
				ctx.Set("list_session", session)

				// Wait for UI to load
				time.Sleep(2 * time.Second)
				return nil
			}),
			harness.NewStep("Verify only hot context files are listed", func(ctx *harness.Context) error {
				session := ctx.Get("list_session").(*tui.Session)

				// Wait for either list content or error message
				result, err := session.WaitForAnyText([]string{
					"Files in Hot Context",
					"hot.go",
					"Error:",
				}, 8*time.Second)

				if err != nil {
					// Capture to see what's actually there
					content, _ := session.Capture()
					fmt.Printf("\n=== TUI Content (no expected text found) ===\n%s\n=== END ===\n", content)
					return fmt.Errorf("TUI did not load the list page: %w", err)
				}

				// If we found an error, capture and display it
				if result == "Error:" {
					content, _ := session.Capture()
					fmt.Printf("\n=== TUI showing error ===\n%s\n=== END ===\n", content)
					return fmt.Errorf("TUI is showing an error")
				}

				fmt.Printf("\n✓ Found text: %s\n", result)

				// Let UI stabilize
				if err := session.WaitForUIStable(2*time.Second, 100*time.Millisecond, 200*time.Millisecond); err != nil {
					fmt.Printf("   ⚠️  UI stability warning: %v\n", err)
				}

				content, err := session.Capture(tui.WithCleanedOutput())
				if err != nil {
					return fmt.Errorf("failed to capture screen: %w", err)
				}

				if !strings.Contains(content, "hot.go") {
					return fmt.Errorf("list view is missing hot context file 'hot.go'")
				}
				if strings.Contains(content, "cold.md") {
					return fmt.Errorf("list view should not contain cold context file 'cold.md'")
				}

				fmt.Printf("\n✓ List page loaded with expected content\n")
				return nil
			}),
			harness.NewStep("Quit the TUI", func(ctx *harness.Context) error {
				session := ctx.Get("list_session").(*tui.Session)
				return session.SendKeys("q")
			}),
		},
	}
}
