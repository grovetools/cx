package rules

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
	"github.com/mattsolo1/grove-core/tui/components/table"
	"github.com/mattsolo1/grove-core/tui/theme"
)

func (m *rulesPickerModel) View() string {
	if m.quitting {
		return ""
	}
	if m.err != nil {
		return fmt.Sprintf("Error loading rule sets: %v\n", m.err)
	}
	if len(m.items) == 0 {
		return "No rule sets found."
	}

	// Save mode: show input prompt
	if m.saveMode {
		prompt := theme.DefaultTheme.Bold.Render("Save current rules as:")
		destDir := ".cx/"
		if m.saveToWork {
			destDir = ".cx.work/"
		}
		hint := theme.DefaultTheme.Muted.Render(fmt.Sprintf("(will save to %s)", destDir))
		cancel := theme.DefaultTheme.Muted.Render("Press q to cancel")

		return lipgloss.JoinVertical(
			lipgloss.Left,
			"",
			prompt,
			"",
			m.saveInput.View(),
			"",
			hint,
			cancel,
		)
	}

	// Build table data
	var rows [][]string
	for i, item := range m.items {
		status := " "
		if item.active {
			status = "✓"
		}

		name := item.name

		// Show loading indicator: arrow from source to destination
		if m.loadingActive || m.loadingComplete {
			if i == m.loadingFromIdx {
				name = theme.DefaultTheme.Highlight.Render("↑ " + item.name + " →")
			} else if i == m.loadingToIdx {
				name = theme.DefaultTheme.Success.Render("→ " + item.name)
			}
		}

		// Show setting indicator: star/checkmark for the item being set as active
		if m.settingActive || m.settingComplete {
			if i == m.settingIdx {
				name = theme.DefaultTheme.Success.Render("★ " + item.name)
			}
		}

		// Show delete confirmation needed indicator
		if m.deleteConfirmNeeded && i == m.deleteConfirmIdx {
			name = theme.DefaultTheme.Error.Render("⚠ " + item.name + " (press 'd' again)")
		}

		// Show deleting indicator: X for the item being deleted
		if m.deletingActive || m.deletingComplete {
			if i == m.deletingIdx {
				name = theme.DefaultTheme.Error.Render("✗ " + item.name)
			}
		}

		rows = append(rows, []string{
			status,
			name,
			item.path,
		})
	}

	// Render header
	header := theme.DefaultTheme.Header.Render("Select an Active Rule Set")

	// Render table
	tableView := table.SelectableTableWithOptions(
		[]string{"", "Name", "Path"},
		rows,
		m.selectedIndex,
		table.SelectableTableOptions{
			HighlightColumn: 1, // Highlight the Name column
		},
	)

	// Render preview pane
	selectedItem := m.items[m.selectedIndex]
	previewTitle := fmt.Sprintf("Preview: %s", selectedItem.path)
	previewStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(theme.DefaultTheme.Colors.MutedText).
		Padding(0, 1)

	previewView := previewStyle.Render(
		lipgloss.JoinVertical(
			lipgloss.Left,
			theme.DefaultTheme.Bold.Render(previewTitle),
			m.preview.View(),
		),
	)

	// Render status message
	statusView := ""
	if m.statusMessage != "" {
		statusView = theme.DefaultTheme.Success.Render("✓ " + m.statusMessage)
	}

	// Render help
	helpContent := m.help.View()

	// Final layout
	parts := []string{header, "", tableView}

	// Add status message if present
	if statusView != "" {
		parts = append(parts, "", statusView)
	}

	parts = append(parts, "", previewView, helpContent)

	return lipgloss.JoinVertical(lipgloss.Left, parts...)
}
