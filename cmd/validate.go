package cmd

import (
	stdctx "context"

	"github.com/spf13/cobra"
	"github.com/mattsolo1/grove-context/pkg/context"
)

func NewValidateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "validate",
		Short: "Verify context file integrity and accessibility",
		Long:  `Check all files in .grove/context-files exist, verify file permissions, detect duplicates, and report any issues.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := stdctx.Background()
			mgr := context.NewManager("")

			// First resolve files from rules
			files, err := mgr.ResolveFilesFromRules()
			if err != nil {
				return err
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
	
	return cmd
}