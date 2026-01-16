package cmd

import (
	stdctx "context"
	"strings"

	"github.com/grovetools/cx/pkg/context"
	"github.com/spf13/cobra"
)

// NewFromCmdCmd creates the from-cmd command
func NewFromCmdCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "from-cmd <command>",
		Short: "Generate context from a shell command's output",
		Long: `Executes a shell command and populates the context rules with its standard output,
treating each line as a file path.

The command is executed via 'sh -c', so shell features like pipes and redirection are supported.
It is recommended to enclose the command in quotes to ensure it is passed correctly.`,
		Example: `  cx from-cmd "rg -l 'some-function' | grep -v '_test.go'"
  cx from-cmd "find . -name '*.go' -not -path './vendor/*'"
  cx from-cmd "rg -il tmux | xargs realpath | grep -Ev '(node_modules|vendor|\.git)' | sort"`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := stdctx.Background()
			// Join all arguments into a single command string
			command := strings.Join(args, " ")

			// Create context manager
			mgr := context.NewManager("")

			// Update from command output
			if err := mgr.UpdateFromCmd(command); err != nil {
				return err
			}

			// Show what was added
			files, err := mgr.ListFiles()
			if err == nil && len(files) > 0 {
				ulog.Info("Files added to context").
					Field("count", len(files)).
					Log(ctx)
				for _, file := range files {
					ulog.Info("Added file").
						Field("file", file).
						Pretty("  " + file).
						Log(ctx)
				}
			}

			return nil
		},
	}

	return cmd
}
