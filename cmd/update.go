package cmd

import (
	"github.com/spf13/cobra"
	"github.com/mattsolo1/grove-context/pkg/context"
	"github.com/mattsolo1/grove-core/cli"
)

func NewUpdateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "update",
		Short: "Update .grove/context-files based on .grovectx",
		Long:  `Reads the .grovectx patterns and updates the .grove/context-files list with matching files.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			logger := cli.GetLogger(cmd)
			mgr := context.NewManager("")
			
			logger.Info("Updating context files from rules...")
			
			if err := mgr.UpdateFromRules(); err != nil {
				return err
			}
			
			logger.Info("Context files updated successfully")
			return nil
		},
	}
	
	return cmd
}