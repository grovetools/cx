package view

import (
	tea "github.com/charmbracelet/bubbletea"
)

// --- Page Implementation ---

type repoPage struct {
	sharedState   *sharedState
	width, height int
}

// --- Constructor ---

func NewRepoPage(state *sharedState) Page {
	return &repoPage{
		sharedState: state,
	}
}

// --- Page Interface ---

func (p *repoPage) Name() string { return "repo" }

func (p *repoPage) Init() tea.Cmd {
	return nil
}

func (p *repoPage) Focus() tea.Cmd {
	return nil
}

func (p *repoPage) Blur() {
}

func (p *repoPage) SetSize(width, height int) {
	p.width, p.height = width, height
}

// --- Update ---

func (p *repoPage) Update(msg tea.Msg) (Page, tea.Cmd) {
	return p, nil
}

// --- View ---

func (p *repoPage) View() string {
	return "Repository view - coming soon"
}
