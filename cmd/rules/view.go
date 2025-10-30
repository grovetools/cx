package rules

import (
	"fmt"

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

	// Build table data
	var rows [][]string
	for _, item := range m.items {
		status := " "
		if item.active {
			status = "✓"
		}
		rows = append(rows, []string{
			status,
			item.name,
			item.path,
		})
	}

	// Render header
	header := theme.DefaultTheme.Header.Render("Select an Active Rule Set")

	// Render table with selection and highlight the Name column (index 1)
	tableView := table.SelectableTableWithOptions(
		[]string{"", "Name", "Path"},
		rows,
		m.selectedIndex,
		table.SelectableTableOptions{
			HighlightColumn: 1, // Highlight the Name column
		},
	)

	// Render help
	helpContent := m.help.View()

	return fmt.Sprintf("%s\n\n%s\n\n%s", header, tableView, helpContent)
}
