package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/grovetools/cx/pkg/context"
)

func NewListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List files in context",
		Long:  `Lists the absolute paths of all files in the context.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			mgr := context.NewManager(".")
			files, err := mgr.ListFiles()
			if err != nil {
				return err
			}

			for _, file := range files {
				fmt.Println(file)
			}
			return nil
		},
	}

	return cmd
}