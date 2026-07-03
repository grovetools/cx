package main

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/grovetools/tend/pkg/fs"
	"github.com/grovetools/tend/pkg/harness"
)

// AliasRootingScenario is the integration net for the @a: alias rooting fix
// (job 31: "cx @a: alias imports resolve to the wrong/unstable worktree"). It
// reuses the XDG ecosystem-worktree fixture — an ecosystem whose repo-b exists
// in both the original checkout and an XDG worktree, with worktree_only.go only
// in the worktree copy — and pins down the three behaviors the fix owns:
//
//   - current-worktree preference resolves @a:repo-b to the worktree copy from
//     both the repo root AND a nested subdir (prefix/ancestor containment, not
//     exact node match);
//   - `cx list` prints ABSOLUTE paths as its help documents, with --rel giving
//     the old rulesBaseDir-relative form;
//   - a rules file where a bare literal and a glob resolve the SAME file emits
//     NO spurious "matched 0 files" warning (the literal loses last-match-wins
//     attribution but still matched).
func AliasRootingScenario() *harness.Scenario {
	return &harness.Scenario{
		Name:        "cx-alias-rooting",
		Description: "Current-worktree @a: preference from root+subdir, absolute list output, and no spurious zero-match warning.",
		Tags:        []string{"cx", "alias", "rooting", "worktree"},
		Steps: []harness.Step{
			harness.NewStep("Setup XDG ecosystem worktree", func(ctx *harness.Context) error {
				_, err := setupXDGEcosystemWorktreeFixture(ctx)
				return err
			}),

			harness.NewStep("cx list prints absolute paths rooted in the current worktree", func(ctx *harness.Context) error {
				f := ctx.Get("xdgFixture").(*XDGEcosystemWorktreeFixture)
				cx, err := FindProjectBinary()
				if err != nil {
					return err
				}
				// A bare @a: file line plus a glob, both aimed at the sibling.
				if err := fs.WriteString(filepath.Join(f.RepoAWorktreeDir, ".grove", "rules"), "@a:repo-b/**/*.go"); err != nil {
					return err
				}
				cmd := ctx.Command(cx, "list").Dir(f.RepoAWorktreeDir)
				result := cmd.Run()
				ctx.ShowCommandOutput(cmd.String(), result.Stdout, result.Stderr)
				if result.Error != nil {
					return fmt.Errorf("cx list failed: %w\nstderr: %s", result.Error, result.Stderr)
				}
				lines := nonEmptyLines(result.Stdout)
				if len(lines) == 0 {
					return fmt.Errorf("expected at least one file, got none\noutput:\n%s", result.Stdout)
				}
				for _, ln := range lines {
					if !filepath.IsAbs(ln) {
						return fmt.Errorf("cx list must print absolute paths (help text contract); got relative %q\noutput:\n%s", ln, result.Stdout)
					}
				}
				// The sibling must root into THIS worktree, not the original checkout:
				// worktree_only.go lives only in the worktree copy and its absolute
				// path must sit under the XDG worktree dir.
				if !containsSuffixUnder(lines, "worktree_only.go", f.WorktreeDir) {
					return fmt.Errorf("@a:repo-b should root into the current worktree (%s); missing absolute worktree_only.go\noutput:\n%s", f.WorktreeDir, result.Stdout)
				}
				return nil
			}),

			harness.NewStep("current-worktree preference holds from a nested subdir", func(ctx *harness.Context) error {
				f := ctx.Get("xdgFixture").(*XDGEcosystemWorktreeFixture)
				cx, err := FindProjectBinary()
				if err != nil {
					return err
				}
				// Run cx from a nested subdir inside repo-a with an explicit rules
				// file (isolating alias resolution from cwd-based rules discovery).
				// The resolver's currentPath is the subdir; ancestor containment
				// must still root @a:repo-b into the current worktree rather than
				// falling back to an arbitrary registered worktree.
				subDir := filepath.Join(f.RepoAWorktreeDir, "internal", "deep")
				if err := fs.WriteString(filepath.Join(subDir, ".keep"), ""); err != nil {
					return err
				}
				rulesPath := filepath.Join(f.RepoAWorktreeDir, "subdir-alias.rules")
				if err := fs.WriteString(rulesPath, "@a:repo-b/**/*.go\n"); err != nil {
					return err
				}
				cmd := ctx.Command(cx, "list", "--rules-file", rulesPath).Dir(subDir)
				result := cmd.Run()
				ctx.ShowCommandOutput(cmd.String(), result.Stdout, result.Stderr)
				if result.Error != nil {
					return fmt.Errorf("cx list from subdir failed: %w\nstderr: %s", result.Error, result.Stderr)
				}
				if !containsSuffixUnder(nonEmptyLines(result.Stdout), "worktree_only.go", f.WorktreeDir) {
					return fmt.Errorf("from a nested subdir @a:repo-b should still root into the current worktree; missing worktree_only.go under %s\noutput:\n%s", f.WorktreeDir, result.Stdout)
				}
				return nil
			}),

			harness.NewStep("default is absolute, --rel restores rulesBaseDir-relative output", func(ctx *harness.Context) error {
				f := ctx.Get("xdgFixture").(*XDGEcosystemWorktreeFixture)
				cx, err := FindProjectBinary()
				if err != nil {
					return err
				}
				// A local repo-a file (main.go lives at the repo root) is the clean
				// probe: only paths under rulesBaseDir are relativized, so default
				// output must be absolute and --rel must print the bare relative
				// form. (Cross-repo sibling files stay absolute in both modes.)
				if err := fs.WriteString(filepath.Join(f.RepoAWorktreeDir, ".grove", "rules"), "main.go\n"); err != nil {
					return err
				}
				absCmd := ctx.Command(cx, "list").Dir(f.RepoAWorktreeDir)
				absRes := absCmd.Run()
				ctx.ShowCommandOutput(absCmd.String(), absRes.Stdout, absRes.Stderr)
				if absRes.Error != nil {
					return fmt.Errorf("cx list failed: %w\nstderr: %s", absRes.Error, absRes.Stderr)
				}
				if !containsSuffixUnder(nonEmptyLines(absRes.Stdout), "main.go", f.RepoAWorktreeDir) {
					return fmt.Errorf("default cx list should print absolute main.go under repo-a\noutput:\n%s", absRes.Stdout)
				}

				relCmd := ctx.Command(cx, "list", "--rel").Dir(f.RepoAWorktreeDir)
				relRes := relCmd.Run()
				ctx.ShowCommandOutput(relCmd.String(), relRes.Stdout, relRes.Stderr)
				if relRes.Error != nil {
					return fmt.Errorf("cx list --rel failed: %w\nstderr: %s", relRes.Error, relRes.Stderr)
				}
				relLines := nonEmptyLines(relRes.Stdout)
				found := false
				for _, ln := range relLines {
					if ln == "main.go" {
						found = true
					}
				}
				if !found {
					return fmt.Errorf("cx list --rel should print bare relative 'main.go'\noutput:\n%s", relRes.Stdout)
				}
				return nil
			}),

			harness.NewStep("cx list --job honors the job's worktree: frontmatter", func(ctx *harness.Context) error {
				f := ctx.Get("xdgFixture").(*XDGEcosystemWorktreeFixture)
				cx, err := FindProjectBinary()
				if err != nil {
					return err
				}
				// Invoke from the ORIGINAL checkout (repo-a in EcoDir), where
				// @a:repo-b would normally root into the original repo-b. A job
				// declaring `worktree: feature-x` must reroot alias resolution
				// into the XDG worktree, so worktree_only.go (present only there)
				// appears. This exercises rerootAliasesToWorktree from within the
				// ecosystem, independent of invoking cwd.
				originalRepoA := filepath.Join(f.EcoDir, "repo-a")
				jobDir := filepath.Join(f.EcoDir, "plans", "job31")
				if err := fs.WriteString(filepath.Join(jobDir, "rules", "job.rules"), "@a:repo-b/**/*.go\n"); err != nil {
					return err
				}
				jobMD := "---\nrules_file: rules/job.rules\nworktree: feature-x\n---\n\n# job\n"
				jobPath := filepath.Join(jobDir, "job.md")
				if err := fs.WriteString(jobPath, jobMD); err != nil {
					return err
				}
				cmd := ctx.Command(cx, "list", "--job", jobPath).Dir(originalRepoA)
				result := cmd.Run()
				ctx.ShowCommandOutput(cmd.String(), result.Stdout, result.Stderr)
				if result.Error != nil {
					return fmt.Errorf("cx list --job failed: %w\nstderr: %s", result.Error, result.Stderr)
				}
				if !containsSuffixUnder(nonEmptyLines(result.Stdout), "worktree_only.go", f.WorktreeDir) {
					return fmt.Errorf("--job worktree: feature-x should root @a:repo-b into the XDG worktree; missing worktree_only.go under %s\noutput:\n%s", f.WorktreeDir, result.Stdout)
				}
				return nil
			}),

			harness.NewStep("no spurious zero-match warning when a literal and a glob share a file", func(ctx *harness.Context) error {
				f := ctx.Get("xdgFixture").(*XDGEcosystemWorktreeFixture)
				cx, err := FindProjectBinary()
				if err != nil {
					return err
				}
				// This is the job-31 repro in miniature: the bare literal
				// @a:repo-b/main.go and the glob @a:repo-b/**/*.go both resolve
				// main.go. Last-match-wins gives the glob the attribution and the
				// literal lands in FilteredResult — it matched, so no warning.
				rules := "@a:repo-b/main.go\n@a:repo-b/**/*.go\n"
				if err := fs.WriteString(filepath.Join(f.RepoAWorktreeDir, ".grove", "rules"), rules); err != nil {
					return err
				}
				cmd := ctx.Command(cx, "list").Dir(f.RepoAWorktreeDir)
				result := cmd.Run()
				ctx.ShowCommandOutput(cmd.String(), result.Stdout, result.Stderr)
				if result.Error != nil {
					return fmt.Errorf("cx list (mixed rules) failed: %w\nstderr: %s", result.Error, result.Stderr)
				}
				if strings.Contains(result.Stderr, "matched 0 files") {
					return fmt.Errorf("a literal that shares a file with a glob must not report 'matched 0 files'\nstderr:\n%s", result.Stderr)
				}
				if !containsSuffix(nonEmptyLines(result.Stdout), "main.go") {
					return fmt.Errorf("expected repo-b/main.go in the resolved set\noutput:\n%s", result.Stdout)
				}
				return nil
			}),
		},
	}
}

// nonEmptyLines splits stdout into trimmed, non-empty lines.
func nonEmptyLines(s string) []string {
	var out []string
	for _, ln := range strings.Split(s, "\n") {
		if t := strings.TrimSpace(ln); t != "" {
			out = append(out, t)
		}
	}
	return out
}

// containsSuffix reports whether any line ends with suffix.
func containsSuffix(lines []string, suffix string) bool {
	for _, ln := range lines {
		if strings.HasSuffix(ln, suffix) {
			return true
		}
	}
	return false
}

// containsSuffixUnder reports whether any line ends with suffix AND is located
// under base (tolerant of the macOS /var -> /private/var symlink), pinning both
// the file identity and the worktree it rooted into.
func containsSuffixUnder(lines []string, suffix, base string) bool {
	realBase, err := filepath.EvalSymlinks(base)
	if err != nil {
		realBase = base
	}
	for _, ln := range lines {
		if !strings.HasSuffix(ln, suffix) {
			continue
		}
		real, err := filepath.EvalSymlinks(ln)
		if err != nil {
			real = ln
		}
		if strings.HasPrefix(real, realBase) || strings.HasPrefix(ln, base) {
			return true
		}
	}
	return false
}
