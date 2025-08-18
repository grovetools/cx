// File: tests/e2e/scenarios_hybrid_context.go
package main

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/mattsolo1/grove-tend/pkg/command"
	"github.com/mattsolo1/grove-tend/pkg/fs"
	"github.com/mattsolo1/grove-tend/pkg/git"
	"github.com/mattsolo1/grove-tend/pkg/harness"
)

// DualContextWorkflowScenario tests the complete workflow for dual hot/cold contexts.
func DualContextWorkflowScenario() *harness.Scenario {
	return &harness.Scenario{
		Name:        "cx-dual-context-workflow",
		Description: "Tests generation, exclusion precedence, and command behavior with dual contexts.",
		Tags:        []string{"cx", "hybrid", "rules"},
		Steps: []harness.Step{
			harness.NewStep("Setup project with hybrid rules file", func(ctx *harness.Context) error {
				git.Init(ctx.RootDir)
				fs.CreateDir(filepath.Join(ctx.RootDir, "src"))
				fs.WriteString(filepath.Join(ctx.RootDir, "src", "main.go"), "package main // main content")
				fs.WriteString(filepath.Join(ctx.RootDir, "src", "utils.go"), "package main // utils content")
				fs.WriteString(filepath.Join(ctx.RootDir, "README.md"), "# Project README")

				// 'src/utils.go' matches both hot and cold patterns, testing precedence.
				rules := `# Hot context: frequently changing files
**/*.go
*.md
---
# Cold/Cached context: stable files
src/utils.go
`
				return fs.WriteString(filepath.Join(ctx.RootDir, ".grove", "rules"), rules)
			}),
			harness.NewStep("Run 'cx generate' and verify output files", func(ctx *harness.Context) error {
				cx, err := FindProjectBinary()
				if err != nil {
					return err
				}
				cmd := command.New(cx, "generate").Dir(ctx.RootDir)
				result := cmd.Run()
				ctx.ShowCommandOutput(cmd.String(), result.Stdout, result.Stderr)
				if result.Error != nil {
					return result.Error
				}

				// Verify main context file
				mainContextPath := filepath.Join(ctx.RootDir, ".grove", "context")
				mainContent, err := fs.ReadString(mainContextPath)
				if err != nil {
					return err
				}
				if !strings.Contains(mainContent, "src/main.go") {
					return fmt.Errorf("main context missing 'src/main.go'")
				}
				if !strings.Contains(mainContent, "README.md") {
					return fmt.Errorf("main context missing 'README.md'")
				}
				if strings.Contains(mainContent, "src/utils.go") {
					return fmt.Errorf("main context should not contain 'src/utils.go' due to cold context precedence")
				}

				// Verify cached context file
				cachedContextPath := filepath.Join(ctx.RootDir, ".grove", "cached-context-files")
				cachedContent, err := fs.ReadString(cachedContextPath)
				if err != nil {
					return err
				}
				if strings.TrimSpace(cachedContent) != "src/utils.go" {
					return fmt.Errorf("cached context file should contain only 'src/utils.go', got: %s", cachedContent)
				}
				return nil
			}),
			harness.NewStep("Verify 'cx list' operates only on hot context", func(ctx *harness.Context) error {
				cx, _ := FindProjectBinary()
				cmd := command.New(cx, "list").Dir(ctx.RootDir)
				result := cmd.Run()
				ctx.ShowCommandOutput(cmd.String(), result.Stdout, result.Stderr)

				if !strings.Contains(result.Stdout, "src/main.go") || !strings.Contains(result.Stdout, "README.md") {
					return fmt.Errorf("'cx list' output is missing hot context files")
				}
				if strings.Contains(result.Stdout, "src/utils.go") {
					return fmt.Errorf("'cx list' should not include cold context files")
				}
				return result.Error
			}),
			harness.NewStep("Verify 'cx stats' shows both hot and cold context statistics", func(ctx *harness.Context) error {
				cx, _ := FindProjectBinary()
				cmd := command.New(cx, "stats").Dir(ctx.RootDir)
				result := cmd.Run()
				ctx.ShowCommandOutput(cmd.String(), result.Stdout, result.Stderr)

				// Verify Hot Context Statistics
				if !strings.Contains(result.Stdout, "Hot Context Statistics:") {
					return fmt.Errorf("output missing 'Hot Context Statistics:' header")
				}
				if !strings.Contains(result.Stdout, "Total Files:    2") {
					return fmt.Errorf("hot context should report 2 files, got: %s", result.Stdout)
				}
				
				// Verify Cold Context Statistics
				if !strings.Contains(result.Stdout, "Cold (Cached) Context Statistics:") {
					return fmt.Errorf("output missing 'Cold (Cached) Context Statistics:' header")
				}
				
				// Verify the separator line between contexts
				if !strings.Contains(result.Stdout, "──────────────────────────────────────────────────") {
					return fmt.Errorf("output missing separator between hot and cold contexts")
				}
				
				// Verify hot context contains main.go and README.md
				if !strings.Contains(result.Stdout, "src/main.go") || !strings.Contains(result.Stdout, "README.md") {
					return fmt.Errorf("hot context should list src/main.go and README.md in largest files")
				}
				
				// Verify cold context contains utils.go
				if !strings.Contains(result.Stdout, "src/utils.go") {
					return fmt.Errorf("cold context should list src/utils.go")
				}
				
				// Verify language distribution shows both Go and Markdown for hot context
				if !strings.Contains(result.Stdout, "Go") || !strings.Contains(result.Stdout, "Markdown") {
					return fmt.Errorf("hot context should show both Go and Markdown in language distribution")
				}
				
				return result.Error
			}),
		},
	}
}

// NoSeparatorBackwardCompatibilityScenario tests behavior without the '---' separator.
func NoSeparatorBackwardCompatibilityScenario() *harness.Scenario {
	return &harness.Scenario{
		Name:        "cx-no-separator-compatibility",
		Description: "Tests that grove-context works as before when no '---' separator is present.",
		Tags:        []string{"cx", "hybrid", "rules"},
		Steps: []harness.Step{
			harness.NewStep("Setup project with standard rules file", func(ctx *harness.Context) error {
				fs.WriteString(filepath.Join(ctx.RootDir, "main.go"), "package main")
				fs.WriteString(filepath.Join(ctx.RootDir, "README.md"), "# Test")
				rules := "*.go\n*.md"
				return fs.WriteString(filepath.Join(ctx.RootDir, ".grove", "rules"), rules)
			}),
			harness.NewStep("Run 'cx generate' and verify output", func(ctx *harness.Context) error {
				cx, _ := FindProjectBinary()
				cmd := command.New(cx, "generate").Dir(ctx.RootDir)
				result := cmd.Run()
				ctx.ShowCommandOutput(cmd.String(), result.Stdout, result.Stderr)
				if result.Error != nil {
					return result.Error
				}

				// Verify main context
				mainContextPath := filepath.Join(ctx.RootDir, ".grove", "context")
				mainContent, err := fs.ReadString(mainContextPath)
				if err != nil {
					return err
				}
				if !strings.Contains(mainContent, "main.go") || !strings.Contains(mainContent, "README.md") {
					return fmt.Errorf("main context should contain both files")
				}

				// Verify cached context file does not exist or is empty
				cachedContextPath := filepath.Join(ctx.RootDir, ".grove", "cached-context-files")
				if fs.Exists(cachedContextPath) {
					content, _ := fs.ReadString(cachedContextPath)
					if content != "" {
						return fmt.Errorf("cached-context-files should be empty, but has content")
					}
				}
				return nil
			}),
		},
	}
}

// EmptyColdContextScenario tests behavior when '---' is present but the cold section is empty.
func EmptyColdContextScenario() *harness.Scenario {
	return &harness.Scenario{
		Name:        "cx-empty-cold-context",
		Description: "Tests behavior when '---' exists but no cold patterns are defined.",
		Tags:        []string{"cx", "hybrid", "rules"},
		Steps: []harness.Step{
			harness.NewStep("Setup project with empty cold context rules", func(ctx *harness.Context) error {
				fs.WriteString(filepath.Join(ctx.RootDir, "main.go"), "package main")
				fs.WriteString(filepath.Join(ctx.RootDir, "README.md"), "# Test")
				rules := "*.go\n---\n"
				return fs.WriteString(filepath.Join(ctx.RootDir, ".grove", "rules"), rules)
			}),
			harness.NewStep("Run 'cx generate' and verify output", func(ctx *harness.Context) error {
				cx, _ := FindProjectBinary()
				cmd := command.New(cx, "generate").Dir(ctx.RootDir)
				result := cmd.Run()
				ctx.ShowCommandOutput(cmd.String(), result.Stdout, result.Stderr)
				if result.Error != nil {
					return result.Error
				}

				// Verify main context
				mainContextPath := filepath.Join(ctx.RootDir, ".grove", "context")
				mainContent, err := fs.ReadString(mainContextPath)
				if err != nil {
					return err
				}
				if !strings.Contains(mainContent, "main.go") {
					return fmt.Errorf("main context should contain main.go")
				}
				if strings.Contains(mainContent, "README.md") {
					return fmt.Errorf("main context should not contain README.md")
				}

				// Verify cached context file exists and is empty
				cachedContextPath := filepath.Join(ctx.RootDir, ".grove", "cached-context-files")
				if !fs.Exists(cachedContextPath) {
					return fmt.Errorf("cached-context-files should exist")
				}
				content, err := fs.ReadString(cachedContextPath)
				if err != nil {
					return err
				}
				if content != "" {
					return fmt.Errorf("cached-context-files should be empty, but has content")
				}
				return nil
			}),
		},
	}
}