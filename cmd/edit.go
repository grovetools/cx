package cmd

import (
	"fmt"
	"os"
	"runtime"

	"github.com/spf13/cobra"
	"github.com/mattsolo1/grove-context/pkg/context"
)

func NewEditCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "edit",
		Short: "Open the rules file in your editor",
		Long:  `Opens .grove/rules in your system's default editor (specified by $EDITOR environment variable).`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// On Windows, if no EDITOR is set, 'vim' won't work.
			// Set a sensible default if EDITOR is not set.
			if os.Getenv("EDITOR") == "" && runtime.GOOS == "windows" {
				os.Setenv("EDITOR", "notepad")
			}

			mgr := context.NewManager("")
			editorCmd, err := mgr.EditRulesCmd()
			if err != nil {
				return fmt.Errorf("failed to prepare editor command: %w", err)
			}

			if err := editorCmd.Run(); err != nil {
				return fmt.Errorf("error opening editor: %w", err)
			}

			return nil
		},
	}

	return cmd
}