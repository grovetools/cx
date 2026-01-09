package cmd

import (
	stdctx "context"
	"fmt"
	"os"
	"runtime"

	"github.com/spf13/cobra"
	"github.com/mattsolo1/grove-context/pkg/context"
)

func NewEditCmd() *cobra.Command {
	var printPath bool
	cmd := &cobra.Command{
		Use:   "edit",
		Short: "Open the rules file in your editor or print its path",
		Long:  `Opens .grove/rules in your system's default editor (specified by $EDITOR environment variable), or prints the path if --print-path is used.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := stdctx.Background()
			mgr := context.NewManager("")

			if printPath {
				rulesPath, err := mgr.EnsureAndGetRulesPath()
				if err != nil {
					return fmt.Errorf("failed to get rules path: %w", err)
				}
				ulog.Info("Rules file path").
					Field("path", rulesPath).
					Pretty(rulesPath).
					Log(ctx)
				return nil
			}

			// On Windows, if no EDITOR is set, 'vim' won't work.
			// Set a sensible default if EDITOR is not set.
			if os.Getenv("EDITOR") == "" && runtime.GOOS == "windows" {
				os.Setenv("EDITOR", "notepad")
			}

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

	cmd.Flags().BoolVar(&printPath, "print-path", false, "Print the absolute path to the rules file instead of opening it")

	return cmd
}