package cmd

import (
	"fmt"
	"github.com/spf13/cobra"
	"github.com/mattsolo1/grove-context/pkg/context"
	"github.com/mattsolo1/grove-core/logging"
)

var (
	loadLog = logging.NewLogger("grove-context")
	loadPrettyLog = logging.NewPrettyLogger()
)

func NewLoadCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "load <name>",
		Short: "Load a saved file list snapshot",
		Long:  `Loads a snapshot from .grove/context-snapshots/<name> to .grove/context-files.`,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			mgr := context.NewManager("")
			
			loadLog.WithField("snapshot", name).Info("Loading snapshot")
			loadPrettyLog.InfoPretty(fmt.Sprintf("Loading snapshot: %s", name))
			
			if err := mgr.LoadSnapshot(name); err != nil {
				return err
			}
			
			loadLog.Info("Snapshot loaded successfully")
			loadPrettyLog.Success("Snapshot loaded successfully")
			return nil
		},
	}
	
	return cmd
}