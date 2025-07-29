package cmd

import (
	"github.com/spf13/cobra"
	"github.com/mattsolo1/grove-context/pkg/context"
	"github.com/mattsolo1/grove-core/cli"
)

func NewSaveCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "save <name>",
		Short: "Save current file list as a snapshot with optional description",
		Long:  `Saves the current .grove/context-files to .grove/context-snapshots/<name> with an optional description.`,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			logger := cli.GetLogger(cmd)
			name := args[0]
			description, _ := cmd.Flags().GetString("desc")
			
			mgr := context.NewManager("")
			
			logger.Infof("Saving snapshot: %s", name)
			
			if err := mgr.SaveSnapshot(name, description); err != nil {
				return err
			}
			
			logger.Info("Snapshot saved successfully")
			return nil
		},
	}
	
	cmd.Flags().String("desc", "", "Description for the snapshot")
	
	return cmd
}