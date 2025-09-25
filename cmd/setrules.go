package cmd

import (
	"fmt"
	"github.com/spf13/cobra"
	"github.com/mattsolo1/grove-context/pkg/context"
	"github.com/mattsolo1/grove-core/logging"
)

var (
	setRulesLog = logging.NewLogger("grove-context")
	setRulesPrettyLog = logging.NewPrettyLogger()
)

// NewSetRulesCmd creates the set-rules command
func NewSetRulesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "set-rules <path-to-rules-file>",
		Short: "Set the active rules from an external file",
		Long:  `Copies the specified rules file to .grove/rules, making it the active set of rules for the current project.`,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			sourcePath := args[0]
			
			setRulesLog.WithField("source", sourcePath).Info("Setting active rules")
			setRulesPrettyLog.InfoPretty(fmt.Sprintf("Setting active rules from: %s", sourcePath))
			
			mgr := context.NewManager("")
			if err := mgr.SetActiveRules(sourcePath); err != nil {
				return fmt.Errorf("failed to set active rules: %w", err)
			}
			
			setRulesLog.Info("Active rules set successfully")
			setRulesPrettyLog.Success("Active rules set successfully")
			return nil
		},
	}
	
	return cmd
}