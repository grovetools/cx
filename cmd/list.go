package cmd

import (
	"fmt"

	"github.com/grovetools/cx/pkg/context"
	"github.com/spf13/cobra"
)

func NewListCmd() *cobra.Command {
	var jobFile, rulesFile string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List files in context",
		Long:  `Lists the absolute paths of all files in the context.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			mgr := context.NewManager(GetWorkDir())

			targetRulesFile, err := ResolveRulesFileFlag(mgr, jobFile, rulesFile)
			if err != nil {
				return err
			}

			var files []string
			if targetRulesFile != "" {
				hotFiles, _, resolveErr := mgr.ResolveFilesFromCustomRulesFile(targetRulesFile)
				if resolveErr != nil {
					return fmt.Errorf("failed to resolve files from rules file: %w", resolveErr)
				}
				files = hotFiles
			} else {
				files, err = mgr.ListFiles()
				if err != nil {
					return err
				}
			}

			for _, file := range files {
				fmt.Println(file)
			}
			return nil
		},
	}

	AddRulesFileFlags(cmd, &jobFile, &rulesFile)

	return cmd
}
