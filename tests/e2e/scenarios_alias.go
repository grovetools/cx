// File: grove-context/tests/e2e/scenarios_alias.go
package main

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/mattsolo1/grove-tend/pkg/command"
	"github.com/mattsolo1/grove-tend/pkg/fs"
	"github.com/mattsolo1/grove-tend/pkg/harness"
)

// AliasSiblingResolutionScenario tests sibling resolution within ecosystem worktrees.
func AliasSiblingResolutionScenario() *harness.Scenario {
	return &harness.Scenario{
		Name:        "cx-alias-siblings",
		Description: "Tests context-aware sibling resolution when working in ecosystem worktree repos.",
		Tags:        []string{"cx", "alias", "siblings", "worktree"},
		Steps: []harness.Step{
			harness.NewStep("Setup ecosystem with main repos and worktree", func(ctx *harness.Context) error {
				grovesDir := filepath.Join(ctx.RootDir, "mock-groves")
				testConfigHome := filepath.Join(ctx.RootDir, ".test-config")
				groveConfigDir := filepath.Join(testConfigHome, "grove")
				ctx.Set("testConfigHome", testConfigHome)

				// Create global grove.yml
				groveConfig := fmt.Sprintf(`groves:
  test:
    path: %s
    enabled: true
`, grovesDir)
				if err := fs.WriteString(filepath.Join(groveConfigDir, "grove.yml"), groveConfig); err != nil {
					return err
				}

				// Create ecosystem
				ecoDir := filepath.Join(grovesDir, "my-ecosystem")
				if err := fs.WriteString(filepath.Join(ecoDir, ".gitmodules"), "# ecosystem"); err != nil {
					return err
				}
				ecoConfig := `name: my-ecosystem
workspaces:
  - "*"`
				if err := fs.WriteString(filepath.Join(ecoDir, "grove.yml"), ecoConfig); err != nil {
					return err
				}
				if result := command.New("git", "init").Dir(ecoDir).Run(); result.Error != nil {
					return fmt.Errorf("failed to init git in ecosystem: %w", result.Error)
				}

				// Create main repos in ecosystem
				repoADir := filepath.Join(ecoDir, "repo-a")
				if err := fs.WriteString(filepath.Join(repoADir, "main.go"), "package main // main version"); err != nil {
					return err
				}
				if err := fs.WriteString(filepath.Join(repoADir, "utils.go"), "package main // main utils"); err != nil {
					return err
				}
				if err := fs.WriteString(filepath.Join(repoADir, "grove.yml"), `name: repo-a`); err != nil {
					return err
				}
				if result := command.New("git", "init").Dir(repoADir).Run(); result.Error != nil {
					return fmt.Errorf("failed to init git in repo-a: %w", result.Error)
				}

				repoBDir := filepath.Join(ecoDir, "repo-b")
				if err := fs.WriteString(filepath.Join(repoBDir, "main.go"), "package main // main version"); err != nil {
					return err
				}
				if err := fs.WriteString(filepath.Join(repoBDir, "helper.go"), "package main // main helper"); err != nil {
					return err
				}
				if err := fs.WriteString(filepath.Join(repoBDir, "grove.yml"), `name: repo-b`); err != nil {
					return err
				}
				if result := command.New("git", "init").Dir(repoBDir).Run(); result.Error != nil {
					return fmt.Errorf("failed to init git in repo-b: %w", result.Error)
				}

				// Create ecosystem worktree: my-ecosystem/.grove-worktrees/feature-x
				worktreeDir := filepath.Join(ecoDir, ".grove-worktrees", "feature-x")
				if err := fs.WriteString(filepath.Join(worktreeDir, ".gitmodules"), "# ecosystem worktree"); err != nil {
					return err
				}
				if err := fs.WriteString(filepath.Join(worktreeDir, "grove.yml"), ecoConfig); err != nil {
					return err
				}
				// Mark as worktree with .git file
				if err := fs.WriteString(filepath.Join(worktreeDir, ".git"), "gitdir: ../../.git/worktrees/feature-x"); err != nil {
					return err
				}

				// Create worktree versions of repos (siblings in same worktree)
				repoAWorktreeDir := filepath.Join(worktreeDir, "repo-a")
				if err := fs.WriteString(filepath.Join(repoAWorktreeDir, "main.go"), "package main // worktree version"); err != nil {
					return err
				}
				if err := fs.WriteString(filepath.Join(repoAWorktreeDir, "utils.go"), "package main // worktree utils"); err != nil {
					return err
				}
				if err := fs.WriteString(filepath.Join(repoAWorktreeDir, "feature.go"), "package main // NEW in worktree"); err != nil {
					return err
				}
				if err := fs.WriteString(filepath.Join(repoAWorktreeDir, "grove.yml"), `name: repo-a`); err != nil {
					return err
				}
				if result := command.New("git", "init").Dir(repoAWorktreeDir).Run(); result.Error != nil {
					return fmt.Errorf("failed to init git in repo-a worktree: %w", result.Error)
				}

				repoBWorktreeDir := filepath.Join(worktreeDir, "repo-b")
				if err := fs.WriteString(filepath.Join(repoBWorktreeDir, "main.go"), "package main // worktree version"); err != nil {
					return err
				}
				if err := fs.WriteString(filepath.Join(repoBWorktreeDir, "helper.go"), "package main // worktree helper"); err != nil {
					return err
				}
				if err := fs.WriteString(filepath.Join(repoBWorktreeDir, "worktree_only.go"), "package main // ONLY in worktree"); err != nil {
					return err
				}
				if err := fs.WriteString(filepath.Join(repoBWorktreeDir, "grove.yml"), `name: repo-b`); err != nil {
					return err
				}
				if result := command.New("git", "init").Dir(repoBWorktreeDir).Run(); result.Error != nil {
					return fmt.Errorf("failed to init git in repo-b worktree: %w", result.Error)
				}

				ctx.Set("repoAWorktreeDir", repoAWorktreeDir)
				ctx.Set("repoBWorktreeDir", repoBWorktreeDir)
				ctx.Set("repoADir", repoADir)

				return nil
			}),

			harness.NewStep("Test sibling resolution from repo-a worktree", func(ctx *harness.Context) error {
				// When working in repo-a worktree, @alias:repo-b should resolve to sibling repo-b worktree
				cx, err := FindProjectBinary()
				if err != nil {
					return err
				}

				repoAWorktreeDir := ctx.Get("repoAWorktreeDir").(string)
				testConfigHome := ctx.Get("testConfigHome").(string)

				// Create rules in repo-a that reference repo-b
				rules := `@alias:repo-b/**/*.go`
				if err := fs.WriteString(filepath.Join(repoAWorktreeDir, ".grove", "rules"), rules); err != nil {
					return err
				}

				cmd := command.New(cx, "list").Dir(repoAWorktreeDir).Env(fmt.Sprintf("XDG_CONFIG_HOME=%s", testConfigHome))
				result := cmd.Run()
				ctx.ShowCommandOutput(cmd.String(), result.Stdout, result.Stderr)

				if result.Error != nil {
					return result.Error
				}

				output := result.Stdout
				// Should include worktree sibling's unique file
				if !strings.Contains(output, "worktree_only.go") {
					return fmt.Errorf("should resolve to sibling repo-b worktree, missing 'worktree_only.go'\nOutput:\n%s", output)
				}
				// Should NOT include main version's file that doesn't exist in worktree
				// (both have helper.go, so we can't check that, but worktree_only.go is unique)

				return nil
			}),

			harness.NewStep("Test explicit ecosystem:repo from worktree context", func(ctx *harness.Context) error {
				// From repo-a worktree, @alias:my-ecosystem:repo-b should resolve to MAIN version
				cx, err := FindProjectBinary()
				if err != nil {
					return err
				}

				repoAWorktreeDir := ctx.Get("repoAWorktreeDir").(string)
				testConfigHome := ctx.Get("testConfigHome").(string)

				// Use explicit ecosystem:repo namespace to get main version
				rules := `@alias:my-ecosystem:repo-b/**/*.go`
				if err := fs.WriteString(filepath.Join(repoAWorktreeDir, ".grove", "rules"), rules); err != nil {
					return err
				}

				cmd := command.New(cx, "list").Dir(repoAWorktreeDir).Env(fmt.Sprintf("XDG_CONFIG_HOME=%s", testConfigHome))
				result := cmd.Run()
				ctx.ShowCommandOutput(cmd.String(), result.Stdout, result.Stderr)

				if result.Error != nil {
					return result.Error
				}

				output := result.Stdout
				// Should include main version's files
				if !strings.Contains(output, "helper.go") {
					return fmt.Errorf("should resolve to main repo-b, missing 'helper.go'\nOutput:\n%s", output)
				}
				// Should NOT include worktree-only file
				if strings.Contains(output, "worktree_only.go") {
					return fmt.Errorf("should NOT include worktree version when using explicit namespace\nOutput:\n%s", output)
				}

				return nil
			}),

			harness.NewStep("Test simple name from main repo resolves to main", func(ctx *harness.Context) error {
				// From main repo-a, @alias:repo-b should resolve to main repo-b (both top-level)
				cx, err := FindProjectBinary()
				if err != nil {
					return err
				}

				repoADir := ctx.Get("repoADir").(string)
				testConfigHome := ctx.Get("testConfigHome").(string)

				rules := `@alias:repo-b/**/*.go`
				if err := fs.WriteString(filepath.Join(repoADir, ".grove", "rules"), rules); err != nil {
					return err
				}

				cmd := command.New(cx, "list").Dir(repoADir).Env(fmt.Sprintf("XDG_CONFIG_HOME=%s", testConfigHome))
				result := cmd.Run()
				ctx.ShowCommandOutput(cmd.String(), result.Stdout, result.Stderr)

				if result.Error != nil {
					return result.Error
				}

				output := result.Stdout
				// Should include main version
				if !strings.Contains(output, "helper.go") {
					return fmt.Errorf("should resolve to main repo-b\nOutput:\n%s", output)
				}
				// Should NOT include worktree-only file
				if strings.Contains(output, "worktree_only.go") {
					return fmt.Errorf("should NOT include worktree version from main context\nOutput:\n%s", output)
				}

				return nil
			}),
		},
	}
}

// AliasNamespacingScenario tests explicit ecosystem:repo namespacing.
func AliasNamespacingScenario() *harness.Scenario {
	return &harness.Scenario{
		Name:        "cx-alias-namespacing",
		Description: "Tests explicit ecosystem:repo namespacing to disambiguate projects with duplicate names.",
		Tags:        []string{"cx", "alias", "namespacing"},
		Steps: []harness.Step{
			harness.NewStep("Setup two ecosystems with duplicate repo names", func(ctx *harness.Context) error {
				// Create groves directory inside test root for isolation
				grovesDir := filepath.Join(ctx.RootDir, "mock-groves")
				testConfigHome := filepath.Join(ctx.RootDir, ".test-config")
				groveConfigDir := filepath.Join(testConfigHome, "grove")
				ctx.Set("testConfigHome", testConfigHome)

				// Create global grove.yml
				groveConfig := fmt.Sprintf(`groves:
  test:
    path: %s
    enabled: true
`, grovesDir)
				if err := fs.WriteString(filepath.Join(groveConfigDir, "grove.yml"), groveConfig); err != nil {
					return err
				}

				// Ecosystem 1: eco-alpha
				ecoAlphaDir := filepath.Join(grovesDir, "eco-alpha")
				if err := fs.WriteString(filepath.Join(ecoAlphaDir, ".gitmodules"), "# ecosystem"); err != nil {
					return err
				}
				ecoAlphaConfig := `name: eco-alpha
workspaces:
  - "*"`
				if err := fs.WriteString(filepath.Join(ecoAlphaDir, "grove.yml"), ecoAlphaConfig); err != nil {
					return err
				}
				if result := command.New("git", "init").Dir(ecoAlphaDir).Run(); result.Error != nil {
					return fmt.Errorf("failed to init git in eco-alpha: %w", result.Error)
				}

				// Repo in eco-alpha: shared-lib
				sharedLibAlphaDir := filepath.Join(ecoAlphaDir, "shared-lib")
				if err := fs.WriteString(filepath.Join(sharedLibAlphaDir, "lib.go"), "package sharedlib"); err != nil {
					return err
				}
				if err := fs.WriteString(filepath.Join(sharedLibAlphaDir, "alpha_feature.go"), "package sharedlib"); err != nil {
					return err
				}
				if err := fs.WriteString(filepath.Join(sharedLibAlphaDir, "grove.yml"), `name: shared-lib`); err != nil {
					return err
				}
				if result := command.New("git", "init").Dir(sharedLibAlphaDir).Run(); result.Error != nil {
					return fmt.Errorf("failed to init git in shared-lib: %w", result.Error)
				}

				// Ecosystem 2: eco-beta
				ecoBetaDir := filepath.Join(grovesDir, "eco-beta")
				if err := fs.WriteString(filepath.Join(ecoBetaDir, ".gitmodules"), "# ecosystem"); err != nil {
					return err
				}
				ecoBetaConfig := `name: eco-beta
workspaces:
  - "*"`
				if err := fs.WriteString(filepath.Join(ecoBetaDir, "grove.yml"), ecoBetaConfig); err != nil {
					return err
				}
				if result := command.New("git", "init").Dir(ecoBetaDir).Run(); result.Error != nil {
					return fmt.Errorf("failed to init git in eco-beta: %w", result.Error)
				}

				// Repo in eco-beta: shared-lib (same name as in eco-alpha)
				sharedLibBetaDir := filepath.Join(ecoBetaDir, "shared-lib")
				if err := fs.WriteString(filepath.Join(sharedLibBetaDir, "lib.go"), "package sharedlib"); err != nil {
					return err
				}
				if err := fs.WriteString(filepath.Join(sharedLibBetaDir, "beta_feature.go"), "package sharedlib"); err != nil {
					return err
				}
				if err := fs.WriteString(filepath.Join(sharedLibBetaDir, "grove.yml"), `name: shared-lib`); err != nil {
					return err
				}
				if result := command.New("git", "init").Dir(sharedLibBetaDir).Run(); result.Error != nil {
					return fmt.Errorf("failed to init git in shared-lib beta: %w", result.Error)
				}

				return nil
			}),

			harness.NewStep("Test @alias:eco-alpha:shared-lib resolves correctly", func(ctx *harness.Context) error {
				cx, err := FindProjectBinary()
				if err != nil {
					return err
				}

				testConfigHome := ctx.Get("testConfigHome").(string)
				rules := `@alias:eco-alpha:shared-lib/**/*.go`
				if err := fs.WriteString(filepath.Join(ctx.RootDir, ".grove", "rules"), rules); err != nil {
					return err
				}
				if err := fs.WriteString(filepath.Join(ctx.RootDir, "grove.yml"), `name: test-main`); err != nil {
					return err
				}

				cmd := command.New(cx, "list").Dir(ctx.RootDir).Env(fmt.Sprintf("XDG_CONFIG_HOME=%s", testConfigHome))
				result := cmd.Run()
				ctx.ShowCommandOutput(cmd.String(), result.Stdout, result.Stderr)

				if result.Error != nil {
					return result.Error
				}

				output := result.Stdout
				if !strings.Contains(output, "alpha_feature.go") {
					return fmt.Errorf("should include alpha_feature.go from eco-alpha\nOutput:\n%s", output)
				}
				if strings.Contains(output, "beta_feature.go") {
					return fmt.Errorf("should NOT include beta_feature.go from eco-beta\nOutput:\n%s", output)
				}

				return nil
			}),

			harness.NewStep("Test @alias:eco-beta:shared-lib resolves correctly", func(ctx *harness.Context) error {
				cx, err := FindProjectBinary()
				if err != nil {
					return err
				}

				testConfigHome := ctx.Get("testConfigHome").(string)
				rules := `@alias:eco-beta:shared-lib/**/*.go`
				if err := fs.WriteString(filepath.Join(ctx.RootDir, ".grove", "rules"), rules); err != nil {
					return err
				}

				cmd := command.New(cx, "list").Dir(ctx.RootDir).Env(fmt.Sprintf("XDG_CONFIG_HOME=%s", testConfigHome))
				result := cmd.Run()
				ctx.ShowCommandOutput(cmd.String(), result.Stdout, result.Stderr)

				if result.Error != nil {
					return result.Error
				}

				output := result.Stdout
				if !strings.Contains(output, "beta_feature.go") {
					return fmt.Errorf("should include beta_feature.go from eco-beta\nOutput:\n%s", output)
				}
				if strings.Contains(output, "alpha_feature.go") {
					return fmt.Errorf("should NOT include alpha_feature.go from eco-alpha\nOutput:\n%s", output)
				}

				return nil
			}),
		},
	}
}

// AliasWorkflowScenario tests the full lifecycle of using aliases in rules files.
func AliasWorkflowScenario() *harness.Scenario {
	return &harness.Scenario{
		Name:        "cx-alias-workflow",
		Description: "Tests alias resolution for hot, cold, exclusion, and view directives.",
		Tags:        []string{"cx", "alias", "rules"},
		Steps: []harness.Step{
			harness.NewStep("Setup multi-project environment", func(ctx *harness.Context) error {
				// Create test projects in a groves directory and configure discovery to find them
				grovesDir := filepath.Join(ctx.RootDir, "mock-groves")
				ctx.Set("grovesDir", grovesDir)

				// Set up XDG_CONFIG_HOME to point to a test config directory
				// This allows us to control what groves discovery finds
				testConfigHome := filepath.Join(ctx.RootDir, ".test-config")
				groveConfigDir := filepath.Join(testConfigHome, "grove")
				ctx.Set("testConfigHome", testConfigHome)

				// Create global grove.yml with groves configuration
				groveConfig := fmt.Sprintf(`groves:
  test:
    path: %s
    enabled: true
`, grovesDir)
				if err := fs.WriteString(filepath.Join(groveConfigDir, "grove.yml"), groveConfig); err != nil {
					return err
				}

				// External project 1: "lib-alpha" in the groves directory
				libAlphaDir := filepath.Join(grovesDir, "lib-alpha")
				if err := fs.WriteString(filepath.Join(libAlphaDir, "alpha.go"), "package alpha"); err != nil {
					return err
				}
				if err := fs.WriteString(filepath.Join(libAlphaDir, "alpha_test.go"), "package alpha_test"); err != nil {
					return err
				}
				if err := fs.WriteString(filepath.Join(libAlphaDir, "docs/README.md"), "Alpha Docs"); err != nil {
					return err
				}
				if err := fs.WriteString(filepath.Join(libAlphaDir, "grove.yml"), `name: lib-alpha`); err != nil {
					return err
				}
				// Initialize as git repo
				if result := command.New("git", "init").Dir(libAlphaDir).Run(); result.Error != nil {
					return fmt.Errorf("failed to init git in lib-alpha: %w", result.Error)
				}

				// External project 2: "app-beta" in the groves directory
				appBetaDir := filepath.Join(grovesDir, "app-beta")
				if err := fs.WriteString(filepath.Join(appBetaDir, "beta.go"), "package beta"); err != nil {
					return err
				}
				if err := fs.WriteString(filepath.Join(appBetaDir, "README.md"), "Beta App"); err != nil {
					return err
				}
				if err := fs.WriteString(filepath.Join(appBetaDir, "grove.yml"), `name: app-beta`); err != nil {
					return err
				}
				// Initialize as git repo
				if result := command.New("git", "init").Dir(appBetaDir).Run(); result.Error != nil {
					return fmt.Errorf("failed to init git in app-beta: %w", result.Error)
				}

				// Files in main project
				if err := fs.WriteString(filepath.Join(ctx.RootDir, "grove.yml"), `name: test-main`); err != nil {
					return err
				}
				return fs.WriteString(filepath.Join(ctx.RootDir, "main.go"), "package main")
			}),
			harness.NewStep("Create rules file using aliases", func(ctx *harness.Context) error {
				rules := `@view: @alias:app-beta
# Hot context
main.go
@alias:lib-alpha/**/*.go
!@alias:lib-alpha/**/*_test.go
---
# Cold context
@alias:lib-alpha/docs/**
`
				return fs.WriteString(filepath.Join(ctx.RootDir, ".grove", "rules"), rules)
			}),
			harness.NewStep("Run 'cx list' and verify hot context", func(ctx *harness.Context) error {
				cx, err := FindProjectBinary()
				if err != nil {
					return err
				}
				testConfigHome := ctx.Get("testConfigHome").(string)
				cmd := command.New(cx, "list").Dir(ctx.RootDir).Env(fmt.Sprintf("XDG_CONFIG_HOME=%s", testConfigHome))

				result := cmd.Run()
				ctx.ShowCommandOutput(cmd.String(), result.Stdout, result.Stderr)
				if result.Error != nil {
					return result.Error
				}

				output := result.Stdout
				if !strings.Contains(output, "main.go") {
					return fmt.Errorf("hot context missing local file 'main.go'\nOutput:\n%s\nStderr:\n%s", output, result.Stderr)
				}
				if !strings.Contains(output, "alpha.go") {
					return fmt.Errorf("hot context missing aliased file 'alpha.go'\nOutput:\n%s\nStderr:\n%s", output, result.Stderr)
				}
				if strings.Contains(output, "alpha_test.go") {
					return fmt.Errorf("hot context should not contain excluded aliased file 'alpha_test.go'")
				}
				if strings.Contains(output, "docs/README.md") {
					return fmt.Errorf("hot context should not contain cold-context aliased file 'docs/README.md'")
				}
				return nil
			}),
			harness.NewStep("Run 'cx list-cache' and verify cold context", func(ctx *harness.Context) error {
				cx, err := FindProjectBinary()
				if err != nil {
					return err
				}
				testConfigHome := ctx.Get("testConfigHome").(string)
				cmd := command.New(cx, "list-cache").Dir(ctx.RootDir).Env(fmt.Sprintf("XDG_CONFIG_HOME=%s", testConfigHome))

				result := cmd.Run()
				ctx.ShowCommandOutput(cmd.String(), result.Stdout, result.Stderr)
				if result.Error != nil {
					return result.Error
				}

				output := result.Stdout
				if !strings.Contains(output, "docs/README.md") {
					return fmt.Errorf("cold context missing aliased file 'docs/README.md'")
				}
				if strings.Contains(output, "alpha.go") {
					return fmt.Errorf("cold context should not contain hot-context aliased file 'alpha.go'")
				}
				return nil
			}),
			// Note: TUI view test commented out due to tmux environment complexities
			// The core alias resolution functionality is verified in the list tests above
		},
	}
}
