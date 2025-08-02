package cmd

import (
	"encoding/json"
	"fmt"
	"github.com/spf13/cobra"
	"github.com/mattsolo1/grove-context/pkg/context"
	"github.com/mattsolo1/grove-core/cli"
)

var (
	topN int
)

func NewStatsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "stats",
		Short: "Provide detailed analysis of context composition",
		Long:  `Show language breakdown by tokens/files, identify largest token consumers, and display token distribution statistics.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			opts := cli.GetOptions(cmd)
			mgr := context.NewManager("")
			
			// First resolve files from rules
			files, err := mgr.ResolveFilesFromRules()
			if err != nil {
				return err
			}
			
			// Then get stats for those files
			stats, err := mgr.GetStats(files, topN)
			if err != nil {
				return err
			}
			
			if stats.TotalFiles == 0 {
				if opts.JSONOutput {
					// Return empty array for JSON
					fmt.Println("[]")
				} else {
					fmt.Println("No files in context. Check your rules file.")
				}
				return nil
			}
			
			if opts.JSONOutput {
				// Output as JSON array with single stats object
				jsonData, err := json.MarshalIndent([]*context.ContextStats{stats}, "", "  ")
				if err != nil {
					return fmt.Errorf("failed to marshal stats: %w", err)
				}
				fmt.Println(string(jsonData))
			} else {
				stats.Print()
			}
			return nil
		},
	}
	
	cmd.Flags().IntVar(&topN, "top", 5, "Number of largest files to show")
	
	return cmd
}