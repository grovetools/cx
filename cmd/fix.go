package cmd

import (
	"github.com/spf13/cobra"
	"github.com/yourorg/grove-context/pkg/context"
	"github.com/yourorg/grove-core/cli"
)

func NewFixCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "fix",
		Short: "Automatically fix context validation issues",
		Long:  `Remove missing files and duplicates from the context file list.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			logger := cli.GetLogger(cmd)
			mgr := context.NewManager("")
			
			logger.Info("Fixing context validation issues...")
			
			if err := mgr.FixContext(); err != nil {
				return err
			}
			
			logger.Info("Context fixed successfully")
			return nil
		},
	}
	
	return cmd
}