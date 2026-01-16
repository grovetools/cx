package cmd

import (
	"github.com/spf13/cobra"
	"github.com/grovetools/cx/pkg/context"
	"github.com/grovetools/core/logging"
)

var fixPrettyLog = logging.NewPrettyLogger()

func NewFixCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "fix",
		Short: "Automatically fix context validation issues",
		Long:  `Remove missing files and duplicates from the context file list.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			mgr := context.NewManager("")
			
			fixPrettyLog.InfoPretty("Fixing context validation issues...")
			
			if err := mgr.FixContext(); err != nil {
				return err
			}
			
			fixPrettyLog.Success("Context fixed successfully")
			return nil
		},
	}
	
	return cmd
}