// File: tests/e2e/scenarios_alias_extended.go
package main

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/grovetools/tend/pkg/command"
	"github.com/grovetools/tend/pkg/fs"
	"github.com/grovetools/tend/pkg/harness"
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
				groveConfigDir := filepath.Join(ctx.ConfigDir(), "grove")

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
				cmd := ctx.Command(cx, "list").Dir(ctx.RootDir)
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

// AliasEcosystemWorktreeScenario tests alias resolution in complex ecosystem worktree hierarchies.
func AliasEcosystemWorktreeScenario() *harness.Scenario {
	return &harness.Scenario{
		Name:        "cx-alias-ecosystem-worktree",
		Description: "Tests alias resolution with multi-part aliases in ecosystem worktree contexts.",
		Tags:        []string{"cx", "alias", "ecosystem", "worktree"},
		Steps: []harness.Step{
			harness.NewStep("Setup complex ecosystem worktree environment", func(ctx *harness.Context) error {
				grovesDir := filepath.Join(ctx.RootDir, "mock-groves")
				groveConfigDir := filepath.Join(ctx.ConfigDir(), "grove")

				// Create global grove.yml to discover projects
				groveConfig := fmt.Sprintf(`groves:
  test:
    path: %s
    enabled: true
`, grovesDir)
				if err := fs.WriteString(filepath.Join(groveConfigDir, "grove.yml"), groveConfig); err != nil {
					return err
				}

				// Create ecosystem root
				ecoRootDir := filepath.Join(grovesDir, "grove-ecosystem")
				ecoConfig := `name: grove-ecosystem
workspaces:
  - "*"`
				if err := fs.WriteString(filepath.Join(ecoRootDir, "grove.yml"), ecoConfig); err != nil {
					return err
				}
				if err := fs.WriteString(filepath.Join(ecoRootDir, ".gitmodules"), "# ecosystem"); err != nil {
					return err
				}
				if result := command.New("git", "init").Dir(ecoRootDir).Run(); result.Error != nil {
					return fmt.Errorf("failed to init git in ecosystem: %w", result.Error)
				}

				// Create ecosystem worktree
				worktreeDir := filepath.Join(ecoRootDir, ".grove-worktrees", "general-refactoring")
				if err := fs.WriteString(filepath.Join(worktreeDir, "grove.yml"), ecoConfig); err != nil {
					return err
				}
				if err := fs.WriteString(filepath.Join(worktreeDir, ".gitmodules"), "# ecosystem worktree"); err != nil {
					return err
				}
				// Mark as worktree with .git file
				if err := fs.WriteString(filepath.Join(worktreeDir, ".git"), "gitdir: ../../.git/worktrees/general-refactoring"); err != nil {
					return err
				}

				// Create grove-core in the worktree with a named ruleset
				coreDir := filepath.Join(worktreeDir, "grove-core")
				if err := fs.WriteString(filepath.Join(coreDir, "grove.yml"), `name: grove-core`); err != nil {
					return err
				}
				if err := fs.WriteString(filepath.Join(coreDir, "core.go"), "package core\n// Core functionality"); err != nil {
					return err
				}
				if err := fs.WriteString(filepath.Join(coreDir, "utils.go"), "package core\n// Utilities"); err != nil {
					return err
				}
				if err := fs.WriteString(filepath.Join(coreDir, "core_test.go"), "package core_test\n// Tests"); err != nil {
					return err
				}
				// Create named ruleset
				if err := fs.WriteString(filepath.Join(coreDir, ".cx", "dev-with-tests.rules"), "**/*.go"); err != nil {
					return err
				}
				// Mark as worktree
				if err := fs.WriteString(filepath.Join(coreDir, ".git"), "gitdir: ../../../grove-core/.git/worktrees/general-refactoring"); err != nil {
					return err
				}

				// Create grove-context in the worktree (our working directory)
				contextDir := filepath.Join(worktreeDir, "grove-context")
				if err := fs.WriteString(filepath.Join(contextDir, "grove.yml"), `name: grove-context`); err != nil {
					return err
				}
				if err := fs.WriteString(filepath.Join(contextDir, "context.go"), "package context\n// Context management"); err != nil {
					return err
				}
				// Mark as worktree
				if err := fs.WriteString(filepath.Join(contextDir, ".git"), "gitdir: ../../../grove-context/.git/worktrees/general-refactoring"); err != nil {
					return err
				}

				ctx.Set("contextDir", contextDir)
				ctx.Set("coreDir", coreDir)
				return nil
			}),

			harness.NewStep("Test simple alias @a:grove-core::dev-with-tests", func(ctx *harness.Context) error {
				cx, err := FindProjectBinary()
				if err != nil {
					return err
				}

				contextDir := ctx.Get("contextDir").(string)

				// Create rules importing from sibling via simple name
				rules := `context.go
@a:grove-core::dev-with-tests`
				if err := fs.WriteString(filepath.Join(contextDir, ".grove", "rules"), rules); err != nil {
					return err
				}

				cmd := ctx.Command(cx, "list").Dir(contextDir)
				result := cmd.Run()
				ctx.ShowCommandOutput(cmd.String(), result.Stdout, result.Stderr)

				if result.Error != nil {
					return result.Error
				}

				output := result.Stdout
				if !strings.Contains(output, "context.go") {
					return fmt.Errorf("context is missing local file 'context.go'\nOutput:\n%s", output)
				}
				if !strings.Contains(output, "core.go") {
					return fmt.Errorf("context is missing imported 'core.go' from grove-core's dev-with-tests ruleset\nOutput:\n%s", output)
				}
				if !strings.Contains(output, "core_test.go") {
					return fmt.Errorf("context is missing imported 'core_test.go' from grove-core's dev-with-tests ruleset\nOutput:\n%s", output)
				}
				return nil
			}),

			harness.NewStep("Test 2-part alias @a:general-refactoring:grove-core::dev-with-tests", func(ctx *harness.Context) error {
				cx, err := FindProjectBinary()
				if err != nil {
					return err
				}

				contextDir := ctx.Get("contextDir").(string)

				// Create rules importing with eco-worktree:project format
				rules := `context.go
@a:general-refactoring:grove-core::dev-with-tests`
				if err := fs.WriteString(filepath.Join(contextDir, ".grove", "rules"), rules); err != nil {
					return err
				}

				cmd := ctx.Command(cx, "list").Dir(contextDir)
				result := cmd.Run()
				ctx.ShowCommandOutput(cmd.String(), result.Stdout, result.Stderr)

				if result.Error != nil {
					return result.Error
				}

				output := result.Stdout
				if !strings.Contains(output, "context.go") {
					return fmt.Errorf("context is missing local file 'context.go'\nOutput:\n%s", output)
				}
				if !strings.Contains(output, "core.go") {
					return fmt.Errorf("context is missing imported 'core.go'\nOutput:\n%s", output)
				}
				return nil
			}),

			harness.NewStep("Test 2-part alias with glob @a:general-refactoring:grove-core/**/*.go", func(ctx *harness.Context) error {
				cx, err := FindProjectBinary()
				if err != nil {
					return err
				}

				contextDir := ctx.Get("contextDir").(string)

				// Create rules using eco-worktree:project format with glob pattern
				rules := `context.go
@a:general-refactoring:grove-core/**/*.go`
				if err := fs.WriteString(filepath.Join(contextDir, ".grove", "rules"), rules); err != nil {
					return err
				}

				cmd := ctx.Command(cx, "list").Dir(contextDir)
				result := cmd.Run()
				ctx.ShowCommandOutput(cmd.String(), result.Stdout, result.Stderr)

				if result.Error != nil {
					return result.Error
				}

				output := result.Stdout
				if !strings.Contains(output, "core.go") {
					return fmt.Errorf("context is missing 'core.go' from 2-part alias with glob\nOutput:\n%s", output)
				}
				if !strings.Contains(output, "utils.go") {
					return fmt.Errorf("context is missing 'utils.go' from 2-part alias with glob\nOutput:\n%s", output)
				}
				return nil
			}),

			harness.NewStep("Test 3-part alias @a:grove-ecosystem:general-refactoring:grove-core/**/*.go", func(ctx *harness.Context) error {
				cx, err := FindProjectBinary()
				if err != nil {
					return err
				}

				contextDir := ctx.Get("contextDir").(string)

				// Create rules using full hierarchy format
				rules := `context.go
@a:grove-ecosystem:general-refactoring:grove-core/**/*.go`
				if err := fs.WriteString(filepath.Join(contextDir, ".grove", "rules"), rules); err != nil {
					return err
				}

				cmd := ctx.Command(cx, "list").Dir(contextDir)
				result := cmd.Run()
				ctx.ShowCommandOutput(cmd.String(), result.Stdout, result.Stderr)

				if result.Error != nil {
					return result.Error
				}

				output := result.Stdout
				if !strings.Contains(output, "core.go") {
					return fmt.Errorf("context is missing 'core.go' from 3-part alias\nOutput:\n%s", output)
				}
				if !strings.Contains(output, "utils.go") {
					return fmt.Errorf("context is missing 'utils.go' from 3-part alias\nOutput:\n%s", output)
				}
				return nil
			}),
		},
	}
}

// AliasResolutionFromEcosystemWorktreeRootScenario tests alias resolution from the root of an ecosystem worktree.
func AliasResolutionFromEcosystemWorktreeRootScenario() *harness.Scenario {
	return &harness.Scenario{
		Name:        "cx-alias-resolution-from-ecosystem-worktree-root",
		Description: "Tests that aliases resolve to child projects when CWD is an ecosystem worktree root.",
		Tags:        []string{"cx", "alias", "ecosystem", "worktree", "regression"},
		Steps: []harness.Step{
			harness.NewStep("Setup ecosystem with worktree and decoy project", func(ctx *harness.Context) error {
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

				// Create ecosystem root
				ecoRootDir := filepath.Join(grovesDir, "my-ecosystem")
				ecoConfig := `name: my-ecosystem
workspaces:
  - "*"`
				fs.WriteString(filepath.Join(ecoRootDir, "grove.yml"), ecoConfig)
				fs.WriteString(filepath.Join(ecoRootDir, ".gitmodules"), "# ecosystem")
				command.New("git", "init").Dir(ecoRootDir).Run()

				// Create ecosystem worktree
				worktreeDir := filepath.Join(ecoRootDir, ".grove-worktrees", "feature-branch")
				fs.WriteString(filepath.Join(worktreeDir, "grove.yml"), ecoConfig)
				fs.WriteString(filepath.Join(worktreeDir, ".gitmodules"), "# ecosystem worktree")
				fs.WriteString(filepath.Join(worktreeDir, ".git"), "gitdir: ../../.git/worktrees/feature-branch")
				ctx.Set("worktreeDir", worktreeDir)

				// Create project-beta inside the worktree (the correct target)
				projectBetaWorktreeDir := filepath.Join(worktreeDir, "project-beta")
				fs.WriteString(filepath.Join(projectBetaWorktreeDir, "grove.yml"), `name: project-beta`)
				fs.WriteString(filepath.Join(projectBetaWorktreeDir, "beta.go"), "package beta // from worktree")
				command.New("git", "init").Dir(projectBetaWorktreeDir).Run()

				// Create a decoy project-beta outside the ecosystem (the incorrect target)
				decoyProjectBetaDir := filepath.Join(grovesDir, "project-beta")
				fs.WriteString(filepath.Join(decoyProjectBetaDir, "grove.yml"), `name: project-beta`)
				fs.WriteString(filepath.Join(decoyProjectBetaDir, "decoy.go"), "package beta // from decoy")
				command.New("git", "init").Dir(decoyProjectBetaDir).Run()

				return nil
			}),
			harness.NewStep("Run 'cx list' from worktree root with simple alias", func(ctx *harness.Context) error {
				cx, _ := FindProjectBinary()
				testConfigHome := ctx.Get("testConfigHome").(string)
				worktreeDir := ctx.Get("worktreeDir").(string)

				// Create rules file in the ecosystem worktree root
				rules := `@a:project-beta/**/*.go`
				fs.WriteString(filepath.Join(worktreeDir, ".grove", "rules"), rules)

				// Run the command from the ecosystem worktree root
				cmd := command.New(cx, "list").Dir(worktreeDir).Env(fmt.Sprintf("XDG_CONFIG_HOME=%s", testConfigHome))
				result := cmd.Run()
				ctx.ShowCommandOutput(cmd.String(), result.Stdout, result.Stderr)
				if result.Error != nil {
					return result.Error
				}

				output := result.Stdout
				// Verify the correct file is included (from the worktree)
				if !strings.Contains(output, "beta.go") {
					return fmt.Errorf("context is missing 'beta.go' from the worktree's child project")
				}
				// Verify the decoy file is NOT included
				if strings.Contains(output, "decoy.go") {
					return fmt.Errorf("context incorrectly included 'decoy.go' from the external project")
				}

				return nil
			}),
		},
	}
}
