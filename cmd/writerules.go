package cmd

import (
	"fmt"
	"github.com/spf13/cobra"
	"github.com/mattsolo1/grove-context/pkg/context"
	"github.com/mattsolo1/grove-core/logging"
)

var (
	writeRulesLog = logging.NewLogger("grove-context")
	writeRulesPrettyLog = logging.NewPrettyLogger()
)

// NewWriteRulesCmd creates the write-rules command
func NewWriteRulesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "write-rules <destination-path>",
		Short: "Write the current active rules to a file",
		Long:  `Copies the current .grove/rules file to the specified destination path.`,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			destPath := args[0]

			writeRulesLog.WithField("destination", destPath).Info("Writing active rules")
			writeRulesPrettyLog.InfoPretty(fmt.Sprintf("Writing active rules to: %s", destPath))

			mgr := context.NewManager("")
			if err := mgr.WriteRulesTo(destPath); err != nil {
				return fmt.Errorf("failed to write rules: %w", err)
			}

			writeRulesLog.Info("Rules written successfully")
			writeRulesPrettyLog.Success("Rules written successfully")
			return nil
		},
	}

	return cmd
}
