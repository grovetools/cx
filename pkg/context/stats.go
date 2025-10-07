package context

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/charmbracelet/lipgloss"
	core_theme "github.com/mattsolo1/grove-core/tui/theme"
)

// LanguageStats contains statistics for a programming language
type LanguageStats struct {
	Name        string  `json:"name"`
	FileCount   int     `json:"file_count"`
	TotalTokens int     `json:"total_tokens"`
	Percentage  float64 `json:"percentage"`
}

// FileStats contains statistics for a single file
type FileStats struct {
	Path       string  `json:"path"`
	Tokens     int     `json:"tokens"`
	Size       int64   `json:"size"`
	Percentage float64 `json:"percentage"`
}

// TokenDistribution represents a range of token counts
type TokenDistribution struct {
	RangeLabel string  `json:"range_label"`
	FileCount  int     `json:"file_count"`
	Percentage float64 `json:"percentage"`
}

// ContextStats contains comprehensive statistics about the context
type ContextStats struct {
	ContextType  string                    `json:"context_type"`
	TotalFiles   int                       `json:"total_files"`
	TotalTokens  int                       `json:"total_tokens"`
	TotalSize    int64                     `json:"total_size"`
	Languages    map[string]*LanguageStats `json:"languages"`
	LargestFiles []FileStats               `json:"largest_files"`
	Distribution []TokenDistribution       `json:"distribution"`
	AvgTokens    int                       `json:"avg_tokens"`
	MedianTokens int                       `json:"median_tokens"`
}

// GetStats analyzes the context and returns comprehensive statistics
func (m *Manager) GetStats(contextType string, files []string, topN int) (*ContextStats, error) {
	if len(files) == 0 {
		return &ContextStats{ContextType: contextType}, nil
	}

	stats := &ContextStats{
		ContextType: contextType,
		TotalFiles:  len(files),
		Languages:   make(map[string]*LanguageStats),
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
		".go":       "Go",
		".js":       "JavaScript",
		".jsx":      "JavaScript",
		".ts":       "TypeScript",
		".tsx":      "TypeScript",
		".py":       "Python",
		".java":     "Java",
		".c":        "C",
		".cpp":      "C++",
		".cc":       "C++",
		".h":        "C/C++",
		".hpp":      "C++",
		".rs":       "Rust",
		".rb":       "Ruby",
		".php":      "PHP",
		".cs":       "C#",
		".swift":    "Swift",
		".kt":       "Kotlin",
		".scala":    "Scala",
		".r":        "R",
		".m":        "Objective-C",
		".mm":       "Objective-C++",
		".sh":       "Shell",
		".bash":     "Shell",
		".zsh":      "Shell",
		".fish":     "Shell",
		".ps1":      "PowerShell",
		".md":       "Markdown",
		".markdown": "Markdown",
		".rst":      "reStructuredText",
		".tex":      "LaTeX",
		".yml":      "YAML",
		".yaml":     "YAML",
		".json":     "JSON",
		".xml":      "XML",
		".html":     "HTML",
		".htm":      "HTML",
		".css":      "CSS",
		".scss":     "SCSS",
		".sass":     "Sass",
		".less":     "Less",
		".sql":      "SQL",
		".toml":     "TOML",
		".ini":      "INI",
		".conf":     "Config",
		".cfg":      "Config",
		".txt":      "Text",
		"":          "Other",
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

// String displays context statistics in a formatted, lipgloss-styled string
func (s *ContextStats) String(title string) string {
	theme := core_theme.DefaultTheme
	var b strings.Builder

	b.WriteString(theme.Title.Render(title) + "\n\n")

	// Summary box
	summaryItems := []string{
		fmt.Sprintf("Total Files:    %d", s.TotalFiles),
		fmt.Sprintf("Total Tokens:   ~%s", FormatTokenCount(s.TotalTokens)),
		fmt.Sprintf("Total Size:     %s", FormatBytes(int(s.TotalSize))),
	}
	summaryBox := theme.Box.Copy().
		BorderForeground(theme.Colors.Cyan).
		Padding(1, 2).
		Render(strings.Join(summaryItems, "\n"))
	b.WriteString(summaryBox + "\n\n")

	// Language distribution
	b.WriteString(theme.Header.Render("Language Distribution:") + "\n")

	var languages []LanguageStats
	for _, lang := range s.Languages {
		languages = append(languages, *lang)
	}
	sort.Slice(languages, func(i, j int) bool {
		return languages[i].TotalTokens > languages[j].TotalTokens
	})

	for _, lang := range languages {
		langName := theme.Info.Render(fmt.Sprintf("%-12s", lang.Name))
		percentage := theme.Highlight.Render(fmt.Sprintf("%5.1f%%", lang.Percentage))
		details := theme.Muted.Render(fmt.Sprintf("(%s tokens, %d files)",
			FormatTokenCount(lang.TotalTokens),
			lang.FileCount,
		))
		b.WriteString(fmt.Sprintf("  %s %s  %s\n", langName, percentage, details))
	}

	// Largest files
	b.WriteString("\n" + theme.Header.Render("Largest Files (by tokens):") + "\n")
	for i, file := range s.LargestFiles {
		displayPath := file.Path
		if len(displayPath) > 50 {
			displayPath = "..." + displayPath[len(displayPath)-47:]
		}

		var tokenStyle lipgloss.Style
		if file.Tokens > 10000 {
			tokenStyle = theme.Error
		} else if file.Tokens > 5000 {
			tokenStyle = theme.Warning
		} else {
			tokenStyle = theme.Info
		}

		line := fmt.Sprintf("  %2d. %-50s %s (%4.1f%%)",
			i+1,
			displayPath,
			tokenStyle.Render(FormatTokenCount(file.Tokens)+" tokens"),
			file.Percentage,
		)
		b.WriteString(line + "\n")
	}

	// Token distribution
	b.WriteString("\n" + theme.Header.Render("Token Distribution:") + "\n")
	for _, dist := range s.Distribution {
		bar := strings.Repeat("█", int(dist.Percentage/5))
		barStyled := theme.Success.Render(bar)
		line := fmt.Sprintf("  %-15s %3d files (%5.1f%%) %s",
			dist.RangeLabel+":",
			dist.FileCount,
			dist.Percentage,
			barStyled,
		)
		b.WriteString(line + "\n")
	}

	// Summary statistics
	b.WriteString(fmt.Sprintf("\nAverage tokens per file: %s\n", theme.Highlight.Render(FormatTokenCount(s.AvgTokens))))
	b.WriteString(fmt.Sprintf("Median tokens per file: %s\n", theme.Highlight.Render(FormatTokenCount(s.MedianTokens))))

	return b.String()
}

// Print displays context statistics by printing the lipgloss-styled string output.
func (s *ContextStats) Print(title string) {
	// Get the styled string from String() and print it directly.
	// This makes the CLI output for `cx stats` styled.
	fmt.Println(s.String(title))
}
