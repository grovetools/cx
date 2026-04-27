package rules

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	"github.com/grovetools/core/config"
	"github.com/grovetools/core/tui/components/help"

	"github.com/grovetools/cx/pkg/context"
)

// Model is the embeddable rules picker TUI. It is exported as a type alias so
// host applications (cx itself, grove-terminal, etc.) can hold a reference of
// type rules.Model and route Bubble Tea messages to it.
type Model = *rulesPickerModel

// New constructs an embeddable rules picker model. mgr is the shared context
// Manager used for rules resolution. cfg supplies user-configurable
// keybindings; pass nil to use defaults.
func New(mgr *context.Manager, cfg *config.Config) Model {
	m := newRulesPickerModel()
	m.manager = mgr
	m.keys = newPickerKeyMap(cfg)
	m.help.SetKeys(m.keys)
	return m
}

type ruleItem struct {
	name        string
	path        string
	active      bool
	content     string
	planContext string // e.g., "plan:my-plan (ws:my-ws)"
	isPlanRule  bool   // New flag
}

func (r *ruleItem) getContentLineCount() int {
	if r.content == "" {
		return 0
	}
	return len(strings.Split(r.content, "\n"))
}

type rulesPickerModel struct {
	manager             *context.Manager
	items               []ruleItem
	selectedIndex       int
	keys                pickerKeyMap
	help                help.Model
	preview             viewport.Model
	width, height       int
	err                 error
	quitting            bool
	loadingFromIdx      int
	loadingToIdx        int
	loadingActive       bool
	loadingComplete     bool
	settingIdx          int
	settingActive       bool
	settingComplete     bool
	statusMessage       string
	saveMode            bool
	saveInput           textinput.Model
	saveToWork          bool
	deletingIdx         int
	deletingActive      bool
	deletingComplete    bool
	deleteConfirmNeeded bool
	deleteConfirmIdx    int
}

func newRulesPickerModel() *rulesPickerModel {
	ti := textinput.New()
	ti.Placeholder = "ruleset-name"
	ti.CharLimit = 50
	ti.Width = 30

	return &rulesPickerModel{
		keys:      defaultPickerKeyMap,
		help:      help.New(defaultPickerKeyMap),
		preview:   viewport.New(0, 0),
		saveInput: ti,
	}
}

// IsSaveMode reports whether the save-name text input is focused so
// the host can gate pager tab jumps (PageWithTextInput).
func (m *rulesPickerModel) IsSaveMode() bool {
	return m.saveMode
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
