package cmd

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mattsolo1/grove-context/cmd/dashboard"
	"github.com/mattsolo1/grove-context/pkg/context"
)

// NewDashboardCmd creates the dashboard command
func NewDashboardCmd() *cobra.Command {
	var horizontal bool
	var plain bool

	cmd := &cobra.Command{
		Use:   "dashboard",
		Short: "Display a live dashboard of context statistics",
		Long:  `Launch an interactive terminal UI to display real-time hot and cold context statistics that update automatically when files change.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if plain {
				// Plain output mode
				return outputPlainStats()
			}

			// Interactive TUI mode
			return dashboard.Run(horizontal)
		},
	}

	cmd.Flags().BoolVarP(&horizontal, "horizontal", "H", false, "Display statistics side by side")
	cmd.Flags().BoolVarP(&plain, "plain", "p", false, "Output plain text statistics (no TUI)")

	return cmd
}

// outputPlainStats outputs statistics in plain text format
func outputPlainStats() error {
	mgr := context.NewManager(".")

	// Get hot context files
	hotFiles, err := mgr.ResolveFilesFromRules()
	if err != nil {
		return fmt.Errorf("error resolving hot context: %w", err)
	}

	// Get cold context files
	coldFiles, err := mgr.ResolveColdContextFiles()
	if err != nil {
		return fmt.Errorf("error resolving cold context: %w", err)
	}

	// Get stats for both
	hotStats, err := mgr.GetStats("hot", hotFiles, 10)
	if err != nil {
		return fmt.Errorf("error getting hot stats: %w", err)
	}

	coldStats, err := mgr.GetStats("cold", coldFiles, 10)
	if err != nil {
		return fmt.Errorf("error getting cold stats: %w", err)
	}

	// Render boxes without TUI (plain text)
	hotBox := renderPlainBox("Hot Context Statistics", hotStats)
	coldBox := renderPlainBox("Cold Context Statistics", coldStats)

	// Split into lines for side-by-side display
	hotLines := strings.Split(hotBox, "\n")
	coldLines := strings.Split(coldBox, "\n")

	// Ensure both have same number of lines
	maxLines := len(hotLines)
	if len(coldLines) > maxLines {
		maxLines = len(coldLines)
	}

	// Pad shorter one with empty lines
	for len(hotLines) < maxLines {
		hotLines = append(hotLines, strings.Repeat(" ", 27))
	}
	for len(coldLines) < maxLines {
		coldLines = append(coldLines, strings.Repeat(" ", 27))
	}

	// Print header
	fmt.Println("Grove Context Dashboard")
	fmt.Printf("Last update: %s\n\n", time.Now().Format("15:04:05"))

	// Print side by side
	for i := 0; i < maxLines; i++ {
		fmt.Printf("%s  %s\n", hotLines[i], coldLines[i])
	}

	return nil
}

// renderPlainBox renders a statistics box without lipgloss
func renderPlainBox(title string, stats *context.ContextStats) string {
	var b strings.Builder

	// Top border
	b.WriteString("╭─────────────────────────╮\n")

	// Empty line
	b.WriteString("│                         │\n")

	// Title lines - split title into two lines
	titleLines := []string{"Hot Context", "Statistics"}
	if strings.Contains(title, "Cold") {
		titleLines[0] = "Cold Context"
	}

	// Center each line
	for _, line := range titleLines {
		padding := (25 - len(line)) / 2
		b.WriteString(fmt.Sprintf("│%*s%s%*s│\n", padding, "", line, 25-padding-len(line), ""))
	}

	// Empty lines
	b.WriteString("│                         │\n")
	b.WriteString("│                         │\n")

	if stats != nil {
		// Total Files
		filesLine := fmt.Sprintf("  Total Files:     %-6d", stats.TotalFiles)
		b.WriteString(fmt.Sprintf("│%-25s│\n", filesLine))

		// Total Tokens
		b.WriteString("│  Total Tokens:          │\n")
		tokenStr := "  ~" + context.FormatTokenCount(stats.TotalTokens)
		b.WriteString(fmt.Sprintf("│%-25s│\n", tokenStr))

		// Total Size
		sizeStr := context.FormatBytes(int(stats.TotalSize))
		// Handle size display
		sizeParts := strings.Fields(sizeStr)
		if len(sizeParts) == 2 && len(sizeParts[0]) <= 6 {
			// Normal case: "123.4 KB"
			b.WriteString(fmt.Sprintf("│  Total Size:      %-6s│\n", sizeParts[0]))
			b.WriteString(fmt.Sprintf("│  %-23s│\n", sizeParts[1]))
		} else if len(sizeStr) <= 6 {
			// Short case: fits on one line
			b.WriteString(fmt.Sprintf("│  Total Size:      %-6s│\n", sizeStr))
			b.WriteString("│                         │\n")
		} else {
			// Long case: split at 6 chars
			b.WriteString(fmt.Sprintf("│  Total Size:      %-6s│\n", sizeStr[:6]))
			b.WriteString(fmt.Sprintf("│  %-23s│\n", sizeStr[6:]))
		}
	} else {
		// No data
		b.WriteString("│  No data available      │\n")
		b.WriteString("│                         │\n")
		b.WriteString("│                         │\n")
		b.WriteString("│                         │\n")
		b.WriteString("│                         │\n")
	}

	// Empty line
	b.WriteString("│                         │\n")

	// Bottom border
	b.WriteString("╰─────────────────────────╯")

	return b.String()
}
