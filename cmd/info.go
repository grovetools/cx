package cmd

import (
	"fmt"
	"github.com/spf13/cobra"
	"github.com/mattsolo1/grove-context/pkg/context"
)

var listFiles bool

func NewInfoCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "info",
		Short: "Display information about the context file",
		Long:  `Reads the .grove/context file and outputs its approximate token count and file summary.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			mgr := context.NewManager("")
			
			if listFiles {
				// List files from .grove/context-files
				files, err := mgr.ListFiles()
				if err != nil {
					return err
				}
				
				for _, file := range files {
					fmt.Println(file)
				}
				return nil
			}
			
			// Get context info
			fileCount, tokenCount, size, err := mgr.GetContextInfo()
			if err != nil {
				return err
			}
			
			// Format numbers with units
			fmt.Printf("Files in context: %d\n", fileCount)
			fmt.Printf("Approximate token count: %s\n", context.FormatTokenCount(tokenCount))
			fmt.Printf("Context file size: %s\n", context.FormatBytes(size))
			
			return nil
		},
	}
	
	cmd.Flags().BoolVar(&listFiles, "list-files", false, "List absolute paths of files in context")
	
	return cmd
}