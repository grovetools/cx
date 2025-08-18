package cmd

import (
	"github.com/spf13/cobra"
	"github.com/mattsolo1/grove-context/pkg/context"
	"github.com/mattsolo1/grove-core/cli"
)

var useXMLFormat bool = true

func NewGenerateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "generate",
		Short: "Generate .grove/context from .grove/context-files",
		Long:  `Reads the .grove/context-files list and generates a concatenated .grove/context file with all specified files.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			logger := cli.GetLogger(cmd)
			mgr := context.NewManager("")
			
			logger.Info("Generating context file...")
			
			if err := mgr.GenerateContext(useXMLFormat); err != nil {
				return err
			}
			
			logger.Info("Context file generated successfully")
			
			// Also generate cached context
			logger.Info("Generating cached context file...")
			
			if err := mgr.GenerateCachedContext(); err != nil {
				return err
			}
			
			logger.Info("Cached context file generated successfully")
			return nil
		},
	}
	
	cmd.Flags().BoolVar(&useXMLFormat, "xml", true, "Use XML-style delimiters (default: true)")
	
	return cmd
}