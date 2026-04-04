package cmd

import (
	"fmt"

	"github.com/grovetools/cx/pkg/context"
	"github.com/spf13/cobra"
)

func NewListCacheCmd() *cobra.Command {
	var jobFile, rulesFile string

	cmd := &cobra.Command{
		Use:   "list-cache",
		Short: "List cached cold context files",
		Long:  `Lists the absolute paths of all files in the cached cold context.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			mgr := context.NewManager("")

			targetRulesFile, err := ResolveRulesFileFlag(mgr, jobFile, rulesFile)
			if err != nil {
				return err
			}

			var coldFiles []string
			if targetRulesFile != "" {
				_, coldFiles, err = mgr.ResolveFilesFromCustomRulesFile(targetRulesFile)
				if err != nil {
					return fmt.Errorf("failed to resolve files from rules file: %w", err)
				}
			} else {
				coldFiles, err = mgr.ResolveColdContextFiles()
				if err != nil {
					return fmt.Errorf("error resolving cold context files: %w", err)
				}
			}

			for _, file := range coldFiles {
				fmt.Println(file)
			}

			return nil
		},
	}

	AddRulesFileFlags(cmd, &jobFile, &rulesFile)

	return cmd
}
