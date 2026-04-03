package main

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/grovetools/tend/pkg/command"
	"github.com/grovetools/tend/pkg/fs"
	"github.com/grovetools/tend/pkg/harness"
)

// EmptyPatternResolveScenario tests that empty, whitespace-only, and empty view patterns
// return a validation error instead of walking the root filesystem.
func EmptyPatternResolveScenario() *harness.Scenario {
	return &harness.Scenario{
		Name:        "cx-empty-pattern-resolve",
		Description: "Tests that empty, whitespace-only, and empty view patterns return an error and do not walk the filesystem.",
		Tags:        []string{"cx", "resolve", "regression"},
		Steps: []harness.Step{
			harness.NewStep("Run 'cx resolve' with empty string", func(ctx *harness.Context) error {
				cx, err := FindProjectBinary()
				if err != nil {
					return err
				}

				cmd := command.New(cx, "resolve", "").Dir(ctx.RootDir)
				result := cmd.Run()
				ctx.ShowCommandOutput(cmd.String(), result.Stdout, result.Stderr)

				if result.Error == nil {
					return fmt.Errorf("expected error when resolving empty pattern, got success")
				}
				if !strings.Contains(result.Stderr, "empty rule pattern provided") && !strings.Contains(result.Error.Error(), "empty rule pattern provided") {
					return fmt.Errorf("expected 'empty rule pattern provided' error, got: %v / %s", result.Error, result.Stderr)
				}
				return nil
			}),
			harness.NewStep("Run 'cx resolve' with whitespace string", func(ctx *harness.Context) error {
				cx, err := FindProjectBinary()
				if err != nil {
					return err
				}

				cmd := command.New(cx, "resolve", "   ").Dir(ctx.RootDir)
				result := cmd.Run()
				ctx.ShowCommandOutput(cmd.String(), result.Stdout, result.Stderr)

				if result.Error == nil {
					return fmt.Errorf("expected error when resolving whitespace pattern, got success")
				}
				if !strings.Contains(result.Stderr, "empty rule pattern provided") && !strings.Contains(result.Error.Error(), "empty rule pattern provided") {
					return fmt.Errorf("expected 'empty rule pattern provided' error, got: %v / %s", result.Error, result.Stderr)
				}
				return nil
			}),
			harness.NewStep("Run 'cx resolve' with empty @view prefix", func(ctx *harness.Context) error {
				cx, err := FindProjectBinary()
				if err != nil {
					return err
				}

				cmd := command.New(cx, "resolve", "@view:   ").Dir(ctx.RootDir)
				result := cmd.Run()
				ctx.ShowCommandOutput(cmd.String(), result.Stdout, result.Stderr)

				if result.Error == nil {
					return fmt.Errorf("expected error when resolving empty @view pattern, got success")
				}
				if !strings.Contains(result.Stderr, "empty rule pattern provided") && !strings.Contains(result.Error.Error(), "empty rule pattern provided") {
					return fmt.Errorf("expected 'empty rule pattern provided' error, got: %v / %s", result.Error, result.Stderr)
				}
				return nil
			}),
		},
	}
}

// EmptyRuleFilePatternsScenario tests that blank lines, whitespace, standalone exclusion marks (!),
// or empty brace expansions in a rules file are ignored and do not walk the root filesystem.
func EmptyRuleFilePatternsScenario() *harness.Scenario {
	return &harness.Scenario{
		Name:        "cx-empty-rule-file-patterns",
		Description: "Tests that blank lines, whitespace, standalone exclusion marks (!), or empty brace expansions in a rules file are ignored.",
		Tags:        []string{"cx", "rules", "regression"},
		Steps: []harness.Step{
			harness.NewStep("Setup test project with empty/invalid rules", func(ctx *harness.Context) error {
				if err := fs.WriteString(filepath.Join(ctx.RootDir, "main.go"), "package main"); err != nil {
					return err
				}
				if err := fs.WriteString(filepath.Join(ctx.RootDir, "README.md"), "# Test"); err != nil {
					return err
				}
				rulesContent := "\n!\n   \n{,  }\n{ , }\nREADME.md\n"
				rulesPath := filepath.Join(ctx.RootDir, ".grove", "rules")
				return fs.WriteString(rulesPath, rulesContent)
			}),
			harness.NewStep("Run 'cx list' and verify only valid patterns are resolved", func(ctx *harness.Context) error {
				cx, err := FindProjectBinary()
				if err != nil {
					return err
				}
				cmd := command.New(cx, "list").Dir(ctx.RootDir)
				result := cmd.Run()
				ctx.ShowCommandOutput(cmd.String(), result.Stdout, result.Stderr)
				if result.Error != nil {
					return result.Error
				}

				output := result.Stdout

				if !strings.Contains(output, "README.md") {
					return fmt.Errorf("output should contain README.md")
				}

				if strings.Contains(output, "main.go") {
					return fmt.Errorf("output should not contain main.go; empty patterns likely matched the root directory")
				}
				return nil
			}),
		},
	}
}

// GitignoreStyleBasenameExclusionScenario tests that a floating, literal exclusion pattern
// (e.g., !main.go) correctly excludes files with that basename in any subdirectory.
func GitignoreStyleBasenameExclusionScenario() *harness.Scenario {
	return &harness.Scenario{
		Name:        "cx-gitignore-style-basename-exclusion",
		Description: "Tests that a floating, literal exclusion pattern (e.g., !main.go) correctly excludes files in any subdirectory.",
		Tags:        []string{"cx", "rules", "exclusion", "regression"},
		Steps: []harness.Step{
			harness.NewStep("Setup file structure with duplicate basenames", func(ctx *harness.Context) error {
				// Create files that should be matched by the broad pattern
				if err := fs.WriteString(filepath.Join(ctx.RootDir, "main.go"), "package main // should be excluded"); err != nil {
					return err
				}
				if err := fs.WriteString(filepath.Join(ctx.RootDir, "app.go"), "package main // should be included"); err != nil {
					return err
				}
				// Create a subdirectory with another file to be excluded
				cmdDir := filepath.Join(ctx.RootDir, "cmd")
				if err := fs.WriteString(filepath.Join(cmdDir, "main.go"), "package cmd // should be excluded"); err != nil {
					return err
				}
				if err := fs.WriteString(filepath.Join(cmdDir, "api.go"), "package cmd // should be included"); err != nil {
					return err
				}
				return nil
			}),
			harness.NewStep("Create rules file with a floating literal exclusion", func(ctx *harness.Context) error {
				// This pattern should exclude BOTH main.go files.
				rules := `**/*.go
!main.go`
				return fs.WriteString(filepath.Join(ctx.RootDir, ".grove", "rules"), rules)
			}),
			harness.NewStep("Run 'cx list' and verify the exclusion", func(ctx *harness.Context) error {
				cx, err := FindProjectBinary()
				if err != nil {
					return err
				}
				cmd := command.New(cx, "list").Dir(ctx.RootDir)
				result := cmd.Run()
				ctx.ShowCommandOutput(cmd.String(), result.Stdout, result.Stderr)
				if result.Error != nil {
					return result.Error
				}

				output := result.Stdout

				// Verify included files ARE present
				if !strings.Contains(output, "app.go") {
					return fmt.Errorf("output is missing 'app.go', which should be included")
				}
				if !strings.Contains(output, "cmd/api.go") {
					return fmt.Errorf("output is missing 'cmd/api.go', which should be included")
				}

				// Verify excluded files are NOT present
				if strings.Contains(output, "main.go") {
					return fmt.Errorf("output incorrectly includes one or more 'main.go' files, which should be excluded")
				}
				return nil
			}),
		},
	}
}
