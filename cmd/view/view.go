package view

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	core_theme "github.com/mattsolo1/grove-core/tui/theme"
	"github.com/spf13/cobra"
)

// NewViewCmd creates the view command
func NewViewCmd() *cobra.Command {
	var startPage string
	cmd := &cobra.Command{
		Use:   "view",
		Short: "Display an interactive visualization of context composition",
		Long:  `Launch an interactive terminal UI that shows which files are included, excluded, or ignored in your context based on rules and git ignore patterns.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			m, err := newPagerModel(startPage)
			if err != nil {
				return err
			}
			p := tea.NewProgram(m, tea.WithAltScreen())
			_, err = p.Run()
			return err
		},
	}
	cmd.Flags().StringVarP(&startPage, "page", "p", "tree", "The page to open on startup (tree, repo, rules, stats, list)")
	return cmd
}

type pagerModel struct {
	pages      []Page
	activePage int
	state      *sharedState
	width      int
	height     int
	keys       pagerKeyMap
	help       help.Model
}

func newPagerModel(startPage string) (*pagerModel, error) {
	state := &sharedState{loading: true}

	pages := []Page{
		NewTreePage(state),
		NewRepoPage(state),
		NewRulesPage(state),
		NewStatsPage(state),
		NewListPage(state),
	}

	activePage := 0
	for i, p := range pages {
		if p.Name() == startPage {
			activePage = i
			break
		}
	}

	return &pagerModel{
		pages:      pages,
		activePage: activePage,
		state:      state,
		keys:       pagerKeys,
		help:       help.New(),
	}, nil
}

func (m *pagerModel) Init() tea.Cmd {
	return tea.Batch(m.pages[m.activePage].Init(), refreshSharedStateCmd())
}

func (m *pagerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		for _, p := range m.pages {
			p.SetSize(m.width, m.height)
		}

	case tea.KeyMsg:
		switch {
		case key.Matches(msg, m.keys.Quit):
			return m, tea.Quit
		case key.Matches(msg, m.keys.NextPage):
			m.nextPage()
			return m, m.pages[m.activePage].Focus()
		case key.Matches(msg, m.keys.PrevPage):
			m.prevPage()
			return m, m.pages[m.activePage].Focus()
		case key.Matches(msg, m.keys.Help):
			m.help.ShowAll = !m.help.ShowAll
		}

	case stateRefreshedMsg:
		*m.state = msg.state
		return m, nil

	case refreshStateMsg:
		m.state.loading = true
		return m, refreshSharedStateCmd()
	}

	// Delegate msg to active page
	activePage, pageCmd := m.pages[m.activePage].Update(msg)
	m.pages[m.activePage] = activePage
	cmds = append(cmds, pageCmd)

	return m, tea.Batch(cmds...)
}

func (m *pagerModel) View() string {
	if m.state.loading {
		return "Loading context..."
	}
	if m.state.err != nil {
		return fmt.Sprintf("Error: %v", m.state.err)
	}

	// Tab header
	header := m.renderTabs()

	// Page content
	content := m.pages[m.activePage].View()

	// Footer
	footer := m.help.View(m.keys)

	return lipgloss.JoinVertical(lipgloss.Left, header, content, footer)
}

func (m *pagerModel) nextPage() {
	m.pages[m.activePage].Blur()
	m.activePage = (m.activePage + 1) % len(m.pages)
}

func (m *pagerModel) prevPage() {
	m.pages[m.activePage].Blur()
	m.activePage--
	if m.activePage < 0 {
		m.activePage = len(m.pages) - 1
	}
}

func (m *pagerModel) renderTabs() string {
	theme := core_theme.DefaultTheme

	inactiveTab := lipgloss.NewStyle().
		Foreground(theme.Colors.MutedText).
		Padding(0, 2)

	activeTab := lipgloss.NewStyle().
		Foreground(theme.Colors.Green).
		Bold(true).
		Padding(0, 2)

	var tabs []string
	for i, p := range m.pages {
		style := inactiveTab
		if i == m.activePage {
			style = activeTab
		}
		tabs = append(tabs, style.Render(strings.ToUpper(p.Name())))
	}

	return lipgloss.JoinHorizontal(lipgloss.Top, tabs...)
}
