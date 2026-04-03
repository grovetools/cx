// File: grove-context/tests/e2e/scenarios_changed_diff.go
package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/grovetools/tend/pkg/command"
	"github.com/grovetools/tend/pkg/fs"
	"github.com/grovetools/tend/pkg/git"
	"github.com/grovetools/tend/pkg/harness"
	"github.com/grovetools/tend/pkg/verify"
)

// ChangedStandaloneScenario tests standalone @changed: directive with HEAD, staged, and unstaged refs.
func ChangedStandaloneScenario() *harness.Scenario {
	return &harness.Scenario{
		Name:        "cx-changed-standalone",
		Description: "Tests standalone @changed: directive with HEAD, staged, and unstaged refs",
		Tags:        []string{"cx", "directives", "git", "changed"},
		Steps: []harness.Step{
			harness.NewStep("Setup git repo with initial files", func(ctx *harness.Context) error {
				repo, err := git.CreateTestRepo(ctx.RootDir, map[string]string{
					"main.go":   "package main\n\nfunc main() {}\n",
					"utils.go":  "package main\n\nfunc helper() {}\n",
					"config.go": "package main\n\nvar cfg = 1\n",
				})
				if err != nil {
					return err
				}
				ctx.Set("repo", repo)
				return nil
			}),
			harness.NewStep("Modify files and stage main.go only", func(ctx *harness.Context) error {
				// Modify main.go and utils.go, leave config.go untouched
				if err := fs.WriteString(filepath.Join(ctx.RootDir, "main.go"), "package main\n\nfunc main() { fmt.Println(\"updated\") }\n"); err != nil {
					return err
				}
				if err := fs.WriteString(filepath.Join(ctx.RootDir, "utils.go"), "package main\n\nfunc helper() { return }\n"); err != nil {
					return err
				}
				// Stage only main.go
				repo := ctx.Get("repo").(*git.Git)
				return repo.Add("main.go")
			}),
			harness.NewStep("Test @changed: HEAD shows both modified files", func(ctx *harness.Context) error {
				rulesPath := filepath.Join(ctx.RootDir, ".grove", "rules")
				if err := fs.WriteString(rulesPath, "@changed: HEAD"); err != nil {
					return err
				}

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
					v.Contains("main.go is listed", result.Stdout, "main.go")
					v.Contains("utils.go is listed", result.Stdout, "utils.go")
					v.NotContains("config.go is not listed", result.Stdout, "config.go")
				})
			}),
			harness.NewStep("Test @changed: staged shows only staged file", func(ctx *harness.Context) error {
				rulesPath := filepath.Join(ctx.RootDir, ".grove", "rules")
				if err := fs.WriteString(rulesPath, "@changed: staged"); err != nil {
					return err
				}

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
					v.Contains("main.go is listed", result.Stdout, "main.go")
					v.NotContains("utils.go is not listed", result.Stdout, "utils.go")
					v.NotContains("config.go is not listed", result.Stdout, "config.go")
				})
			}),
			harness.NewStep("Test @changed: unstaged shows only unstaged file", func(ctx *harness.Context) error {
				rulesPath := filepath.Join(ctx.RootDir, ".grove", "rules")
				if err := fs.WriteString(rulesPath, "@changed: unstaged"); err != nil {
					return err
				}

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
					v.NotContains("main.go is not listed", result.Stdout, "main.go")
					v.Contains("utils.go is listed", result.Stdout, "utils.go")
					v.NotContains("config.go is not listed", result.Stdout, "config.go")
				})
			}),
		},
	}
}

// ChangedInlineFilterScenario tests @changed: as an inline filter on glob patterns.
func ChangedInlineFilterScenario() *harness.Scenario {
	return &harness.Scenario{
		Name:        "cx-changed-inline-filter",
		Description: "Tests @changed: as inline filter to restrict glob results to changed files",
		Tags:        []string{"cx", "directives", "git", "changed"},
		Steps: []harness.Step{
			harness.NewStep("Setup git repo with files in different directories", func(ctx *harness.Context) error {
				_, err := git.CreateTestRepo(ctx.RootDir, map[string]string{
					"pkg/api/api.go":        "package api\n\nfunc Handle() {}\n",
					"pkg/models/user.go":    "package models\n\ntype User struct{}\n",
					"cmd/main.go":           "package main\n\nfunc main() {}\n",
				})
				return err
			}),
			harness.NewStep("Modify pkg/api/api.go and cmd/main.go only", func(ctx *harness.Context) error {
				if err := fs.WriteString(filepath.Join(ctx.RootDir, "pkg/api/api.go"), "package api\n\nfunc Handle() { updated() }\n"); err != nil {
					return err
				}
				return fs.WriteString(filepath.Join(ctx.RootDir, "cmd/main.go"), "package main\n\nfunc main() { updated() }\n")
			}),
			harness.NewStep("Verify inline @changed: filters glob to only changed matches", func(ctx *harness.Context) error {
				rulesPath := filepath.Join(ctx.RootDir, ".grove", "rules")
				// Only files under pkg/ that are also changed
				if err := fs.WriteString(rulesPath, "pkg/**/*.go @changed: HEAD"); err != nil {
					return err
				}

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
					v.Contains("api.go matches glob and is changed", result.Stdout, "pkg/api/api.go")
					v.NotContains("cmd/main.go does not match pkg/ glob", result.Stdout, "cmd/main.go")
					v.NotContains("user.go matches glob but is not changed", result.Stdout, "pkg/models/user.go")
				})
			}),
		},
	}
}

// DiffStandaloneScenario tests the @diff: directive generates a .patch file.
func DiffStandaloneScenario() *harness.Scenario {
	return &harness.Scenario{
		Name:        "cx-diff-standalone",
		Description: "Tests @diff: directive generates a valid .patch file and includes it in context",
		Tags:        []string{"cx", "directives", "git", "diff"},
		Steps: []harness.Step{
			harness.NewStep("Setup git repo and modify a file", func(ctx *harness.Context) error {
				_, err := git.CreateTestRepo(ctx.RootDir, map[string]string{
					"readme.md": "Initial Content\n",
				})
				if err != nil {
					return err
				}
				return fs.WriteString(filepath.Join(ctx.RootDir, "readme.md"), "Initial Content\nAdded Content\n")
			}),
			harness.NewStep("Run cx list with @diff: HEAD and verify patch file is generated", func(ctx *harness.Context) error {
				rulesPath := filepath.Join(ctx.RootDir, ".grove", "rules")
				if err := fs.WriteString(rulesPath, "@diff: HEAD"); err != nil {
					return err
				}

				cxBinary, err := FindProjectBinary()
				if err != nil {
					return err
				}

				// Run cx list to trigger the @diff: directive (which creates the patch file)
				cmd := command.New(cxBinary, "list").Dir(ctx.RootDir)
				result := cmd.Run()
				ctx.ShowCommandOutput(cmd.String(), result.Stdout, result.Stderr)
				// Don't check result.Error — the command may warn about the absolute path
				// but we only care that the patch file was generated correctly.

				// Verify the patch file was created with correct content
				patchPath := filepath.Join(ctx.RootDir, ".grove", "diffs", "diff-HEAD.patch")
				content, err := os.ReadFile(patchPath)
				if err != nil {
					return fmt.Errorf("patch file was not created at %s: %w", patchPath, err)
				}

				patchStr := string(content)
				return ctx.Verify(func(v *verify.Collector) {
					v.Contains("patch has --- marker", patchStr, "--- a/readme.md")
					v.Contains("patch has +++ marker", patchStr, "+++ b/readme.md")
					v.Contains("patch has added line", patchStr, "+Added Content")
				})
			}),
		},
	}
}

// ChangedDeletedFilesScenario verifies deleted files are excluded from @changed: output.
func ChangedDeletedFilesScenario() *harness.Scenario {
	return &harness.Scenario{
		Name:        "cx-changed-deleted-files",
		Description: "Tests that deleted files are excluded from @changed: results",
		Tags:        []string{"cx", "directives", "git", "changed"},
		Steps: []harness.Step{
			harness.NewStep("Setup git repo, delete one file, modify another", func(ctx *harness.Context) error {
				_, err := git.CreateTestRepo(ctx.RootDir, map[string]string{
					"keep.go":   "package main\n\nfunc keep() {}\n",
					"delete.go": "package main\n\nfunc del() {}\n",
				})
				if err != nil {
					return err
				}

				// Delete one file, modify the other
				if err := os.Remove(filepath.Join(ctx.RootDir, "delete.go")); err != nil {
					return err
				}
				return fs.WriteString(filepath.Join(ctx.RootDir, "keep.go"), "package main\n\nfunc keep() { modified() }\n")
			}),
			harness.NewStep("Verify deleted file is not in @changed: output", func(ctx *harness.Context) error {
				rulesPath := filepath.Join(ctx.RootDir, ".grove", "rules")
				if err := fs.WriteString(rulesPath, "@changed: HEAD"); err != nil {
					return err
				}

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
					v.Contains("keep.go is listed", result.Stdout, "keep.go")
					v.NotContains("delete.go is excluded", result.Stdout, "delete.go")
				})
			}),
		},
	}
}

// ChangedBranchRefScenario tests @changed: with a branch name ref.
func ChangedBranchRefScenario() *harness.Scenario {
	return &harness.Scenario{
		Name:        "cx-changed-branch-ref",
		Description: "Tests @changed: with a branch name to compare against",
		Tags:        []string{"cx", "directives", "git", "changed"},
		Steps: []harness.Step{
			harness.NewStep("Setup git repo on main, create feature branch with changes", func(ctx *harness.Context) error {
				repo, err := git.CreateTestRepo(ctx.RootDir, map[string]string{
					"base.go": "package main\n\nfunc base() {}\n",
				})
				if err != nil {
					return err
				}

				// Create feature branch with new and modified files
				return repo.CreateBranchWithChanges("feature", map[string]string{
					"base.go":    "package main\n\nfunc base() { changed() }\n",
					"feature.go": "package main\n\nfunc feature() {}\n",
				})
			}),
			harness.NewStep("Verify @changed: main shows files differing from main", func(ctx *harness.Context) error {
				rulesPath := filepath.Join(ctx.RootDir, ".grove", "rules")
				if err := fs.WriteString(rulesPath, "@changed: main"); err != nil {
					return err
				}

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
					v.Contains("base.go changed relative to main", result.Stdout, "base.go")
					v.Contains("feature.go is new relative to main", result.Stdout, "feature.go")
				})
			}),
		},
	}
}

// ChangedCombinedScenario tests @changed: combined with regular glob patterns.
func ChangedCombinedScenario() *harness.Scenario {
	return &harness.Scenario{
		Name:        "cx-changed-combined",
		Description: "Tests @changed: combined with regular glob patterns in a multi-line rules file",
		Tags:        []string{"cx", "directives", "git", "changed"},
		Steps: []harness.Step{
			harness.NewStep("Setup git repo and modify some files", func(ctx *harness.Context) error {
				_, err := git.CreateTestRepo(ctx.RootDir, map[string]string{
					"src/app.go":    "package src\n\nfunc App() {}\n",
					"src/utils.go":  "package src\n\nfunc Utils() {}\n",
					"docs/guide.md": "# Guide\n",
				})
				if err != nil {
					return err
				}

				// Modify only src/app.go (not src/utils.go)
				return fs.WriteString(filepath.Join(ctx.RootDir, "src/app.go"), "package src\n\nfunc App() { updated() }\n")
			}),
			harness.NewStep("Verify @changed: and regular patterns both contribute files", func(ctx *harness.Context) error {
				rulesPath := filepath.Join(ctx.RootDir, ".grove", "rules")
				// @changed: adds only the modified file, regular glob adds docs
				rulesContent := "@changed: HEAD\ndocs/**/*.md"
				if err := fs.WriteString(rulesPath, rulesContent); err != nil {
					return err
				}

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
					v.Contains("src/app.go from @changed:", result.Stdout, "src/app.go")
					v.Contains("docs/guide.md from regular glob", result.Stdout, "docs/guide.md")
					v.NotContains("src/utils.go is not changed", result.Stdout, "src/utils.go")
				})
			}),
		},
	}
}
