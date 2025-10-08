package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"github.com/spf13/cobra"
	"github.com/mattsolo1/grove-context/pkg/context"
	"github.com/mattsolo1/grove-core/cli"
)

var (
	topN    int
	perLine bool
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
			// Handle --per-line flag
			if perLine {
				return outputPerLineStats(args)
			}

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
	cmd.Flags().BoolVar(&perLine, "per-line", false, "Provide stats for each line in the rules file")

	return cmd
}

// outputPerLineStats handles the --per-line flag logic
func outputPerLineStats(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("a rules file path must be provided when using --per-line")
	}
	rulesFilePath := args[0]

	rulesContent, err := os.ReadFile(rulesFilePath)
	if err != nil {
		return fmt.Errorf("failed to read rules file: %w", err)
	}

	mgr := context.NewManager(".")
	attribution, rules, exclusions, err := mgr.ResolveFilesWithAttribution(string(rulesContent))
	if err != nil {
		return fmt.Errorf("failed to analyze rules: %w", err)
	}

	type PerLineStat struct {
		LineNumber        int      `json:"lineNumber"`
		Rule              string   `json:"rule"`
		FileCount         int      `json:"fileCount"`
		ExcludedFileCount int      `json:"excludedFileCount,omitempty"`
		ExcludedTokens    int      `json:"excludedTokens,omitempty"`
		TotalTokens       int      `json:"totalTokens"`
		TotalSize         int64    `json:"totalSize"`
		ResolvedPaths     []string `json:"resolvedPaths"`
	}

	var results []PerLineStat
	ruleMap := make(map[int]string)
	for _, r := range rules {
		ruleMap[r.LineNum] = r.Pattern
		if r.IsExclude {
			ruleMap[r.LineNum] = "!" + r.Pattern
		}
	}

	for lineNum, files := range attribution {
		var totalTokens int
		var totalSize int64
		for _, file := range files {
			if info, err := os.Stat(file); err == nil {
				totalSize += info.Size()
				totalTokens += int(info.Size() / 4) // Rough estimate: 4 bytes per token
			}
		}

		// Calculate excluded tokens for this line if any
		var excludedTokens int
		if excludedFiles, ok := exclusions[lineNum]; ok {
			for _, file := range excludedFiles {
				if info, err := os.Stat(file); err == nil {
					excludedTokens += int(info.Size() / 4)
				}
			}
		}

		results = append(results, PerLineStat{
			LineNumber:        lineNum,
			Rule:              ruleMap[lineNum],
			FileCount:         len(files),
			ExcludedFileCount: len(exclusions[lineNum]),
			ExcludedTokens:    excludedTokens,
			TotalTokens:       totalTokens,
			TotalSize:         totalSize,
			ResolvedPaths:     files,
		})
	}

	// Add entries for exclusion rules that have exclusions but no inclusions
	for lineNum, excludedFiles := range exclusions {
		// Check if this line already has an entry in results
		found := false
		for i := range results {
			if results[i].LineNumber == lineNum {
				found = true
				break
			}
		}

		// If not found, add an entry for this exclusion rule
		if !found && len(excludedFiles) > 0 {
			var excludedTokens int
			for _, file := range excludedFiles {
				if info, err := os.Stat(file); err == nil {
					excludedTokens += int(info.Size() / 4)
				}
			}

			results = append(results, PerLineStat{
				LineNumber:        lineNum,
				Rule:              ruleMap[lineNum],
				FileCount:         0,
				ExcludedFileCount: len(excludedFiles),
				ExcludedTokens:    excludedTokens,
				TotalTokens:       0,
				TotalSize:         0,
				ResolvedPaths:     []string{},
			})
		}
	}

	// Sort results by line number for consistent output
	sort.Slice(results, func(i, j int) bool {
		return results[i].LineNumber < results[j].LineNumber
	})

	jsonData, err := json.MarshalIndent(results, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal results to JSON: %w", err)
	}
	fmt.Println(string(jsonData))
	return nil
}