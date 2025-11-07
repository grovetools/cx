package rules

import (
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	"github.com/mattsolo1/grove-core/tui/components/help"
)

type ruleItem struct {
	name    string
	path    string
	active  bool
	content string
}

func (r *ruleItem) getContentLineCount() int {
	if r.content == "" {
		return 0
	}
	return len(strings.Split(r.content, "\n"))
}

type rulesPickerModel struct {
	items            []ruleItem
	selectedIndex    int
	keys             pickerKeyMap
	help             help.Model
	preview          viewport.Model
	width, height    int
	err              error
	quitting         bool
	loadingFromIdx   int
	loadingToIdx     int
	loadingActive    bool
	loadingComplete  bool
	settingIdx       int
	settingActive    bool
	settingComplete  bool
	statusMessage    string
}

func newRulesPickerModel() *rulesPickerModel {
	return &rulesPickerModel{
		keys:    defaultPickerKeyMap,
		help:    help.New(defaultPickerKeyMap),
		preview: viewport.New(0, 0),
	}
}

func (m *rulesPickerModel) updatePreviewSize() {
	// Calculate space needed for fixed elements
	headerHeight := 3               // Header + empty line
	tableHeight := len(m.items) + 3 // Table with borders
	helpHeight := 2                 // Help text
	previewBorderHeight := 4        // Preview border, title, padding, empty line

	// Calculate available space for preview content
	usedHeight := headerHeight + tableHeight + helpHeight + previewBorderHeight
	availableHeight := m.height - usedHeight

	// Get content line count for selected item
	contentLines := 0
	if len(m.items) > 0 && m.selectedIndex < len(m.items) {
		contentLines = m.items[m.selectedIndex].getContentLineCount()
	}

	// Determine max preview height: 20 lines or 40% of screen, whichever is smaller
	maxPreviewHeight := 20
	dynamicMaxHeight := m.height * 40 / 100
	if dynamicMaxHeight < maxPreviewHeight {
		maxPreviewHeight = dynamicMaxHeight
	}

	// Use content line count, but cap at max and available space
	previewHeight := contentLines
	if previewHeight > maxPreviewHeight {
		previewHeight = maxPreviewHeight
	}
	if previewHeight > availableHeight {
		previewHeight = availableHeight
	}
	if previewHeight < 3 {
		previewHeight = 3 // Minimum height
	}

	m.preview.Width = m.width - 4 // Account for padding/border
	m.preview.Height = previewHeight
}
