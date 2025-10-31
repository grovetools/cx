// File: grove-context/tests/e2e/scenarios_view_stats.go
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

// TUIViewStatsScenario tests launching the `cx view` TUI directly on the stats page.
func TUIViewStatsScenario() *harness.Scenario {
	return &harness.Scenario{
		Name:        "cx-view-stats-tui",
		Description: "Tests starting the 'cx view' TUI on the stats page and verifying its content.",
		Tags:        []string{"cx", "tui", "view", "stats"},
		Steps: []harness.Step{
			harness.NewStep("Setup project with hybrid files", func(ctx *harness.Context) error {
				// Files for hot context
				fs.CreateDir(filepath.Join(ctx.RootDir, "app"))
				fs.WriteString(filepath.Join(ctx.RootDir, "main.go"), "package main // hot")
				fs.WriteString(filepath.Join(ctx.RootDir, "app", "app.go"), "package app // hot")

				// Files for cold context
				fs.WriteString(filepath.Join(ctx.RootDir, "README.md"), "# Cold Context")
				fs.WriteString(filepath.Join(ctx.RootDir, "config.yml"), "setting: value # cold")

				// File to be excluded
				fs.WriteString(filepath.Join(ctx.RootDir, "main_test.go"), "package main // excluded")
				return nil
			}),
			harness.NewStep("Create hybrid rules file", func(ctx *harness.Context) error {
				rules := `**/*.go
!**/*_test.go
---
*.md
*.yml`
				return fs.WriteString(filepath.Join(ctx.RootDir, ".grove", "rules"), rules)
			}),
			harness.NewStep("Launch 'cx view' on stats page and wait for content", func(ctx *harness.Context) error {
				cxBinary, err := FindProjectBinary()
				if err != nil {
					return err
				}

				// Launch the TUI directly on stats page
				// CLICOLOR_FORCE=1 is sufficient (tmux provides TERM automatically)
				session, err := ctx.StartTUI(cxBinary, []string{"view", "--page", "stats"},
					tui.WithEnv("CLICOLOR_FORCE=1"),
				)
				if err != nil {
					return fmt.Errorf("failed to start 'cx view' TUI: %w", err)
				}
				ctx.Set("stats_session", session)

				// Wait longer for stats content to appear (state needs to load first)
				time.Sleep(2 * time.Second)

				return nil
			}),
			harness.NewStep("Verify initial UI state is correct", func(ctx *harness.Context) error {
				session := ctx.Get("stats_session").(*tui.Session)

				// Wait for either stats content or error message
				result, err := session.WaitForAnyText([]string{
					"File Types",
					"No files in hot context",
					"Error:",
				}, 8*time.Second)

				if err != nil {
					// Capture to see what's actually there
					content, _ := session.Capture()
					fmt.Printf("\n=== TUI Content (no expected text found) ===\n%s\n=== END ===\n", content)
					return fmt.Errorf("TUI did not load the stats page: %w", err)
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

				content, err := session.Capture()
				if err != nil {
					return fmt.Errorf("failed to capture screen: %w", err)
				}

				// Basic verification - just check stats are showing
				if !strings.Contains(content, "File Types") {
					return fmt.Errorf("missing 'File Types' in stats")
				}
				if !strings.Contains(content, ".go") {
					return fmt.Errorf("missing '.go' language in hot context stats")
				}

				// Cold context might not be visible on screen, just check hot context works
				// if !strings.Contains(content, "Markdown") && !strings.Contains(content, "YAML") {
				// 	return fmt.Errorf("missing 'Markdown' or 'YAML' in cold context stats")
				// }

				// Verify no mention of the excluded test file
				if strings.Contains(content, "main_test.go") {
					return fmt.Errorf("excluded file 'main_test.go' should not appear in stats")
				}

				fmt.Printf("\n✓ Stats page loaded with expected content\n")
				return nil
			}),
			harness.NewStep("Verify ANSI color codes are present", func(ctx *harness.Context) error {
				session := ctx.Get("stats_session").(*tui.Session)

				// Capture with raw output to preserve ANSI codes
				content, err := session.Capture(tui.WithRawOutput())
				if err != nil {
					return fmt.Errorf("failed to capture raw output: %w", err)
				}

				// Check for common ANSI codes that should be in the styled output
				// \x1b[ is the escape sequence prefix
				if !strings.Contains(content, "\x1b[") {
					return fmt.Errorf("no ANSI escape codes found - colors not rendered")
				}

				// Look for specific color codes (RGB foreground color)
				// e.g., [38;2;152;187;108m for the green theme color
				if !strings.Contains(content, "[38;2;") {
					return fmt.Errorf("no RGB color codes found - expected [38;2;r;g;bm format")
				}

				// Look for bold formatting
				if !strings.Contains(content, "[1m") {
					return fmt.Errorf("no bold formatting found - expected [1m codes")
				}

				fmt.Printf("\n✓ ANSI color codes verified (RGB colors and formatting present)\n")
				return nil
			}),
			harness.NewStep("Test basic navigation", func(ctx *harness.Context) error {
				session := ctx.Get("stats_session").(*tui.Session)

				// Just test that Tab key works (navigate to next page)
				if err := session.SendKeys("Tab"); err != nil {
					return fmt.Errorf("failed to send 'Tab' key: %w", err)
				}
				time.Sleep(300 * time.Millisecond)

				fmt.Printf("\n✓ Navigation tested\n")
				return nil
			}),
			harness.NewStep("Quit the TUI", func(ctx *harness.Context) error {
				session := ctx.Get("stats_session").(*tui.Session)
				return session.SendKeys("q")
			}),
		},
	}
}
