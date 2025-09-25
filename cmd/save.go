package cmd

import (
	"fmt"
	"github.com/spf13/cobra"
	"github.com/mattsolo1/grove-context/pkg/context"
	"github.com/mattsolo1/grove-core/logging"
)

var (
	saveLog = logging.NewLogger("grove-context")
	savePrettyLog = logging.NewPrettyLogger("grove-context")
)

func NewSaveCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "save <name>",
		Short: "Save current file list as a snapshot with optional description",
		Long:  `Saves the current .grove/context-files to .grove/context-snapshots/<name> with an optional description.`,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			description, _ := cmd.Flags().GetString("desc")
			
			mgr := context.NewManager("")
			
			saveLog.WithField("snapshot", name).Info("Saving snapshot")
			savePrettyLog.InfoPretty(fmt.Sprintf("Saving snapshot: %s", name))
			
			if err := mgr.SaveSnapshot(name, description); err != nil {
				return err
			}
			
			saveLog.Info("Snapshot saved successfully")
			savePrettyLog.Success("Snapshot saved successfully")
			return nil
		},
	}
	
	cmd.Flags().String("desc", "", "Description for the snapshot")
	
	return cmd
}