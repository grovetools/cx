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
		Use:   "stats [rules-file]",
		Short: "Provide detailed analysis of context composition",
		Long: `Show language breakdown by tokens/files, identify largest token consumers, and display token distribution statistics.

If a rules file path is provided, stats will be computed from that file.
Otherwise, stats will be computed from the active rules file (.grove/rules).

Examples:
  cx stats                              # Use active .grove/rules
  cx stats plans/my-plan/rules/job.rules  # Use custom rules file`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts := cli.GetOptions(cmd)
			mgr := context.NewManager("")

			// Collect stats for both hot and cold contexts
			var allStats []*context.ContextStats
			var hotFiles, coldFiles []string
			var err error

			// Check if a custom rules file was provided
			if len(args) > 0 {
				rulesFilePath := args[0]

				// Resolve files from the custom rules file
				hotFiles, coldFiles, err = mgr.ResolveFilesFromCustomRulesFile(rulesFilePath)
				if err != nil {
					return fmt.Errorf("failed to resolve files from custom rules file: %w", err)
				}
			} else {
				// Use default behavior - resolve from active rules
				hotFiles, err = mgr.ResolveFilesFromRules()
				if err != nil {
					return err
				}

				coldFiles, err = mgr.ResolveColdContextFiles()
				if err != nil {
					return err
				}
			}

			// Get stats for hot files
			if len(hotFiles) > 0 {
				hotStats, err := mgr.GetStats("hot", hotFiles, topN)
				if err != nil {
					return err
				}
				allStats = append(allStats, hotStats)
			}

			// Get stats for cold files
			if len(coldFiles) > 0 {
				coldStats, err := mgr.GetStats("cold", coldFiles, topN)
				if err != nil {
					return err
				}
				allStats = append(allStats, coldStats)
			}
			
			// Handle case where no files found in either context
			if len(allStats) == 0 {
				if opts.JSONOutput {
					// Return empty array for JSON
					fmt.Println("[]")
				} else {
					prettyLog.WarnPretty("No files in context. Check your rules file.")
				}
				return nil
			}
			
			// Output results
			if opts.JSONOutput {
				// Output as JSON array with both stats objects
				jsonData, err := json.MarshalIndent(allStats, "", "  ")
				if err != nil {
					return fmt.Errorf("failed to marshal stats: %w", err)
				}
				fmt.Println(string(jsonData))
			} else {
				// Print both hot and cold context stats
				for i, stats := range allStats {
					if i > 0 {
						fmt.Print("\n──────────────────────────────────────────────────\n\n")
					}
					title := "Hot Context Statistics"
					if stats.ContextType == "cold" {
						title = "Cold (Cached) Context Statistics"
					}
					stats.Print(title)
				}
			}
			return nil
		},
	}
	
	cmd.Flags().IntVar(&topN, "top", 5, "Number of largest files to show")
	
	return cmd
}