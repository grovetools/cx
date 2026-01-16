// File: tests/e2e/scenarios_rules_management.go
package main

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/grovetools/tend/pkg/command"
	"github.com/grovetools/tend/pkg/fs"
	"github.com/grovetools/tend/pkg/harness"
)

// RulesWorkflowScenario tests the full lifecycle of `cx rules` subcommands.
func RulesWorkflowScenario() *harness.Scenario {
	return &harness.Scenario{
		Name:        "cx-rules-workflow",
		Description: "Tests the cx rules list, set, and save commands.",
		Tags:        []string{"cx", "rules"},
		Steps: []harness.Step{
			harness.NewStep("Setup project with multiple rule sets", func(ctx *harness.Context) error {
				// Create files to be referenced by rules
				fs.WriteString(filepath.Join(ctx.RootDir, "main.go"), "package main")
				fs.WriteString(filepath.Join(ctx.RootDir, "README.md"), "# Docs")
				fs.WriteString(filepath.Join(ctx.RootDir, "api.go"), "package api")

				// Create named rule sets in .cx/
				fs.WriteString(filepath.Join(ctx.RootDir, ".cx", "dev.rules"), "*.go")
				fs.WriteString(filepath.Join(ctx.RootDir, ".cx", "docs.rules"), "README.md")
				return nil
			}),
			harness.NewStep("Run 'cx rules list' and verify initial state", func(ctx *harness.Context) error {
				cx, _ := FindProjectBinary()
				cmd := command.New(cx, "rules", "list").Dir(ctx.RootDir)
				result := cmd.Run()
				ctx.ShowCommandOutput(cmd.String(), result.Stdout, result.Stderr)

				// Output can be in stdout or stderr depending on logging configuration
				output := result.Stdout + result.Stderr
				if !strings.Contains(output, "dev") || !strings.Contains(output, "docs") {
					return fmt.Errorf("list output missing rule sets")
				}
				if strings.Contains(output, "*") {
					return fmt.Errorf("no rule set should be active initially")
				}
				if !strings.Contains(output, "Active Source: (default)") {
					return fmt.Errorf("expected active source to be (default)")
				}
				return nil
			}),
			harness.NewStep("Run 'cx rules set dev'", func(ctx *harness.Context) error {
				cx, _ := FindProjectBinary()
				cmd := command.New(cx, "rules", "set", "dev").Dir(ctx.RootDir)
				result := cmd.Run()
				ctx.ShowCommandOutput(cmd.String(), result.Stdout, result.Stderr)
				return result.Error
			}),
			harness.NewStep("Verify 'dev' is active and context is correct", func(ctx *harness.Context) error {
				cx, _ := FindProjectBinary()
				// Check list output
				listCmd := command.New(cx, "rules", "list").Dir(ctx.RootDir)
				listResult := listCmd.Run()
				listOutput := listResult.Stdout + listResult.Stderr
				if !strings.Contains(listOutput, "* dev") {
					return fmt.Errorf("'dev' is not marked as active in rules list")
				}

				// Check context content
				contextListCmd := command.New(cx, "list").Dir(ctx.RootDir)
				contextListResult := contextListCmd.Run()
				contextOutput := contextListResult.Stdout + contextListResult.Stderr
				if !strings.Contains(contextOutput, "main.go") || !strings.Contains(contextOutput, "api.go") {
					return fmt.Errorf("context does not contain Go files from 'dev' rules")
				}
				if strings.Contains(contextOutput, "README.md") {
					return fmt.Errorf("context should not contain README.md")
				}
				return nil
			}),
			harness.NewStep("Modify active rules and save as 'custom'", func(ctx *harness.Context) error {
				// Manually append a new rule to the active file
				activeRulesPath := filepath.Join(ctx.RootDir, ".grove", "rules")
				content, err := fs.ReadString(activeRulesPath)
				if err != nil {
					return err
				}
				if err := fs.WriteString(activeRulesPath, content+"\nREADME.md"); err != nil {
					return err
				}

				// Save the modified rules
				cx, _ := FindProjectBinary()
				cmd := command.New(cx, "rules", "save", "custom").Dir(ctx.RootDir)
				result := cmd.Run()
				ctx.ShowCommandOutput(cmd.String(), result.Stdout, result.Stderr)
				return result.Error
			}),
			harness.NewStep("Verify 'custom.rules' was created correctly", func(ctx *harness.Context) error {
				customRulesPath := filepath.Join(ctx.RootDir, ".cx", "custom.rules")
				content, err := fs.ReadString(customRulesPath)
				if err != nil {
					return err
				}
				if !strings.Contains(content, "*.go") || !strings.Contains(content, "README.md") {
					return fmt.Errorf("saved 'custom.rules' has incorrect content")
				}
				return nil
			}),
		},
	}
}
