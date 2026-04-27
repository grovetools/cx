// File: grove-context/tests/e2e/scenarios_external_rules.go
package main

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/grovetools/tend/pkg/fs"
	"github.com/grovetools/tend/pkg/harness"
)

// ExternalRulesFileScenario tests that when a --rules-file points to a file
// outside the project directory, patterns resolve against the project root
// and stats show correct token counts.
//
// This simulates the groveterm BSP cx panel scenario: the rules file lives
// in a notebook plan directory (external), but patterns like "*.go" must
// resolve files in the project workspace.
func ExternalRulesFileScenario() *harness.Scenario {
	return &harness.Scenario{
		Name:        "cx-external-rules-file",
		Description: "Patterns in an external rules file resolve against project dir with correct stats",
		Tags:        []string{"cx", "rules", "patterns", "external"},
		Steps: []harness.Step{
			harness.NewStep("Setup project and external notebook rules", func(ctx *harness.Context) error {
				// --- Project directory (RootDir simulates the project workspace) ---
				if err := fs.WriteString(filepath.Join(ctx.RootDir, "grove.yml"), `name: test-project`); err != nil {
					return err
				}
				if err := fs.WriteString(filepath.Join(ctx.RootDir, "main.go"), "package main\n\nfunc main() {}\n"); err != nil {
					return err
				}
				if err := fs.WriteString(filepath.Join(ctx.RootDir, "lib.go"), "package main\n\nfunc helper() {}\n"); err != nil {
					return err
				}
				if err := fs.WriteString(filepath.Join(ctx.RootDir, "README.md"), "# Test Project\n\nA description.\n"); err != nil {
					return err
				}

				// --- External notebook directory (simulates notebook plan rules/) ---
				notebookDir := filepath.Join(ctx.HomeDir(), "notebooks", "test-workspace", "plans", "test-plan", "rules")
				rulesFile := filepath.Join(notebookDir, "job.rules")
				if err := fs.WriteString(rulesFile, "# Include Go files from the project\n*.go\n"); err != nil {
					return err
				}

				return fs.WriteString(filepath.Join(ctx.RootDir, ".rules_file_path"), rulesFile)
			}),

			harness.NewStep("cx list from project dir finds files", func(ctx *harness.Context) error {
				rulesFileBytes, _ := fs.ReadString(filepath.Join(ctx.RootDir, ".rules_file_path"))
				rulesFile := strings.TrimSpace(rulesFileBytes)

				cx, err := FindProjectBinary()
				if err != nil {
					return err
				}
				cmd := ctx.Command(cx, "list", "--rules-file", rulesFile).Dir(ctx.RootDir)
				result := cmd.Run()
				ctx.ShowCommandOutput(cmd.String(), result.Stdout, result.Stderr)
				if result.Error != nil {
					return result.Error
				}

				if !strings.Contains(result.Stdout, "main.go") {
					return fmt.Errorf("expected main.go in output, got:\n%s", result.Stdout)
				}
				if !strings.Contains(result.Stdout, "lib.go") {
					return fmt.Errorf("expected lib.go in output, got:\n%s", result.Stdout)
				}
				if strings.Contains(result.Stdout, "README.md") {
					return fmt.Errorf("unexpected README.md in output (pattern is *.go only), got:\n%s", result.Stdout)
				}
				return nil
			}),

			harness.NewStep("cx stats from project dir shows non-zero tokens", func(ctx *harness.Context) error {
				rulesFileBytes, _ := fs.ReadString(filepath.Join(ctx.RootDir, ".rules_file_path"))
				rulesFile := strings.TrimSpace(rulesFileBytes)

				cx, err := FindProjectBinary()
				if err != nil {
					return err
				}
				cmd := ctx.Command(cx, "stats", rulesFile).Dir(ctx.RootDir)
				result := cmd.Run()
				ctx.ShowCommandOutput(cmd.String(), result.Stdout, result.Stderr)
				if result.Error != nil {
					return result.Error
				}

				if strings.Contains(result.Stdout, "Total Files:    0") {
					return fmt.Errorf("expected non-zero file count, got:\n%s", result.Stdout)
				}
				if strings.Contains(result.Stdout, "~0") {
					return fmt.Errorf("expected non-zero token count, got:\n%s", result.Stdout)
				}
				return nil
			}),
			harness.NewStep("Teardown test repos", func(ctx *harness.Context) error {
				_ = CleanupTestRepos(ctx)
				return nil
			}),
		},
	}
}
