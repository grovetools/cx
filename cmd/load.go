package cmd

import (
	"github.com/spf13/cobra"
	"github.com/mattsolo1/grove-context/pkg/context"
	"github.com/mattsolo1/grove-core/cli"
)

func NewLoadCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "load <name>",
		Short: "Load a saved file list snapshot",
		Long:  `Loads a snapshot from .grove/context-snapshots/<name> to .grove/context-files.`,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			logger := cli.GetLogger(cmd)
			name := args[0]
			mgr := context.NewManager("")
			
			logger.Infof("Loading snapshot: %s", name)
			
			if err := mgr.LoadSnapshot(name); err != nil {
				return err
			}
			
			logger.Info("Snapshot loaded successfully")
			return nil
		},
	}
	
	return cmd
}