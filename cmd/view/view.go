package view

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/mattsolo1/grove-context/pkg/context"
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
			finalModel, err := p.Run()
			if err != nil {
				return err
			}

			// After the TUI exits, check if it was for Neovim editing integration
			if pagerM, ok := finalModel.(*pagerModel); ok && pagerM.exitForNvimEdit {
				// Print the special string for the Neovim plugin to capture
				fmt.Println("EDIT_FILE:" + pagerM.nvimEditPath)
			}

			return nil
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

	// State for Neovim integration
	exitForNvimEdit bool
	nvimEditPath    string
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
		pages:           pages,
		activePage:      activePage,
		state:           state,
		keys:            pagerKeys,
		help:            help.New(),
		exitForNvimEdit: false,
		nvimEditPath:    "",
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
		case key.Matches(msg, m.keys.Edit):
			// Edit action is only available on the RULES page
			if m.pages[m.activePage].Name() == "rules" {
				mgr := context.NewManager("")

				// Check if running inside the Neovim plugin
				if os.Getenv("GROVE_NVIM_PLUGIN") == "true" {
					// Signal to Neovim to open the file
					rulesPath, err := mgr.EnsureAndGetRulesPath()
					if err != nil {
						m.state.err = fmt.Errorf("failed to get rules path: %w", err)
						return m, nil
					}
					m.exitForNvimEdit = true
					m.nvimEditPath = rulesPath
					return m, tea.Quit
				}

				// Standard terminal: suspend TUI and open editor
				editorCmd, err := mgr.EditRulesCmd()
				if err != nil {
					m.state.err = fmt.Errorf("failed to prepare editor command: %w", err)
					return m, nil
				}
				return m, tea.ExecProcess(editorCmd, func(err error) tea.Msg {
					// After editor closes, refresh the state
					return refreshStateMsg{}
				})
			}
		case key.Matches(msg, m.keys.SelectRules):
			// SelectRules action is only available on the RULES page
			if m.pages[m.activePage].Name() == "rules" {
				// Run cx rules command to select a rule set
				cxCmd := exec.Command("cx", "rules")
				cxCmd.Stdin = os.Stdin
				cxCmd.Stdout = os.Stdout
				cxCmd.Stderr = os.Stderr

				return m, tea.ExecProcess(cxCmd, func(err error) tea.Msg {
					// After cx rules exits, refresh the state to show new active rule
					return refreshStateMsg{}
				})
			}
		case key.Matches(msg, m.keys.Help):
			m.help.ShowAll = !m.help.ShowAll
		}

	case stateRefreshedMsg:
		*m.state = msg.state
		// Refresh the active page's content with the new state
		return m, m.pages[m.activePage].Focus()

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
		Padding(0, 2).
		UnderlineSpaces(false).
		Underline(false)

	activeTab := lipgloss.NewStyle().
		Foreground(theme.Colors.Green).
		Bold(true).
		Padding(0, 2).
		UnderlineSpaces(false).
		Underline(false)

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
