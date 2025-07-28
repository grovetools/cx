package cmd

import (
    "fmt"
    "github.com/spf13/cobra"
    "github.com/yourorg/grove-core/cli"
)

func NewShowCmd() *cobra.Command {
    cmd := &cobra.Command{
        Use:   "show",
        Short: "Show current context information",
        Long:  `Display information about the current LLM context including size, last update, and included files.`,
        RunE: func(cmd *cobra.Command, args []string) error {
            logger := cli.GetLogger(cmd)
            
            logger.Debug("Loading context information...")
            
            // TODO: Implement actual context loading and display
            // This would read the stored context and show summary information
            
            fmt.Println("Current Context Information:")
            fmt.Println("===========================")
            fmt.Println("Project: example-project")
            fmt.Println("Last Updated: 2025-07-28 14:30:00")
            fmt.Println("Context Size: 8.3KB")
            fmt.Println("Files Included: 42")
            fmt.Println("Directories: 12")
            fmt.Println()
            fmt.Println("Key Patterns:")
            fmt.Println("- Go modules: 3")
            fmt.Println("- Main packages: 2")
            fmt.Println("- Test files: 15")
            
            detailed, _ := cmd.Flags().GetBool("detailed")
            if detailed {
                fmt.Println("\nIncluded Files:")
                fmt.Println("- main.go")
                fmt.Println("- cmd/root.go")
                fmt.Println("- pkg/handler/handler.go")
                fmt.Println("... and 39 more")
            }
            
            return nil
        },
    }
    
    // Add command-specific flags
    cmd.Flags().Bool("detailed", false, "Show detailed file listing")
    cmd.Flags().String("filter", "", "Filter output by pattern")
    
    return cmd
}