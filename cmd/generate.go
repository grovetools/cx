package cmd

import (
	"github.com/spf13/cobra"
	"github.com/mattsolo1/grove-context/pkg/context"
)

var (
	useXMLFormat bool = true
)

func NewGenerateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "generate",
		Short: "Generate .grove/context from .grove/context-files",
		Long:  `Reads the .grove/context-files list and generates a concatenated .grove/context file with all specified files.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			mgr := context.NewManager(".")
			
			prettyLog.InfoPretty("Generating context file...")
			
			if err := mgr.GenerateContext(useXMLFormat); err != nil {
				return err
			}
			
			prettyLog.Success("Context file generated successfully")
			
			// Also generate cached context
			prettyLog.InfoPretty("Generating cached context file...")
			
			if err := mgr.GenerateCachedContext(); err != nil {
				return err
			}
			
			prettyLog.Success("Cached context file generated successfully")
			return nil
		},
	}
	
	cmd.Flags().BoolVar(&useXMLFormat, "xml", true, "Use XML-style delimiters (default: true)")
	
	return cmd
}