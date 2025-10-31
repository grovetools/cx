// File: grove-context/tests/e2e/scenarios_advanced.go
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mattsolo1/grove-tend/pkg/fs"
	"github.com/mattsolo1/grove-tend/pkg/git"
	"github.com/mattsolo1/grove-tend/pkg/harness"
	"github.com/mattsolo1/grove-tend/pkg/tui"
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
				cmd := ctx.Command(cx, "validate").Dir(ctx.RootDir)
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
				cmd := ctx.Command(cx, "stats").Dir(ctx.RootDir)
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
				cmd := ctx.Command(cx, "save", "snapshot-ab").Dir(ctx.RootDir)
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
				cmd := ctx.Command(cx, "diff", "snapshot-ab").Dir(ctx.RootDir)
				result := cmd.Run()
				ctx.ShowCommandOutput(cmd.String(), result.Stdout, result.Stderr)

				// Use Stderr since diff command uses prettyLog which outputs to stderr
				output := result.Stderr

				if !strings.Contains(output, "Added files (1)") || !strings.Contains(output, "fileC.txt") {
					return fmt.Errorf("diff did not show added fileC.txt")
				}
				if !strings.Contains(output, "Removed files (1)") || !strings.Contains(output, "fileA.txt") {
					return fmt.Errorf("diff did not show removed fileA.txt")
				}
				return nil
			}),
			harness.NewStep("Run 'cx load snapshot-ab'", func(ctx *harness.Context) error {
				cx, _ := FindProjectBinary()
				cmd := ctx.Command(cx, "load", "snapshot-ab").Dir(ctx.RootDir)
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
				cmd := ctx.Command(cx, "from-git", "--staged").Dir(ctx.RootDir)
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
				cmd := ctx.Command(cx, "list").Dir(ctx.RootDir)
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

// PlainDirectoryPatternScenario tests plain directory patterns like ../grove-flow
func PlainDirectoryPatternScenario() *harness.Scenario {
	return &harness.Scenario{
		Name:        "cx-plain-directory-pattern",
		Description: "Tests that plain directory patterns like ../grove-flow are treated as recursive includes.",
		Tags:        []string{"cx", "rules", "patterns", "directory"},
		Steps: []harness.Step{
			harness.NewStep("Create sibling projects with various files", func(ctx *harness.Context) error {
				// Create sibling directories
				parentDir := filepath.Dir(ctx.RootDir)
				groveFlowDir := filepath.Join(parentDir, "grove-flow")
				groveNotebookDir := filepath.Join(parentDir, "grove-notebook")
				nvimPluginDir := filepath.Join(groveNotebookDir, "nvim-plugin")

				// Create grove-flow structure
				fs.CreateDir(filepath.Join(groveFlowDir, "src"))
				fs.CreateDir(filepath.Join(groveFlowDir, "pkg/core"))
				fs.WriteString(filepath.Join(groveFlowDir, "main.go"), "package main")
				fs.WriteString(filepath.Join(groveFlowDir, "README.md"), "# Grove Flow")
				fs.WriteString(filepath.Join(groveFlowDir, "src/app.go"), "package src")
				fs.WriteString(filepath.Join(groveFlowDir, "pkg/core/flow.go"), "package core")

				// Create grove-notebook/nvim-plugin structure
				fs.CreateDir(filepath.Join(nvimPluginDir, "lua"))
				fs.CreateDir(filepath.Join(nvimPluginDir, "plugin"))
				fs.WriteString(filepath.Join(nvimPluginDir, "init.lua"), "-- Neovim plugin")
				fs.WriteString(filepath.Join(nvimPluginDir, "README.md"), "# Nvim Plugin")
				fs.WriteString(filepath.Join(nvimPluginDir, "lua/config.lua"), "-- Config")
				fs.WriteString(filepath.Join(nvimPluginDir, "plugin/main.vim"), "\" Main plugin")

				// Create local grove.yml with allowed_paths configuration
				groveConfig := fmt.Sprintf(`context:
  allowed_paths:
    - %s
`, parentDir)
				groveYmlPath := filepath.Join(ctx.RootDir, "grove.yml")
				if err := fs.WriteString(groveYmlPath, groveConfig); err != nil {
					return fmt.Errorf("failed to write grove.yml to %s: %w", groveYmlPath, err)
				}

				return nil
			}),
			harness.NewStep("Create rules with plain directory patterns", func(ctx *harness.Context) error {
				rules := `*
---
../grove-flow
../grove-notebook/nvim-plugin`
				return fs.WriteString(filepath.Join(ctx.RootDir, ".grove", "rules"), rules)
			}),
			harness.NewStep("Run 'cx generate' and verify files are in cached context", func(ctx *harness.Context) error {
				cx, _ := FindProjectBinary()

				// First generate the context
				// Use ctx.Command to ensure sandboxed environment variables are injected
				cmd := ctx.Command(cx, "generate").Dir(ctx.RootDir)
				result := cmd.Run()
				ctx.ShowCommandOutput(cmd.String(), result.Stdout, result.Stderr)
				if result.Error != nil {
					return result.Error
				}

				// Read the cached-context-files list to verify cold context files
				cachedFilesPath := filepath.Join(ctx.RootDir, ".grove", "cached-context-files")
				content, err := os.ReadFile(cachedFilesPath)
				if err != nil {
					return fmt.Errorf("failed to read cached-context-files: %v", err)
				}

				output := string(content)

				// Check that files from grove-flow are included (cold context section)
				expectedFlowFiles := []string{
					"grove-flow/main.go",
					"grove-flow/README.md",
					"grove-flow/src/app.go",
					"grove-flow/pkg/core/flow.go",
				}

				// Check that files from grove-notebook/nvim-plugin are included
				expectedNvimFiles := []string{
					"grove-notebook/nvim-plugin/init.lua",
					"grove-notebook/nvim-plugin/README.md",
					"grove-notebook/nvim-plugin/lua/config.lua",
					"grove-notebook/nvim-plugin/plugin/main.vim",
				}

				allExpectedFiles := append(expectedFlowFiles, expectedNvimFiles...)

				missingFiles := []string{}
				for _, file := range allExpectedFiles {
					if !strings.Contains(output, file) {
						missingFiles = append(missingFiles, file)
					}
				}

				if len(missingFiles) > 0 {
					return fmt.Errorf("cached-context-files missing files: %v", missingFiles)
				}

				return nil
			}),
		},
	}
}

// RecursiveParentPatternScenario tests ** patterns with ../ prefix for files in sibling directories.
func RecursiveParentPatternScenario() *harness.Scenario {
	return &harness.Scenario{
		Name:        "cx-recursive-parent-patterns",
		Description: "Tests ** patterns with ../ prefix to match files in sibling directories recursively.",
		Tags:        []string{"cx", "rules", "recursive"},
		Steps: []harness.Step{
			harness.NewStep("Setup sibling project structure", func(ctx *harness.Context) error {
				// Create a parent directory with two sibling projects
				parentDir := filepath.Dir(ctx.RootDir)
				siblingDir := filepath.Join(parentDir, "sibling-project")

				// Clean up any existing sibling project first
				os.RemoveAll(siblingDir)

				// Create sibling project structure with nested directories
				fs.CreateDir(filepath.Join(siblingDir, "cmd"))
				fs.CreateDir(filepath.Join(siblingDir, "pkg/util"))
				fs.CreateDir(filepath.Join(siblingDir, "internal/core/db"))

				// Create Go files at various depths
				fs.WriteString(filepath.Join(siblingDir, "main.go"), "package main")
				fs.WriteString(filepath.Join(siblingDir, "cmd/root.go"), "package cmd")
				fs.WriteString(filepath.Join(siblingDir, "pkg/util/helper.go"), "package util")
				fs.WriteString(filepath.Join(siblingDir, "internal/core/db/manager.go"), "package db")

				// Create non-Go files to ensure pattern matching works
				fs.WriteString(filepath.Join(siblingDir, "README.md"), "# Sibling")
				fs.WriteString(filepath.Join(siblingDir, "pkg/util/config.json"), "{}")

				// Create local grove.yml with allowed_paths configuration
				groveConfig := fmt.Sprintf(`context:
  allowed_paths:
    - %s
`, parentDir)
				groveYmlPath := filepath.Join(ctx.RootDir, "grove.yml")
				if err := fs.WriteString(groveYmlPath, groveConfig); err != nil {
					return err
				}

				return nil
			}),
			harness.NewStep("Create rules with ../**/*.go pattern", func(ctx *harness.Context) error {
				rules := "../sibling-project/**/*.go"
				return fs.WriteString(filepath.Join(ctx.RootDir, ".grove", "rules"), rules)
			}),
			harness.NewStep("Run 'cx list' and verify recursive matching", func(ctx *harness.Context) error {
				cx, _ := FindProjectBinary()
				cmd := ctx.Command(cx, "list").Dir(ctx.RootDir)
				result := cmd.Run()
				ctx.ShowCommandOutput(cmd.String(), result.Stdout, result.Stderr)
				if result.Error != nil {
					return result.Error
				}

				output := result.Stdout

				// Check that files at all depths are included
				expectedFiles := []string{
					"main.go",                     // Root level
					"cmd/root.go",                 // First level subdirectory
					"pkg/util/helper.go",          // Second level subdirectory
					"internal/core/db/manager.go", // Third level subdirectory
				}

				for _, file := range expectedFiles {
					if !strings.Contains(output, file) {
						return fmt.Errorf("list output missing %s from sibling project", file)
					}
				}

				// Ensure non-Go files are not included
				if strings.Contains(output, "README.md") || strings.Contains(output, "config.json") {
					return fmt.Errorf("list output should not contain non-Go files")
				}

				// Count total files - should be exactly 4
				lines := strings.Split(strings.TrimSpace(output), "\n")
				if len(lines) != 4 {
					return fmt.Errorf("expected 4 files, got %d", len(lines))
				}

				return nil
			}),
		},
	}
}

// ExclusionPatternsScenario tests various exclusion patterns including gitignore-compatible ones
func ExclusionPatternsScenario() *harness.Scenario {
	return &harness.Scenario{
		Name:        "cx-exclusion-patterns",
		Description: "Tests exclusion patterns including gitignore-compatible patterns and cross-directory exclusions.",
		Tags:        []string{"cx", "rules", "exclusion"},
		Steps: []harness.Step{
			harness.NewStep("Setup project with test directories", func(ctx *harness.Context) error {
				// Create main project structure
				fs.CreateDir(filepath.Join(ctx.RootDir, "src"))
				fs.CreateDir(filepath.Join(ctx.RootDir, "tests/unit"))
				fs.CreateDir(filepath.Join(ctx.RootDir, "tests/integration"))
				fs.CreateDir(filepath.Join(ctx.RootDir, "pkg/tests"))
				fs.CreateDir(filepath.Join(ctx.RootDir, "internal/testutils"))

				// Create Go files
				fs.WriteString(filepath.Join(ctx.RootDir, "main.go"), "package main")
				fs.WriteString(filepath.Join(ctx.RootDir, "src/app.go"), "package src")
				fs.WriteString(filepath.Join(ctx.RootDir, "src/app_test.go"), "package src")
				fs.WriteString(filepath.Join(ctx.RootDir, "tests/unit/user_test.go"), "package unit")
				fs.WriteString(filepath.Join(ctx.RootDir, "tests/integration/api_test.go"), "package integration")
				fs.WriteString(filepath.Join(ctx.RootDir, "pkg/tests/helper.go"), "package tests")
				fs.WriteString(filepath.Join(ctx.RootDir, "internal/testutils/mock.go"), "package testutils")

				// Create sibling project for cross-directory testing
				parentDir := filepath.Dir(ctx.RootDir)
				siblingDir := filepath.Join(parentDir, "sibling-project")
				fs.CreateDir(filepath.Join(siblingDir, "cmd"))
				fs.CreateDir(filepath.Join(siblingDir, "tests/e2e"))
				fs.CreateDir(filepath.Join(siblingDir, "pkg/util"))
				fs.CreateDir(filepath.Join(siblingDir, "internal/core/db"))

				fs.WriteString(filepath.Join(siblingDir, "main.go"), "package main")
				fs.WriteString(filepath.Join(siblingDir, "cmd/cli.go"), "package cmd")
				fs.WriteString(filepath.Join(siblingDir, "cmd/root.go"), "package cmd")
				fs.WriteString(filepath.Join(siblingDir, "tests/e2e/flow_test.go"), "package e2e")
				fs.WriteString(filepath.Join(siblingDir, "pkg/util/helper.go"), "package util")
				fs.WriteString(filepath.Join(siblingDir, "internal/core/db/manager.go"), "package db")

				// Create local grove.yml with allowed_paths configuration
				groveConfig := fmt.Sprintf(`context:
  allowed_paths:
    - %s
`, parentDir)
				groveYmlPath := filepath.Join(ctx.RootDir, "grove.yml")
				if err := fs.WriteString(groveYmlPath, groveConfig); err != nil {
					return err
				}

				return nil
			}),
			harness.NewStep("Test !tests pattern (gitignore compatible)", func(ctx *harness.Context) error {
				rules := `**/*.go
!tests`
				fs.WriteString(filepath.Join(ctx.RootDir, ".grove", "rules"), rules)

				cx, _ := FindProjectBinary()
				cmd := ctx.Command(cx, "list").Dir(ctx.RootDir)
				result := cmd.Run()
				if result.Error != nil {
					return result.Error
				}

				output := result.Stdout

				// Should exclude any file in directories named "tests"
				if strings.Contains(output, "tests/unit/user_test.go") ||
					strings.Contains(output, "tests/integration/api_test.go") ||
					strings.Contains(output, "pkg/tests/helper.go") {
					return fmt.Errorf("!tests pattern should exclude files in 'tests' directories")
				}

				// Should NOT exclude testutils or files with test in the name
				if !strings.Contains(output, "src/app_test.go") {
					return fmt.Errorf("!tests pattern should not exclude files ending with _test.go")
				}
				if !strings.Contains(output, "internal/testutils/mock.go") {
					return fmt.Errorf("!tests pattern should not exclude 'testutils' directory")
				}

				return nil
			}),
			harness.NewStep("Test !**/tests/** pattern", func(ctx *harness.Context) error {
				rules := `**/*.go
!**/tests/**`
				fs.WriteString(filepath.Join(ctx.RootDir, ".grove", "rules"), rules)

				cx, _ := FindProjectBinary()
				cmd := ctx.Command(cx, "list").Dir(ctx.RootDir)
				result := cmd.Run()
				if result.Error != nil {
					return result.Error
				}

				// Same behavior as !tests for this case
				output := result.Stdout
				if strings.Contains(output, "tests/") {
					return fmt.Errorf("!**/tests/** should exclude all files under tests directories")
				}

				return nil
			}),
			harness.NewStep("Test cross-directory exclusions", func(ctx *harness.Context) error {
				rules := `../sibling-project/**/*.go
!tests`
				fs.WriteString(filepath.Join(ctx.RootDir, ".grove", "rules"), rules)

				cx, _ := FindProjectBinary()
				cmd := ctx.Command(cx, "list").Dir(ctx.RootDir)
				result := cmd.Run()
				if result.Error != nil {
					return result.Error
				}

				output := result.Stdout
				ctx.ShowCommandOutput(cmd.String(), output, result.Stderr)

				// Check that we got some files
				if output == "" {
					return fmt.Errorf("No files found. Expected files from sibling project")
				}

				// Should include files from sibling project (checking for path components)
				if !strings.Contains(output, "sibling-project") {
					return fmt.Errorf("Output should contain files from sibling-project. Got: %s", output)
				}

				// Should include main.go and cmd/cli.go
				if !strings.Contains(output, "main.go") || !strings.Contains(output, "cmd/cli.go") {
					return fmt.Errorf("Should include main.go and cmd/cli.go from sibling project. Got: %s", output)
				}

				// Should exclude test directories in sibling project
				if strings.Contains(output, "tests/e2e/flow_test.go") {
					return fmt.Errorf("Should exclude test directories from sibling project")
				}

				return nil
			}),
		},
	}
}

// WorktreeExclusionScenario tests that .grove-worktrees are excluded even with broad parent-directory patterns.
func WorktreeExclusionScenario() *harness.Scenario {
	return &harness.Scenario{
		Name:        "cx-worktree-exclusion",
		Description: "Ensures .grove-worktrees are excluded even when matched by broad ../ patterns.",
		Tags:        []string{"cx", "rules", "worktree"},
		Steps: []harness.Step{
			harness.NewStep("Setup sibling project with a worktree", func(ctx *harness.Context) error {
				// Harness root is the "main" project. Create a sibling project next to it.
				parentDir := filepath.Dir(ctx.RootDir)
				siblingDir := filepath.Join(parentDir, "sibling-lib")

				// Create a file that should be included
				fs.CreateDir(filepath.Join(siblingDir, "src"))
				fs.WriteString(filepath.Join(siblingDir, "src", "lib.go"), "package lib")

				// Create a file in a worktree that MUST be excluded
				fs.CreateDir(filepath.Join(siblingDir, ".grove-worktrees", "feature", "src"))
				fs.WriteString(filepath.Join(siblingDir, ".grove-worktrees", "feature", "src", "feature.go"), "package feature")

				// Create local grove.yml with allowed_paths configuration
				groveConfig := fmt.Sprintf(`context:
  allowed_paths:
    - %s
`, parentDir)
				groveYmlPath := filepath.Join(ctx.RootDir, "grove.yml")
				if err := fs.WriteString(groveYmlPath, groveConfig); err != nil {
					return err
				}

				return nil
			}),
			harness.NewStep("Create rules to include sibling project files", func(ctx *harness.Context) error {
				// This broad pattern would normally include the worktree file if not for our hardcoded exclusion.
				rules := `../sibling-lib/**/*.go`
				return fs.WriteString(filepath.Join(ctx.RootDir, ".grove", "rules"), rules)
			}),
			harness.NewStep("Run 'cx list' and verify exclusion", func(ctx *harness.Context) error {
				cx, _ := FindProjectBinary()
				cmd := ctx.Command(cx, "list").Dir(ctx.RootDir)
				result := cmd.Run()
				ctx.ShowCommandOutput(cmd.String(), result.Stdout, result.Stderr)
				if result.Error != nil {
					return result.Error
				}

				output := result.Stdout

				// Verify the legitimate file IS included
				if !strings.Contains(output, "lib.go") {
					return fmt.Errorf("list output is missing the legitimate sibling file 'lib.go'")
				}

				// Verify the worktree file IS NOT included
				if strings.Contains(output, ".grove-worktrees") || strings.Contains(output, "feature.go") {
					return fmt.Errorf("list output MUST NOT include files from .grove-worktrees")
				}
				return nil
			}),
		},
	}
}

// ExplicitWorktreeInclusionScenario tests that an explicit rule can bypass the .grove-worktrees exclusion.
func ExplicitWorktreeInclusionScenario() *harness.Scenario {
	return &harness.Scenario{
		Name:        "cx-explicit-worktree-inclusion",
		Description: "Ensures an explicit rule for a .grove-worktrees path includes the files.",
		Tags:        []string{"cx", "rules", "worktree", "regression"},
		Steps: []harness.Step{
			harness.NewStep("Setup sibling project with a worktree", func(ctx *harness.Context) error {
				// Create a directory outside the test's RootDir to simulate a real absolute path.
				externalDir, err := os.MkdirTemp("", "grove-e2e-abs-worktree-")
				if err != nil {
					return fmt.Errorf("failed to create external temp dir: %w", err)
				}
				ctx.Set("externalDir", externalDir) // Save for later steps and cleanup

				// Create a file in a worktree that we want to explicitly include
				worktreePath := filepath.Join(externalDir, "project-meta", ".grove-worktrees", "feature-branch")
				fs.CreateDir(worktreePath)
				fs.WriteString(filepath.Join(worktreePath, "feature.go"), "package feature")

				// Create a file that should be ignored by the rule
				fs.WriteString(filepath.Join(worktreePath, "README.md"), "ignore this")

				// Create local grove.yml with allowed_paths configuration
				groveConfig := fmt.Sprintf(`context:
  allowed_paths:
    - %s
`, externalDir)
				groveYmlPath := filepath.Join(ctx.RootDir, "grove.yml")
				if err := fs.WriteString(groveYmlPath, groveConfig); err != nil {
					return err
				}

				return nil
			}),
			harness.NewStep("Create rules to include the worktree directory", func(ctx *harness.Context) error {
				externalDir := ctx.Get("externalDir").(string)
				// This is an explicit rule that contains `.grove-worktrees`.
				// It should bypass the default exclusion.
				worktreePath := filepath.Join(externalDir, "project-meta", ".grove-worktrees", "feature-branch")
				rules := fmt.Sprintf("%s/**/*.go", worktreePath)
				return fs.WriteString(filepath.Join(ctx.RootDir, ".grove", "rules"), rules)
			}),
			harness.NewStep("Run 'cx list' and verify inclusion", func(ctx *harness.Context) error {
				externalDir := ctx.Get("externalDir").(string)
				defer os.RemoveAll(externalDir) // Cleanup after the test

				cx, _ := FindProjectBinary()
				cmd := ctx.Command(cx, "list").Dir(ctx.RootDir)
				result := cmd.Run()
				ctx.ShowCommandOutput(cmd.String(), result.Stdout, result.Stderr)
				if result.Error != nil {
					return result.Error
				}

				output := result.Stdout

				// Check if the output contains the worktree file
				if !strings.Contains(output, "feature.go") {
					return fmt.Errorf("list output is missing the explicitly included worktree file 'feature.go'")
				}
				return nil
			}),
		},
	}
}

// NonExistentPathInRulesScenario tests that `cx view` does not crash when a rule references a non-existent path.
func NonExistentPathInRulesScenario() *harness.Scenario {
	return &harness.Scenario{
		Name:        "cx-non-existent-path-in-rules",
		Description: "Tests that `cx view` does not crash when a rule references a non-existent path.",
		Tags:        []string{"cx", "rules", "view", "regression"},
		Steps: []harness.Step{
			harness.NewStep("Setup project with a rule for a non-existent path", func(ctx *harness.Context) error {
				// 1. Create a local file that should be visible in the view.
				if err := fs.WriteString(filepath.Join(ctx.RootDir, "main.go"), "package main"); err != nil {
					return err
				}

				// 2. Configure a default ruleset in grove.yml.
				groveYml := `name: test-project
context:
  default_rules_path: .cx/default.rules`
				if err := fs.WriteString(filepath.Join(ctx.RootDir, "grove.yml"), groveYml); err != nil {
					return err
				}

				// 3. Create the default ruleset with a pattern for a directory that does not exist.
				//    This is the problematic rule.
				defaultRules := `!non-existent-dir/**`
				if err := fs.WriteString(filepath.Join(ctx.RootDir, ".cx/default.rules"), defaultRules); err != nil {
					return err
				}

				// 4. Create the main rules file that includes a valid file and imports the default rules.
				mainRules := `main.go
@default: .`
				return fs.WriteString(filepath.Join(ctx.RootDir, ".grove/rules"), mainRules)
			}),
			harness.NewStep("Launch 'cx view' and verify it does not crash", func(ctx *harness.Context) error {
				cxBinary, err := FindProjectBinary()
				if err != nil {
					return err
				}

				// Start the TUI. Before the fix, this will fail because the process crashes.
				session, err := ctx.StartTUI(cxBinary, []string{"view"})
				if err != nil {
					return fmt.Errorf("failed to start 'cx view' TUI, it may have crashed: %w", err)
				}
				ctx.Set("view_session", session)

				// Wait for some text from the UI to confirm it has loaded successfully.
				if err := session.WaitForText("main.go", 5*time.Second); err != nil {
					content, _ := session.Capture()
					return fmt.Errorf("TUI did not display expected content 'main.go'. Got:\n%s", content)
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
