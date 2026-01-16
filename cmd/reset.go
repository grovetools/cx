package cmd

import (
	stdctx "context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/grovetools/cx/pkg/context"
	"github.com/grovetools/core/state"
)

func NewResetCmd() *cobra.Command {
	var force bool
	
	cmd := &cobra.Command{
		Use:   "reset",
		Short: "Reset the rules file to project defaults",
		Long:  `Resets the .grove/rules file to the default rules defined in grove.yml. If no default is configured, creates a basic rules file with sensible defaults.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := stdctx.Background()

			// Check for zombie worktree - refuse to create rules in deleted worktrees
			if context.IsZombieWorktreeCwd() {
				ulog.Warn("Zombie worktree detected").
					Pretty("Refusing to create rules file in deleted worktree").
					Log(ctx)
				return fmt.Errorf("cannot create rules file: worktree has been deleted")
			}

			mgr := context.NewManager("")

			// Get ONLY the default rules content (not the current rules)
			rulesContent, rulesPath := mgr.LoadDefaultRulesContent()

			// Check if there's an existing rules file
			if _, err := os.Stat(rulesPath); err == nil && !force {
				// Rules file exists, ask for confirmation
				ulog.Warn("This will overwrite existing rules file").
					Field("path", rulesPath).
					Pretty(fmt.Sprintf("This will overwrite the existing rules file at %s", rulesPath)).
					Log(ctx)
				fmt.Print("Are you sure you want to reset to defaults? (y/N): ")

				var response string
				fmt.Scanln(&response)
				response = strings.ToLower(strings.TrimSpace(response))

				if response != "y" && response != "yes" {
					ulog.Info("Reset cancelled").Log(ctx)
					return nil
				}
			}

			// Ensure .grove directory exists
			groveDir := filepath.Dir(rulesPath)
			if err := os.MkdirAll(groveDir, 0755); err != nil {
				return fmt.Errorf("error creating %s directory: %w", groveDir, err)
			}

			// Determine what content to write
			if rulesContent != nil {
				// We have default rules from grove.yml
				if err := os.WriteFile(rulesPath, rulesContent, 0644); err != nil {
					return fmt.Errorf("error writing rules file: %w", err)
				}
				ulog.Success("Reset rules file to project defaults").
					Field("path", rulesPath).
					Pretty(fmt.Sprintf("Reset rules file to project defaults: %s", rulesPath)).
					Log(ctx)
			} else {
				// No defaults configured, use basic boilerplate
				boilerplate := []byte(`# Context rules file
# Add patterns to include files, one per line
# Use ! prefix to exclude
# Examples:
#   *.go
#   !*_test.go
#   src/**/*.js

*
`)
				if err := os.WriteFile(rulesPath, boilerplate, 0644); err != nil {
					return fmt.Errorf("error writing rules file: %w", err)
				}
				ulog.Success("Reset rules file to basic defaults").
					Field("path", rulesPath).
					Pretty(fmt.Sprintf("Reset rules file to basic defaults: %s", rulesPath)).
					Log(ctx)
				ulog.Info("Configuration tip").
					Pretty("Tip: Configure default rules in grove.yml with:").
					Log(ctx)
				ulog.Info("Config example").Pretty("  context:").Log(ctx)
				ulog.Info("Config example").Pretty("    default_rules_path: .grove/default.rules").Log(ctx)
			}

			// Show what was written
			ulog.Info("New rules content:").Log(ctx)

			content, err := os.ReadFile(rulesPath)
			if err == nil {
				ulog.Info("Rules content").
					Field("content", string(content)).
					Pretty(string(content)).
					Log(ctx)
			}

			// Unset any active rule set to ensure the reset rules are now active.
			if err := state.Delete(context.StateSourceKey); err != nil {
				ulog.Warn("Could not unset active rule set in state").
					Err(err).
					Log(ctx)
			}

			return nil
		},
	}
	
	cmd.Flags().BoolVarP(&force, "force", "f", false, "Reset without confirmation prompt")
	
	return cmd
}