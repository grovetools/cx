package cmd

import (
	stdctx "context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/mattsolo1/grove-context/pkg/context"
)

func NewListCacheCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list-cache",
		Short: "List cached cold context files",
		Long:  `Lists the absolute paths of all files in the cached cold context.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := stdctx.Background()
			mgr := context.NewManager("")

			coldFiles, err := mgr.ResolveColdContextFiles()
			if err != nil {
				return fmt.Errorf("error resolving cold context files: %w", err)
			}

			for _, file := range coldFiles {
				ulog.Info("Cached cold context file").
					Field("file", file).
					Pretty(file).
					Log(ctx)
			}

			return nil
		},
	}

	return cmd
}