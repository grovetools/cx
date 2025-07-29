package cmd

import (
	"fmt"
	"github.com/spf13/cobra"
	"github.com/mattsolo1/grove-context/pkg/context"
)

var (
	showDetailed bool
	topN         int
)

func NewStatsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "stats",
		Short: "Provide detailed analysis of context composition",
		Long:  `Show language breakdown by tokens/files, identify largest token consumers, and display token distribution statistics.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			mgr := context.NewManager("")
			
			stats, err := mgr.GetStats(topN)
			if err != nil {
				return err
			}
			
			if stats.TotalFiles == 0 {
				fmt.Println("No files in context. Run 'cx update' to generate from rules.")
				return nil
			}
			
			stats.Print(showDetailed)
			return nil
		},
	}
	
	cmd.Flags().BoolVar(&showDetailed, "detailed", false, "Show detailed statistics")
	cmd.Flags().IntVar(&topN, "top", 5, "Number of largest files to show")
	
	return cmd
}