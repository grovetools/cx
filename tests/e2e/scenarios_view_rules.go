// File: grove-context/tests/e2e/scenarios_view_rules.go
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

// TUIViewRulesScenario tests launching `cx view` directly on the rules page.
func TUIViewRulesScenario() *harness.Scenario {
	return &harness.Scenario{
		Name:        "cx-view-rules-tui",
		Description: "Tests starting the 'cx view' TUI on the rules page and verifying its content.",
		Tags:        []string{"cx", "tui", "view", "rules"},
		Steps: []harness.Step{
			harness.NewStep("Create a detailed rules file and some test files", func(ctx *harness.Context) error {
				// Create test files so stats page has content
				fs.WriteString(filepath.Join(ctx.RootDir, "main.go"), "package main")
				fs.WriteString(filepath.Join(ctx.RootDir, "README.md"), "# README")

				rules := `# This is a comment
*.go
!*_test.go
---
# Cold context starts here
*.md
config.yml`
				return fs.WriteString(filepath.Join(ctx.RootDir, ".grove", "rules"), rules)
			}),
			harness.NewStep("Launch 'cx view' on rules page", func(ctx *harness.Context) error {
				cxBinary, err := FindProjectBinary()
				if err != nil {
					return err
				}

				session, err := ctx.StartTUI(cxBinary, []string{"view", "--page", "rules"},
					tui.WithEnv("CLICOLOR_FORCE=1"),
				)
				if err != nil {
					return fmt.Errorf("failed to start 'cx view' TUI: %w", err)
				}
				ctx.Set("rules_session", session)

				// Wait for UI to load
				time.Sleep(2 * time.Second)
				return nil
			}),
			harness.NewStep("Verify rules content is displayed", func(ctx *harness.Context) error {
				session := ctx.Get("rules_session").(*tui.Session)

				// Wait for either rules content or error message
				result, err := session.WaitForAnyText([]string{
					".grove/rules content:",
					"*.go",
					"Error:",
				}, 8*time.Second)

				if err != nil {
					// Capture to see what's actually there
					content, _ := session.Capture()
					fmt.Printf("\n=== TUI Content (no expected text found) ===\n%s\n=== END ===\n", content)
					return fmt.Errorf("TUI did not load the rules page: %w", err)
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

				if !strings.Contains(content, "# This is a comment") {
					return fmt.Errorf("rules view is missing comment from rules file")
				}
				if !strings.Contains(content, "*.go") {
					return fmt.Errorf("rules view is missing hot pattern '*.go'")
				}
				if !strings.Contains(content, "---") {
					return fmt.Errorf("rules view is missing cold context separator '---'")
				}
				if !strings.Contains(content, "*.md") {
					return fmt.Errorf("rules view is missing cold pattern '*.md'")
				}

				fmt.Printf("\n✓ Rules page loaded with expected content\n")
				return nil
			}),
			harness.NewStep("Test navigation to next page", func(ctx *harness.Context) error {
				session := ctx.Get("rules_session").(*tui.Session)
				// Press Tab to switch from 'rules' to 'stats' page
				if err := session.SendKeys("Tab"); err != nil {
					return err
				}
				// Wait for the stats page content to appear
				// Look for "Hot Context Statistics" or "Total Files:" which should appear on stats page
				_, err := session.WaitForAnyText([]string{
					"Hot Context Statistics",
					"Total Files:",
					"No files in hot context",
				}, 3*time.Second)
				return err
			}),
			harness.NewStep("Quit the TUI", func(ctx *harness.Context) error {
				session := ctx.Get("rules_session").(*tui.Session)
				return session.SendKeys("q")
			}),
		},
	}
}
