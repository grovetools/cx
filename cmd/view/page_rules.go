package view

import (
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	core_theme "github.com/mattsolo1/grove-core/tui/theme"
)

type rulesPage struct {
	sharedState *sharedState
	viewport    viewport.Model
	width       int
	height      int
}

func NewRulesPage(state *sharedState) Page {
	vp := viewport.New(0, 0)
	return &rulesPage{
		sharedState: state,
		viewport:    vp,
	}
}

func (p *rulesPage) Name() string { return "rules" }

func (p *rulesPage) Keys() interface{} {
	return pagerKeys
}

func (p *rulesPage) Init() tea.Cmd { return nil }

func (p *rulesPage) Focus() tea.Cmd {
	header := core_theme.DefaultTheme.Header.Render(".grove/rules content:")
	content := header + "\n\n" + p.sharedState.rulesContent
	p.viewport.SetContent(content)
	return nil
}

func (p *rulesPage) Blur() {}

func (p *rulesPage) SetSize(width, height int) {
	p.width = width
	p.height = height
	// Subtract header/footer height from viewport
	p.viewport.Width = width
	p.viewport.Height = height - 5
}

func (p *rulesPage) Update(msg tea.Msg) (Page, tea.Cmd) {
	var cmd tea.Cmd
	p.viewport, cmd = p.viewport.Update(msg)
	return p, cmd
}

func (p *rulesPage) View() string {
	return lipgloss.NewStyle().
		Width(p.width).
		Height(p.height - 5). // Reserve space for pager header and footer
		Render(p.viewport.View())
}
