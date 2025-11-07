package context

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// FileInfo represents information about a file
type FileInfo struct {
	Path   string
	Tokens int
	Size   int64
}

// DiffResult contains the results of a context diff operation
type DiffResult struct {
	Added              []FileInfo
	Removed            []FileInfo
	CurrentFiles       map[string]FileInfo
	CompareFiles       map[string]FileInfo
	CurrentTotalTokens int
	CompareTotalTokens int
	CurrentTotalSize   int64
	CompareTotalSize   int64
}

// DiffContext compares the current context with a named rule set or another context
func (m *Manager) DiffContext(rulesetName string) (*DiffResult, error) {
	// Get current context files dynamically from rules
	currentFiles, err := m.ResolveFilesFromRules()
	if err != nil {
		return nil, fmt.Errorf("error resolving current context: %w", err)
	}

	var compareFiles []string

	if rulesetName == "" || rulesetName == "empty" {
		// Compare with empty context
		compareFiles = []string{}
	} else if rulesetName == "current" {
		// Compare with self (no-op, but supported)
		compareFiles = currentFiles
	} else {
		// Compare with a named rule set
		rulesetPath, err := FindRulesetFile(m.workDir, rulesetName)
		if err != nil {
			return nil, fmt.Errorf("could not find rule set '%s': %w", rulesetName, err)
		}

		// Resolve files from the rule set
		hotFiles, _, err := m.ResolveFilesFromCustomRulesFile(rulesetPath)
		if err != nil {
			return nil, fmt.Errorf("error resolving rule set '%s': %w", rulesetName, err)
		}
		compareFiles = hotFiles
	}

	// Calculate diff
	return calculateDiff(currentFiles, compareFiles), nil
}

// calculateDiff computes the difference between two file lists
func calculateDiff(currentFiles, compareFiles []string) *DiffResult {
	result := &DiffResult{
		CurrentFiles: make(map[string]FileInfo),
		CompareFiles: make(map[string]FileInfo),
	}

	// Build map of compare files
	for _, file := range compareFiles {
		info := getFileInfo(file)
		result.CompareFiles[file] = info
		result.CompareTotalTokens += info.Tokens
		result.CompareTotalSize += info.Size
	}

	// Build map of current files and find additions
	for _, file := range currentFiles {
		info := getFileInfo(file)
		result.CurrentFiles[file] = info
		result.CurrentTotalTokens += info.Tokens
		result.CurrentTotalSize += info.Size

		if _, exists := result.CompareFiles[file]; !exists {
			result.Added = append(result.Added, info)
		}
	}

	// Find removals
	for file, info := range result.CompareFiles {
		if _, exists := result.CurrentFiles[file]; !exists {
			result.Removed = append(result.Removed, info)
		}
	}

	return result
}

// getFileInfo returns information about a file
func getFileInfo(path string) FileInfo {
	info := FileInfo{Path: path}

	// Get file size and estimate tokens
	if stat, err := os.Stat(path); err == nil {
		info.Size = stat.Size()
		// Rough estimate: 4 characters per token
		info.Tokens = int(info.Size / 4)
	}

	return info
}

// PrintDiff displays the diff result in a formatted way
func (d *DiffResult) Print(compareName string) {
	fmt.Printf("Comparing current context with '%s':\n\n", compareName)

	// Show added files
	if len(d.Added) > 0 {
		fmt.Printf("Added files (%d):\n", len(d.Added))
		sort.Slice(d.Added, func(i, j int) bool {
			return d.Added[i].Tokens > d.Added[j].Tokens
		})
		for _, f := range d.Added {
			fmt.Printf("  + %-50s (%s tokens)\n", TruncatePath(f.Path, 50), FormatTokenCount(f.Tokens))
		}
		fmt.Println()
	}

	// Show removed files
	if len(d.Removed) > 0 {
		fmt.Printf("Removed files (%d):\n", len(d.Removed))
		sort.Slice(d.Removed, func(i, j int) bool {
			return d.Removed[i].Tokens > d.Removed[j].Tokens
		})
		for _, f := range d.Removed {
			fmt.Printf("  - %-50s (%s tokens)\n", TruncatePath(f.Path, 50), FormatTokenCount(f.Tokens))
		}
		fmt.Println()
	}

	// Show summary
	fmt.Println("Summary:")
	fileDiff := len(d.CurrentFiles) - len(d.CompareFiles)
	fileSign := ""
	if fileDiff > 0 {
		fileSign = "+"
	}
	fmt.Printf("  Files: %d → %d (%s%d)\n",
		len(d.CompareFiles), len(d.CurrentFiles), fileSign, fileDiff)

	tokenDiff := d.CurrentTotalTokens - d.CompareTotalTokens
	tokenSign := ""
	if tokenDiff > 0 {
		tokenSign = "+"
	}
	fmt.Printf("  Tokens: %s → %s (%s%s)\n",
		FormatTokenCount(d.CompareTotalTokens),
		FormatTokenCount(d.CurrentTotalTokens),
		tokenSign,
		FormatTokenCount(abs(tokenDiff)))

	sizeDiff := d.CurrentTotalSize - d.CompareTotalSize
	sizeSign := ""
	if sizeDiff > 0 {
		sizeSign = "+"
	}
	fmt.Printf("  Size: %s → %s (%s%s)\n",
		FormatBytes(int(d.CompareTotalSize)),
		FormatBytes(int(d.CurrentTotalSize)),
		sizeSign,
		FormatBytes(int(abs64(sizeDiff))))
}

// TruncatePath shortens a path for display
func TruncatePath(path string, maxLen int) string {
	if len(path) <= maxLen {
		return path
	}

	// Try to keep the most important parts
	parts := strings.Split(path, string(filepath.Separator))
	if len(parts) <= 2 {
		return path[:maxLen-3] + "..."
	}

	// Keep first and last parts
	result := parts[0] + "/.../" + parts[len(parts)-1]
	if len(result) > maxLen {
		return "..." + path[len(path)-(maxLen-3):]
	}

	return result
}

// abs returns the absolute value of an integer
func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

// abs64 returns the absolute value of an int64
func abs64(n int64) int64 {
	if n < 0 {
		return -n
	}
	return n
}
