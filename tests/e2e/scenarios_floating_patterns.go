// File: tests/e2e/scenarios_floating_patterns.go
package main

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/grovetools/tend/pkg/command"
	"github.com/grovetools/tend/pkg/fs"
	"github.com/grovetools/tend/pkg/harness"
)

// FloatingInclusionScopeScenario tests that floating inclusion patterns are scoped to the current project.
func FloatingInclusionScopeScenario() *harness.Scenario {
	return &harness.Scenario{
		Name:        "cx-floating-inclusion-scope",
		Description: "Tests that floating inclusion patterns like `*.go` are scoped to the current project.",
		Tags:        []string{"cx", "rules", "patterns", "scope", "regression"},
		Steps: []harness.Step{
			harness.NewStep("Setup multi-project environment", func(ctx *harness.Context) error {
				// Create a groves directory OUTSIDE the current project (as a sibling)
				parentDir := filepath.Dir(ctx.RootDir)
				grovesDir := filepath.Join(parentDir, "mock-groves")
				groveConfigDir := filepath.Join(ctx.ConfigDir(), "grove")

				// Create global grove.yml to enable project discovery
				groveConfig := fmt.Sprintf(`search_paths:
  test: { path: %q, enabled: true }`, grovesDir)
				if err := fs.WriteString(filepath.Join(groveConfigDir, "grove.yml"), groveConfig); err != nil {
					return err
				}

				// Project A (current working directory)
				projectA := ctx.RootDir
				if err := fs.WriteString(filepath.Join(projectA, "grove.yml"), `name: project-a`); err != nil {
					return err
				}
				if err := fs.WriteString(filepath.Join(projectA, "main.go"), "package main // in project-a"); err != nil {
					return err
				}
				command.New("git", "init").Dir(projectA).Run()

				// Project B (external project, sibling to project-a)
				projectB := filepath.Join(grovesDir, "project-b")
				if err := fs.WriteString(filepath.Join(projectB, "grove.yml"), `name: project-b`); err != nil {
					return err
				}
				if err := fs.WriteString(filepath.Join(projectB, "lib.go"), "package lib // in project-b"); err != nil {
					return err
				}
				if err := fs.WriteString(filepath.Join(projectB, "docs.md"), "# Docs // in project-b"); err != nil {
					return err
				}
				command.New("git", "init").Dir(projectB).Run()

				return nil
			}),
			harness.NewStep("Create rules file with floating inclusion", func(ctx *harness.Context) error {
				rules := `
# Floating inclusion, should only apply to project-a
*.go

# Alias to bring a specific file from project-b into context
@a:project-b/docs.md`
				return fs.WriteString(filepath.Join(ctx.RootDir, ".grove", "rules"), rules)
			}),
			harness.NewStep("Run 'cx list' and verify scope", func(ctx *harness.Context) error {
				cx, err := FindProjectBinary()
				if err != nil {
					return err
				}

				cmd := ctx.Command(cx, "list").Dir(ctx.RootDir)
				result := cmd.Run()
				ctx.ShowCommandOutput(cmd.String(), result.Stdout, result.Stderr)
				if result.Error != nil {
					return result.Error
				}

				output := result.Stdout

				// MUST contain main.go from project-a
				if !strings.Contains(output, "main.go") {
					return fmt.Errorf("output is missing local file 'main.go'")
				}

				// MUST contain docs.md from project-b (explicitly included)
				if !strings.Contains(output, "docs.md") {
					return fmt.Errorf("output is missing aliased file 'docs.md' from project-b")
				}

				// MUST NOT contain lib.go from project-b (matched by floating inclusion)
				if strings.Contains(output, "lib.go") {
					return fmt.Errorf("output incorrectly includes 'lib.go' from project-b; floating pattern was not scoped")
				}
				return nil
			}),
		},
	}
}
