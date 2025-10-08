// File: tests/e2e/scenarios_alias_extended.go
package main

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/mattsolo1/grove-tend/pkg/command"
	"github.com/mattsolo1/grove-tend/pkg/fs"
	"github.com/mattsolo1/grove-tend/pkg/harness"
)

// AliasRulesetImportScenario tests the @alias:project:ruleset directive.
func AliasRulesetImportScenario() *harness.Scenario {
	return &harness.Scenario{
		Name:        "cx-alias-ruleset-import",
		Description: "Tests importing a named rule set from another project via an alias.",
		Tags:        []string{"cx", "alias", "rules"},
		Steps: []harness.Step{
			harness.NewStep("Setup multi-project environment", func(ctx *harness.Context) error {
				grovesDir := filepath.Join(ctx.RootDir, "mock-groves")
				testConfigHome := filepath.Join(ctx.RootDir, ".test-config")
				groveConfigDir := filepath.Join(testConfigHome, "grove")
				ctx.Set("testConfigHome", testConfigHome)

				// Create global grove.yml to discover projects
				groveConfig := fmt.Sprintf(`groves:
  test:
    path: %s
    enabled: true
`, grovesDir)
				fs.WriteString(filepath.Join(groveConfigDir, "grove.yml"), groveConfig)

				// Create target project with named rule sets
				projectB := filepath.Join(grovesDir, "project-b")
				fs.WriteString(filepath.Join(projectB, "grove.yml"), `name: project-b`)
				fs.WriteString(filepath.Join(projectB, "docs/guide.md"), "Guide content")
				fs.WriteString(filepath.Join(projectB, "src/api.go"), "package api")
				fs.WriteString(filepath.Join(projectB, ".cx/docs.rules"), "docs/**/*.md")
				fs.WriteString(filepath.Join(projectB, ".cx/api.rules"), "src/**/*.go")
				command.New("git", "init").Dir(projectB).Run() // Needed for discovery

				// Create main project
				fs.WriteString(filepath.Join(ctx.RootDir, "main.go"), "package main")
				fs.WriteString(filepath.Join(ctx.RootDir, "grove.yml"), `name: project-a`)
				command.New("git", "init").Dir(ctx.RootDir).Run()
				return nil
			}),
			harness.NewStep("Create rules file importing from project-b", func(ctx *harness.Context) error {
				rules := `main.go
@alias:project-b::docs`
				return fs.WriteString(filepath.Join(ctx.RootDir, ".grove", "rules"), rules)
			}),
			harness.NewStep("Run 'cx list' and verify imported rules", func(ctx *harness.Context) error {
				cx, _ := FindProjectBinary()
				testConfigHome := ctx.Get("testConfigHome").(string)
				cmd := command.New(cx, "list").Dir(ctx.RootDir).Env(fmt.Sprintf("XDG_CONFIG_HOME=%s", testConfigHome))
				result := cmd.Run()
				ctx.ShowCommandOutput(cmd.String(), result.Stdout, result.Stderr)
				if result.Error != nil {
					return result.Error
				}

				output := result.Stdout
				if !strings.Contains(output, "main.go") {
					return fmt.Errorf("context is missing local file 'main.go'")
				}
				if !strings.Contains(output, "guide.md") {
					return fmt.Errorf("context is missing imported 'guide.md' from project-b's docs ruleset")
				}
				if strings.Contains(output, "api.go") {
					return fmt.Errorf("context should not include 'api.go' from project-b's api ruleset")
				}
				return nil
			}),
		},
	}
}
