// File: tests/e2e/scenarios_enhanced_rules.go
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mattsolo1/grove-tend/pkg/command"
	"github.com/mattsolo1/grove-tend/pkg/fs"
	"github.com/mattsolo1/grove-tend/pkg/harness"
)

// EnhancedRulesWorkflowScenario tests the enhanced rules workflow with .cx.work/, diff, load, and rm commands.
func EnhancedRulesWorkflowScenario() *harness.Scenario {
	return &harness.Scenario{
		Name:        "cx-enhanced-rules-workflow",
		Description: "Tests enhanced rules: save --work, diff, load, and rm with protection.",
		Tags:        []string{"cx", "rules", "enhanced"},
		Steps: []harness.Step{
			harness.NewStep("Setup project files", func(ctx *harness.Context) error {
				fs.WriteString(filepath.Join(ctx.RootDir, "fileA.txt"), "content A")
				fs.WriteString(filepath.Join(ctx.RootDir, "fileB.txt"), "content B")
				fs.WriteString(filepath.Join(ctx.RootDir, "fileC.txt"), "content C")
				fs.WriteString(filepath.Join(ctx.RootDir, "fileD.txt"), "content D")
				return nil
			}),
			harness.NewStep("Create initial rules and save as 'baseline' to .cx/", func(ctx *harness.Context) error {
				rules := "fileA.txt\nfileB.txt"
				fs.WriteString(filepath.Join(ctx.RootDir, ".grove", "rules"), rules)

				cx, _ := FindProjectBinary()
				cmd := command.New(cx, "rules", "save", "baseline").Dir(ctx.RootDir)
				result := cmd.Run()
				ctx.ShowCommandOutput(cmd.String(), result.Stdout, result.Stderr)

				output := result.Stdout + result.Stderr
				if !strings.Contains(output, "Saved current rules as 'baseline' in .cx/") {
					return fmt.Errorf("save did not report saving to .cx/")
				}

				// Verify file exists in .cx/
				baselinePath := filepath.Join(ctx.RootDir, ".cx", "baseline.rules")
				if _, err := os.Stat(baselinePath); os.IsNotExist(err) {
					return fmt.Errorf("baseline.rules not created in .cx/")
				}

				return result.Error
			}),
			harness.NewStep("Modify rules to different files", func(ctx *harness.Context) error {
				rules := "fileC.txt\nfileD.txt"
				return fs.WriteString(filepath.Join(ctx.RootDir, ".grove", "rules"), rules)
			}),
			harness.NewStep("Save modified rules as 'experimental' to .cx.work/ using --work flag", func(ctx *harness.Context) error {
				cx, _ := FindProjectBinary()
				cmd := command.New(cx, "rules", "save", "experimental", "--work").Dir(ctx.RootDir)
				result := cmd.Run()
				ctx.ShowCommandOutput(cmd.String(), result.Stdout, result.Stderr)

				output := result.Stdout + result.Stderr
				if !strings.Contains(output, "Saved current rules as 'experimental' in .cx.work/") {
					return fmt.Errorf("save --work did not report saving to .cx.work/")
				}

				// Verify file exists in .cx.work/
				expPath := filepath.Join(ctx.RootDir, ".cx.work", "experimental.rules")
				if _, err := os.Stat(expPath); os.IsNotExist(err) {
					return fmt.Errorf("experimental.rules not created in .cx.work/")
				}

				return result.Error
			}),
			harness.NewStep("List rules and verify both rule sets appear", func(ctx *harness.Context) error {
				cx, _ := FindProjectBinary()
				cmd := command.New(cx, "rules", "list").Dir(ctx.RootDir)
				result := cmd.Run()
				ctx.ShowCommandOutput(cmd.String(), result.Stdout, result.Stderr)

				output := result.Stdout + result.Stderr
				if !strings.Contains(output, "baseline") {
					return fmt.Errorf("list did not show 'baseline'")
				}
				if !strings.Contains(output, "experimental") {
					return fmt.Errorf("list did not show 'experimental'")
				}
				return nil
			}),
			harness.NewStep("Set experimental as active and diff against baseline", func(ctx *harness.Context) error {
				cx, _ := FindProjectBinary()

				// Set experimental as active
				setCmd := command.New(cx, "rules", "set", "experimental").Dir(ctx.RootDir)
				setResult := setCmd.Run()
				ctx.ShowCommandOutput(setCmd.String(), setResult.Stdout, setResult.Stderr)
				if setResult.Error != nil {
					return setResult.Error
				}

				// Run diff
				diffCmd := command.New(cx, "diff", "baseline").Dir(ctx.RootDir)
				diffResult := diffCmd.Run()
				ctx.ShowCommandOutput(diffCmd.String(), diffResult.Stdout, diffResult.Stderr)

				output := diffResult.Stdout + diffResult.Stderr
				if !strings.Contains(output, "Added files (2)") {
					return fmt.Errorf("diff did not show 2 added files")
				}
				if !strings.Contains(output, "fileC.txt") || !strings.Contains(output, "fileD.txt") {
					return fmt.Errorf("diff did not show fileC.txt and fileD.txt as added")
				}
				if !strings.Contains(output, "Removed files (2)") {
					return fmt.Errorf("diff did not show 2 removed files")
				}
				if !strings.Contains(output, "fileA.txt") || !strings.Contains(output, "fileB.txt") {
					return fmt.Errorf("diff did not show fileA.txt and fileB.txt as removed")
				}
				return nil
			}),
			harness.NewStep("Load baseline into .grove/rules as working copy", func(ctx *harness.Context) error {
				cx, _ := FindProjectBinary()
				cmd := command.New(cx, "rules", "load", "baseline").Dir(ctx.RootDir)
				result := cmd.Run()
				ctx.ShowCommandOutput(cmd.String(), result.Stdout, result.Stderr)

				output := result.Stdout + result.Stderr
				if !strings.Contains(output, "Loaded 'baseline' into .grove/rules as working copy") {
					return fmt.Errorf("load did not report success")
				}

				// Verify .grove/rules has the baseline content
				rulesContent, err := fs.ReadString(filepath.Join(ctx.RootDir, ".grove", "rules"))
				if err != nil {
					return err
				}
				if !strings.Contains(rulesContent, "fileA.txt") || !strings.Contains(rulesContent, "fileB.txt") {
					return fmt.Errorf("loaded rules do not contain expected content")
				}
				if strings.Contains(rulesContent, "fileC.txt") || strings.Contains(rulesContent, "fileD.txt") {
					return fmt.Errorf("loaded rules contain unexpected content")
				}

				return result.Error
			}),
			harness.NewStep("Try to remove baseline without --force (should fail)", func(ctx *harness.Context) error {
				cx, _ := FindProjectBinary()
				cmd := command.New(cx, "rules", "rm", "baseline").Dir(ctx.RootDir)
				result := cmd.Run()
				ctx.ShowCommandOutput(cmd.String(), result.Stdout, result.Stderr)

				// This should fail
				if result.Error == nil {
					return fmt.Errorf("rm without --force should have failed for .cx/ rule set")
				}

				output := result.Stdout + result.Stderr
				if !strings.Contains(output, "version-controlled") || !strings.Contains(output, "--force") {
					return fmt.Errorf("error message did not mention version control or --force")
				}

				// Verify file still exists
				baselinePath := filepath.Join(ctx.RootDir, ".cx", "baseline.rules")
				if _, err := os.Stat(baselinePath); os.IsNotExist(err) {
					return fmt.Errorf("baseline.rules was deleted when it should have been protected")
				}

				return nil
			}),
			harness.NewStep("Remove experimental from .cx.work/ without --force (should succeed)", func(ctx *harness.Context) error {
				cx, _ := FindProjectBinary()
				cmd := command.New(cx, "rules", "rm", "experimental").Dir(ctx.RootDir)
				result := cmd.Run()
				ctx.ShowCommandOutput(cmd.String(), result.Stdout, result.Stderr)

				if result.Error != nil {
					return fmt.Errorf("rm experimental failed: %v", result.Error)
				}

				output := result.Stdout + result.Stderr
				if !strings.Contains(output, "Removed rule set 'experimental'") {
					return fmt.Errorf("rm did not report successful removal")
				}

				// Verify file is gone
				expPath := filepath.Join(ctx.RootDir, ".cx.work", "experimental.rules")
				if _, err := os.Stat(expPath); !os.IsNotExist(err) {
					return fmt.Errorf("experimental.rules was not deleted")
				}

				return nil
			}),
			harness.NewStep("Force remove baseline from .cx/", func(ctx *harness.Context) error {
				cx, _ := FindProjectBinary()
				cmd := command.New(cx, "rules", "rm", "baseline", "--force").Dir(ctx.RootDir)
				result := cmd.Run()
				ctx.ShowCommandOutput(cmd.String(), result.Stdout, result.Stderr)

				if result.Error != nil {
					return fmt.Errorf("rm baseline --force failed: %v", result.Error)
				}

				output := result.Stdout + result.Stderr
				if !strings.Contains(output, "Removed rule set 'baseline'") {
					return fmt.Errorf("rm --force did not report successful removal")
				}

				// Verify file is gone
				baselinePath := filepath.Join(ctx.RootDir, ".cx", "baseline.rules")
				if _, err := os.Stat(baselinePath); !os.IsNotExist(err) {
					return fmt.Errorf("baseline.rules was not deleted with --force")
				}

				return nil
			}),
		},
	}
}
