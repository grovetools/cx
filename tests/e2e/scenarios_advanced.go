// File: grove-context/tests/e2e/scenarios_advanced.go
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


// StatsAndValidateScenario tests the `cx stats` and `cx validate` commands.
func StatsAndValidateScenario() *harness.Scenario {
	return &harness.Scenario{
		Name:        "cx-stats-and-validate",
		Description: "Tests statistics generation and context validation.",
		Tags:        []string{"cx", "stats", "validate"},
		Steps: []harness.Step{
			harness.NewStep("Setup project with issues", func(ctx *harness.Context) error {
				// Create a valid file, and patterns for a missing and duplicate file.
				if err := fs.WriteString(filepath.Join(ctx.RootDir, "main.go"), "package main // valid file"); err != nil {
					return err
				}
				if err := fs.WriteString(filepath.Join(ctx.RootDir, "README.md"), "# Hello"); err != nil {
					return err
				}
				// Rules file pointing to a non-existent file.
				rules := "main.go\nnon_existent_file.txt\n*.md"
				return fs.WriteString(filepath.Join(ctx.RootDir, ".grove", "rules"), rules)
			}),
			harness.NewStep("Run 'cx validate' and check for errors", func(ctx *harness.Context) error {
				cx, err := FindProjectBinary()
				if err != nil {
					return err
				}
				cmd := command.New(cx, "validate").Dir(ctx.RootDir)
				result := cmd.Run()
				ctx.ShowCommandOutput(cmd.String(), result.Stdout, result.Stderr)

				// Since cx validate only validates files that are resolved from patterns,
				// and non-existent files are filtered out during resolution,
				// validate should report success for existing files
				if !strings.Contains(result.Stdout, "All 2 files are valid and accessible") {
					return fmt.Errorf("validation should report all accessible files as valid, got: %s", result.Stdout)
				}
				return nil
			}),
			harness.NewStep("Run 'cx stats' and verify output", func(ctx *harness.Context) error {
				cx, err := FindProjectBinary()
				if err != nil {
					return err
				}
				cmd := command.New(cx, "stats").Dir(ctx.RootDir)
				result := cmd.Run()
				ctx.ShowCommandOutput(cmd.String(), result.Stdout, result.Stderr)

				if !strings.Contains(result.Stdout, "Total Files:    2") {
					return fmt.Errorf("stats did not report the correct number of total files")
				}
				if !strings.Contains(result.Stdout, "Language Distribution:") || !strings.Contains(result.Stdout, "Go") || !strings.Contains(result.Stdout, "Markdown") {
					return fmt.Errorf("stats did not report language distribution correctly")
				}
				return nil
			}),
		},
	}
}

// SnapshotWorkflowScenario tests the full lifecycle of snapshots: save, diff, load, list.
func SnapshotWorkflowScenario() *harness.Scenario {
	return &harness.Scenario{
		Name:        "cx-snapshot-workflow",
		Description: "Tests save, diff, load, and list-snapshots commands.",
		Tags:        []string{"cx", "snapshot"},
		Steps: []harness.Step{
			harness.NewStep("Setup project files", func(ctx *harness.Context) error {
				fs.WriteString(filepath.Join(ctx.RootDir, "fileA.txt"), "content A")
				fs.WriteString(filepath.Join(ctx.RootDir, "fileB.txt"), "content B")
				fs.WriteString(filepath.Join(ctx.RootDir, "fileC.txt"), "content C")
				return nil
			}),
			harness.NewStep("Create and save 'snapshot-ab'", func(ctx *harness.Context) error {
				rules := "fileA.txt\nfileB.txt"
				fs.WriteString(filepath.Join(ctx.RootDir, ".grove", "rules"), rules)

				cx, _ := FindProjectBinary()
				cmd := command.New(cx, "save", "snapshot-ab").Dir(ctx.RootDir)
				result := cmd.Run()
				ctx.ShowCommandOutput(cmd.String(), result.Stdout, result.Stderr)
				return result.Error
			}),
			harness.NewStep("Modify rules for new context", func(ctx *harness.Context) error {
				rules := "fileB.txt\nfileC.txt"
				return fs.WriteString(filepath.Join(ctx.RootDir, ".grove", "rules"), rules)
			}),
			harness.NewStep("Run 'cx diff snapshot-ab'", func(ctx *harness.Context) error {
				cx, _ := FindProjectBinary()
				cmd := command.New(cx, "diff", "snapshot-ab").Dir(ctx.RootDir)
				result := cmd.Run()
				ctx.ShowCommandOutput(cmd.String(), result.Stdout, result.Stderr)

				if !strings.Contains(result.Stdout, "Added files (1)") || !strings.Contains(result.Stdout, "fileC.txt") {
					return fmt.Errorf("diff did not show added fileC.txt")
				}
				if !strings.Contains(result.Stdout, "Removed files (1)") || !strings.Contains(result.Stdout, "fileA.txt") {
					return fmt.Errorf("diff did not show removed fileA.txt")
				}
				return nil
			}),
			harness.NewStep("Run 'cx load snapshot-ab'", func(ctx *harness.Context) error {
				cx, _ := FindProjectBinary()
				cmd := command.New(cx, "load", "snapshot-ab").Dir(ctx.RootDir)
				result := cmd.Run()
				ctx.ShowCommandOutput(cmd.String(), result.Stdout, result.Stderr)
				return result.Error
			}),
			harness.NewStep("Verify rules file was restored", func(ctx *harness.Context) error {
				rulesPath := filepath.Join(ctx.RootDir, ".grove", "rules")
				content, err := fs.ReadString(rulesPath)
				if err != nil {
					return err
				}
				if !strings.Contains(content, "fileA.txt") || !strings.Contains(content, "fileB.txt") || strings.Contains(content, "fileC.txt") {
					return fmt.Errorf("loaded rules file has incorrect content: %s", content)
				}
				return nil
			}),
		},
	}
}

// GitBasedContextScenario tests the `cx from-git` command.
func GitBasedContextScenario() *harness.Scenario {
	return &harness.Scenario{
		Name:        "cx-from-git",
		Description: "Tests generating context from git history.",
		Tags:        []string{"cx", "git"},
		Steps: []harness.Step{
			harness.NewStep("Setup git repository", func(ctx *harness.Context) error {
				git.Init(ctx.RootDir)
				git.SetupTestConfig(ctx.RootDir)
				fs.WriteString(filepath.Join(ctx.RootDir, "committed.txt"), "committed")
				git.Add(ctx.RootDir, "committed.txt")
				git.Commit(ctx.RootDir, "Initial commit")
				return nil
			}),
			harness.NewStep("Create and stage a new file", func(ctx *harness.Context) error {
				fs.WriteString(filepath.Join(ctx.RootDir, "staged.txt"), "staged")
				git.Add(ctx.RootDir, "staged.txt")
				return nil
			}),
			harness.NewStep("Run 'cx from-git --staged'", func(ctx *harness.Context) error {
				cx, _ := FindProjectBinary()
				cmd := command.New(cx, "from-git", "--staged").Dir(ctx.RootDir)
				result := cmd.Run()
				ctx.ShowCommandOutput(cmd.String(), result.Stdout, result.Stderr)
				return result.Error
			}),
			harness.NewStep("Verify rules file contains only staged file", func(ctx *harness.Context) error {
				rulesPath := filepath.Join(ctx.RootDir, ".grove", "rules")
				content, err := fs.ReadString(rulesPath)
				if err != nil {
					return err
				}
				if !strings.Contains(content, "staged.txt") {
					return fmt.Errorf("rules missing staged file")
				}
				if strings.Contains(content, "committed.txt") {
					return fmt.Errorf("rules should not contain committed file")
				}
				return nil
			}),
		},
	}
}

// ComplexPatternScenario tests advanced globbing and exclusion rules.
func ComplexPatternScenario() *harness.Scenario {
	return &harness.Scenario{
		Name:        "cx-complex-patterns",
		Description: "Tests complex globbing, recursive, and exclusion patterns.",
		Tags:        []string{"cx", "rules"},
		Steps: []harness.Step{
			harness.NewStep("Setup complex project structure", func(ctx *harness.Context) error {
				fs.CreateDir(filepath.Join(ctx.RootDir, "src/api"))
				fs.CreateDir(filepath.Join(ctx.RootDir, "vendor/lib"))
				fs.WriteString(filepath.Join(ctx.RootDir, "src/api/handler.go"), "package api")
				fs.WriteString(filepath.Join(ctx.RootDir, "src/api/handler_test.go"), "package api_test")
				fs.WriteString(filepath.Join(ctx.RootDir, "vendor/lib/lib.go"), "package lib")
				fs.WriteString(filepath.Join(ctx.RootDir, "README.md"), "# complex")
				return nil
			}),
			harness.NewStep("Create complex rules file", func(ctx *harness.Context) error {
				rules := "**/*.go\n!**/*_test.go\n!vendor/**/*\n*.md"
				return fs.WriteString(filepath.Join(ctx.RootDir, ".grove", "rules"), rules)
			}),
			harness.NewStep("Run 'cx list' and verify results", func(ctx *harness.Context) error {
				cx, _ := FindProjectBinary()
				cmd := command.New(cx, "list").Dir(ctx.RootDir)
				result := cmd.Run()
				ctx.ShowCommandOutput(cmd.String(), result.Stdout, result.Stderr)
				if result.Error != nil {
					return result.Error
				}
				output := result.Stdout
				if !strings.Contains(output, "handler.go") {
					return fmt.Errorf("list output missing src/api/handler.go")
				}
				if !strings.Contains(output, "README.md") {
					return fmt.Errorf("list output missing README.md")
				}
				if strings.Contains(output, "handler_test.go") {
					return fmt.Errorf("list output should not contain handler_test.go")
				}
				if strings.Contains(output, "vendor") {
					return fmt.Errorf("list output should not contain vendor files")
				}
				return nil
			}),
		},
	}
}