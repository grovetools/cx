package cmd

import (
	"github.com/spf13/cobra"
	"github.com/grovetools/cx/pkg/context"
)

func NewShowCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "show",
		Short: "Print the entire context file",
		Long:  `Outputs the contents of .grove/context for piping to other applications.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			mgr := context.NewManager("")
			return mgr.ShowContext()
		},
	}
	
	return cmd
}