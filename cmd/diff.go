package cmd

import (
	"github.com/spf13/cobra"
	"github.com/mattsolo1/grove-context/pkg/context"
)

func NewDiffCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "diff [snapshot|current]",
		Short: "Compare contexts to understand changes",
		Long:  `Compare the current context with a saved snapshot or another context to see added/removed files, token count changes, and size differences.`,
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			mgr := context.NewManager("")
			
			compareName := "empty"
			if len(args) > 0 {
				compareName = args[0]
			}
			
			diff, err := mgr.DiffContext(compareName)
			if err != nil {
				return err
			}
			
			diff.Print(compareName)
			return nil
		},
	}
	
	return cmd
}