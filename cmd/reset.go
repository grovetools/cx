package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	
	"github.com/spf13/cobra"
	"github.com/mattsolo1/grove-context/pkg/context"
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
				fmt.Printf("Warning: This will overwrite the existing rules file at %s\n", rulesPath)
				fmt.Print("Are you sure you want to reset to defaults? (y/N): ")
				
				var response string
				fmt.Scanln(&response)
				response = strings.ToLower(strings.TrimSpace(response))
				
				if response != "y" && response != "yes" {
					fmt.Println("Reset cancelled.")
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
				fmt.Printf("✓ Reset rules file to project defaults: %s\n", rulesPath)
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
				fmt.Printf("✓ Reset rules file to basic defaults: %s\n", rulesPath)
				fmt.Println("Tip: Configure default rules in grove.yml with:")
				fmt.Println("  context:")
				fmt.Println("    default_rules_path: .grove/default.rules")
			}
			
			// Show what was written
			fmt.Println("\nNew rules content:")
			fmt.Println(strings.Repeat("-", 40))
			
			content, err := os.ReadFile(rulesPath)
			if err == nil {
				fmt.Print(string(content))
			}
			
			return nil
		},
	}
	
	cmd.Flags().BoolVarP(&force, "force", "f", false, "Reset without confirmation prompt")
	
	return cmd
}