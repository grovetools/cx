package cmd

import (
	"github.com/grovetools/core/logging"
	"github.com/grovetools/cx/pkg/context"
	"github.com/spf13/cobra"
)

var fixPrettyLog = logging.NewPrettyLogger()

func NewFixCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "fix",
		Short: "Automatically fix context validation issues",
		Long:  `Remove missing files and duplicates from the context file list.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			mgr := context.NewManager(GetWorkDir())

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
