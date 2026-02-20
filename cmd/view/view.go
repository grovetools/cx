package view

import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/grovetools/core/config"
	"github.com/grovetools/core/tui/components/help"
	"github.com/grovetools/core/tui/components/nvim"
	core_theme "github.com/grovetools/core/tui/theme"
	"github.com/grovetools/core/util/delegation"
	"github.com/grovetools/cx/pkg/context"
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
	cmd.Flags().StringVarP(&startPage, "page", "p", "tree", "The page to open on startup (tree, rules, stats, list)")
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

	// State for embedded editor
	editorModel      *nvim.Model
	isEditing        bool
	nvimEmbedEnabled bool
	editingFilePath  string
}

func newPagerModel(startPage string) (*pagerModel, error) {
	if startPage == "repo" {
		fmt.Println("The 'repo' view is deprecated and has been removed.")
		fmt.Println("Please use 'gmux sessionize' (or 'gmux sz') for a more powerful workspace view.")
		return nil, fmt.Errorf("repo view deprecated")
	}

	state := &sharedState{loading: true}

	pages := []Page{
		NewTreePage(state),
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

	// Read config to see if nvim embed is enabled
	nvimEmbedEnabled := false
	cfg, err := config.LoadFrom(".")
	// It's okay if config doesn't exist or fails to load, we just use the default (false)
	if err == nil && cfg != nil && cfg.TUI != nil && cfg.TUI.NvimEmbed != nil {
		nvimEmbedEnabled = cfg.TUI.NvimEmbed.UserConfig
	}

	return &pagerModel{
		pages:            pages,
		activePage:       activePage,
		state:            state,
		keys:             pagerKeys,
		help:             help.New(pagerKeys),
		exitForNvimEdit:  false,
		nvimEditPath:     "",
		nvimEmbedEnabled: nvimEmbedEnabled,
	}, nil
}

func (m *pagerModel) Init() tea.Cmd {
	return tea.Batch(m.pages[m.activePage].Init(), refreshSharedStateCmd())
}

func (m *pagerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	if m.isEditing && m.editorModel != nil {
		switch msg := msg.(type) {
		case tea.KeyMsg:
			if msg.Type == tea.KeyCtrlC {
				m.editorModel.Save()
				m.editorModel.Close()
				m.isEditing = false
				m.editorModel = nil
				return m, refreshSharedStateCmd()
			}
			if msg.Type == tea.KeyTab || msg.Type == tea.KeyShiftTab {
				m.editorModel.Save()
				m.editorModel.Close()
				m.isEditing = false
				m.editorModel = nil
				if msg.Type == tea.KeyShiftTab {
					m.prevPage()
				} else {
					m.nextPage()
				}
				return m, tea.Batch(refreshSharedStateCmd(), m.pages[m.activePage].Focus())
			}
		case tea.WindowSizeMsg:
			var cmd tea.Cmd
			if m.editorModel != nil {
				editorHeight := msg.Height - 10
				editorWidth := msg.Width - 4
				if editorHeight < 10 {
					editorHeight = 10
				}
				if editorWidth < 20 {
					editorWidth = 20
				}
				cmd = m.editorModel.SetSize(editorWidth, editorHeight)
			}
			return m, cmd
		}

		updatedModel, cmd := m.editorModel.Update(msg)
		if editorModel, ok := updatedModel.(nvim.Model); ok {
			*m.editorModel = editorModel
		}
		return m, cmd
	}

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.help.SetSize(m.width, m.height)
		pageHeight := m.height - 6
		for _, p := range m.pages {
			p.SetSize(m.width, pageHeight)
		}

	case tea.KeyMsg:
		// If help is showing, let it handle all keys except quit
		if m.help.ShowAll {
			if key.Matches(msg, m.keys.Quit) {
				return m, tea.Quit
			}
			var cmd tea.Cmd
			m.help, cmd = m.help.Update(msg)
			return m, cmd
		}

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
			if m.pages[m.activePage].Name() == "rules" {
				rulesPath := m.state.rulesPath
				if rulesPath == "" {
					mgr := context.NewManager("")
					var err error
					rulesPath, err = mgr.EnsureAndGetRulesPath()
					if err != nil {
						m.state.err = fmt.Errorf("failed to get rules path: %w", err)
						return m, nil
					}
				}

				if m.nvimEmbedEnabled {
					m.editingFilePath = rulesPath
					editorHeight := m.height - 10
					editorWidth := m.width - 4
					if editorHeight < 10 {
						editorHeight = 10
					}
					if editorWidth < 20 {
						editorWidth = 20
					}

					opts := nvim.Options{
						Width:      editorWidth,
						Height:     editorHeight,
						FileToOpen: rulesPath,
						UseConfig:  true,
					}
					editorModel, err := nvim.New(opts)
					if err != nil {
						m.state.err = fmt.Errorf("failed to start nvim: %w", err)
						return m, nil
					}

					m.editorModel = &editorModel
					m.editorModel.SetFocused(true)
					m.isEditing = true
					return m, m.editorModel.Init()
				}

				if os.Getenv("GROVE_NVIM_PLUGIN") == "true" {
					m.exitForNvimEdit = true
					m.nvimEditPath = rulesPath
					return m, tea.Quit
				}

				mgr := context.NewManager("")
				editorCmd, err := mgr.EditRulesCmd()
				if err != nil {
					m.state.err = fmt.Errorf("failed to prepare editor command: %w", err)
					return m, nil
				}
				return m, tea.ExecProcess(editorCmd, func(err error) tea.Msg {
					return refreshStateMsg{}
				})
			}
		case key.Matches(msg, m.keys.SelectRules):
			if m.pages[m.activePage].Name() == "rules" {
				cxCmd := delegation.Command("cx", "rules")
				cxCmd.Stdin = os.Stdin
				cxCmd.Stdout = os.Stdout
				cxCmd.Stderr = os.Stderr
				return m, tea.ExecProcess(cxCmd, func(err error) tea.Msg {
					return refreshStateMsg{}
				})
			}
		case key.Matches(msg, m.keys.Help):
			m.help.Toggle()
			return m, nil
		}

	case stateRefreshedMsg:
		*m.state = msg.state
		return m, m.pages[m.activePage].Focus()

	case refreshStateMsg:
		m.state.loading = true
		return m, refreshSharedStateCmd()
	}

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

	// Show full help overlay if requested
	if m.help.ShowAll {
		return m.help.View()
	}

	header := m.renderTabs()

	var pageContent string
	if m.isEditing && m.editorModel != nil {
		pageContent = "\n" + m.editorModel.View()
	} else {
		pageContent = m.pages[m.activePage].View()
	}

	var footer string
	if m.isEditing && m.editorModel != nil {
		footer = ""
	} else {
		footer = m.help.View()
	}

	fullContent := lipgloss.JoinVertical(lipgloss.Left, header, pageContent, footer)

	if m.isEditing {
		return lipgloss.NewStyle().Padding(0, 2).Render(fullContent)
	}
	return lipgloss.NewStyle().Padding(1, 2).Render(fullContent)
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

	// First tab styles with no left padding to align with content
	inactiveFirstTab := lipgloss.NewStyle().
		Foreground(theme.Colors.MutedText).
		PaddingRight(2).
		UnderlineSpaces(false).
		Underline(false)

	activeFirstTab := lipgloss.NewStyle().
		Foreground(theme.Colors.Green).
		Bold(true).
		PaddingRight(2).
		UnderlineSpaces(false).
		Underline(false)

	var tabs []string
	for i, p := range m.pages {
		var style lipgloss.Style
		if i == 0 {
			// First tab: no left padding
			if i == m.activePage {
				style = activeFirstTab
			} else {
				style = inactiveFirstTab
			}
		} else {
			// Other tabs: normal padding
			if i == m.activePage {
				style = activeTab
			} else {
				style = inactiveTab
			}
		}
		tabs = append(tabs, style.Render(strings.ToUpper(p.Name())))
	}

	return lipgloss.JoinHorizontal(lipgloss.Top, tabs...)
}
