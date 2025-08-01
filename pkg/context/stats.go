package context

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// LanguageStats contains statistics for a programming language
type LanguageStats struct {
	Name       string
	FileCount  int
	TotalTokens int
	Percentage float64
}

// FileStats contains statistics for a single file
type FileStats struct {
	Path       string
	Tokens     int
	Size       int64
	Percentage float64
}

// TokenDistribution represents a range of token counts
type TokenDistribution struct {
	RangeLabel string
	FileCount  int
	Percentage float64
}

// ContextStats contains comprehensive statistics about the context
type ContextStats struct {
	TotalFiles   int
	TotalTokens  int
	TotalSize    int64
	Languages    map[string]*LanguageStats
	LargestFiles []FileStats
	Distribution []TokenDistribution
	AvgTokens    int
	MedianTokens int
}

// GetStats analyzes the context and returns comprehensive statistics
func (m *Manager) GetStats(files []string, topN int) (*ContextStats, error) {
	if len(files) == 0 {
		return &ContextStats{}, nil
	}

	stats := &ContextStats{
		TotalFiles: len(files),
		Languages:  make(map[string]*LanguageStats),
	}
	
	var allFiles []FileStats
	var tokenCounts []int
	
	// Collect file information
	for _, file := range files {
		info, err := os.Stat(file)
		if err != nil {
			continue
		}
		
		// Calculate tokens (rough estimate)
		tokens := int(info.Size() / 4)
		tokenCounts = append(tokenCounts, tokens)
		
		fileInfo := FileStats{
			Path:   file,
			Tokens: tokens,
			Size:   info.Size(),
		}
		
		allFiles = append(allFiles, fileInfo)
		stats.TotalTokens += tokens
		stats.TotalSize += info.Size()
		
		// Determine language by extension
		ext := strings.ToLower(filepath.Ext(file))
		lang := getLanguageFromExt(ext)
		
		if _, exists := stats.Languages[lang]; !exists {
			stats.Languages[lang] = &LanguageStats{Name: lang}
		}
		stats.Languages[lang].FileCount++
		stats.Languages[lang].TotalTokens += tokens
	}
	
	// Calculate percentages
	for _, lang := range stats.Languages {
		if stats.TotalTokens > 0 {
			lang.Percentage = float64(lang.TotalTokens) * 100 / float64(stats.TotalTokens)
		}
	}
	
	// Sort files by token count and get top N
	sort.Slice(allFiles, func(i, j int) bool {
		return allFiles[i].Tokens > allFiles[j].Tokens
	})
	
	limit := topN
	if limit > len(allFiles) {
		limit = len(allFiles)
	}
	stats.LargestFiles = allFiles[:limit]
	
	// Calculate percentages for largest files
	for i := range stats.LargestFiles {
		if stats.TotalTokens > 0 {
			stats.LargestFiles[i].Percentage = float64(stats.LargestFiles[i].Tokens) * 100 / float64(stats.TotalTokens)
		}
	}
	
	// Calculate distribution
	stats.Distribution = calculateDistribution(tokenCounts)
	
	// Calculate average and median
	if len(tokenCounts) > 0 {
		stats.AvgTokens = stats.TotalTokens / len(tokenCounts)
		stats.MedianTokens = calculateMedian(tokenCounts)
	}
	
	return stats, nil
}

// getLanguageFromExt returns the language name for a file extension
func getLanguageFromExt(ext string) string {
	langMap := map[string]string{
		".go":   "Go",
		".js":   "JavaScript",
		".jsx":  "JavaScript",
		".ts":   "TypeScript",
		".tsx":  "TypeScript",
		".py":   "Python",
		".java": "Java",
		".c":    "C",
		".cpp":  "C++",
		".cc":   "C++",
		".h":    "C/C++",
		".hpp":  "C++",
		".rs":   "Rust",
		".rb":   "Ruby",
		".php":  "PHP",
		".cs":   "C#",
		".swift": "Swift",
		".kt":   "Kotlin",
		".scala": "Scala",
		".r":    "R",
		".m":    "Objective-C",
		".mm":   "Objective-C++",
		".sh":   "Shell",
		".bash": "Shell",
		".zsh":  "Shell",
		".fish": "Shell",
		".ps1":  "PowerShell",
		".md":   "Markdown",
		".markdown": "Markdown",
		".rst":  "reStructuredText",
		".tex":  "LaTeX",
		".yml":  "YAML",
		".yaml": "YAML",
		".json": "JSON",
		".xml":  "XML",
		".html": "HTML",
		".htm":  "HTML",
		".css":  "CSS",
		".scss": "SCSS",
		".sass": "Sass",
		".less": "Less",
		".sql":  "SQL",
		".toml": "TOML",
		".ini":  "INI",
		".conf": "Config",
		".cfg":  "Config",
		".txt":  "Text",
		"":      "Other",
	}
	
	if lang, exists := langMap[ext]; exists {
		return lang
	}
	
	if ext == "" {
		return "Other"
	}
	
	return "Other (" + strings.TrimPrefix(ext, ".") + ")"
}

// calculateDistribution calculates token distribution across files
func calculateDistribution(tokenCounts []int) []TokenDistribution {
	ranges := []struct {
		min   int
		max   int
		label string
	}{
		{0, 1000, "< 1k tokens"},
		{1000, 5000, "1k-5k tokens"},
		{5000, 10000, "5k-10k tokens"},
		{10000, math.MaxInt, "> 10k tokens"},
	}
	
	distribution := make([]TokenDistribution, len(ranges))
	
	for i, r := range ranges {
		distribution[i].RangeLabel = r.label
		for _, count := range tokenCounts {
			if count >= r.min && count < r.max {
				distribution[i].FileCount++
			}
		}
		if len(tokenCounts) > 0 {
			distribution[i].Percentage = float64(distribution[i].FileCount) * 100 / float64(len(tokenCounts))
		}
	}
	
	return distribution
}

// calculateMedian calculates the median of a slice of integers
func calculateMedian(counts []int) int {
	if len(counts) == 0 {
		return 0
	}
	
	sorted := make([]int, len(counts))
	copy(sorted, counts)
	sort.Ints(sorted)
	
	mid := len(sorted) / 2
	if len(sorted)%2 == 0 {
		return (sorted[mid-1] + sorted[mid]) / 2
	}
	return sorted[mid]
}

// Color codes for terminal output
const (
	colorReset  = "\033[0m"
	colorRed    = "\033[31m"
	colorYellow = "\033[33m"
	colorCyan   = "\033[36m"
	colorBold   = "\033[1m"
)

// PrintStats displays context statistics in a formatted way
func (s *ContextStats) Print() {
	fmt.Println("Context Statistics:")
	fmt.Println()
	
	// Summary box
	fmt.Println("╭─ Summary ────────────────────────────────────────╮")
	fmt.Printf("│ Total Files:    %-32d │\n", s.TotalFiles)
	fmt.Printf("│ Total Tokens:   ~%-31s │\n", FormatTokenCount(s.TotalTokens))
	fmt.Printf("│ Total Size:     %-32s │\n", FormatBytes(int(s.TotalSize)))
	fmt.Println("╰──────────────────────────────────────────────────╯")
	fmt.Println()
	
	// Language distribution
	fmt.Println("Language Distribution:")
	
	// Sort languages by token count
	var languages []LanguageStats
	for _, lang := range s.Languages {
		languages = append(languages, *lang)
	}
	sort.Slice(languages, func(i, j int) bool {
		return languages[i].TotalTokens > languages[j].TotalTokens
	})
	
	for _, lang := range languages {
		fmt.Printf("  %-12s %5.1f%%  (%s tokens, %d files)\n",
			lang.Name,
			lang.Percentage,
			FormatTokenCount(lang.TotalTokens),
			lang.FileCount,
		)
	}
	
	// Largest files
	fmt.Printf("\nLargest Files (by tokens):\n")
	for i, file := range s.LargestFiles {
		// Truncate path for display
		displayPath := file.Path
		if len(displayPath) > 50 {
			displayPath = "..." + displayPath[len(displayPath)-47:]
		}
		
		// Color code based on token count
		tokenColor := ""
		if file.Tokens > 10000 {
			tokenColor = colorRed
		} else if file.Tokens > 5000 {
			tokenColor = colorYellow
		}
		
		fmt.Printf("  %2d. %-50s %s%s tokens%s (%4.1f%%)\n",
			i+1,
			displayPath,
			tokenColor,
			FormatTokenCount(file.Tokens),
			colorReset,
			file.Percentage,
		)
	}
	
	// Token distribution
	fmt.Printf("\nToken Distribution:\n")
	for _, dist := range s.Distribution {
		bar := strings.Repeat("█", int(dist.Percentage/5))
		fmt.Printf("  %-15s %3d files (%5.1f%%) %s\n",
			dist.RangeLabel+":",
			dist.FileCount,
			dist.Percentage,
			bar,
		)
	}
	
	// Summary statistics
	fmt.Printf("\nAverage tokens per file: %s\n", FormatTokenCount(s.AvgTokens))
	fmt.Printf("Median tokens per file: %s\n", FormatTokenCount(s.MedianTokens))
}