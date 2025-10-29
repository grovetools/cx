package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/mattsolo1/grove-context/pkg/context"
	"github.com/mattsolo1/grove-core/state"
)

func NewResetCmd() *cobra.Command {
	var force bool
	
	cmd := &cobra.Command{
		Use:   "reset",
		Short: "Reset the rules file to project defaults",
		Long:  `Resets the .grove/rules file to the default rules defined in grove.yml. If no default is configured, creates a basic rules file with sensible defaults.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			mgr := context.NewManager("")
			
			// Get ONLY the default rules content (not the current rules)
			rulesContent, rulesPath := mgr.LoadDefaultRulesContent()
			
			// Check if there's an existing rules file
			if _, err := os.Stat(rulesPath); err == nil && !force {
				// Rules file exists, ask for confirmation
				prettyLog.WarnPretty(fmt.Sprintf("This will overwrite the existing rules file at %s", rulesPath))
				fmt.Print("Are you sure you want to reset to defaults? (y/N): ")
				
				var response string
				fmt.Scanln(&response)
				response = strings.ToLower(strings.TrimSpace(response))
				
				if response != "y" && response != "yes" {
					prettyLog.InfoPretty("Reset cancelled.")
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
				prettyLog.Success(fmt.Sprintf("Reset rules file to project defaults: %s", rulesPath))
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
				prettyLog.Success(fmt.Sprintf("Reset rules file to basic defaults: %s", rulesPath))
				prettyLog.InfoPretty("Tip: Configure default rules in grove.yml with:")
				prettyLog.InfoPretty("  context:")
				prettyLog.InfoPretty("    default_rules_path: .grove/default.rules")
			}
			
			// Show what was written
			prettyLog.Blank()
			prettyLog.InfoPretty("New rules content:")
			prettyLog.Divider()
			
			content, err := os.ReadFile(rulesPath)
			if err == nil {
				prettyLog.Code(string(content))
			}

			// Unset any active rule set to ensure the reset rules are now active.
			if err := state.Delete(context.StateSourceKey); err != nil {
				prettyLog.WarnPretty(fmt.Sprintf("Warning: could not unset active rule set in state: %v", err))
			}

			return nil
		},
	}
	
	cmd.Flags().BoolVarP(&force, "force", "f", false, "Reset without confirmation prompt")
	
	return cmd
}