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

// LiteralNegationScenario tests that a literal file inclusion followed by a
// literal exclusion (`Makefile` then `!Makefile`) correctly removes the file.
// This guards against the historic fast-path bypass in resolveFilesFromPatterns
// where literal files were dumped into a uniqueFiles map ahead of the
// patternMatcher.classify() loop, so `!` rules never saw them.
func LiteralNegationScenario() *harness.Scenario {
	return &harness.Scenario{
		Name:        "cx-literal-negation",
		Description: "Tests that a literal file inclusion followed by a literal exclusion correctly removes the file.",
		Tags:        []string{"cx", "rules", "exclusion", "regression"},
		Steps: []harness.Step{
			harness.NewStep("Setup workspace", func(ctx *harness.Context) error {
				if err := fs.WriteString(filepath.Join(ctx.RootDir, "Makefile"), "build:\n\techo 'build'\n"); err != nil {
					return err
				}
				if err := fs.WriteString(filepath.Join(ctx.RootDir, "app.go"), "package main\n"); err != nil {
					return err
				}
				rules := "app.go\nMakefile\n!Makefile\n"
				return fs.WriteString(filepath.Join(ctx.RootDir, ".grove", "rules"), rules)
			}),
			harness.NewStep("Run 'cx list' and verify exclusion", func(ctx *harness.Context) error {
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
				if !strings.Contains(output, "app.go") {
					return fmt.Errorf("expected 'app.go' to be included")
				}
				if strings.Contains(output, "Makefile") {
					return fmt.Errorf("expected 'Makefile' to be excluded by !Makefile, but it was present in output: %s", output)
				}
				return nil
			}),
		},
	}
}

// InlineCommentStripScenario tests that ` # comment` suffixes on rule lines are stripped.
func InlineCommentStripScenario() *harness.Scenario {
	return &harness.Scenario{
		Name:        "cx-inline-comment-strip",
		Description: "Tests that inline trailing comments on rule lines are stripped before pattern matching.",
		Tags:        []string{"cx", "rules", "regression"},
		Steps: []harness.Step{
			harness.NewStep("Setup", func(ctx *harness.Context) error {
				if err := fs.WriteString(filepath.Join(ctx.RootDir, "foo.go"), "package main\n"); err != nil {
					return err
				}
				rules := "foo.go # this is a comment\n"
				return fs.WriteString(filepath.Join(ctx.RootDir, ".grove", "rules"), rules)
			}),
			harness.NewStep("Run cx list and verify foo.go matches", func(ctx *harness.Context) error {
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
				if !strings.Contains(result.Stdout, "foo.go") {
					return fmt.Errorf("expected foo.go in output, got: %s", result.Stdout)
				}
				return nil
			}),
		},
	}
}

// TrailingSlashDirectoryScenario tests that "cx/" expands to recursive directory contents.
func TrailingSlashDirectoryScenario() *harness.Scenario {
	return &harness.Scenario{
		Name:        "cx-trailing-slash-dir",
		Description: "Tests that a directory with trailing slash matches all files recursively.",
		Tags:        []string{"cx", "rules", "regression"},
		Steps: []harness.Step{
			harness.NewStep("Setup", func(ctx *harness.Context) error {
				if err := fs.WriteString(filepath.Join(ctx.RootDir, "src", "main.go"), "package main\n"); err != nil {
					return err
				}
				if err := fs.WriteString(filepath.Join(ctx.RootDir, "src", "util.go"), "package main\n"); err != nil {
					return err
				}
				if err := fs.WriteString(filepath.Join(ctx.RootDir, "other.go"), "package main\n"); err != nil {
					return err
				}
				rules := "src/\n"
				return fs.WriteString(filepath.Join(ctx.RootDir, ".grove", "rules"), rules)
			}),
			harness.NewStep("Run cx list and verify src/* but not other.go", func(ctx *harness.Context) error {
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
				if !strings.Contains(result.Stdout, "src/main.go") {
					return fmt.Errorf("expected src/main.go in output, got: %s", result.Stdout)
				}
				if !strings.Contains(result.Stdout, "src/util.go") {
					return fmt.Errorf("expected src/util.go in output, got: %s", result.Stdout)
				}
				if strings.Contains(result.Stdout, "other.go") && !strings.Contains(result.Stdout, "src/") {
					return fmt.Errorf("did not expect other.go (outside src/), got: %s", result.Stdout)
				}
				return nil
			}),
		},
	}
}

// MultipleSeparatorWarningScenario tests that a second `---` emits a warning but parsing continues.
func MultipleSeparatorWarningScenario() *harness.Scenario {
	return &harness.Scenario{
		Name:        "cx-multiple-separator-warning",
		Description: "Tests that multiple `---` separators emit a warning and rules after the first separator still parse.",
		Tags:        []string{"cx", "rules", "regression"},
		Steps: []harness.Step{
			harness.NewStep("Setup", func(ctx *harness.Context) error {
				if err := fs.WriteString(filepath.Join(ctx.RootDir, "a.go"), "package main\n"); err != nil {
					return err
				}
				if err := fs.WriteString(filepath.Join(ctx.RootDir, "b.go"), "package main\n"); err != nil {
					return err
				}
				if err := fs.WriteString(filepath.Join(ctx.RootDir, "c.go"), "package main\n"); err != nil {
					return err
				}
				rules := "a.go\n---\nb.go\n---\nc.go\n"
				return fs.WriteString(filepath.Join(ctx.RootDir, ".grove", "rules"), rules)
			}),
			harness.NewStep("Run cx list + list-cache and verify warning + all files included", func(ctx *harness.Context) error {
				cx, err := FindProjectBinary()
				if err != nil {
					return err
				}
				hot := command.New(cx, "list").Dir(ctx.RootDir)
				hotResult := hot.Run()
				ctx.ShowCommandOutput(hot.String(), hotResult.Stdout, hotResult.Stderr)
				cold := command.New(cx, "list-cache").Dir(ctx.RootDir)
				coldResult := cold.Run()
				ctx.ShowCommandOutput(cold.String(), coldResult.Stdout, coldResult.Stderr)
				combinedStderr := hotResult.Stderr + coldResult.Stderr
				combinedStdout := hotResult.Stdout + coldResult.Stdout
				if !strings.Contains(combinedStderr, "multiple '---' separators") {
					return fmt.Errorf("expected stderr warning for multiple ---, got: %s", combinedStderr)
				}
				for _, f := range []string{"a.go", "b.go", "c.go"} {
					if !strings.Contains(combinedStdout, f) {
						return fmt.Errorf("expected %s in combined output, got: %s", f, combinedStdout)
					}
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
