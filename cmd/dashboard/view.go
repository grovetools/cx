package dashboard

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
	"github.com/mattsolo1/grove-context/pkg/context"
	core_theme "github.com/mattsolo1/grove-core/tui/theme"
)

// View renders the UI
func (m dashboardModel) View() string {
	theme := core_theme.DefaultTheme

	// If help is visible, show it and return
	if m.help.ShowAll {
		m.help.SetSize(m.width, m.height)
		return m.help.View()
	}

	// Header
	header := theme.Header.Render("Grove Context Dashboard")

	// Status line
	statusStyle := theme.Muted.Copy().MarginBottom(1)

	updateIndicator := ""
	if m.isUpdating {
		updateIndicator = " (updating...)"
	}

	status := statusStyle.Render(fmt.Sprintf("Last update: %s%s",
		m.lastUpdate.Format("15:04:05"),
		updateIndicator))

	// Error display
	if m.err != nil {
		errorStyle := theme.Error.Copy().MarginTop(1).MarginBottom(1)

		return header + "\n" + status + "\n" +
			errorStyle.Render(fmt.Sprintf("Error: %v", m.err)) + "\n\n" +
			statusStyle.Render("Press 'r' to retry, 'q' to quit")
	}

	// Hot context box
	hotBox := renderSummaryBox("Hot Context Statistics", m.hotStats, m.horizontal)

	// Cold context box
	coldBox := renderSummaryBox("Cold Context Statistics", m.coldStats, m.horizontal)

	// Help text
	help := m.help.View()

	// Combine all elements based on layout
	if m.horizontal {
		// Horizontal layout
		statsRow := lipgloss.JoinHorizontal(
			lipgloss.Top,
			hotBox,
			"  ", // spacing between boxes
			coldBox,
		)

		return lipgloss.JoinVertical(
			lipgloss.Left,
			header,
			status,
			"",
			statsRow,
			"",
			help,
		)
	} else {
		// Vertical layout
		return lipgloss.JoinVertical(
			lipgloss.Left,
			header,
			status,
			"",
			hotBox,
			"",
			coldBox,
			"",
			help,
		)
	}
}

// renderSummaryBox renders a statistics summary box
func renderSummaryBox(title string, stats *context.ContextStats, horizontal bool) string {
	if stats == nil {
		return renderEmptyBox(title, horizontal)
	}
	theme := core_theme.DefaultTheme

	// Box style - adjust width for horizontal layout
	boxWidth := 50
	if horizontal {
		boxWidth = 25 // Half width for side-by-side display
	}

	boxStyle := theme.Box.Copy().
		BorderForeground(theme.Colors.Cyan).
		Width(boxWidth)

	// Title style - use bold for emphasis without explicit color
	titleStyle := theme.Bold.Copy().
		MarginBottom(1)

	// Content
	content := titleStyle.Render(title) + "\n\n"

	// Stats - use terminal defaults for regular text
	labelStyle := lipgloss.NewStyle()
	valueStyle := theme.Bold

	// Format file count
	fileCount := fmt.Sprintf("%-16s %s", "Total Files:", valueStyle.Render(fmt.Sprintf("%d", stats.TotalFiles)))
	content += labelStyle.Render(fileCount) + "\n"

	// Format token count
	tokenStr := "~" + context.FormatTokenCount(stats.TotalTokens)
	tokenCount := fmt.Sprintf("%-16s %s", "Total Tokens:", valueStyle.Render(tokenStr))
	content += labelStyle.Render(tokenCount) + "\n"

	// Format size
	sizeStr := context.FormatBytes(int(stats.TotalSize))
	size := fmt.Sprintf("%-16s %s", "Total Size:", valueStyle.Render(sizeStr))
	content += labelStyle.Render(size)

	return boxStyle.Render(content)
}

// renderEmptyBox renders an empty statistics box
func renderEmptyBox(title string, horizontal bool) string {
	theme := core_theme.DefaultTheme
	boxWidth := 50
	if horizontal {
		boxWidth = 25 // Half width for side-by-side display
	}

	boxStyle := theme.Box.Copy().
		BorderForeground(theme.Colors.MutedText).
		Width(boxWidth)

	titleStyle := theme.Muted.Copy().Bold(true).MarginBottom(1)

	emptyStyle := theme.Muted.Copy().Italic(true)

	content := titleStyle.Render(title) + "\n\n" +
		emptyStyle.Render("No data available")

	return boxStyle.Render(content)
}
