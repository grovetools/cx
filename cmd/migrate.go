package cmd

import (
	stdctx "context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/grovetools/core/config"
	"github.com/grovetools/core/pkg/workspace"
	"github.com/grovetools/cx/pkg/context"
	"github.com/spf13/cobra"
)

func NewMigrateRulesNbCmd() *cobra.Command {
	var dryRun bool

	cmd := &cobra.Command{
		Use:   "migrate-rules-nb",
		Short: "Migrate legacy .cx presets and stale context files to notebook",
		Long: `Migrates local context artifacts to the centralized notebook location:

  .cx/*.rules        → notebook context/presets/
  .cx.work/*.rules   → notebook context/presets.work/
  .grove/rules       → notebook context/rules  (if notebook is configured)
  .grove/context     → removed (regenerated via cx generate)
  .grove/cached-*    → removed (regenerated via cx generate)
  .grove/context-files → removed
  .grovectx          → removed (legacy)

After migration, empty .cx/, .cx.work/, and .grove/ directories are removed.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := stdctx.Background()
			mgr := context.NewManager(GetWorkDir())
			workDir := mgr.GetWorkDir()

			node, err := workspace.GetProjectByPath(workDir)
			if err != nil {
				return fmt.Errorf("not in a workspace project: %w", err)
			}

			locator := mgr.Locator()

			presetsDir, err := locator.GetContextPresetsDir(node)
			if err != nil {
				return fmt.Errorf("could not resolve notebook presets dir: %w", err)
			}

			presetsWorkDir, err := locator.GetContextPresetsWorkDir(node)
			if err != nil {
				return fmt.Errorf("could not resolve notebook presets.work dir: %w", err)
			}

			nbRulesFile, err := locator.GetContextRulesFile(node)
			if err != nil {
				return fmt.Errorf("could not resolve notebook rules path: %w", err)
			}

			var moved, removed int

			// Migrate .cx/*.rules → notebook presets/
			cxDir := filepath.Join(workDir, context.RulesDir)
			moved += migrateRulesDir(ctx, cxDir, presetsDir, dryRun)

			// Migrate .cx.work/*.rules → notebook presets.work/
			cxWorkDir := filepath.Join(workDir, context.RulesWorkDir)
			moved += migrateRulesDir(ctx, cxWorkDir, presetsWorkDir, dryRun)

			// Migrate .grove/rules → plan-scoped default.rules (if in worktree) or notebook context/rules
			localRules := filepath.Join(workDir, context.ActiveRulesFile)
			if _, err := os.Stat(localRules); err == nil {
				// Determine target: plan-scoped if active plan, else shared notebook
				rulesTarget := nbRulesFile
				if planName := mgr.GetActivePlanName(); planName != "" {
					if planPath := mgr.GetPlanRulesPath(planName); planPath != "" {
						rulesTarget = planPath
					}
				}

				if _, err := os.Stat(rulesTarget); err == nil {
					ulog.Warn("Target rules already exists, skipping .grove/rules").
						Pretty(fmt.Sprintf("⚠ Target rules already exists, skipping: %s", rulesTarget)).
						Log(ctx)
				} else {
					if dryRun {
						ulog.Info("Would move .grove/rules").
							Pretty(fmt.Sprintf("  %s → %s", localRules, rulesTarget)).
							Log(ctx)
					} else {
						if err := os.MkdirAll(filepath.Dir(rulesTarget), 0755); err == nil {
							content, _ := os.ReadFile(localRules)
							if content != nil {
								if err := os.WriteFile(rulesTarget, content, 0644); err == nil {
									os.Remove(localRules)
									moved++
									ulog.Success("Moved").
										Pretty(fmt.Sprintf("  %s → %s", localRules, rulesTarget)).
										Log(ctx)
								}
							}
						}
					}
				}
			}

			// Remove stale context files
			staleFiles := []string{
				filepath.Join(workDir, context.ContextFile),                // .grove/context
				filepath.Join(workDir, context.CachedContextFile),          // .grove/cached-context
				filepath.Join(workDir, context.CachedContextFilesListFile), // .grove/cached-context-files
				filepath.Join(workDir, context.FilesListFile),              // .grove/context-files
				filepath.Join(workDir, context.RulesFile),                  // .grovectx
			}
			for _, f := range staleFiles {
				if _, err := os.Stat(f); err == nil {
					if dryRun {
						ulog.Info("Would remove").
							Pretty(fmt.Sprintf("  rm %s", f)).
							Log(ctx)
					} else {
						if err := os.Remove(f); err == nil {
							removed++
							ulog.Success("Removed").
								Pretty(fmt.Sprintf("  rm %s", f)).
								Log(ctx)
						}
					}
				}
			}

			// Migrate grove.toml: default_rules_path → default_rules
			if updated := migrateGroveToml(ctx, workDir, dryRun); updated {
				moved++
			}

			// Clean up empty directories
			for _, dir := range []string{
				filepath.Join(workDir, context.GroveDir),
				cxDir,
				cxWorkDir,
			} {
				if !dryRun {
					removeIfEmpty(dir)
				}
			}

			if dryRun {
				ulog.Info("Dry run complete — no changes made").Log(ctx)
			} else if moved == 0 && removed == 0 {
				ulog.Info("Nothing to migrate").Log(ctx)
			} else {
				ulog.Success("Migration complete").
					Pretty(fmt.Sprintf("Migration complete: %d moved, %d removed", moved, removed)).
					Log(ctx)
			}

			return nil
		},
	}

	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Show what would be done without making changes")

	return cmd
}

// migrateRulesDir copies *.rules files from srcDir to destDir, removes originals, and returns count moved.
func migrateRulesDir(ctx stdctx.Context, srcDir, destDir string, dryRun bool) int {
	entries, err := os.ReadDir(srcDir)
	if err != nil {
		return 0
	}

	moved := 0
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != context.RulesExt {
			continue
		}

		srcPath := filepath.Join(srcDir, e.Name())
		destPath := filepath.Join(destDir, e.Name())

		if _, err := os.Stat(destPath); err == nil {
			ulog.Warn("Already exists in notebook, skipping").
				Pretty(fmt.Sprintf("⚠ Already exists, skipping: %s", e.Name())).
				Log(ctx)
			continue
		}

		if dryRun {
			ulog.Info("Would move").
				Pretty(fmt.Sprintf("  %s → %s", srcPath, destPath)).
				Log(ctx)
			moved++
			continue
		}

		if err := os.MkdirAll(destDir, 0755); err != nil {
			ulog.Warn("Failed to create destination dir").Err(err).Log(ctx)
			continue
		}

		content, err := os.ReadFile(srcPath)
		if err != nil {
			ulog.Warn("Failed to read file").Err(err).Log(ctx)
			continue
		}

		if err := os.WriteFile(destPath, content, 0644); err != nil {
			ulog.Warn("Failed to write file").Err(err).Log(ctx)
			continue
		}

		os.Remove(srcPath)
		moved++
		ulog.Success("Moved").
			Pretty(fmt.Sprintf("  %s → %s", srcPath, destPath)).
			Log(ctx)
	}

	return moved
}

// migrateGroveToml updates grove.toml to replace default_rules_path with default_rules.
// e.g. default_rules_path = ".cx/dev-no-tests.rules" → default_rules = "dev-no-tests"
func migrateGroveToml(ctx stdctx.Context, workDir string, dryRun bool) bool {
	configPath, err := config.FindConfigFile(workDir)
	if err != nil || configPath == "" {
		return false
	}

	content, err := os.ReadFile(configPath)
	if err != nil {
		return false
	}

	text := string(content)

	// Match default_rules_path = "..." (with optional quotes)
	re := regexp.MustCompile(`(?m)^(\s*)default_rules_path\s*=\s*"?([^"\n]+)"?[ \t]*$`)
	match := re.FindStringSubmatch(text)
	if match == nil {
		return false
	}

	indent := match[1]
	oldValue := match[2]

	// Extract preset name: strip path and .rules extension
	presetName := filepath.Base(oldValue)
	presetName = strings.TrimSuffix(presetName, ".rules")

	newLine := fmt.Sprintf("%sdefault_rules = %q", indent, presetName)

	if dryRun {
		ulog.Info("Would update grove.toml").
			Pretty(fmt.Sprintf("  %s: default_rules_path → default_rules = %q", configPath, presetName)).
			Log(ctx)
		return true
	}

	updated := re.ReplaceAllString(text, newLine)
	if err := os.WriteFile(configPath, []byte(updated), 0644); err != nil {
		ulog.Warn("Failed to update grove.toml").Err(err).Log(ctx)
		return false
	}

	ulog.Success("Updated grove.toml").
		Pretty(fmt.Sprintf("  %s: default_rules_path → default_rules = %q", configPath, presetName)).
		Log(ctx)
	return true
}

// removeIfEmpty removes a directory if it contains no files.
func removeIfEmpty(dir string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, e := range entries {
		if !e.IsDir() {
			return
		}
		sub := filepath.Join(dir, e.Name())
		var hasFiles bool
		filepath.WalkDir(sub, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if !d.IsDir() {
				hasFiles = true
				return filepath.SkipAll
			}
			return nil
		})
		if hasFiles {
			return
		}
	}
	os.RemoveAll(dir)
}
