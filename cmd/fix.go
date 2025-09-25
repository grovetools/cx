package cmd

import (
	"github.com/spf13/cobra"
	"github.com/mattsolo1/grove-context/pkg/context"
	"github.com/mattsolo1/grove-core/logging"
)

var (
	fixLog = logging.NewLogger("grove-context")
	fixPrettyLog = logging.NewPrettyLogger("grove-context")
)

func NewFixCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "fix",
		Short: "Automatically fix context validation issues",
		Long:  `Remove missing files and duplicates from the context file list.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			mgr := context.NewManager("")
			
			fixLog.Info("Fixing context validation issues")
			fixPrettyLog.InfoPretty("Fixing context validation issues...")
			
			if err := mgr.FixContext(); err != nil {
				return err
			}
			
			fixLog.Info("Context fixed successfully")
			fixPrettyLog.Success("Context fixed successfully")
			return nil
		},
	}
	
	return cmd
}