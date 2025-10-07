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

// TUIViewTreeScenario tests the interactive `cx view` tree page.
func TUIViewTreeScenario() *harness.Scenario {
	return &harness.Scenario{
		Name:        "cx-view-tui-tree",
		Description: "Tests the interactive `cx view` tree page for correctness.",
		Tags:        []string{"cx", "tui", "view", "tree"},
		Steps: []harness.Step{
			// harness.NewStep("Clean up existing tmux sessions", func(ctx *harness.Context) error {
			// 	if err := CleanupExistingTestSessions(); err != nil {
			// 		fmt.Printf("   ⚠️  Warning: Could not clean existing sessions: %v\n", err)
			// 	}
			// 	return nil
			// }),
			harness.NewStep("Setup project with diverse file statuses", func(ctx *harness.Context) error {
				// Create a file structure that will test all status types.
				fs.WriteString(filepath.Join(ctx.RootDir, "hot-file.go"), "package main // Should be in hot context")
				fs.CreateDir(filepath.Join(ctx.RootDir, "docs"))
				fs.WriteString(filepath.Join(ctx.RootDir, "docs", "guide.md"), "# Guide // Should be in cold context")
				fs.WriteString(filepath.Join(ctx.RootDir, "hot-file_test.go"), "package main // Should be excluded")
				fs.WriteString(filepath.Join(ctx.RootDir, "untracked.txt"), "Omitted file")

				// Define rules to classify the files.
				rules := `# Hot context rules
**/*.go
!**/*_test.go
---
# Cold context rules
docs/**/*.md`
				return fs.WriteString(filepath.Join(ctx.RootDir, ".grove", "rules"), rules)
			}),
			harness.NewStep("Launch 'cx view' on tree page", func(ctx *harness.Context) error {
				cxBinary, err := FindProjectBinary()
				if err != nil {
					return err
				}

				// Start the TUI on the 'tree' page in an isolated tmux session.
				session, err := ctx.StartTUI(cxBinary, []string{"view", "--page", "tree"}, tui.WithEnv("CLICOLOR_FORCE=1"))
				if err != nil {
					return fmt.Errorf("failed to start 'cx view' TUI: %w", err)
				}
				ctx.Set("view_session", session)

				// Wait for a key element to ensure the TUI has loaded.
				return session.WaitForText("hot-file.go", 5*time.Second)
			}),
			harness.NewStep("Verify initial file status indicators", func(ctx *harness.Context) error {
				session := ctx.Get("view_session").(*tui.Session)
				content, err := session.Capture(tui.WithCleanedOutput())
				if err != nil {
					return fmt.Errorf("failed to capture screen: %w", err)
				}

				// Debug: print the captured content
				fmt.Printf("\n=== DEBUG: Captured TUI Content ===\n%s\n=== END DEBUG ===\n", content)

				// Verify hot file - status symbol comes AFTER the filename
				if !strings.Contains(content, "hot-file.go") || !strings.Contains(content, "✓") {
					return fmt.Errorf("expected 'hot-file.go' with hot context indicator (✓)")
				}

				// Verify excluded file - status symbol comes AFTER the filename
				if !strings.Contains(content, "hot-file_test.go") || !strings.Contains(content, "🚫") {
					return fmt.Errorf("expected 'hot-file_test.go' with excluded indicator (🚫)")
				}

				// Verify omitted file (no indicator)
				if !strings.Contains(content, "untracked.txt") {
					return fmt.Errorf("'untracked.txt' should be visible")
				}
				// Verify untracked.txt doesn't have status indicators (should not have ✓ or 🚫 on the same line)
				lines := strings.Split(content, "\n")
				for _, line := range lines {
					if strings.Contains(line, "untracked.txt") {
						if strings.Contains(line, "✓") || strings.Contains(line, "🚫") || strings.Contains(line, "❄️") {
							return fmt.Errorf("'untracked.txt' should not have any status indicator, but line contains: %s", line)
						}
						break
					}
				}

				// Verify collapsed directory - expandIndicator comes before icon
				if !strings.Contains(content, "▶") || !strings.Contains(content, "docs") {
					return fmt.Errorf("'docs' directory should be visible and collapsed (▶)")
				}
				return nil
			}),
			harness.NewStep("Test directory expansion and verify cold indicator", func(ctx *harness.Context) error {
				session := ctx.Get("view_session").(*tui.Session)

				// Navigate to the 'docs' directory. `NavigateToText` is robust against layout changes.
				if err := session.NavigateToText("docs"); err != nil {
					return fmt.Errorf("failed to navigate to 'docs' directory: %w", err)
				}

				// Press Enter to expand the directory.
				if err := session.SendKeys("enter"); err != nil {
					return fmt.Errorf("failed to send 'enter' key: %w", err)
				}

				// Wait for the UI to redraw and stabilize after expansion.
				if err := session.WaitForUIStable(2*time.Second, 100*time.Millisecond, 200*time.Millisecond); err != nil {
					return fmt.Errorf("UI did not stabilize after expanding directory: %w", err)
				}

				content, err := session.Capture(tui.WithCleanedOutput())
				if err != nil {
					return fmt.Errorf("failed to capture screen after expansion: %w", err)
				}

				// Debug: print the captured content after expansion
				fmt.Printf("\n=== DEBUG: After Expansion ===\n%s\n=== END DEBUG ===\n", content)

				// Verify directory is now expanded (▼ should be present, docs should still be there)
				if !strings.Contains(content, "▼") || !strings.Contains(content, "docs") {
					return fmt.Errorf("expected 'docs' directory to be expanded (▼)")
				}
				// Verify guide.md is visible with cold indicator (❄️ comes AFTER filename)
				if !strings.Contains(content, "guide.md") || !strings.Contains(content, "❄️") {
					return fmt.Errorf("expected 'guide.md' to be visible with cold context indicator (❄️)")
				}
				return nil
			}),
			harness.NewStep("Verify ANSI color codes are present", func(ctx *harness.Context) error {
				session := ctx.Get("view_session").(*tui.Session)
				content, err := session.Capture(tui.WithRawOutput())
				if err != nil {
					return fmt.Errorf("failed to capture raw output: %w", err)
				}

				// A simple but effective check for ANSI codes.
				if !strings.Contains(content, "\x1b[") {
					return fmt.Errorf("no ANSI escape codes found; colors are not being rendered")
				}
				// Check for 24-bit color codes, which indicates the theme is working.
				if !strings.Contains(content, "[38;2;") {
					return fmt.Errorf("no RGB color codes found; theme may not be applied correctly")
				}
				return nil
			}),
			harness.NewStep("Quit the TUI", func(ctx *harness.Context) error {
				session := ctx.Get("view_session").(*tui.Session)
				return session.SendKeys("q")
			}),
		},
	}
}

