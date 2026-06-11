package main

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/grovetools/tend/pkg/command"
	"github.com/grovetools/tend/pkg/fs"
	"github.com/grovetools/tend/pkg/harness"
)

// rulesWhereJSON is the shape of `cx rules where --json`.
type rulesWhereJSON struct {
	Paths map[string]string `json:"paths"`
}

// AliasSiblingResolutionXDGScenario is the XDG-layout sibling of
// AliasSiblingResolutionScenario: it drives cx from an ecosystem worktree that
// lives under the sandboxed XDG data dir (paths.WorktreesDir()/<id>/feature-x)
// rather than <eco>/.grove-worktrees/feature-x, and verifies that cx, running
// from inside the XDG worktree's repo-a:
//
//   - resolves the @a: short-alias for a sibling to the sibling's WORKTREE copy
//     (output carries repo-b's worktree_only.go);
//   - reports the correct workspace identity via `cx rules where`
//     (my-ecosystem:feature-x:repo-a);
//   - resolves plan-scoped rules (a branch-activated plan's default.rules takes
//     over and its alias still resolves the XDG sibling).
func AliasSiblingResolutionXDGScenario() *harness.Scenario {
	return &harness.Scenario{
		Name:        "cx-alias-siblings-xdg",
		Description: "Sibling alias resolution, identity, and plan-scoped rules from an XDG-located ecosystem worktree.",
		Tags:        []string{"cx", "alias", "siblings", "worktree", "xdg"},
		Steps: []harness.Step{
			harness.NewStep("Setup XDG ecosystem worktree", func(ctx *harness.Context) error {
				_, err := setupXDGEcosystemWorktreeFixture(ctx)
				return err
			}),

			harness.NewStep("Sibling @a: resolves to the XDG worktree sibling", func(ctx *harness.Context) error {
				f := ctx.Get("xdgFixture").(*XDGEcosystemWorktreeFixture)
				cx, err := FindProjectBinary()
				if err != nil {
					return err
				}
				// Legacy .grove/rules is honored while no notebook rules file
				// exists yet (the identity step below creates one), so run this
				// short-alias check first.
				if err := fs.WriteString(filepath.Join(f.RepoAWorktreeDir, ".grove", "rules"), "@a:repo-b/**/*.go"); err != nil {
					return err
				}
				cmd := ctx.Command(cx, "list").Dir(f.RepoAWorktreeDir)
				result := cmd.Run()
				ctx.ShowCommandOutput(cmd.String(), result.Stdout, result.Stderr)
				if result.Error != nil {
					return fmt.Errorf("cx list failed: %w\nstderr: %s", result.Error, result.Stderr)
				}
				if !strings.Contains(result.Stdout, "worktree_only.go") {
					return fmt.Errorf("@a:repo-b should resolve to the XDG sibling worktree (missing worktree_only.go)\noutput:\n%s", result.Stdout)
				}
				return nil
			}),

			harness.NewStep("cx rules where reports the XDG worktree identity", func(ctx *harness.Context) error {
				f := ctx.Get("xdgFixture").(*XDGEcosystemWorktreeFixture)
				cx, err := FindProjectBinary()
				if err != nil {
					return err
				}
				cmd := ctx.Command(cx, "rules", "where", "--json").Dir(f.RepoAWorktreeDir)
				result := cmd.Run()
				ctx.ShowCommandOutput(cmd.String(), result.Stdout, result.Stderr)
				if result.Error != nil {
					return fmt.Errorf("cx rules where failed: %w\nstderr: %s", result.Error, result.Stderr)
				}
				var rw rulesWhereJSON
				if err := json.Unmarshal([]byte(result.Stdout), &rw); err != nil {
					return fmt.Errorf("parse rules where json: %w\nstdout: %s", err, result.Stdout)
				}
				if !strings.Contains(rw.Paths["workspace"], "my-ecosystem:feature-x:repo-a") {
					return fmt.Errorf("workspace identity = %q, want it to contain my-ecosystem:feature-x:repo-a", rw.Paths["workspace"])
				}
				// Stash the context dir so the plan step can derive the plans dir
				// without reconstructing it from HOME.
				ctx.Set("contextDir", rw.Paths["context_dir"])
				return nil
			}),

			harness.NewStep("Plan-scoped rules resolve in the XDG worktree", func(ctx *harness.Context) error {
				f := ctx.Get("xdgFixture").(*XDGEcosystemWorktreeFixture)
				cx, err := FindProjectBinary()
				if err != nil {
					return err
				}
				contextDir := ctx.GetString("contextDir")
				if contextDir == "" {
					return fmt.Errorf("contextDir not set by the identity step")
				}
				// plansDir is the sibling of the context dir:
				// <notebook>/workspaces/repo-a/{context,plans}.
				workspaceDir := filepath.Dir(contextDir)
				const planName = "xdg-plan"
				planRulesPath := filepath.Join(workspaceDir, "plans", planName, "rules", "default.rules")

				// Activate the plan by matching the worktree's branch to a plan
				// directory, and seed its plan-scoped rules. A commit is required
				// so the branch is born and git reports it (an unborn branch
				// resolves to HEAD, which ActivePlan ignores).
				if r := command.New("git", "checkout", "-b", planName).Dir(f.RepoAWorktreeDir).Run(); r.Error != nil {
					return fmt.Errorf("git checkout -b %s: %w (%s)", planName, r.Error, r.Stderr)
				}
				if r := command.New("git", "-c", "user.email=t@t", "-c", "user.name=t",
					"commit", "--allow-empty", "-m", "plan").Dir(f.RepoAWorktreeDir).Run(); r.Error != nil {
					return fmt.Errorf("git commit: %w (%s)", r.Error, r.Stderr)
				}
				if err := fs.WriteString(planRulesPath, "@a:repo-b/**/*.go"); err != nil {
					return err
				}

				// rules where must now report the plan-scoped file as active.
				whereCmd := ctx.Command(cx, "rules", "where", "--json").Dir(f.RepoAWorktreeDir)
				whereRes := whereCmd.Run()
				ctx.ShowCommandOutput(whereCmd.String(), whereRes.Stdout, whereRes.Stderr)
				if whereRes.Error != nil {
					return fmt.Errorf("cx rules where (plan) failed: %w\nstderr: %s", whereRes.Error, whereRes.Stderr)
				}
				var rw rulesWhereJSON
				if err := json.Unmarshal([]byte(whereRes.Stdout), &rw); err != nil {
					return fmt.Errorf("parse rules where json: %w\nstdout: %s", err, whereRes.Stdout)
				}
				if !samePathCX(rw.Paths["rules"], planRulesPath) {
					return fmt.Errorf("active rules = %q, want plan-scoped %q", rw.Paths["rules"], planRulesPath)
				}

				// And the plan-scoped alias still resolves the XDG sibling.
				listCmd := ctx.Command(cx, "list").Dir(f.RepoAWorktreeDir)
				listRes := listCmd.Run()
				ctx.ShowCommandOutput(listCmd.String(), listRes.Stdout, listRes.Stderr)
				if listRes.Error != nil {
					return fmt.Errorf("cx list (plan) failed: %w\nstderr: %s", listRes.Error, listRes.Stderr)
				}
				if !strings.Contains(listRes.Stdout, "worktree_only.go") {
					return fmt.Errorf("plan-scoped @a:repo-b should resolve the XDG sibling (missing worktree_only.go)\noutput:\n%s", listRes.Stdout)
				}
				return nil
			}),
		},
	}
}

// samePathCX compares two paths tolerant of the macOS /var -> /private/var
// symlink and case-insensitive filesystem differences between cx's normalized
// output and the harness-constructed path.
func samePathCX(a, b string) bool {
	ra, err := filepath.EvalSymlinks(a)
	if err != nil {
		ra = a
	}
	rb, err := filepath.EvalSymlinks(b)
	if err != nil {
		rb = b
	}
	return strings.EqualFold(filepath.Clean(ra), filepath.Clean(rb))
}
