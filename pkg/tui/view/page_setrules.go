package view

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/grovetools/core/tui/components/pager"
	"github.com/grovetools/core/tui/embed"
	"github.com/grovetools/cx/pkg/tui/rules"
)

// setRulesPage wraps the rules picker TUI as a pager tab so users
// can browse, select, save, and delete context rule presets without
// leaving the cx view.
type setRulesPage struct {
	sharedState *sharedState
	inner       rules.Model
	width       int
	height      int
}

// NewSetRulesPage constructs the set-rules page.
func NewSetRulesPage(state *sharedState) Page {
	return &setRulesPage{sharedState: state}
}

func (p *setRulesPage) Name() string  { return "set-rules" }
func (p *setRulesPage) Init() tea.Cmd { return nil }

func (p *setRulesPage) View() string {
	if p.inner == nil {
		return ""
	}
	return p.inner.View()
}

func (p *setRulesPage) Update(msg tea.Msg) (pager.Page, tea.Cmd) {
	if p.inner == nil {
		return p, nil
	}
	updated, cmd := p.inner.Update(msg)
	if m, ok := updated.(rules.Model); ok {
		p.inner = m
	}
	return p, cmd
}

func (p *setRulesPage) Focus() tea.Cmd {
	if p.inner == nil {
		// Lazy-init the picker on first focus.
		p.inner = rules.New(p.sharedState.workDir, nil)
		var cmds []tea.Cmd
		if p.width > 0 && p.height > 0 {
			updated, c := p.inner.Update(tea.WindowSizeMsg{Width: p.width, Height: p.height})
			if m, ok := updated.(rules.Model); ok {
				p.inner = m
			}
			if c != nil {
				cmds = append(cmds, c)
			}
		}
		if c := p.inner.Init(); c != nil {
			cmds = append(cmds, c)
		}
		return tea.Batch(cmds...)
	}
	// Re-init on re-entry so the preset list refreshes.
	return p.inner.Init()
}

func (p *setRulesPage) Blur() {
	if p.inner == nil {
		return
	}
	updated, _ := p.inner.Update(embed.BlurMsg{})
	if m, ok := updated.(rules.Model); ok {
		p.inner = m
	}
}

func (p *setRulesPage) SetSize(w, h int) {
	p.width = w
	p.height = h
	if p.inner == nil {
		return
	}
	updated, _ := p.inner.Update(tea.WindowSizeMsg{Width: w, Height: h})
	if m, ok := updated.(rules.Model); ok {
		p.inner = m
	}
}

// IsTextEntryActive gates pager tab jumps while the save-name input
// is focused inside the rules picker.
func (p *setRulesPage) IsTextEntryActive() bool {
	if p.inner == nil {
		return false
	}
	return p.inner.IsSaveMode()
}

// Compile-time checks.
var _ pager.Page = (*setRulesPage)(nil)
var _ pager.PageWithTextInput = (*setRulesPage)(nil)
