// File: grove-context/tests/e2e/scenarios_composite_rulesets.go
package main

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/grovetools/tend/pkg/command"
	"github.com/grovetools/tend/pkg/fs"
	"github.com/grovetools/tend/pkg/harness"
	"github.com/grovetools/tend/pkg/verify"
)

// CompositeRulesetBasicScenario tests basic @include: resolution, multiple includes,
// local overrides, and missing include warnings.
func CompositeRulesetBasicScenario() *harness.Scenario {
	return &harness.Scenario{
		Name:        "cx-composite-ruleset-basic",
		Description: "Tests @include: basic resolution, multiple includes, local overrides, and missing include warning.",
		Tags:        []string{"cx", "composite", "include"},
		Steps: []harness.Step{
			harness.NewStep("Setup test project with presets", func(ctx *harness.Context) error {
				// Create grove.yml so FindRulesetFile can locate the project root
				if err := fs.WriteString(filepath.Join(ctx.RootDir, "grove.yml"), "context: {}"); err != nil {
					return err
				}

				// Create test files
				for _, f := range []struct{ name, content string }{
					{"main.go", "package main"},
					{"main_test.go", "package main_test"},
					{"utils.go", "package utils"},
					{"utils_test.go", "package utils_test"},
					{"README.md", "# Readme"},
				} {
					if err := fs.WriteString(filepath.Join(ctx.RootDir, f.name), f.content); err != nil {
						return err
					}
				}

				// Create preset rulesets in .cx/
				if err := fs.WriteString(filepath.Join(ctx.RootDir, ".cx", "base.rules"), "*.go"); err != nil {
					return err
				}
				if err := fs.WriteString(filepath.Join(ctx.RootDir, ".cx", "docs.rules"), "*.md"); err != nil {
					return err
				}

				// Create active rules with includes and a local override
				rulesContent := `@include: nonexistent_preset
@include: base
@include: docs
!*_test.go`
				return fs.WriteString(filepath.Join(ctx.RootDir, ".grove", "rules"), rulesContent)
			}),
			harness.NewStep("Run cx list and verify output", func(ctx *harness.Context) error {
				cxBinary, err := FindProjectBinary()
				if err != nil {
					return err
				}

				cmd := command.New(cxBinary, "list").Dir(ctx.RootDir)
				result := cmd.Run()
				ctx.ShowCommandOutput(cmd.String(), result.Stdout, result.Stderr)
				if result.Error != nil {
					return result.Error
				}

				return ctx.Verify(func(v *verify.Collector) {
					// Included via base preset
					v.Contains("main.go included", result.Stdout, "main.go")
					v.Contains("utils.go included", result.Stdout, "utils.go")
					// Included via docs preset
					v.Contains("README.md included", result.Stdout, "README.md")
					// Excluded by local !*_test.go override
					v.NotContains("main_test.go excluded", result.Stdout, "main_test.go")
					v.NotContains("utils_test.go excluded", result.Stdout, "utils_test.go")
				})
			}),
			harness.NewStep("Verify missing include warning", func(ctx *harness.Context) error {
				cxBinary, err := FindProjectBinary()
				if err != nil {
					return err
				}

				cmd := command.New(cxBinary, "list").Dir(ctx.RootDir)
				result := cmd.Run()

				combinedOutput := result.Stdout + result.Stderr
				if !strings.Contains(combinedOutput, "nonexistent_preset") {
					return fmt.Errorf("expected warning about nonexistent_preset in output, got stdout: %s\nstderr: %s", result.Stdout, result.Stderr)
				}
				return nil
			}),
		},
	}
}

// CompositeRulesetColdContextScenario tests @include: in the cold section.
func CompositeRulesetColdContextScenario() *harness.Scenario {
	return &harness.Scenario{
		Name:        "cx-composite-ruleset-cold",
		Description: "Tests @include: in cold context section puts included rules into cold context.",
		Tags:        []string{"cx", "composite", "include", "cold"},
		Steps: []harness.Step{
			harness.NewStep("Setup test project with cold include", func(ctx *harness.Context) error {
				if err := fs.WriteString(filepath.Join(ctx.RootDir, "grove.yml"), "context: {}"); err != nil {
					return err
				}

				// Create test files
				if err := fs.WriteString(filepath.Join(ctx.RootDir, "hot.go"), "package main"); err != nil {
					return err
				}
				if err := fs.WriteString(filepath.Join(ctx.RootDir, "cold.txt"), "cold text content"); err != nil {
					return err
				}
				if err := fs.WriteString(filepath.Join(ctx.RootDir, "cold.md"), "# Cold doc"); err != nil {
					return err
				}

				// Create preset for text files
				if err := fs.WriteString(filepath.Join(ctx.RootDir, ".cx", "text.rules"), "*.txt\n*.md"); err != nil {
					return err
				}

				// Active rules: hot section has *.go, cold section includes text preset
				rulesContent := "*.go\n---\n@include: text"
				return fs.WriteString(filepath.Join(ctx.RootDir, ".grove", "rules"), rulesContent)
			}),
			harness.NewStep("Run cx generate", func(ctx *harness.Context) error {
				cxBinary, err := FindProjectBinary()
				if err != nil {
					return err
				}

				cmd := ctx.Command(cxBinary, "generate").Dir(ctx.RootDir)
				result := cmd.Run()
				ctx.ShowCommandOutput(cmd.String(), result.Stdout, result.Stderr)
				return result.Error
			}),
			harness.NewStep("Verify hot context has only .go files", func(ctx *harness.Context) error {
				contextPath := findContextFileOrFallback(ctx.RootDir)
				content, err := fs.ReadString(contextPath)
				if err != nil {
					return fmt.Errorf("could not read context file: %w", err)
				}

				return ctx.Verify(func(v *verify.Collector) {
					v.Contains("hot.go in hot context", content, "hot.go")
					v.NotContains("cold.txt not in hot context", content, "cold.txt")
					v.NotContains("cold.md not in hot context", content, "cold.md")
				})
			}),
			harness.NewStep("Verify cold context has text files", func(ctx *harness.Context) error {
				cachedPath := findCachedContextFileOrFallback(ctx.RootDir)
				content, err := fs.ReadString(cachedPath)
				if err != nil {
					return fmt.Errorf("could not read cached context file: %w", err)
				}

				return ctx.Verify(func(v *verify.Collector) {
					v.Contains("cold.txt in cold context", content, "cold.txt")
					v.Contains("cold.md in cold context", content, "cold.md")
				})
			}),
		},
	}
}

// CompositeRulesetPathAndNestedScenario tests path-based includes and nested includes.
func CompositeRulesetPathAndNestedScenario() *harness.Scenario {
	return &harness.Scenario{
		Name:        "cx-composite-ruleset-path-nested",
		Description: "Tests path-based @include: with relative paths and nested includes.",
		Tags:        []string{"cx", "composite", "include", "nested"},
		Steps: []harness.Step{
			harness.NewStep("Setup test project with nested path includes", func(ctx *harness.Context) error {
				if err := fs.WriteString(filepath.Join(ctx.RootDir, "grove.yml"), "context: {}"); err != nil {
					return err
				}

				// Create test files
				if err := fs.WriteString(filepath.Join(ctx.RootDir, "app.js"), "console.log('app')"); err != nil {
					return err
				}
				if err := fs.WriteString(filepath.Join(ctx.RootDir, "api.js"), "console.log('api')"); err != nil {
					return err
				}

				// Create nested rules: inner.rules includes *.js
				if err := fs.WriteString(filepath.Join(ctx.RootDir, ".grove", "nested", "inner.rules"), "*.js"); err != nil {
					return err
				}

				// Create custom.rules that includes inner.rules via relative path
				if err := fs.WriteString(filepath.Join(ctx.RootDir, ".grove", "subdir", "custom.rules"), "@include: ../nested/inner.rules"); err != nil {
					return err
				}

				// Active rules include the custom.rules via relative path
				return fs.WriteString(filepath.Join(ctx.RootDir, ".grove", "rules"), "@include: subdir/custom.rules")
			}),
			harness.NewStep("Run cx list and verify nested path resolution", func(ctx *harness.Context) error {
				cxBinary, err := FindProjectBinary()
				if err != nil {
					return err
				}

				cmd := command.New(cxBinary, "list").Dir(ctx.RootDir)
				result := cmd.Run()
				ctx.ShowCommandOutput(cmd.String(), result.Stdout, result.Stderr)
				if result.Error != nil {
					return result.Error
				}

				return ctx.Verify(func(v *verify.Collector) {
					v.Contains("app.js included via nested path", result.Stdout, "app.js")
					v.Contains("api.js included via nested path", result.Stdout, "api.js")
				})
			}),
		},
	}
}

// CompositeRulesetCircularScenario tests that circular @include: references don't loop infinitely.
func CompositeRulesetCircularScenario() *harness.Scenario {
	return &harness.Scenario{
		Name:        "cx-composite-ruleset-circular",
		Description: "Tests that circular @include: references are handled without infinite loops.",
		Tags:        []string{"cx", "composite", "include", "circular"},
		Steps: []harness.Step{
			harness.NewStep("Setup circular include presets", func(ctx *harness.Context) error {
				if err := fs.WriteString(filepath.Join(ctx.RootDir, "grove.yml"), "context: {}"); err != nil {
					return err
				}

				// Create test files
				if err := fs.WriteString(filepath.Join(ctx.RootDir, "a.go"), "package a"); err != nil {
					return err
				}
				if err := fs.WriteString(filepath.Join(ctx.RootDir, "b.go"), "package b"); err != nil {
					return err
				}

				// preset_a includes preset_b (and vice versa — circular)
				if err := fs.WriteString(filepath.Join(ctx.RootDir, ".cx", "preset_a.rules"), "a.go\n@include: preset_b"); err != nil {
					return err
				}
				if err := fs.WriteString(filepath.Join(ctx.RootDir, ".cx", "preset_b.rules"), "b.go\n@include: preset_a"); err != nil {
					return err
				}

				// Active rules include preset_a
				return fs.WriteString(filepath.Join(ctx.RootDir, ".grove", "rules"), "@include: preset_a")
			}),
			harness.NewStep("Run cx list - no infinite loop", func(ctx *harness.Context) error {
				cxBinary, err := FindProjectBinary()
				if err != nil {
					return err
				}

				cmd := command.New(cxBinary, "list").Dir(ctx.RootDir)
				result := cmd.Run()
				ctx.ShowCommandOutput(cmd.String(), result.Stdout, result.Stderr)
				if result.Error != nil {
					return result.Error
				}

				return ctx.Verify(func(v *verify.Collector) {
					v.Contains("a.go present", result.Stdout, "a.go")
					v.Contains("b.go present", result.Stdout, "b.go")
				})
			}),
		},
	}
}

// CompositeRulesetSearchDirectiveScenario tests that search directives propagate through @include:.
func CompositeRulesetSearchDirectiveScenario() *harness.Scenario {
	return &harness.Scenario{
		Name:        "cx-composite-ruleset-search-directive",
		Description: "Tests @grep: directive propagation through @include: to filter included rules.",
		Tags:        []string{"cx", "composite", "include", "search-directives"},
		Steps: []harness.Step{
			harness.NewStep("Setup test project with search directive include", func(ctx *harness.Context) error {
				if err := fs.WriteString(filepath.Join(ctx.RootDir, "grove.yml"), "context: {}"); err != nil {
					return err
				}

				// Create test files - one with magic word, one without
				if err := fs.WriteString(filepath.Join(ctx.RootDir, "file1.go"), "package main\n\n// magic-word"); err != nil {
					return err
				}
				if err := fs.WriteString(filepath.Join(ctx.RootDir, "file2.go"), "package main\n\n// normal text"); err != nil {
					return err
				}

				// Create base preset that includes all .go files
				if err := fs.WriteString(filepath.Join(ctx.RootDir, ".cx", "base.rules"), "*.go"); err != nil {
					return err
				}

				// Active rules include base with a grep filter
				return fs.WriteString(filepath.Join(ctx.RootDir, ".grove", "rules"), `@include: base @grep: "magic-word"`)
			}),
			harness.NewStep("Run cx list and verify grep filtering", func(ctx *harness.Context) error {
				cxBinary, err := FindProjectBinary()
				if err != nil {
					return err
				}

				cmd := command.New(cxBinary, "list").Dir(ctx.RootDir)
				result := cmd.Run()
				ctx.ShowCommandOutput(cmd.String(), result.Stdout, result.Stderr)
				if result.Error != nil {
					return result.Error
				}

				return ctx.Verify(func(v *verify.Collector) {
					v.Contains("file1.go included (has magic-word)", result.Stdout, "file1.go")
					v.NotContains("file2.go excluded (no magic-word)", result.Stdout, "file2.go")
				})
			}),
		},
	}
}
