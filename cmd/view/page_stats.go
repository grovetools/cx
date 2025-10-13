package view

import (
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type statsPage struct {
	sharedState *sharedState
	viewport    viewport.Model
	width       int
	height      int
}

func NewStatsPage(state *sharedState) Page {
	vp := viewport.New(0, 0)
	return &statsPage{
		sharedState: state,
		viewport:    vp,
	}
}

func (p *statsPage) Name() string { return "stats" }

func (p *statsPage) Keys() interface{} {
	return pagerKeys
}

func (p *statsPage) Init() tea.Cmd { return nil }

func (p *statsPage) Focus() tea.Cmd {
	var b strings.Builder

	if p.sharedState.hotStats != nil && p.sharedState.hotStats.TotalFiles > 0 {
		b.WriteString(p.sharedState.hotStats.String("Hot Context Statistics"))
	} else {
		b.WriteString("No files in hot context.\n")
	}

	if p.sharedState.coldStats != nil && p.sharedState.coldStats.TotalFiles > 0 {
		b.WriteString("\n" + strings.Repeat("─", 50) + "\n\n")
		b.WriteString(p.sharedState.coldStats.String("Cold (Cached) Context Statistics"))
	}

	p.viewport.SetContent(b.String())
	p.viewport.GotoTop()
	return nil
}

func (p *statsPage) Blur() {}

func (p *statsPage) SetSize(width, height int) {
	p.width = width
	p.height = height
	p.viewport.Width = width
	p.viewport.Height = height - 5
}

func (p *statsPage) Update(msg tea.Msg) (Page, tea.Cmd) {
	var cmd tea.Cmd
	p.viewport, cmd = p.viewport.Update(msg)
	return p, cmd
}

func (p *statsPage) View() string {
	if p.sharedState.hotStats == nil && p.sharedState.coldStats == nil {
		return "No context files found to generate statistics."
	}
	return lipgloss.NewStyle().
		Width(p.width).
		Height(p.height - 5).
		Render(p.viewport.View())
}
