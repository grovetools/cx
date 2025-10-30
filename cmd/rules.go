package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mattsolo1/grove-context/cmd/rules"
	"github.com/mattsolo1/grove-context/pkg/context"
	"github.com/mattsolo1/grove-core/state"
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
			if err := state.Delete(context.StateSourceKey); err != nil {
				return fmt.Errorf("failed to update state: %w", err)
			}
			prettyLog.Success("Active rule set unset.")
			prettyLog.InfoPretty(fmt.Sprintf("Now using fallback file: %s (if it exists).", context.ActiveRulesFile))
			return nil
		},
	}
}

func newRulesLoadCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "load <name>",
		Short: "Copy a named rule set to .grove/rules as a working copy",
		Long: `Copy a named rule set from .cx/ or .cx.work/ to .grove/rules.
This creates a working copy that you can edit freely without affecting the original.
The state is automatically unset so .grove/rules becomes active.

Examples:
  cx rules load default          # Copy .cx/default.rules to .grove/rules
  cx rules load dev-no-tests     # Copy from either .cx/ or .cx.work/`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]

			// Try to find the source file in .cx/ or .cx.work/
			var sourcePath string
			cxPath := filepath.Join(context.RulesDir, name+context.RulesExt)
			cxWorkPath := filepath.Join(context.RulesWorkDir, name+context.RulesExt)

			if _, err := os.Stat(cxPath); err == nil {
				sourcePath = cxPath
			} else if _, err := os.Stat(cxWorkPath); err == nil {
				sourcePath = cxWorkPath
			} else {
				return fmt.Errorf("rule set '%s' not found in %s/ or %s/", name, context.RulesDir, context.RulesWorkDir)
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
				prettyLog.WarnPretty(fmt.Sprintf("Warning: could not unset active rule set in state: %v", err))
			}

			prettyLog.Success(fmt.Sprintf("Loaded '%s' into .grove/rules as working copy", name))
			prettyLog.InfoPretty(fmt.Sprintf("Source: %s", sourcePath))
			prettyLog.InfoPretty("You can now edit .grove/rules freely without affecting the original.")
			return nil
		},
	}
}

// listRulesForProject lists rule sets for a specific project alias.
func listRulesForProject(projectAlias string, jsonOutput bool) error {
	// Import the context package to use AliasResolver
	resolver := context.NewAliasResolver()
	projectPath, err := resolver.Resolve(projectAlias)
	if err != nil {
		return fmt.Errorf("failed to resolve project alias '%s': %w", projectAlias, err)
	}

	// Scan the .cx/ directory in the resolved project path
	cxDir := filepath.Join(projectPath, context.RulesDir)
	entries, err := os.ReadDir(cxDir)
	if err != nil {
		if os.IsNotExist(err) {
			if jsonOutput {
				fmt.Println("[]")
				return nil
			}
			return fmt.Errorf("no %s directory found in project '%s' at %s", context.RulesDir, projectAlias, projectPath)
		}
		return fmt.Errorf("error reading %s directory: %w", cxDir, err)
	}

	// Collect rule set names
	var ruleNames []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), context.RulesExt) {
			name := strings.TrimSuffix(entry.Name(), context.RulesExt)
			ruleNames = append(ruleNames, name)
		}
	}

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

			// Collect from .cx/
			cxRules, err := collectRules(context.RulesDir)
			if err != nil {
				return err
			}

			// Collect from .cx.work/
			cxWorkRules, err := collectRules(context.RulesWorkDir)
			if err != nil {
				return err
			}

			// Combine all rules
			var ruleNames []string
			ruleNames = append(ruleNames, cxRules...)
			ruleNames = append(ruleNames, cxWorkRules...)

			if jsonOutput {
				return outputJSON(ruleNames)
			}

			// Human-readable output
			prettyLog.InfoPretty("Available Rule Sets:")
			if len(ruleNames) == 0 {
				prettyLog.InfoPretty("  No rule sets found.")
			} else {
				for _, name := range ruleNames {
					// Check both directories for the rule
					var path string
					if _, err := os.Stat(filepath.Join(context.RulesDir, name+context.RulesExt)); err == nil {
						path = filepath.Join(context.RulesDir, name+context.RulesExt)
					} else if _, err := os.Stat(filepath.Join(context.RulesWorkDir, name+context.RulesExt)); err == nil {
						path = filepath.Join(context.RulesWorkDir, name+context.RulesExt)
					}

					indicator := "  "
					if path == activeSource {
						indicator = "✓ "
					}
					prettyLog.InfoPretty(fmt.Sprintf("%s%s", indicator, name))
				}
			}

			prettyLog.Blank()
			prettyLog.InfoPretty(fmt.Sprintf("Active Source: %s", activeSource))
			return nil
		},
	}

	cmd.Flags().StringVar(&forProject, "for-project", "", "List rule sets for a specific project alias")
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output results in JSON format")

	return cmd
}

func newRulesSetCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "set <name>",
		Short: "Set the active rule set",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			sourcePath := filepath.Join(context.RulesDir, name+context.RulesExt)

			if _, err := os.Stat(sourcePath); os.IsNotExist(err) {
				return fmt.Errorf("rule set '%s' not found at %s", name, sourcePath)
			}

			if err := state.Set(context.StateSourceKey, sourcePath); err != nil {
				return fmt.Errorf("failed to update state: %w", err)
			}

			// Warn user if a .grove/rules file exists, as it will now be ignored.
			if _, err := os.Stat(context.ActiveRulesFile); err == nil {
				prettyLog.WarnPretty(fmt.Sprintf("Warning: %s exists but will be ignored while '%s' is active.", context.ActiveRulesFile, name))
			}

			prettyLog.Success(fmt.Sprintf("Active context rules set to '%s'", name))
			return nil
		},
	}
	return cmd
}

func newRulesSaveCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "save <name>",
		Short: "Save the current active rules to a new named rule set",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]

			mgr := context.NewManager("")
			content, _, err := mgr.LoadRulesContent()
			if err != nil {
				return fmt.Errorf("failed to load active rules to save: %w", err)
			}
			if content == nil {
				return fmt.Errorf("no active rules found to save")
			}

			if err := os.MkdirAll(context.RulesDir, 0755); err != nil {
				return fmt.Errorf("failed to create %s directory: %w", context.RulesDir, err)
			}

			destPath := filepath.Join(context.RulesDir, name+context.RulesExt)
			if err := os.WriteFile(destPath, content, 0644); err != nil {
				return fmt.Errorf("failed to save rule set: %w", err)
			}

			prettyLog.Success(fmt.Sprintf("Saved current rules as '%s'", name))
			return nil
		},
	}
	return cmd
}
