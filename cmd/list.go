package cmd

import (
	stdctx "context"

	"github.com/spf13/cobra"
	"github.com/mattsolo1/grove-context/pkg/context"
)

func NewListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List files in context",
		Long:  `Lists the absolute paths of all files in the context.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := stdctx.Background()
			mgr := context.NewManager(".")
			files, err := mgr.ListFiles()
			if err != nil {
				return err
			}

			for _, file := range files {
				ulog.Info("Context file").
					Field("file", file).
					Pretty(file).
					Log(ctx)
			}
			return nil
		},
	}

	return cmd
}