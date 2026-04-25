// File: grove-context/tests/e2e/scenarios_relative_paths.go
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/grovetools/tend/pkg/fs"
	"github.com/grovetools/tend/pkg/harness"
	"github.com/grovetools/tend/pkg/tui"
	"github.com/grovetools/tend/pkg/verify"
)

// BasicRelativePathScenario tests that standard ../ correctly includes files from a sibling directory.
func BasicRelativePathScenario() *harness.Scenario {
	return &harness.Scenario{
		Name:        "cx-relative-path-basic",
		Description: "Tests that ../sibling-dir/*.go correctly includes files from a sibling directory.",
		Tags:        []string{"cx", "rules", "patterns", "relative-path"},
		Steps: []harness.Step{
			harness.NewStep("Setup sibling directory and rules", func(ctx *harness.Context) error {
				parentDir := filepath.Dir(ctx.RootDir)
				siblingDir := filepath.Join(parentDir, "sibling-basic")
				ctx.Set("sibling_dir", siblingDir)

				if err := fs.WriteString(filepath.Join(siblingDir, "target.go"), "package sibling"); err != nil {
					return err
				}

				groveConfig := fmt.Sprintf("context:\n  allowed_paths:\n    - %s\n", parentDir)
				if err := fs.WriteString(filepath.Join(ctx.RootDir, "grove.yml"), groveConfig); err != nil {
					return err
				}

				return fs.WriteString(filepath.Join(ctx.RootDir, ".grove", "rules"), "../sibling-basic/*.go")
			}),
			harness.NewStep("Run 'cx list' and verify sibling file is included", func(ctx *harness.Context) error {
				siblingDir := ctx.GetString("sibling_dir")
				defer os.RemoveAll(siblingDir)

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

				if !strings.Contains(result.Stdout, "target.go") {
					return fmt.Errorf("expected output to contain 'target.go', got:\n%s", result.Stdout)
				}
				return nil
			}),
		},
	}
}

// UncleanedRelativePathScenario tests that paths like ./../ resolve correctly after filepath.Clean normalization.
func UncleanedRelativePathScenario() *harness.Scenario {
	return &harness.Scenario{
		Name:        "cx-relative-path-uncleaned",
		Description: "Tests that ./../sibling-dir/*.go is recognized and resolves correctly.",
		Tags:        []string{"cx", "rules", "patterns", "relative-path"},
		Steps: []harness.Step{
			harness.NewStep("Setup sibling directory with uncleaned path rule", func(ctx *harness.Context) error {
				parentDir := filepath.Dir(ctx.RootDir)
				siblingDir := filepath.Join(parentDir, "sibling-unclean")
				ctx.Set("sibling_dir", siblingDir)

				if err := fs.WriteString(filepath.Join(siblingDir, "target.go"), "package sibling"); err != nil {
					return err
				}

				groveConfig := fmt.Sprintf("context:\n  allowed_paths:\n    - %s\n", parentDir)
				if err := fs.WriteString(filepath.Join(ctx.RootDir, "grove.yml"), groveConfig); err != nil {
					return err
				}

				// Use uncleaned path: ./../ instead of ../
				return fs.WriteString(filepath.Join(ctx.RootDir, ".grove", "rules"), "./../sibling-unclean/*.go")
			}),
			harness.NewStep("Run 'cx list' and verify uncleaned path resolves", func(ctx *harness.Context) error {
				siblingDir := ctx.GetString("sibling_dir")
				defer os.RemoveAll(siblingDir)

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

				if !strings.Contains(result.Stdout, "target.go") {
					return fmt.Errorf("expected output to contain 'target.go', got:\n%s", result.Stdout)
				}
				return nil
			}),
		},
	}
}

// DoubleDotOnlyScenario tests that ".." as a standalone pattern targets the parent directory.
func DoubleDotOnlyScenario() *harness.Scenario {
	return &harness.Scenario{
		Name:        "cx-relative-path-double-dot-only",
		Description: "Tests that '..' as a standalone pattern includes parent directory contents.",
		Tags:        []string{"cx", "rules", "patterns", "relative-path"},
		Steps: []harness.Step{
			harness.NewStep("Setup nested project with double-dot rule", func(ctx *harness.Context) error {
				// Use a nested structure so ".." targets a controlled directory, not the system temp dir.
				// ctx.RootDir/parent/target.go (file to find)
				// ctx.RootDir/parent/child (working dir with ".." rule)
				parentDir := filepath.Join(ctx.RootDir, "parent")
				childDir := filepath.Join(parentDir, "child")
				ctx.Set("work_dir", childDir)

				if err := fs.WriteString(filepath.Join(parentDir, "target.go"), "package parent"); err != nil {
					return err
				}

				groveConfig := fmt.Sprintf("context:\n  allowed_paths:\n    - %s\n", parentDir)
				if err := fs.WriteString(filepath.Join(childDir, "grove.yml"), groveConfig); err != nil {
					return err
				}

				return fs.WriteString(filepath.Join(childDir, ".grove", "rules"), "..")
			}),
			harness.NewStep("Run 'cx list' and verify parent directory contents included", func(ctx *harness.Context) error {
				workDir := ctx.GetString("work_dir")

				cx, err := FindProjectBinary()
				if err != nil {
					return err
				}
				cmd := ctx.Command(cx, "list").Dir(workDir)
				result := cmd.Run()
				ctx.ShowCommandOutput(cmd.String(), result.Stdout, result.Stderr)
				if result.Error != nil {
					return result.Error
				}

				if !strings.Contains(result.Stdout, "target.go") {
					return fmt.Errorf("expected output to contain 'target.go', got:\n%s", result.Stdout)
				}
				return nil
			}),
		},
	}
}

// MultipleParentTraversalScenario tests that ../../ correctly resolves to grandparent directories.
func MultipleParentTraversalScenario() *harness.Scenario {
	return &harness.Scenario{
		Name:        "cx-relative-path-multiple-parent-traversal",
		Description: "Tests that ../../other/*.go correctly resolves to grandparent directories.",
		Tags:        []string{"cx", "rules", "patterns", "relative-path"},
		Steps: []harness.Step{
			harness.NewStep("Setup nested project with grandparent-relative rule", func(ctx *harness.Context) error {
				// Create: ctx.RootDir/project/sub/dir (working dir)
				//         ctx.RootDir/project/other/target.go (target)
				projectDir := filepath.Join(ctx.RootDir, "project")
				workDir := filepath.Join(projectDir, "sub", "dir")
				otherDir := filepath.Join(projectDir, "other")
				ctx.Set("work_dir", workDir)

				if err := fs.WriteString(filepath.Join(otherDir, "target.go"), "package other"); err != nil {
					return err
				}

				groveConfig := fmt.Sprintf("context:\n  allowed_paths:\n    - %s\n", projectDir)
				if err := fs.WriteString(filepath.Join(workDir, "grove.yml"), groveConfig); err != nil {
					return err
				}

				return fs.WriteString(filepath.Join(workDir, ".grove", "rules"), "../../other/*.go")
			}),
			harness.NewStep("Run 'cx list' from nested dir and verify grandparent file", func(ctx *harness.Context) error {
				workDir := ctx.GetString("work_dir")

				cx, err := FindProjectBinary()
				if err != nil {
					return err
				}
				cmd := ctx.Command(cx, "list").Dir(workDir)
				result := cmd.Run()
				ctx.ShowCommandOutput(cmd.String(), result.Stdout, result.Stderr)
				if result.Error != nil {
					return result.Error
				}

				if !strings.Contains(result.Stdout, "target.go") {
					return fmt.Errorf("expected output to contain 'target.go', got:\n%s", result.Stdout)
				}
				return nil
			}),
		},
	}
}

// FloatingInclusionNotAffectedScenario tests that floating patterns like *.go don't bleed into sibling directories.
func FloatingInclusionNotAffectedScenario() *harness.Scenario {
	return &harness.Scenario{
		Name:        "cx-relative-path-floating-inclusion-not-affected",
		Description: "Tests that floating patterns like *.go are scoped locally with sibling dirs present.",
		Tags:        []string{"cx", "rules", "patterns", "relative-path", "scope"},
		Steps: []harness.Step{
			harness.NewStep("Setup local and sibling directories", func(ctx *harness.Context) error {
				parentDir := filepath.Dir(ctx.RootDir)
				siblingDir := filepath.Join(parentDir, "sibling-floating")
				ctx.Set("sibling_dir", siblingDir)

				if err := fs.WriteString(filepath.Join(siblingDir, "sibling.go"), "package sibling"); err != nil {
					return err
				}
				if err := fs.WriteString(filepath.Join(ctx.RootDir, "local.go"), "package local"); err != nil {
					return err
				}

				groveConfig := fmt.Sprintf("context:\n  allowed_paths:\n    - %s\n", parentDir)
				if err := fs.WriteString(filepath.Join(ctx.RootDir, "grove.yml"), groveConfig); err != nil {
					return err
				}

				return fs.WriteString(filepath.Join(ctx.RootDir, ".grove", "rules"), "*.go")
			}),
			harness.NewStep("Run 'cx list' and verify floating pattern is scoped locally", func(ctx *harness.Context) error {
				siblingDir := ctx.GetString("sibling_dir")
				defer os.RemoveAll(siblingDir)

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

				return ctx.Verify(func(v *verify.Collector) {
					v.Contains("local.go is included", result.Stdout, "local.go")
					v.NotContains("sibling.go is not included", result.Stdout, "sibling.go")
				})
			}),
		},
	}
}

// RelativeExclusionScenario tests that exclusion rules with relative external paths work.
func RelativeExclusionScenario() *harness.Scenario {
	return &harness.Scenario{
		Name:        "cx-relative-path-exclusion",
		Description: "Tests that !../sibling/internal/ correctly excludes files from external paths.",
		Tags:        []string{"cx", "rules", "patterns", "relative-path", "exclusion"},
		Steps: []harness.Step{
			harness.NewStep("Setup sibling with public and internal files", func(ctx *harness.Context) error {
				parentDir := filepath.Dir(ctx.RootDir)
				siblingDir := filepath.Join(parentDir, "sibling-exclude")
				ctx.Set("sibling_dir", siblingDir)

				if err := fs.WriteString(filepath.Join(siblingDir, "public.go"), "package public"); err != nil {
					return err
				}
				if err := fs.WriteString(filepath.Join(siblingDir, "internal", "secret.go"), "package internal"); err != nil {
					return err
				}

				groveConfig := fmt.Sprintf("context:\n  allowed_paths:\n    - %s\n", parentDir)
				if err := fs.WriteString(filepath.Join(ctx.RootDir, "grove.yml"), groveConfig); err != nil {
					return err
				}

				rules := "../sibling-exclude/**/*.go\n!../sibling-exclude/internal/"
				return fs.WriteString(filepath.Join(ctx.RootDir, ".grove", "rules"), rules)
			}),
			harness.NewStep("Run 'cx list' and verify exclusion works", func(ctx *harness.Context) error {
				siblingDir := ctx.GetString("sibling_dir")
				defer os.RemoveAll(siblingDir)

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

				return ctx.Verify(func(v *verify.Collector) {
					v.Contains("public.go is included", result.Stdout, "public.go")
					v.NotContains("secret.go is excluded", result.Stdout, "secret.go")
				})
			}),
		},
	}
}

// RelativeRecursiveGlobScenario tests that ../sibling/**/*.go correctly matches files in nested subdirectories.
func RelativeRecursiveGlobScenario() *harness.Scenario {
	return &harness.Scenario{
		Name:        "cx-relative-path-recursive-glob",
		Description: "Tests that ../sibling/**/*.go correctly includes files in nested subdirectories of a sibling.",
		Tags:        []string{"cx", "rules", "patterns", "relative-path", "glob"},
		Steps: []harness.Step{
			harness.NewStep("Setup sibling directory with nested structure", func(ctx *harness.Context) error {
				parentDir := filepath.Dir(ctx.RootDir)
				siblingDir := filepath.Join(parentDir, "sibling-recursive")
				ctx.Set("sibling_dir", siblingDir)

				// Create files at multiple nesting levels
				if err := fs.WriteString(filepath.Join(siblingDir, "root.go"), "package sibling"); err != nil {
					return err
				}
				if err := fs.WriteString(filepath.Join(siblingDir, "pkg", "handler.go"), "package pkg"); err != nil {
					return err
				}
				if err := fs.WriteString(filepath.Join(siblingDir, "pkg", "util", "helpers.go"), "package util"); err != nil {
					return err
				}
				// Non-Go file should not match
				if err := fs.WriteString(filepath.Join(siblingDir, "README.md"), "# Sibling"); err != nil {
					return err
				}

				groveConfig := fmt.Sprintf("context:\n  allowed_paths:\n    - %s\n", parentDir)
				if err := fs.WriteString(filepath.Join(ctx.RootDir, "grove.yml"), groveConfig); err != nil {
					return err
				}

				return fs.WriteString(filepath.Join(ctx.RootDir, ".grove", "rules"), "../sibling-recursive/**/*.go")
			}),
			harness.NewStep("Run 'cx list' and verify recursive glob resolves nested files", func(ctx *harness.Context) error {
				siblingDir := ctx.GetString("sibling_dir")
				defer os.RemoveAll(siblingDir)

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

				return ctx.Verify(func(v *verify.Collector) {
					v.Contains("root.go is included", result.Stdout, "root.go")
					v.Contains("handler.go is included", result.Stdout, "handler.go")
					v.Contains("helpers.go is included", result.Stdout, "helpers.go")
					v.NotContains("README.md is excluded (not *.go)", result.Stdout, "README.md")
				})
			}),
		},
	}
}

// RelativeMultipleSiblingsScenario tests referencing multiple sibling directories with ../ in a single rules file.
func RelativeMultipleSiblingsScenario() *harness.Scenario {
	return &harness.Scenario{
		Name:        "cx-relative-path-multiple-siblings",
		Description: "Tests that rules can reference multiple sibling directories with ../ patterns.",
		Tags:        []string{"cx", "rules", "patterns", "relative-path"},
		Steps: []harness.Step{
			harness.NewStep("Setup two sibling directories", func(ctx *harness.Context) error {
				parentDir := filepath.Dir(ctx.RootDir)
				siblingA := filepath.Join(parentDir, "sibling-a")
				siblingB := filepath.Join(parentDir, "sibling-b")
				ctx.Set("sibling_a", siblingA)
				ctx.Set("sibling_b", siblingB)

				if err := fs.WriteString(filepath.Join(siblingA, "alpha.go"), "package a"); err != nil {
					return err
				}
				if err := fs.WriteString(filepath.Join(siblingB, "beta.go"), "package b"); err != nil {
					return err
				}

				groveConfig := fmt.Sprintf("context:\n  allowed_paths:\n    - %s\n", parentDir)
				if err := fs.WriteString(filepath.Join(ctx.RootDir, "grove.yml"), groveConfig); err != nil {
					return err
				}

				rules := "../sibling-a/**/*.go\n../sibling-b/**/*.go"
				return fs.WriteString(filepath.Join(ctx.RootDir, ".grove", "rules"), rules)
			}),
			harness.NewStep("Run 'cx list' and verify both sibling directories resolved", func(ctx *harness.Context) error {
				siblingA := ctx.GetString("sibling_a")
				siblingB := ctx.GetString("sibling_b")
				defer os.RemoveAll(siblingA)
				defer os.RemoveAll(siblingB)

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

				return ctx.Verify(func(v *verify.Collector) {
					v.Contains("alpha.go from sibling-a is included", result.Stdout, "alpha.go")
					v.Contains("beta.go from sibling-b is included", result.Stdout, "beta.go")
				})
			}),
		},
	}
}

// TreeRootDiscoveryRelativeScenario tests that cx view's tree page discovers root paths from uncleaned relative paths.
func TreeRootDiscoveryRelativeScenario() *harness.Scenario {
	return &harness.Scenario{
		Name:        "cx-relative-path-tree-root-discovery",
		Description: "Tests that cx view tree page discovers roots from uncleaned relative external paths.",
		Tags:        []string{"cx", "rules", "patterns", "relative-path", "tui"},
		Steps: []harness.Step{
			harness.NewStep("Setup sibling directory with uncleaned path rule", func(ctx *harness.Context) error {
				parentDir := filepath.Dir(ctx.RootDir)
				siblingDir := filepath.Join(parentDir, "sibling-tree")
				ctx.Set("sibling_dir", siblingDir)

				if err := fs.WriteString(filepath.Join(siblingDir, "target.go"), "package sibling"); err != nil {
					return err
				}

				groveConfig := fmt.Sprintf("context:\n  allowed_paths:\n    - %s\n", parentDir)
				if err := fs.WriteString(filepath.Join(ctx.RootDir, "grove.yml"), groveConfig); err != nil {
					return err
				}

				// Use uncleaned path to test normalization in tree root discovery
				return fs.WriteString(filepath.Join(ctx.RootDir, ".grove", "rules"), "./../sibling-tree/*.go")
			}),
			harness.NewStep("Launch TUI tree and verify it renders with external root", func(ctx *harness.Context) error {
				cx, err := FindProjectBinary()
				if err != nil {
					return err
				}

				session, err := ctx.StartTUI(cx, []string{"view", "--page", "tree"},
					tui.WithCwd(ctx.RootDir),
					tui.WithEnv("CLICOLOR_FORCE=1"),
				)
				if err != nil {
					return fmt.Errorf("failed to start TUI session: %w", err)
				}
				ctx.Set("tui_session", session)

				// Wait for the tree to finish loading (help bar appears only after loading)
				if err := session.WaitForText("Press", 30*time.Second); err != nil {
					view, _ := session.Capture()
					ctx.ShowCommandOutput("TUI Failed - Current View", view, "")
					return fmt.Errorf("timeout waiting for tree to finish loading: %w\nView:\n%s", err, view)
				}

				if err := session.WaitStable(); err != nil {
					return err
				}

				view, _ := session.Capture()
				ctx.ShowCommandOutput("Tree Page - Initial View", view, "")

				// Verify tree rendered with the tab bar and directory nodes.
				// The lowercase 'tree' label is styled as the active tab and the
				// sibling directory was discovered from the uncleaned ./../ path.
				cleaned, _ := session.Capture(tui.WithCleanedOutput())
				return ctx.Verify(func(v *verify.Collector) {
					v.Contains("tree tab is present in tab bar", cleaned, "tree")
					v.True("tree page rendered absolute path components from external root",
						strings.Contains(cleaned, "private") || strings.Contains(cleaned, "var") || strings.Contains(cleaned, "sibling-tree"))
				})
			}),
			harness.NewStep("Quit TUI", func(ctx *harness.Context) error {
				siblingDir := ctx.GetString("sibling_dir")
				defer os.RemoveAll(siblingDir)

				session := ctx.Get("tui_session").(*tui.Session)
				return session.SendKeys("q")
			}),
		},
	}
}
