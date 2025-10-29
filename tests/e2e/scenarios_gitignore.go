// File: grove-context/tests/e2e/scenarios_gitignore.go
package main

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/mattsolo1/grove-tend/pkg/command"
	"github.com/mattsolo1/grove-tend/pkg/fs"
	"github.com/mattsolo1/grove-tend/pkg/harness"
)

// GitignoreStatsPerLineScenario tests that `cx stats --per-line` respects .gitignore
//
// This reproduces the bug where per-line stats (displayed in neovim) show incorrect file/token counts
// as if .gitignore is not being respected, even though `cx list` correctly excludes gitignored files.
//
// Example of the bug:
// - `cx list` with `*` shows 35 files (correct, respects gitignore)
// - Stats for `*` show "~25.1M tokens (9806 files)" (incorrect, includes gitignored files)
//
// Root cause: symlink resolution inconsistency between filepath.WalkDir and filepath.Abs/git commands
// on systems where temp directories involve symlinks (e.g., /var -> /private/var on macOS).
func GitignoreStatsPerLineScenario() *harness.Scenario {
	return &harness.Scenario{
		Name:        "cx-gitignore-stats-per-line",
		Description: "Tests that `cx stats --per-line` correctly respects .gitignore and doesn't count ignored files",
		Tags:        []string{"cx", "gitignore", "stats", "regression"},
		Steps: []harness.Step{
			harness.NewStep("Setup git repository with gitignored files", func(ctx *harness.Context) error {
				// Initialize git repo
				if result := command.New("git", "init").Dir(ctx.RootDir).Run(); result.Error != nil {
					return fmt.Errorf("failed to init git repo: %w", result.Error)
				}

				// Create .gitignore that excludes node_modules and dist
				gitignore := `node_modules/
dist/
*.log
`
				if err := fs.WriteString(filepath.Join(ctx.RootDir, ".gitignore"), gitignore); err != nil {
					return err
				}

				// Create files that should be included
				if err := fs.WriteString(filepath.Join(ctx.RootDir, "main.go"), "package main\n\nfunc main() {}\n"); err != nil {
					return err
				}
				if err := fs.WriteString(filepath.Join(ctx.RootDir, "helper.go"), "package main\n\nfunc helper() {}\n"); err != nil {
					return err
				}
				if err := fs.WriteString(filepath.Join(ctx.RootDir, "README.md"), "# Test Project\n"); err != nil {
					return err
				}

				// Create files that should be ignored
				// Create a node_modules directory with many files
				for i := 0; i < 100; i++ {
					pkgDir := filepath.Join(ctx.RootDir, "node_modules", fmt.Sprintf("package-%d", i))
					if err := fs.WriteString(filepath.Join(pkgDir, "index.js"), fmt.Sprintf("// Package %d\nmodule.exports = {}\n", i)); err != nil {
						return err
					}
					if err := fs.WriteString(filepath.Join(pkgDir, "package.json"), fmt.Sprintf(`{"name":"package-%d"}`, i)); err != nil {
						return err
					}
				}

				// Create a dist directory with files
				for i := 0; i < 50; i++ {
					bundleFile := fmt.Sprintf("bundle-%d.js", i)
					if err := fs.WriteString(filepath.Join(ctx.RootDir, "dist", bundleFile), fmt.Sprintf("// Bundle %d\nvar app%d;\n", i, i)); err != nil {
						return err
					}
				}

				// Create some log files that should be ignored
				for i := 0; i < 10; i++ {
					if err := fs.WriteString(filepath.Join(ctx.RootDir, fmt.Sprintf("debug-%d.log", i)), fmt.Sprintf("Log entry %d\n", i)); err != nil {
						return err
					}
				}

				return nil
			}),
			harness.NewStep("Create rules file with wildcard pattern", func(ctx *harness.Context) error {
				// Use a broad wildcard pattern that would include everything if not for .gitignore
				rules := `**/*`
				return fs.WriteString(filepath.Join(ctx.RootDir, ".grove", "rules"), rules)
			}),
			harness.NewStep("Verify cx list respects gitignore", func(ctx *harness.Context) error {
				cx, err := FindProjectBinary()
				if err != nil {
					return err
				}
				cmd := ctx.Command(cx, "list").Dir(ctx.RootDir)

				result := cmd.Run()
				ctx.ShowCommandOutput(cmd.String(), result.Stdout, result.Stderr)
				if result.Error != nil {
					return result.Error
				}

				output := result.Stdout
				lines := strings.Split(strings.TrimSpace(output), "\n")

				// Should have .gitignore, main.go, helper.go, README.md = 4 files
				// Allow some flexibility for hidden files that might be included
				if len(lines) > 10 {
					return fmt.Errorf("cx list returned %d files, expected around 4 (should exclude gitignored files). Files:\n%s", len(lines), output)
				}

				// Verify gitignored files are NOT in the list
				if strings.Contains(output, "node_modules") {
					return fmt.Errorf("cx list should not include node_modules files")
				}
				if strings.Contains(output, "dist") {
					return fmt.Errorf("cx list should not include dist files")
				}
				if strings.Contains(output, ".log") {
					return fmt.Errorf("cx list should not include .log files")
				}

				// Verify expected files ARE in the list
				if !strings.Contains(output, "main.go") {
					return fmt.Errorf("cx list should include main.go")
				}
				if !strings.Contains(output, "helper.go") {
					return fmt.Errorf("cx list should include helper.go")
				}

				return nil
			}),
			harness.NewStep("Run cx stats --per-line and verify counts respect gitignore", func(ctx *harness.Context) error {
				cx, err := FindProjectBinary()
				if err != nil {
					return err
				}
				cmd := ctx.Command(cx, "stats", "--per-line", ".grove/rules").Dir(ctx.RootDir)

				result := cmd.Run()
				ctx.ShowCommandOutput(cmd.String(), result.Stdout, result.Stderr)
				if result.Error != nil {
					return result.Error
				}

				output := result.Stdout

				// Parse JSON output
				var stats []map[string]interface{}
				if err := json.Unmarshal([]byte(output), &stats); err != nil {
					return fmt.Errorf("failed to parse JSON output: %w\nOutput:\n%s", err, output)
				}

				if len(stats) == 0 {
					return fmt.Errorf("expected at least one line of stats")
				}

				// Check the stats for line 1 (the **/* pattern)
				line1Stats := stats[0]

				fileCount, ok := line1Stats["fileCount"].(float64)
				if !ok {
					return fmt.Errorf("fileCount is not a number: %v", line1Stats["fileCount"])
				}

				// The file count should be around 4 (main.go, helper.go, README.md, .gitignore)
				// NOT 160+ (which would include all the node_modules and dist files)
				if fileCount > 10 {
					return fmt.Errorf("fileCount is %d, expected around 4 (should exclude gitignored files)\nFull stats: %v", int(fileCount), line1Stats)
				}

				// Verify totalTokens is reasonable (should be small, not millions)
				totalTokens, ok := line1Stats["totalTokens"].(float64)
				if !ok {
					return fmt.Errorf("totalTokens is not a number: %v", line1Stats["totalTokens"])
				}

				// With 4 small files, we should have well under 1000 tokens, not millions
				if totalTokens > 10000 {
					return fmt.Errorf("totalTokens is %.0f, expected < 10000 (should exclude gitignored files)\nFull stats: %v", totalTokens, line1Stats)
				}

				fmt.Printf("✓ Stats correctly show %d files with %.0f tokens (gitignored files excluded)\n", int(fileCount), totalTokens)
				return nil
			}),
		},
	}
}

// StarPatternRespectsGitignoreScenario tests that the `*` pattern respects .gitignore
//
// This is a regression test for the issue where a rules file with just `*` would incorrectly
// show gitignored directories like node_modules, dist, and coverage in the statistics,
// even though they should be automatically excluded by .gitignore.
//
// Reported issue:
// *                                                  ~45.0k tokens (78 files)
//
// !node_modules                                      ⚠ no matches
// !coverage                                          ⚠ no matches
// !dist                                              ⚠ no matches
//
// The `*` pattern was matching files before gitignore filtering, causing incorrect stats.
func StarPatternRespectsGitignoreScenario() *harness.Scenario {
	return &harness.Scenario{
		Name:        "cx-star-pattern-respects-gitignore",
		Description: "Tests that the `*` pattern respects .gitignore and doesn't require explicit exclusions",
		Tags:        []string{"cx", "gitignore", "pattern-matching", "regression"},
		Steps: []harness.Step{
			harness.NewStep("Setup git repository with common gitignored directories", func(ctx *harness.Context) error {
				// Initialize git repo
				if result := command.New("git", "init").Dir(ctx.RootDir).Run(); result.Error != nil {
					return fmt.Errorf("failed to init git repo: %w", result.Error)
				}

				// Create .gitignore with common directories that are usually ignored
				gitignore := `node_modules
dist
coverage
`
				if err := fs.WriteString(filepath.Join(ctx.RootDir, ".gitignore"), gitignore); err != nil {
					return err
				}

				// Create source files that should be included
				if err := fs.WriteString(filepath.Join(ctx.RootDir, "main.go"), "package main\n"); err != nil {
					return err
				}
				if err := fs.WriteString(filepath.Join(ctx.RootDir, "helper.go"), "package main\n"); err != nil {
					return err
				}

				// Create gitignored directories with files
				if err := fs.WriteString(filepath.Join(ctx.RootDir, "node_modules", "package.json"), "{}"); err != nil {
					return err
				}
				if err := fs.WriteString(filepath.Join(ctx.RootDir, "dist", "bundle.js"), "var app;"); err != nil {
					return err
				}
				if err := fs.WriteString(filepath.Join(ctx.RootDir, "coverage", "lcov.info"), "data"); err != nil {
					return err
				}

				return nil
			}),
			harness.NewStep("Create rules file with only * pattern", func(ctx *harness.Context) error {
				// Use just * pattern - it should respect gitignore without explicit exclusions
				rules := `*`
				return fs.WriteString(filepath.Join(ctx.RootDir, ".grove", "rules"), rules)
			}),
			harness.NewStep("Verify * pattern respects gitignore in cx list", func(ctx *harness.Context) error {
				cx, err := FindProjectBinary()
				if err != nil {
					return err
				}
				cmd := ctx.Command(cx, "list").Dir(ctx.RootDir)

				result := cmd.Run()
				ctx.ShowCommandOutput(cmd.String(), result.Stdout, result.Stderr)
				if result.Error != nil {
					return result.Error
				}

				output := result.Stdout

				// Verify gitignored directories are NOT in the list
				if strings.Contains(output, "node_modules") {
					return fmt.Errorf("* pattern should not include node_modules (it's in .gitignore)")
				}
				if strings.Contains(output, "dist") {
					return fmt.Errorf("* pattern should not include dist (it's in .gitignore)")
				}
				if strings.Contains(output, "coverage") {
					return fmt.Errorf("* pattern should not include coverage (it's in .gitignore)")
				}

				// Verify expected files ARE in the list
				if !strings.Contains(output, "main.go") {
					return fmt.Errorf("* pattern should include main.go")
				}
				if !strings.Contains(output, "helper.go") {
					return fmt.Errorf("* pattern should include helper.go")
				}

				return nil
			}),
			harness.NewStep("Verify * pattern stats don't show 'no matches' warnings", func(ctx *harness.Context) error {
				cx, err := FindProjectBinary()
				if err != nil {
					return err
				}
				cmd := ctx.Command(cx, "stats", "--per-line", ".grove/rules").Dir(ctx.RootDir)

				result := cmd.Run()
				ctx.ShowCommandOutput(cmd.String(), result.Stdout, result.Stderr)
				if result.Error != nil {
					return result.Error
				}

				output := result.Stdout

				// Parse JSON output
				var stats []map[string]interface{}
				if err := json.Unmarshal([]byte(output), &stats); err != nil {
					return fmt.Errorf("failed to parse JSON output: %w\nOutput:\n%s", err, output)
				}

				if len(stats) == 0 {
					return fmt.Errorf("expected stats for the * pattern")
				}

				// Check the stats for line 1 (the * pattern)
				line1Stats := stats[0]

				fileCount, ok := line1Stats["fileCount"].(float64)
				if !ok {
					return fmt.Errorf("fileCount is not a number: %v", line1Stats["fileCount"])
				}

				// Should have around 3-4 files (.gitignore, main.go, helper.go)
				// NOT including the gitignored files
				if fileCount > 10 {
					return fmt.Errorf("* pattern shows %d files, expected around 3-4 (should exclude gitignored dirs)\nFull stats: %v", int(fileCount), line1Stats)
				}

				fmt.Printf("✓ * pattern correctly shows %d files (gitignored dirs excluded)\n", int(fileCount))
				return nil
			}),
		},
	}
}
