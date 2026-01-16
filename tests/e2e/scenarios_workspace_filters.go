// File: tests/e2e/scenarios_workspace_filters.go
package main

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/grovetools/tend/pkg/command"
	"github.com/grovetools/tend/pkg/fs"
	"github.com/grovetools/tend/pkg/harness"
)

// WorkspaceExclusionScenario tests that the excluded_workspaces config is respected.
func WorkspaceExclusionScenario() *harness.Scenario {
	return &harness.Scenario{
		Name:        "cx-workspace-exclusion",
		Description: "Tests that 'excluded_workspaces' in global config prevents aliased projects from being included.",
		Tags:        []string{"cx", "config", "alias", "regression"},
		Steps: []harness.Step{
			harness.NewStep("Setup environment with excluded workspace", func(ctx *harness.Context) error {
				// 1. Set up a temporary global config directory.
				groveConfigDir := filepath.Join(ctx.ConfigDir(), "grove")
				grovesDir := filepath.Join(ctx.RootDir, "mock-groves")

				// 2. Create global grove.yml with excluded_workspaces.
				globalConfig := fmt.Sprintf(`search_paths:
  test: { path: %q, enabled: true }
context:
  excluded_workspaces: ["project-b"]`, grovesDir)
				if err := fs.WriteString(filepath.Join(groveConfigDir, "grove.yml"), globalConfig); err != nil {
					return err
				}

				// 3. Create project-a (allowed).
				projectA := filepath.Join(grovesDir, "project-a")
				if err := fs.WriteString(filepath.Join(projectA, "grove.yml"), `name: project-a`); err != nil {
					return err
				}
				if err := fs.WriteString(filepath.Join(projectA, "a.txt"), "content from a"); err != nil {
					return err
				}
				command.New("git", "init").Dir(projectA).Run()

				// 4. Create project-b (excluded).
				projectB := filepath.Join(grovesDir, "project-b")
				if err := fs.WriteString(filepath.Join(projectB, "grove.yml"), `name: project-b`); err != nil {
					return err
				}
				if err := fs.WriteString(filepath.Join(projectB, "b.txt"), "content from b"); err != nil {
					return err
				}
				command.New("git", "init").Dir(projectB).Run()

				// 5. In the main test directory, create rules that try to include both.
				rules := `@a:project-a/**
@a:project-b/**`
				return fs.WriteString(filepath.Join(ctx.RootDir, ".grove", "rules"), rules)
			}),
			harness.NewStep("Run 'cx list' and verify exclusion", func(ctx *harness.Context) error {
				cx, err := FindProjectBinary()
				if err != nil {
					return err
				}

				// Run 'cx list' with the custom config home.
				cmd := ctx.Command(cx, "list").Dir(ctx.RootDir)
				result := cmd.Run()
				ctx.ShowCommandOutput(cmd.String(), result.Stdout, result.Stderr)

				// Stderr should contain the warning about the skipped rule.
				if !strings.Contains(result.Stderr, "skipping rule") || !strings.Contains(result.Stderr, "is in your 'excluded_workspaces' list") {
					return fmt.Errorf("expected stderr to contain a warning about the excluded workspace")
				}

				// Stdout should contain the file from project-a.
				if !strings.Contains(result.Stdout, "a.txt") {
					return fmt.Errorf("output is missing file from allowed project-a")
				}

				// Stdout should NOT contain the file from project-b.
				if strings.Contains(result.Stdout, "b.txt") {
					return fmt.Errorf("output incorrectly includes file from excluded project-b")
				}
				return nil
			}),
		},
	}
}
