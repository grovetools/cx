package view

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/grovetools/core/config"
	"github.com/grovetools/core/tui/components/help"
	"github.com/grovetools/core/tui/components/nvim"
	"github.com/grovetools/core/tui/embed"
	core_theme "github.com/grovetools/core/tui/theme"
	"github.com/grovetools/cx/pkg/context"
	rulestui "github.com/grovetools/cx/pkg/tui/rules"
)

// Model is the embeddable cx view meta-panel. It hosts the 4 pages
// (tree, rules, stats, list) with internal page navigation and a
// shared state that is reactively refreshed on workspace changes.
//
// Host applications hold a *Model (exported via the Model type alias)
// and route Bubble Tea messages to it through Update.
type Model = *pagerModel

// New constructs an embeddable cx view model rooted at workDir. cfg
// supplies user keybindings and nvim-embed settings; pass nil to load
// defaults. startPage selects the initial page ("tree", "rules",
// "stats", "list"); empty defaults to "tree".
func New(workDir string, cfg *config.Config) Model {
	m, _ := NewWithStartPage("tree", workDir, cfg)
	return m
}

// NewWithStartPage is like New but allows the caller to select the
// initial page. It returns an error if startPage is the deprecated
// "repo" page.
func NewWithStartPage(startPage, workDir string, cfg *config.Config) (Model, error) {
	if startPage == "repo" {
		return nil, fmt.Errorf("repo view deprecated")
	}
	if startPage == "" {
		startPage = "tree"
	}

	state := &sharedState{workDir: workDir, loading: true}

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

	if cfg == nil {
		cfg, _ = config.LoadFrom(".")
	}
	nvimEmbedEnabled := false
	if cfg != nil && cfg.TUI != nil && cfg.TUI.NvimEmbed != nil {
		nvimEmbedEnabled = cfg.TUI.NvimEmbed.UserConfig
	}

	return &pagerModel{
		pages:            pages,
		activePage:       activePage,
		state:            state,
		keys:             pagerKeys,
		help:             help.New(pagerKeys),
		ExitForNvimEdit:  false,
		NvimEditPath:     "",
		nvimEmbedEnabled: nvimEmbedEnabled,
		cfg:              cfg,
	}, nil
}

type pagerModel struct {
	pages      []Page
	activePage int
	state      *sharedState
	width      int
	height     int
	keys       pagerKeyMap
	help       help.Model

	// Exported Neovim IPC state so standalone CLI wrappers can detect
	// a "quit to let nvim edit this file" exit and print the
	// EDIT_FILE: marker.
	ExitForNvimEdit bool
	NvimEditPath    string

	// State for embedded editor
	editorModel      *nvim.Model
	isEditing        bool
	nvimEmbedEnabled bool
	editingFilePath  string

	// Embedded rules picker (replaces the old `cx rules` subprocess delegation).
	cfg       *config.Config
	rulesTUI  rulestui.Model
	showRules bool
}

func (m *pagerModel) Init() tea.Cmd {
	return tea.Batch(m.pages[m.activePage].Init(), refreshSharedStateCmd(m.state.workDir))
}

func (m *pagerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	// Host-level embed contract messages take priority. A workspace
	// switch rebuilds the shared state so every page reacts. Focus
	// and blur are forwarded to the active page if it implements
	// them; pages without a Focus action simply no-op.
	switch msg := msg.(type) {
	case embed.SetWorkspaceMsg:
		if msg.Node != nil {
			m.state.workDir = msg.Node.Path
		}
		m.state.loading = true
		return m, refreshSharedStateCmd(m.state.workDir)
	case embed.FocusMsg:
		return m, m.pages[m.activePage].Focus()
	case embed.BlurMsg:
		m.pages[m.activePage].Blur()
		return m, nil
	}

	// When the embedded rules picker is active, give it first crack at the
	// message and translate the embed contract messages it emits.
	if m.showRules && m.rulesTUI != nil {
		switch msg := msg.(type) {
		case tea.WindowSizeMsg:
			m.width = msg.Width
			m.height = msg.Height
		case embed.CloseRequestMsg, embed.CloseConfirmMsg, embed.DoneMsg:
			m.showRules = false
			m.rulesTUI = nil
			return m, refreshSharedStateCmd(m.state.workDir)
		case embed.EditRequestMsg:
			editor := os.Getenv("EDITOR")
			if editor == "" {
				editor = "vi"
			}
			editCmd := exec.Command(editor, msg.Path)
			return m, tea.ExecProcess(editCmd, func(err error) tea.Msg {
				return embed.EditFinishedMsg{Err: err}
			})
		}
		updated, cmd := m.rulesTUI.Update(msg)
		if rm, ok := updated.(rulestui.Model); ok {
			m.rulesTUI = rm
		}
		return m, cmd
	}

	if m.isEditing && m.editorModel != nil {
		switch msg := msg.(type) {
		case tea.KeyMsg:
			if msg.Type == tea.KeyCtrlC {
				m.editorModel.Save()
				m.editorModel.Close()
				m.isEditing = false
				m.editorModel = nil
				return m, refreshSharedStateCmd(m.state.workDir)
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
				return m, tea.Batch(refreshSharedStateCmd(m.state.workDir), m.pages[m.activePage].Focus())
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
					mgr := context.NewManager(m.state.workDir)
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
					m.ExitForNvimEdit = true
					m.NvimEditPath = rulesPath
					return m, tea.Quit
				}

				mgr := context.NewManager(m.state.workDir)
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
				// Embed the rules picker model in-process instead of
				// shelling out to `cx rules`. The picker speaks the
				// embed contract so we can route messages and editor
				// requests through this host.
				m.rulesTUI = rulestui.New(m.state.workDir, m.cfg)
				m.showRules = true
				return m, tea.Batch(
					m.rulesTUI.Init(),
					func() tea.Msg {
						return tea.WindowSizeMsg{Width: m.width, Height: m.height}
					},
				)
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
		return m, refreshSharedStateCmd(m.state.workDir)
	}

	activePage, pageCmd := m.pages[m.activePage].Update(msg)
	m.pages[m.activePage] = activePage
	cmds = append(cmds, pageCmd)

	return m, tea.Batch(cmds...)
}

func (m *pagerModel) View() string {
	if m.showRules && m.rulesTUI != nil {
		return m.rulesTUI.View()
	}
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
