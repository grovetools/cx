package cmd

import (
	stdctx "context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/grovetools/cx/cmd/rules"
	"github.com/grovetools/cx/pkg/context"
	"github.com/grovetools/core/pkg/alias"
	"github.com/grovetools/core/pkg/workspace"
	"github.com/grovetools/core/state"
	"github.com/spf13/cobra"
)

// NewRulesCmd creates the 'rules' command and its subcommands.
func NewRulesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "rules",
		Short: "Manage and switch between different context rule sets",
		Long:  `Provides commands to list, set, and save named context rule sets stored in the .cx/ directory.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// If no subcommand is given, run the interactive selector
			return rules.Run()
		},
	}

	cmd.AddCommand(newRulesListCmd())
	cmd.AddCommand(newRulesSetCmd())
	cmd.AddCommand(newRulesSaveCmd())
	cmd.AddCommand(newRulesSelectCmd())
	cmd.AddCommand(newRulesUnsetCmd())
	cmd.AddCommand(newRulesLoadCmd())
	cmd.AddCommand(newRulesRmCmd())
	cmd.AddCommand(newRulesPrintPathCmd())
	cmd.AddCommand(newRulesWhereCmd())

	return cmd
}

func newRulesSelectCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "select",
		Short: "Select the active rule set interactively",
		RunE: func(cmd *cobra.Command, args []string) error {
			return rules.Run()
		},
	}
}

func newRulesUnsetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "unset",
		Short: "Unset the active rule set and fall back to .grove/rules",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := stdctx.Background()
			if err := state.Delete(context.StateSourceKey); err != nil {
				return fmt.Errorf("failed to update state: %w", err)
			}
			ulog.Success("Active rule set unset").Log(ctx)
			ulog.Info("Now using fallback file").
				Field("file", context.ActiveRulesFile).
				Pretty(fmt.Sprintf("Now using fallback file: %s (if it exists).", context.ActiveRulesFile)).
				Log(ctx)
			return nil
		},
	}
}

func newRulesLoadCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "load <name-or-path>",
		Short: "Copy a named set to .grove/rules as a modifiable working copy",
		Long: `Copy a named rule set from .cx/ or .cx.work/ to .grove/rules.
This creates a working copy that you can edit freely without affecting the original.
The state is automatically unset so .grove/rules becomes active.

You can also provide a direct file path to a rules file (including plan-specific rules).

Examples:
  cx rules load default          # Copy .cx/default.rules to .grove/rules
  cx rules load dev-no-tests     # Copy from either .cx/ or .cx.work/
  cx rules load /path/to/plan/rules/file.rules  # Copy from absolute path`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := stdctx.Background()

			// Check for zombie worktree - refuse to create rules in deleted worktrees
			if context.IsZombieWorktreeCwd() {
				return fmt.Errorf("cannot create rules file: worktree has been deleted")
			}

			nameOrPath := args[0]
			var sourcePath string

			// First, try to find as a named ruleset.
			mgr := context.NewManager("")
			path, err := mgr.FindRulesetFile(".", nameOrPath)
			if err == nil {
				sourcePath = path
			} else {
				// If not found, check if the argument is a valid file path.
				if _, statErr := os.Stat(nameOrPath); statErr == nil {
					sourcePath, _ = filepath.Abs(nameOrPath)
				} else {
					// Not a named rule and not a file path.
					return fmt.Errorf("ruleset or file not found: %s", nameOrPath)
				}
			}

			// Read the source file
			content, err := os.ReadFile(sourcePath)
			if err != nil {
				return fmt.Errorf("failed to read rule set: %w", err)
			}

			// Ensure .grove directory exists
			if err := os.MkdirAll(filepath.Dir(context.ActiveRulesFile), 0755); err != nil {
				return fmt.Errorf("failed to create .grove directory: %w", err)
			}

			// Write to .grove/rules
			if err := os.WriteFile(context.ActiveRulesFile, content, 0644); err != nil {
				return fmt.Errorf("failed to write to .grove/rules: %w", err)
			}

			// Unset any active rule set state so .grove/rules becomes active
			if err := state.Delete(context.StateSourceKey); err != nil {
				// Non-fatal, just warn
				ulog.Warn("Could not unset active rule set in state").
					Err(err).
					Log(ctx)
			}

			ulog.Success("Loaded rule set into working copy").
				Field("name", nameOrPath).
				Field("source", sourcePath).
				Pretty(fmt.Sprintf("Loaded '%s' into .grove/rules as working copy", nameOrPath)).
				Log(ctx)
			ulog.Info("Source location").
				Field("source", sourcePath).
				Pretty(fmt.Sprintf("Source: %s", sourcePath)).
				Log(ctx)
			ulog.Info("You can now edit .grove/rules freely without affecting the original").Log(ctx)
			return nil
		},
	}
}

// listRulesForProject lists rule sets for a specific project alias.
func listRulesForProject(projectAlias string, jsonOutput bool) error {
	// Import the context package to use AliasResolver
	resolver := alias.NewAliasResolver()
	projectPath, err := resolver.Resolve(projectAlias)
	if err != nil {
		return fmt.Errorf("failed to resolve project alias '%s': %w", projectAlias, err)
	}

	seen := make(map[string]bool)
	var ruleNames []string

	collectFromDir := func(dir string) {
		entries, err := os.ReadDir(dir)
		if err != nil {
			return
		}
		for _, entry := range entries {
			if !entry.IsDir() && strings.HasSuffix(entry.Name(), context.RulesExt) {
				name := strings.TrimSuffix(entry.Name(), context.RulesExt)
				if !seen[name] {
					seen[name] = true
					ruleNames = append(ruleNames, name)
				}
			}
		}
	}

	// Scan notebook presets directories first
	mgr := context.NewManager(projectPath)
	if node, nodeErr := workspace.GetProjectByPath(projectPath); nodeErr == nil {
		if presetsDir, locErr := mgr.Locator().GetContextPresetsDir(node); locErr == nil {
			collectFromDir(presetsDir)
		}
		if workDir, locErr := mgr.Locator().GetContextPresetsWorkDir(node); locErr == nil {
			collectFromDir(workDir)
		}
	}

	// Scan legacy .cx/ directory
	collectFromDir(filepath.Join(projectPath, context.RulesDir))

	return outputJSON(ruleNames)
}

// outputJSON outputs a slice of strings as JSON.
func outputJSON(data []string) error {
	// Use encoding/json to marshal the data
	jsonBytes, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %w", err)
	}
	fmt.Println(string(jsonBytes))
	return nil
}

func newRulesListCmd() *cobra.Command {
	var forProject string
	var jsonOutput bool

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List available rule sets",
		RunE: func(cmd *cobra.Command, args []string) error {
			// If --for-project is set, list rules for that project
			if forProject != "" {
				return listRulesForProject(forProject, jsonOutput)
			}

			// Original behavior: list rules for current project
			activeSource, _ := state.GetString(context.StateSourceKey)
			if activeSource == "" {
				activeSource = "(default)"
			}

			// Helper to collect rules from a directory
			collectRules := func(dir string) ([]string, error) {
				entries, err := os.ReadDir(dir)
				if err != nil {
					if os.IsNotExist(err) {
						return nil, nil // Directory doesn't exist, that's ok
					}
					return nil, fmt.Errorf("error reading %s directory: %w", dir, err)
				}

				var names []string
				for _, entry := range entries {
					if !entry.IsDir() && strings.HasSuffix(entry.Name(), context.RulesExt) {
						name := strings.TrimSuffix(entry.Name(), context.RulesExt)
						names = append(names, name)
					}
				}
				return names, nil
			}

			// New: Collect plan rules and notebook presets
			ctx := stdctx.Background()
			mgr := context.NewManager("")

			// Collect from notebook presets directories first
			var nbPresetRules, nbWorkRules []string
			if node, nodeErr := workspace.GetProjectByPath(mgr.GetWorkDir()); nodeErr == nil {
				if presetsDir, locErr := mgr.Locator().GetContextPresetsDir(node); locErr == nil {
					nbPresetRules, _ = collectRules(presetsDir)
				}
				if workDir, locErr := mgr.Locator().GetContextPresetsWorkDir(node); locErr == nil {
					nbWorkRules, _ = collectRules(workDir)
				}
			}

			// Collect from legacy .cx/
			cxRules, err := collectRules(context.RulesDir)
			if err != nil {
				return err
			}

			// Collect from legacy .cx.work/
			cxWorkRules, err := collectRules(context.RulesWorkDir)
			if err != nil {
				return err
			}
			planRules, err := mgr.ListPlanRules()
			if err != nil {
				// Non-fatal, just warn
				ulog.Warn("Could not list plan rules").
					Err(err).
					Log(ctx)
			}

			// Combine all rules, deduplicating notebook vs legacy
			seen := make(map[string]bool)
			var ruleNames []string
			for _, lists := range [][]string{nbPresetRules, nbWorkRules, cxRules, cxWorkRules} {
				for _, name := range lists {
					if !seen[name] {
						seen[name] = true
						ruleNames = append(ruleNames, name)
					}
				}
			}

			if jsonOutput {
				return outputJSON(ruleNames)
			}

			// Human-readable output
			ulog.Info("Available Rule Sets").Log(ctx)
			if len(ruleNames) == 0 && len(planRules) == 0 {
				ulog.Info("No rule sets found").Pretty("  No rule sets found.").Log(ctx)
			} else {
				for _, name := range ruleNames {
					// Find the path for this ruleset (errors are ignored for display purposes)
					path, _ := mgr.FindRulesetFile(".", name)

					indicator := "  "
					if path == activeSource {
						indicator = "* "
					}
					ulog.Info("Rule set").
						Field("name", name).
						Field("path", path).
						Field("active", path == activeSource).
						Pretty(fmt.Sprintf("%s%s", indicator, name)).
						Log(ctx)
				}

				// New: Display plan rules
				if len(planRules) > 0 {
					ulog.Info("Plan-Specific Rules").Log(ctx)
					for _, rule := range planRules {
						indicator := "  "
						if rule.Path == activeSource {
							indicator = "* "
						}
						sourceInfo := fmt.Sprintf("plan:%s (ws:%s)", rule.PlanName, rule.WorkspaceName)
						ulog.Info("Plan rule").
							Field("name", rule.Name).
							Field("plan", rule.PlanName).
							Field("workspace", rule.WorkspaceName).
							Field("path", rule.Path).
							Field("active", rule.Path == activeSource).
							Pretty(fmt.Sprintf("%s%s (%s)", indicator, rule.Name, sourceInfo)).
							Log(ctx)
					}
				}
			}

			ulog.Info("Active source").
				Field("source", activeSource).
				Pretty(fmt.Sprintf("Active Source: %s", activeSource)).
				Log(ctx)
			return nil
		},
	}

	cmd.Flags().StringVar(&forProject, "for-project", "", "List rule sets for a specific project alias")
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output results in JSON format")

	return cmd
}

func newRulesSetCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "set <name-or-path>",
		Short: "Set a named rule set as active (read-only)",
		Long: `Sets a named rule set from .cx/ or .cx.work/ as the active context source.
This makes the context read-only from that file. To create a modifiable copy, use 'cx rules load'.

You can also provide a direct file path to a rules file (including plan-specific rules).`,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := stdctx.Background()
			nameOrPath := args[0]
			var sourcePath string

			// First, try to find as a named ruleset.
			mgr := context.NewManager("")
			path, err := mgr.FindRulesetFile(".", nameOrPath)
			if err == nil {
				sourcePath = path
			} else {
				// If not found, check if the argument is a valid file path.
				if _, statErr := os.Stat(nameOrPath); statErr == nil {
					sourcePath, _ = filepath.Abs(nameOrPath)
				} else {
					// Not a named rule and not a file path.
					return fmt.Errorf("ruleset or file not found: %s", nameOrPath)
				}
			}

			if err := state.Set(context.StateSourceKey, sourcePath); err != nil {
				return fmt.Errorf("failed to update state: %w", err)
			}

			// Warn user if a .grove/rules file exists, as it will now be ignored.
			if _, err := os.Stat(context.ActiveRulesFile); err == nil {
				ulog.Warn("Existing rules file will be ignored").
					Field("file", context.ActiveRulesFile).
					Pretty(fmt.Sprintf("Warning: %s exists but will be ignored while active source is set.", context.ActiveRulesFile)).
					Log(ctx)
			}

			ulog.Success("Active context rules set").
				Field("source", sourcePath).
				Pretty(fmt.Sprintf("Active context rules set to: %s", sourcePath)).
				Log(ctx)
			return nil
		},
	}
	return cmd
}

func newRulesSaveCmd() *cobra.Command {
	var work bool
	cmd := &cobra.Command{
		Use:   "save <name>",
		Short: "Save active rules to a named set in .cx/ or .cx.work/",
		Long: `Saves the currently active rules (from .grove/rules or another set) to a new named file.
By default, saves to .cx/ for version-controlled rule sets.
Use the --work flag to save to .cx.work/ for temporary, untracked sets.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := stdctx.Background()
			name := args[0]

			mgr := context.NewManager("")
			content, _, err := mgr.LoadRulesContent()
			if err != nil {
				return fmt.Errorf("failed to load active rules to save: %w", err)
			}
			if content == nil {
				return fmt.Errorf("no active rules found to save")
			}

			destDir := context.RulesDir
			if work {
				destDir = context.RulesWorkDir
			}

			// Prioritize notebook location
			if node, nodeErr := workspace.GetProjectByPath(mgr.GetWorkDir()); nodeErr == nil {
				if work {
					if nbDir, locErr := mgr.Locator().GetContextPresetsWorkDir(node); locErr == nil {
						destDir = nbDir
					}
				} else {
					if nbDir, locErr := mgr.Locator().GetContextPresetsDir(node); locErr == nil {
						destDir = nbDir
					}
				}
			}

			if err := os.MkdirAll(destDir, 0755); err != nil {
				return fmt.Errorf("failed to create %s directory: %w", destDir, err)
			}

			destPath := filepath.Join(destDir, name+context.RulesExt)
			if err := os.WriteFile(destPath, content, 0644); err != nil {
				return fmt.Errorf("failed to save rule set: %w", err)
			}

			ulog.Success("Saved current rules").
				Field("name", name).
				Field("directory", destDir).
				Field("path", destPath).
				Pretty(fmt.Sprintf("Saved current rules as '%s' in %s/", name, destDir)).
				Log(ctx)
			return nil
		},
	}
	cmd.Flags().BoolVarP(&work, "work", "w", false, "Save to .cx.work/ for temporary, untracked rule sets")
	return cmd
}

func newRulesRmCmd() *cobra.Command {
	var force bool
	cmd := &cobra.Command{
		Use:   "rm <name>",
		Short: "Remove a named rule set",
		Long: `Deletes a named rule set from .cx/ or .cx.work/.
Rule sets in .cx/ are considered version-controlled and require the --force flag to delete.
Rule sets in .cx.work/ can be deleted without force.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := stdctx.Background()
			name := args[0]

			// Find the ruleset file
			mgr := context.NewManager("")
			rulesPath, err := mgr.FindRulesetFile(".", name)
			if err != nil {
				return err // Returns a helpful "not found" error
			}

			// Check if it's in the version-controlled directory
			// Make sure we don't match .cx.work when checking for .cx
			isVersionControlled := strings.Contains(rulesPath, context.RulesDir+string(filepath.Separator)) &&
				!strings.Contains(rulesPath, context.RulesWorkDir)

			if isVersionControlled && !force {
				return fmt.Errorf("rule set '%s' is in %s/ and is likely version-controlled. Use --force to delete", name, context.RulesDir)
			}

			// Check if this is the currently active rule set
			activeSource, _ := state.GetString(context.StateSourceKey)
			if activeSource == rulesPath {
				// Unset it first before deleting
				if err := state.Delete(context.StateSourceKey); err != nil {
					ulog.Warn("Could not unset active state before deleting").
						Field("name", name).
						Err(err).
						Log(ctx)
				}
			}

			if err := os.Remove(rulesPath); err != nil {
				return fmt.Errorf("failed to remove rule set '%s': %w", name, err)
			}

			ulog.Success("Removed rule set").
				Field("name", name).
				Field("path", rulesPath).
				Pretty(fmt.Sprintf("Removed rule set '%s' from %s", name, rulesPath)).
				Log(ctx)
			return nil
		},
	}
	cmd.Flags().BoolVarP(&force, "force", "f", false, "Force delete a version-controlled rule set from .cx/")
	return cmd
}

func newRulesWhereCmd() *cobra.Command {
	var jsonOutput bool
	cmd := &cobra.Command{
		Use:   "where",
		Short: "Show where context directories and files are located",
		Long:  `Shows the resolved paths for all context-related directories: active rules, presets, generated context, and cache.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := stdctx.Background()
			mgr := context.NewManager("")

			paths := make(map[string]string)

			// Active rules file
			if rulesPath, err := mgr.EnsureAndGetRulesPath(); err == nil {
				paths["rules"] = rulesPath
			}

			// Notebook context directories
			if node, err := workspace.GetProjectByPath(mgr.GetWorkDir()); err == nil {
				if dir, err := mgr.Locator().GetContextDir(node); err == nil {
					paths["context_dir"] = dir
				}
				if dir, err := mgr.Locator().GetContextPresetsDir(node); err == nil {
					paths["presets"] = dir
				}
				if dir, err := mgr.Locator().GetContextPresetsWorkDir(node); err == nil {
					paths["presets_work"] = dir
				}
				if dir, err := mgr.Locator().GetContextGeneratedDir(node); err == nil {
					paths["generated"] = dir
				}
				if dir, err := mgr.Locator().GetContextCacheDir(node); err == nil {
					paths["cache"] = dir
				}
			}

			// Collect preset files with paths
			type presetInfo struct {
				Name string `json:"name"`
				Path string `json:"path"`
			}
			var presetFiles []presetInfo
			var workFiles []presetInfo
			collectFiles := func(dir string) []presetInfo {
				entries, err := os.ReadDir(dir)
				if err != nil {
					return nil
				}
				var items []presetInfo
				for _, e := range entries {
					if !e.IsDir() && strings.HasSuffix(e.Name(), context.RulesExt) {
						items = append(items, presetInfo{
							Name: strings.TrimSuffix(e.Name(), context.RulesExt),
							Path: filepath.Join(dir, e.Name()),
						})
					}
				}
				return items
			}
			if p, ok := paths["presets"]; ok {
				presetFiles = collectFiles(p)
			}
			if p, ok := paths["presets_work"]; ok {
				workFiles = collectFiles(p)
			}

			if jsonOutput {
				output := struct {
					Paths       map[string]string `json:"paths"`
					Presets     []presetInfo      `json:"presets,omitempty"`
					PresetsWork []presetInfo      `json:"presets_work,omitempty"`
				}{
					Paths:       paths,
					Presets:     presetFiles,
					PresetsWork: workFiles,
				}
				jsonBytes, err := json.MarshalIndent(output, "", "  ")
				if err != nil {
					return fmt.Errorf("failed to marshal JSON: %w", err)
				}
				fmt.Println(string(jsonBytes))
				return nil
			}

			ulog.Info("Context Locations").Log(ctx)
			order := []struct{ key, label string }{
				{"rules", "Active Rules"},
				{"context_dir", "Context Dir"},
				{"presets", "Presets"},
				{"presets_work", "Presets (work)"},
				{"generated", "Generated"},
				{"cache", "Cache"},
			}
			for _, item := range order {
				if p, ok := paths[item.key]; ok {
					ulog.Info(item.label).
						Field("path", p).
						Pretty(fmt.Sprintf("  %-16s %s", item.label+":", p)).
						Log(ctx)
				}
			}

			if len(presetFiles) > 0 {
				ulog.Info("").Pretty("").Log(ctx)
				ulog.Info("Preset Rules").Log(ctx)
				for _, p := range presetFiles {
					ulog.Info("preset").
						Field("name", p.Name).
						Field("path", p.Path).
						Pretty(fmt.Sprintf("  %-28s %s", p.Name, p.Path)).
						Log(ctx)
				}
			}
			if len(workFiles) > 0 {
				ulog.Info("").Pretty("").Log(ctx)
				ulog.Info("Work Presets").Log(ctx)
				for _, p := range workFiles {
					ulog.Info("work_preset").
						Field("name", p.Name).
						Field("path", p.Path).
						Pretty(fmt.Sprintf("  %-28s %s", p.Name, p.Path)).
						Log(ctx)
				}
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output as JSON")
	return cmd
}

func newRulesPrintPathCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "print-path",
		Short: "Print the absolute path to the active rules file",
		Long:  `Prints the absolute path to the currently active rules file. Useful for scripting and integration with external tools.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			mgr := context.NewManager("")
			rulesPath, err := mgr.EnsureAndGetRulesPath()
			if err != nil {
				return fmt.Errorf("failed to get rules path: %w", err)
			}
			fmt.Println(rulesPath)
			return nil
		},
	}
}
