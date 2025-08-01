package cmd

import (
	"fmt"
	"github.com/spf13/cobra"
	"github.com/mattsolo1/grove-context/pkg/context"
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
				fmt.Println("No files in context. Check your rules file.")
				return nil
			}
			
			stats.Print()
			return nil
		},
	}
	
	cmd.Flags().IntVar(&topN, "top", 5, "Number of largest files to show")
	
	return cmd
}