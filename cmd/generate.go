package cmd

import (
	"github.com/spf13/cobra"
	"github.com/mattsolo1/grove-context/pkg/context"
	"github.com/mattsolo1/grove-core/logging"
)

var (
	useXMLFormat bool = true
	log = logging.NewLogger("grove-context")
	prettyLog = logging.NewPrettyLogger("grove-context")
)

func NewGenerateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "generate",
		Short: "Generate .grove/context from .grove/context-files",
		Long:  `Reads the .grove/context-files list and generates a concatenated .grove/context file with all specified files.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			mgr := context.NewManager("")
			
			log.Info("Generating context file")
			prettyLog.InfoPretty("Generating context file...")
			
			if err := mgr.GenerateContext(useXMLFormat); err != nil {
				return err
			}
			
			log.Info("Context file generated successfully")
			prettyLog.Success("Context file generated successfully")
			
			// Also generate cached context
			log.Info("Generating cached context file")
			prettyLog.InfoPretty("Generating cached context file...")
			
			if err := mgr.GenerateCachedContext(); err != nil {
				return err
			}
			
			log.Info("Cached context file generated successfully")
			prettyLog.Success("Cached context file generated successfully")
			return nil
		},
	}
	
	cmd.Flags().BoolVar(&useXMLFormat, "xml", true, "Use XML-style delimiters (default: true)")
	
	return cmd
}