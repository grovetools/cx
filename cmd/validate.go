package cmd

import (
	stdctx "context"
	"fmt"

	"github.com/grovetools/cx/pkg/context"
	"github.com/spf13/cobra"
)

func NewValidateCmd() *cobra.Command {
	var jobFile, rulesFile string

	cmd := &cobra.Command{
		Use:   "validate",
		Short: "Verify context file integrity and accessibility",
		Long:  `Check all files in .grove/context-files exist, verify file permissions, detect duplicates, and report any issues.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := stdctx.Background()
			mgr := context.NewManager(GetWorkDir())

			targetRulesFile, err := ResolveRulesFileFlag(mgr, jobFile, rulesFile)
			if err != nil {
				return err
			}

			// Resolve files from rules
			var files []string
			if targetRulesFile != "" {
				hotFiles, coldFiles, resolveErr := mgr.ResolveFilesFromCustomRulesFile(targetRulesFile)
				if resolveErr != nil {
					return fmt.Errorf("failed to resolve files from rules file: %w", resolveErr)
				}
				files = append(hotFiles, coldFiles...)
			} else {
				files, err = mgr.ResolveFilesFromRules()
				if err != nil {
					return err
				}
			}

			// Then validate those files
			result, err := mgr.ValidateContext(files)
			if err != nil {
				return err
			}

			if result.TotalFiles == 0 {
				ulog.Warn("No files in context").
					Pretty("No files in context. Check your rules file.").
					Log(ctx)
				return nil
			}

			result.Print()
			return nil
		},
	}

	AddRulesFileFlags(cmd, &jobFile, &rulesFile)

	return cmd
}
