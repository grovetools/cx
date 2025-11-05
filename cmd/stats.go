package cmd

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/mattsolo1/grove-context/pkg/context"
	"github.com/mattsolo1/grove-core/cli"
	"github.com/mattsolo1/grove-core/pkg/repo"
	"github.com/spf13/cobra"
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

	// Convert to absolute path first to handle relative paths correctly
	absRulesPath, err := filepath.Abs(rulesFilePath)
	if err != nil {
		return fmt.Errorf("failed to get absolute path: %w", err)
	}

	// Determine the project root for workDir
	// Patterns in rules files are relative to the project root
	// If the rules file is in .grove/, .cx/, or .cx.work/, use the parent directory as workDir
	// Otherwise, use the current working directory
	rulesDir := filepath.Dir(absRulesPath)
	baseName := filepath.Base(rulesDir)
	var workDir string
	if baseName == ".grove" || baseName == ".cx" || baseName == ".cx.work" {
		workDir = filepath.Dir(rulesDir)
	} else {
		// For rules files not in standard directories, use current working directory
		workDir = "."
	}

	mgr := context.NewManager(workDir)
	attribution, _, exclusions, filteredMatches, err := mgr.ResolveFilesWithAttribution(string(rulesContent))
	if err != nil {
		return fmt.Errorf("failed to analyze rules: %w", err)
	}

	type GitInfo struct {
		URL     string `json:"url"`
		Version string `json:"version,omitempty"`
		Commit  string `json:"commit,omitempty"`
		Status  string `json:"status,omitempty"`
	}

	type FilteredByLine struct {
		LineNumber int      `json:"lineNumber"`
		Count      int      `json:"count"`
		Files      []string `json:"files,omitempty"`
	}

	type PerLineStat struct {
		LineNumber        int              `json:"lineNumber"`
		Rule              string           `json:"rule"`
		FileCount         int              `json:"fileCount"`
		ExcludedFileCount int              `json:"excludedFileCount,omitempty"`
		ExcludedTokens    int              `json:"excludedTokens,omitempty"`
		FilteredByLine    []FilteredByLine `json:"filteredByLine,omitempty"`
		TotalTokens       int              `json:"totalTokens"`
		TotalSize         int64            `json:"totalSize"`
		GitInfo           *GitInfo         `json:"gitInfo,omitempty"`
		ResolvedPaths     []string         `json:"resolvedPaths"`
		SkipReason        string           `json:"skipReason,omitempty"` // Reason why this rule was skipped
	}

	var results []PerLineStat
	// Build a map of line number to original rule text
	ruleMap := make(map[int]string)
	scanner := bufio.NewScanner(bytes.NewReader(rulesContent))
	lineNum := 1
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		// Store non-empty, non-comment lines
		// Include @alias: and @a: lines as they are rules, but exclude config directives
		isConfigDirective := strings.HasPrefix(line, "@view:") || strings.HasPrefix(line, "@v:") ||
			strings.HasPrefix(line, "@default:") || strings.HasPrefix(line, "@freeze-cache") ||
			strings.HasPrefix(line, "@no-expire") || strings.HasPrefix(line, "@disable-cache") ||
			strings.HasPrefix(line, "@expire-time") || strings.HasPrefix(line, "@find:") ||
			strings.HasPrefix(line, "@grep:")

		if line != "" && !strings.HasPrefix(line, "#") && !isConfigDirective && line != "---" {
			ruleMap[lineNum] = line
		}
		lineNum++
	}

	// Load repo manifest for git alias info
	repoManager, err := repo.NewManager()
	var manifest *repo.Manifest
	if err == nil {
		manifest, _ = repoManager.LoadManifest()
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

		// Group filtered files by their winning line number
		var filteredByLine []FilteredByLine
		if filteredInfos, ok := filteredMatches[lineNum]; ok {
			lineGroupMap := make(map[int][]string)
			for _, info := range filteredInfos {
				// Skip self-references (when a ruleset import's rules supersede each other)
				if info.WinningLineNum != lineNum {
					lineGroupMap[info.WinningLineNum] = append(lineGroupMap[info.WinningLineNum], info.File)
				}
			}
			for winningLine, filesForLine := range lineGroupMap {
				filteredByLine = append(filteredByLine, FilteredByLine{
					LineNumber: winningLine,
					Count:      len(filesForLine),
					Files:      filesForLine,
				})
			}
			// Sort by line number for consistent output
			sort.Slice(filteredByLine, func(i, j int) bool {
				return filteredByLine[i].LineNumber < filteredByLine[j].LineNumber
			})
		}

		// Check for Git info
		var gitInfo *GitInfo
		ruleText := ruleMap[lineNum]
		if ruleText != "" && manifest != nil {
			var repoURL string
			var version string

			isGitAlias := strings.HasPrefix(ruleText, "@a:git:") || strings.HasPrefix(ruleText, "@alias:git:")
			if isGitAlias {
				prefix := "@a:git:"
				if strings.HasPrefix(ruleText, "@alias:git:") {
					prefix = "@alias:git:"
				}
				repoPart := strings.TrimPrefix(ruleText, prefix)
				fullURL := "https://github.com/" + repoPart
				_, repoURL, version = mgr.ParseGitRule(fullURL)
			} else {
				_, repoURL, version = mgr.ParseGitRule(ruleText)
			}

			if repoURL != "" {
				if repoData, ok := manifest.Repositories[repoURL]; ok {
					commit := repoData.ResolvedCommit
					if len(commit) > 7 {
						commit = commit[:7]
					}
					if version == "" { // If rule didn't specify version, use pinned one
						version = repoData.PinnedVersion
					}
					if version == "default" {
						version = ""
					}

					gitInfo = &GitInfo{
						URL:     repoURL,
						Version: version,
						Commit:  commit,
						Status:  repoData.Audit.Status,
					}
				}
			}
		}

		results = append(results, PerLineStat{
			LineNumber:        lineNum,
			Rule:              ruleMap[lineNum],
			FileCount:         len(files),
			ExcludedFileCount: len(exclusions[lineNum]),
			ExcludedTokens:    excludedTokens,
			FilteredByLine:    filteredByLine,
			TotalTokens:       totalTokens,
			TotalSize:         totalSize,
			GitInfo:           gitInfo,
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

			// Check for Git info for exclusion-only rules
			var gitInfo *GitInfo
			ruleText := ruleMap[lineNum]
			if ruleText != "" && manifest != nil {
				var repoURL string
				var version string

				isGitAlias := strings.HasPrefix(ruleText, "@a:git:") || strings.HasPrefix(ruleText, "@alias:git:")
				if isGitAlias {
					prefix := "@a:git:"
					if strings.HasPrefix(ruleText, "@alias:git:") {
						prefix = "@alias:git:"
					}
					repoPart := strings.TrimPrefix(ruleText, prefix)
					fullURL := "https://github.com/" + repoPart
					_, repoURL, version = mgr.ParseGitRule(fullURL)
				} else {
					_, repoURL, version = mgr.ParseGitRule(ruleText)
				}

				if repoURL != "" {
					if repoData, ok := manifest.Repositories[repoURL]; ok {
						commit := repoData.ResolvedCommit
						if len(commit) > 7 {
							commit = commit[:7]
						}
						if version == "" {
							version = repoData.PinnedVersion
						}
						if version == "default" {
							version = ""
						}
						gitInfo = &GitInfo{
							URL:     repoURL,
							Version: version,
							Commit:  commit,
							Status:  repoData.Audit.Status,
						}
					}
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
				GitInfo:           gitInfo,
				ResolvedPaths:     []string{},
			})
		}
	}

	// Add entries for lines that have filtered matches (superseded rules)
	for lineNum, filteredInfos := range filteredMatches {
		// Check if this line already has an entry in results
		found := false
		for i := range results {
			if results[i].LineNumber == lineNum {
				found = true
				break
			}
		}

		// If not found, add an entry for this superseded rule
		if !found && len(filteredInfos) > 0 {
			// Group filtered files by their winning line number
			var filteredByLine []FilteredByLine
			lineGroupMap := make(map[int][]string)
			for _, info := range filteredInfos {
				// Skip self-references (when a ruleset import's rules supersede each other)
				if info.WinningLineNum != lineNum {
					lineGroupMap[info.WinningLineNum] = append(lineGroupMap[info.WinningLineNum], info.File)
				}
			}
			for winningLine, filesForLine := range lineGroupMap {
				filteredByLine = append(filteredByLine, FilteredByLine{
					LineNumber: winningLine,
					Count:      len(filesForLine),
					Files:      filesForLine,
				})
			}
			// Sort by line number for consistent output
			sort.Slice(filteredByLine, func(i, j int) bool {
				return filteredByLine[i].LineNumber < filteredByLine[j].LineNumber
			})

			// Only add an entry if there are actually filtered files after removing self-references
			if len(filteredByLine) > 0 {
				results = append(results, PerLineStat{
					LineNumber:        lineNum,
					Rule:              ruleMap[lineNum],
					FileCount:         0,
					ExcludedFileCount: 0,
					ExcludedTokens:    0,
					FilteredByLine:    filteredByLine,
					TotalTokens:       0,
					TotalSize:         0,
					GitInfo:           nil,
					ResolvedPaths:     []string{},
				})
			}
		}
	}

	// Add synthetic entries for Git alias/URL lines that don't have attribution yet
	// This happens because Git URLs get transformed to local paths, breaking attribution tracking
	for lineNum, ruleText := range ruleMap {
		// Check if this line already has an entry
		found := false
		for i := range results {
			if results[i].LineNumber == lineNum {
				found = true
				break
			}
		}

		// If not found, check if it's a Git alias or URL
		if !found && ruleText != "" && manifest != nil {
			var repoURL string
			var version string

			isGitAlias := strings.HasPrefix(ruleText, "@a:git:") || strings.HasPrefix(ruleText, "@alias:git:")
			isGitURL := false
			if isGitAlias {
				prefix := "@a:git:"
				if strings.HasPrefix(ruleText, "@alias:git:") {
					prefix = "@alias:git:"
				}
				repoPart := strings.TrimPrefix(ruleText, prefix)
				fullURL := "https://github.com/" + repoPart
				isGitURL, repoURL, version = mgr.ParseGitRule(fullURL)
			} else {
				isGitURL, repoURL, version = mgr.ParseGitRule(ruleText)
			}

			// If this is a Git URL/alias, add a synthetic entry with gitInfo
			if isGitURL && repoURL != "" {
				if repoData, ok := manifest.Repositories[repoURL]; ok {
					commit := repoData.ResolvedCommit
					if len(commit) > 7 {
						commit = commit[:7]
					}
					if version == "" {
						version = repoData.PinnedVersion
					}
					if version == "default" {
						version = ""
					}

					gitInfo := &GitInfo{
						URL:     repoURL,
						Version: version,
						Commit:  commit,
						Status:  repoData.Audit.Status,
					}

					// Add entry with gitInfo but empty file list
					// (files are included but not properly attributed due to path transformation)
					results = append(results, PerLineStat{
						LineNumber:        lineNum,
						Rule:              ruleText,
						FileCount:         0,
						ExcludedFileCount: 0,
						ExcludedTokens:    0,
						TotalTokens:       0,
						TotalSize:         0,
						GitInfo:           gitInfo,
						ResolvedPaths:     []string{},
					})
				}
			}
		}
	}

	// Add entries for skipped rules
	skippedRules := mgr.GetSkippedRules()
	for _, skipped := range skippedRules {
		// If line number is 0, try to find it by matching the rule pattern
		lineNum := skipped.LineNum
		if lineNum == 0 {
			// Search through ruleMap to find matching rule
			for num, rule := range ruleMap {
				if rule == skipped.Rule {
					lineNum = num
					break
				}
			}
		}

		// If we still don't have a line number, skip this entry
		if lineNum == 0 {
			continue
		}

		// Check if this line already has an entry in results
		found := false
		for i := range results {
			if results[i].LineNumber == lineNum {
				// Update the existing entry with skip reason if it doesn't have one
				if results[i].SkipReason == "" {
					results[i].SkipReason = skipped.Reason
				}
				found = true
				break
			}
		}

		// If not found, add an entry for this skipped rule
		if !found {
			results = append(results, PerLineStat{
				LineNumber: lineNum,
				Rule:       skipped.Rule,
				FileCount:  0,
				SkipReason: skipped.Reason,
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