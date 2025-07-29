package cmd

import (
	"fmt"
	"github.com/spf13/cobra"
	"github.com/mattsolo1/grove-context/pkg/context"
)

func NewValidateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "validate",
		Short: "Verify context file integrity and accessibility",
		Long:  `Check all files in .grove/context-files exist, verify file permissions, detect duplicates, and report any issues.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			mgr := context.NewManager("")
			
			result, err := mgr.ValidateContext()
			if err != nil {
				return err
			}
			
			if result.TotalFiles == 0 {
				fmt.Println("No files in context. Run 'cx update' to generate from rules.")
				return nil
			}
			
			result.Print()
			return nil
		},
	}
	
	return cmd
}