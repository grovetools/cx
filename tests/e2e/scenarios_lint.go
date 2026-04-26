// File: grove-context/tests/e2e/scenarios_lint.go
package main

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/grovetools/tend/pkg/command"
	"github.com/grovetools/tend/pkg/fs"
	"github.com/grovetools/tend/pkg/harness"
)

// LintCleanRulesScenario tests that linting clean rules returns no issues.
func LintCleanRulesScenario() *harness.Scenario {
	return &harness.Scenario{
		Name:        "cx-lint-clean",
		Description: "Tests that linting clean rules returns no issues.",
		Tags:        []string{"cx", "lint"},
		Steps: []harness.Step{
			harness.NewStep("Setup test project", func(ctx *harness.Context) error {
				if err := fs.WriteString(filepath.Join(ctx.RootDir, "main.go"), "package main"); err != nil {
					return err
				}
				return nil
			}),
			harness.NewStep("Create clean rules file", func(ctx *harness.Context) error {
				rules := "main.go"
				return fs.WriteString(filepath.Join(ctx.RootDir, ".grove", "rules"), rules)
			}),
			harness.NewStep("Run 'cx lint' and verify no issues", func(ctx *harness.Context) error {
				cxBinary, err := FindProjectBinary()
				if err != nil {
					return err
				}

				cmd := command.New(cxBinary, "lint").Dir(ctx.RootDir)
				result := cmd.Run()
				ctx.ShowCommandOutput(cmd.String(), result.Stdout, result.Stderr)

				if result.Error != nil {
					return fmt.Errorf("expected no error, got: %w\nStderr: %s", result.Error, result.Stderr)
				}
				if !strings.Contains(result.Stdout, "Rules look good! No issues found.") {
					return fmt.Errorf("expected success message, got: %s", result.Stdout)
				}
				return nil
			}),
		},
	}
}

// LintDirectiveTypoScenario tests that linting flags unrecognized directives.
func LintDirectiveTypoScenario() *harness.Scenario {
	return &harness.Scenario{
		Name:        "cx-lint-directive-typo",
		Description: "Tests that linting flags unrecognized directives.",
		Tags:        []string{"cx", "lint"},
		Steps: []harness.Step{
			harness.NewStep("Setup test project", func(ctx *harness.Context) error {
				return fs.WriteString(filepath.Join(ctx.RootDir, "main.go"), "package main")
			}),
			harness.NewStep("Create rules file with directive typos", func(ctx *harness.Context) error {
				rules := "main.go @flind: \"main\"\n@grepp: \"test\""
				return fs.WriteString(filepath.Join(ctx.RootDir, ".grove", "rules"), rules)
			}),
			harness.NewStep("Run 'cx lint' and verify typo warnings", func(ctx *harness.Context) error {
				cxBinary, err := FindProjectBinary()
				if err != nil {
					return err
				}

				cmd := command.New(cxBinary, "lint").Dir(ctx.RootDir)
				result := cmd.Run()
				ctx.ShowCommandOutput(cmd.String(), result.Stdout, result.Stderr)

				if !strings.Contains(result.Stdout, "Unrecognized directive '@flind' - possible typo") {
					return fmt.Errorf("expected warning for @flind, got: %s", result.Stdout)
				}
				if !strings.Contains(result.Stdout, "Unrecognized directive '@grepp' - possible typo") {
					return fmt.Errorf("expected warning for @grepp, got: %s", result.Stdout)
				}
				return nil
			}),
		},
	}
}

// LintZeroMatchScenario tests that linting warns when a pattern matches 0 files.
func LintZeroMatchScenario() *harness.Scenario {
	return &harness.Scenario{
		Name:        "cx-lint-zero-match",
		Description: "Tests that linting warns when a pattern matches 0 files.",
		Tags:        []string{"cx", "lint"},
		Steps: []harness.Step{
			harness.NewStep("Setup test project", func(ctx *harness.Context) error {
				return fs.WriteString(filepath.Join(ctx.RootDir, "main.go"), "package main")
			}),
			harness.NewStep("Create rules file with zero-match pattern", func(ctx *harness.Context) error {
				rules := "nonexistent/**/*.go"
				return fs.WriteString(filepath.Join(ctx.RootDir, ".grove", "rules"), rules)
			}),
			harness.NewStep("Run 'cx lint' and verify zero-match warning", func(ctx *harness.Context) error {
				cxBinary, err := FindProjectBinary()
				if err != nil {
					return err
				}

				cmd := command.New(cxBinary, "lint").Dir(ctx.RootDir)
				result := cmd.Run()
				ctx.ShowCommandOutput(cmd.String(), result.Stdout, result.Stderr)

				if !strings.Contains(result.Stdout, "Pattern matches 0 files in the workspace") {
					return fmt.Errorf("expected zero files warning, got: %s", result.Stdout)
				}
				return nil
			}),
		},
	}
}

// LintOverlyBroadScenario tests that linting warns on overly broad patterns like * or .*
func LintOverlyBroadScenario() *harness.Scenario {
	return &harness.Scenario{
		Name:        "cx-lint-overly-broad",
		Description: "Tests that linting warns on overly broad patterns like * or .*",
		Tags:        []string{"cx", "lint"},
		Steps: []harness.Step{
			harness.NewStep("Setup test project", func(ctx *harness.Context) error {
				return fs.WriteString(filepath.Join(ctx.RootDir, "main.go"), "package main")
			}),
			harness.NewStep("Create rules file with broad patterns", func(ctx *harness.Context) error {
				rules := "*\n.*\n**"
				return fs.WriteString(filepath.Join(ctx.RootDir, ".grove", "rules"), rules)
			}),
			harness.NewStep("Run 'cx lint' and verify broad pattern warnings", func(ctx *harness.Context) error {
				cxBinary, err := FindProjectBinary()
				if err != nil {
					return err
				}

				cmd := command.New(cxBinary, "lint").Dir(ctx.RootDir)
				result := cmd.Run()
				ctx.ShowCommandOutput(cmd.String(), result.Stdout, result.Stderr)

				if !strings.Contains(result.Stdout, "Pattern is overly broad and may match too many files") {
					return fmt.Errorf("expected overly broad warning for * or .*, got: %s", result.Stdout)
				}
				if !strings.Contains(result.Stdout, "is too broad and could include system files") {
					return fmt.Errorf("expected validateRuleSafety warning for **, got: %s", result.Stdout)
				}
				return nil
			}),
		},
	}
}

// LintMultipleIssuesScenario tests that linting reports multiple issues across different lines.
func LintMultipleIssuesScenario() *harness.Scenario {
	return &harness.Scenario{
		Name:        "cx-lint-multiple-issues",
		Description: "Tests that linting reports multiple issues across different lines.",
		Tags:        []string{"cx", "lint"},
		Steps: []harness.Step{
			harness.NewStep("Setup test project", func(ctx *harness.Context) error {
				return fs.WriteString(filepath.Join(ctx.RootDir, "main.go"), "package main")
			}),
			harness.NewStep("Create rules file with multiple issues", func(ctx *harness.Context) error {
				// Line 1: directive typo + zero match (whole line as pattern)
				// Line 2: zero match (nonexistent file)
				// Line 3: overly broad
				rules := "main.go @flind: \"main\"\nnonexistent.go\n*"
				return fs.WriteString(filepath.Join(ctx.RootDir, ".grove", "rules"), rules)
			}),
			harness.NewStep("Run 'cx lint' and verify all issues reported", func(ctx *harness.Context) error {
				cxBinary, err := FindProjectBinary()
				if err != nil {
					return err
				}

				cmd := command.New(cxBinary, "lint").Dir(ctx.RootDir)
				result := cmd.Run()
				ctx.ShowCommandOutput(cmd.String(), result.Stdout, result.Stderr)

				if !strings.Contains(result.Stdout, "Found 4 issue(s) in rules file:") {
					return fmt.Errorf("expected 4 issues, got: %s", result.Stdout)
				}
				if !strings.Contains(result.Stdout, "Line 1: Unrecognized directive '@flind' - possible typo") {
					return fmt.Errorf("missing line 1 directive typo issue: %s", result.Stdout)
				}
				if !strings.Contains(result.Stdout, "Line 2: Pattern matches 0 files in the workspace") {
					return fmt.Errorf("missing line 2 zero-match issue: %s", result.Stdout)
				}
				if !strings.Contains(result.Stdout, "Line 3: Pattern is overly broad and may match too many files") {
					return fmt.Errorf("missing line 3 overly broad issue: %s", result.Stdout)
				}
				return nil
			}),
		},
	}
}

// LintDangerousTraversalScenario asserts that path traversal escaping the workspace
// is reported as an Error and causes a non-zero exit code.
func LintDangerousTraversalScenario() *harness.Scenario {
	return &harness.Scenario{
		Name:        "cx-lint-dangerous-traversal",
		Description: "Tests that '../../etc/passwd' is flagged as an Error and rc=1.",
		Tags:        []string{"cx", "lint"},
		Steps: []harness.Step{
			harness.NewStep("Setup test project", func(ctx *harness.Context) error {
				return fs.WriteString(filepath.Join(ctx.RootDir, "main.go"), "package main")
			}),
			harness.NewStep("Write traversal pattern to rules", func(ctx *harness.Context) error {
				return fs.WriteString(filepath.Join(ctx.RootDir, ".grove", "rules"), "../../etc/passwd\n")
			}),
			harness.NewStep("Run 'cx lint' and verify rc=1 + Error", func(ctx *harness.Context) error {
				cxBinary, err := FindProjectBinary()
				if err != nil {
					return err
				}
				cmd := command.New(cxBinary, "lint").Dir(ctx.RootDir)
				result := cmd.Run()
				ctx.ShowCommandOutput(cmd.String(), result.Stdout, result.Stderr)
				if result.Error == nil {
					return fmt.Errorf("expected non-zero exit, got rc=0; stdout=%s", result.Stdout)
				}
				if !strings.Contains(result.Stdout, "[Error]") {
					return fmt.Errorf("expected [Error] in output, got: %s", result.Stdout)
				}
				if !strings.Contains(result.Stdout, "traverse outside the workspace") {
					return fmt.Errorf("expected traversal message, got: %s", result.Stdout)
				}
				return nil
			}),
		},
	}
}

// LintNoRulesScenario tests that linting gracefully handles absence of a rules file.
func LintNoRulesScenario() *harness.Scenario {
	return &harness.Scenario{
		Name:        "cx-lint-no-rules",
		Description: "Tests that linting gracefully handles absence of a rules file.",
		Tags:        []string{"cx", "lint"},
		Steps: []harness.Step{
			harness.NewStep("Run 'cx lint' without a rules file", func(ctx *harness.Context) error {
				cxBinary, err := FindProjectBinary()
				if err != nil {
					return err
				}

				cmd := command.New(cxBinary, "lint").Dir(ctx.RootDir)
				result := cmd.Run()
				ctx.ShowCommandOutput(cmd.String(), result.Stdout, result.Stderr)

				if result.Error != nil {
					return fmt.Errorf("expected no error, got: %w\nStderr: %s", result.Error, result.Stderr)
				}
				if !strings.Contains(result.Stdout, "Rules look good! No issues found.") {
					return fmt.Errorf("expected success message, got: %s", result.Stdout)
				}
				return nil
			}),
		},
	}
}
