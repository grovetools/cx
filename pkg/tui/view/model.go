package view

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/grovetools/core/config"
	"github.com/grovetools/core/tui/components/help"
	"github.com/grovetools/core/tui/components/nvim"
	"github.com/grovetools/core/tui/components/pager"
	"github.com/grovetools/core/tui/embed"

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
// defaults. rulesFile, when non-empty, overrides the normal rules file
// discovery and scopes the view to a specific rules file path.
func New(workDir, rulesFile string, cfg *config.Config) Model {
	m, _ := NewWithStartPage("tree", workDir, rulesFile, cfg)
	return m
}

// NewWithStartPage is like New but allows the caller to select the
// initial page. It returns an error if startPage is the deprecated
// "repo" page.
func NewWithStartPage(startPage, workDir, rulesFile string, cfg *config.Config) (Model, error) {
	if startPage == "repo" {
		return nil, fmt.Errorf("repo view deprecated")
	}
	if startPage == "" {
		startPage = "rules"
	}

	// Create the manager synchronously so page Init commands (which run
	// concurrently via tea.Cmd goroutines) never see a nil manager.
	// NewManagerWithOverride is memoized, so refreshSharedStateCmd will
	// safely reuse the same instance from the cache.
	mgr := context.NewManagerWithOverride(workDir, rulesFile)
	state := &sharedState{workDir: workDir, rulesFileOverride: rulesFile, manager: mgr, loading: true}

	pages := []Page{
		NewRulesPage(state),
		NewStatsPage(state),
		NewSetRulesPage(state),
		NewListPage(state),
		NewTreePage(state),
		NewSuggestionsPage(state),
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

	keys := pagerKeys
	p := pager.NewAt(pages, pager.KeyMapFromBase(keys.Base), activePage)
	p.SetConfig(pager.Config{
		OuterPadding: [4]int{1, 2, 1, 2},
		FooterHeight: 1,
	})
	var watcher *RulesWatcher
	watcher, _ = NewRulesWatcher() // best-effort; nil watcher is handled gracefully

	return &pagerModel{
		pager:            p,
		state:            state,
		keys:             keys,
		help:             help.New(keys),
		ExitForNvimEdit:  false,
		NvimEditPath:     "",
		nvimEmbedEnabled: nvimEmbedEnabled,
		cfg:              cfg,
		watcher:          watcher,
	}, nil
}

type pagerModel struct {
	pager      pager.Model
	state      *sharedState
	currentSeq uint64 // monotonic counter for discarding stale refreshes
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

	// File watcher for the active rules file — triggers refresh on external edits.
	watcher *RulesWatcher
}

// dispatchRefresh increments the sequence counter and returns a
// refreshSharedStateCmd tagged with the new seq. Stale results
// (from earlier seq values) are discarded by the stateRefreshedMsg handler.
func (m *pagerModel) dispatchRefresh() tea.Cmd {
	m.currentSeq++
	return refreshSharedStateCmd(m.state.workDir, m.state.rulesFileOverride, m.currentSeq)
}

func (m *pagerModel) Init() tea.Cmd {
	cmds := []tea.Cmd{m.pager.Init(), m.dispatchRefresh()}
	if m.watcher != nil {
		cmds = append(cmds, m.watcher.NextEvent())
	}
	return tea.Batch(cmds...)
}

// activePageName returns the Name() of whichever cx page is currently
// focused in the pager. Used by key handlers that only apply to
// specific tabs (e.g. SelectRules only fires on the rules page).
func (m *pagerModel) activePageName() string {
	if p := m.pager.Active(); p != nil {
		return p.Name()
	}
	return ""
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
		return m, m.dispatchRefresh()
	case embed.UpdateContextScopeMsg:
		m.state.rulesFileOverride = msg.RulesFile
		// Don't set loading=true here — it causes a visible "Loading
		// context..." flash during sticky navigation. The stale content
		// stays visible until the refresh completes.
		return m, m.dispatchRefresh()
	case embed.FocusMsg:
		var cmd tea.Cmd
		m.pager, cmd = m.pager.Update(msg)
		return m, cmd
	case embed.BlurMsg:
		var cmd tea.Cmd
		m.pager, cmd = m.pager.Update(msg)
		return m, cmd
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
			return m, m.dispatchRefresh()
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
		case nvim.NvimExitMsg:
			// Nvim exited via :wq or :q — clean up and refresh.
			m.isEditing = false
			m.editorModel = nil
			return m, m.dispatchRefresh()
		case tea.KeyMsg:
			if msg.Type == tea.KeyCtrlC {
				_ = m.editorModel.Save()
				m.editorModel.Close()
				m.isEditing = false
				m.editorModel = nil
				return m, m.dispatchRefresh()
			}
			if msg.Type == tea.KeyTab || msg.Type == tea.KeyShiftTab {
				_ = m.editorModel.Save()
				m.editorModel.Close()
				m.isEditing = false
				m.editorModel = nil
				// Cycle the pager manually so the editor-exit Tab
				// keystroke advances to the next/prev tab, then
				// refresh shared state so the new tab sees any edits
				// that were just saved.
				var cycleCmd tea.Cmd
				if msg.Type == tea.KeyShiftTab {
					m.pager, cycleCmd = m.pager.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'['}})
				} else {
					m.pager, cycleCmd = m.pager.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{']'}})
				}
				return m, tea.Batch(m.dispatchRefresh(), cycleCmd)
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
		// Pager owns the chrome budget now: OuterPadding(1,2,1,2)
		// + tab bar (2) + footer slot (1) are subtracted via SubSize
		// internally, so we just forward the raw dimensions.
		var cmd tea.Cmd
		m.pager, cmd = m.pager.Update(msg)
		cmds = append(cmds, cmd)

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
		case key.Matches(msg, m.keys.Edit):
			if m.activePageName() == "rules" {
				rulesPath := m.state.rulesPath
				if rulesPath == "" {
					var err error
					rulesPath, err = m.state.manager.EnsureAndGetRulesPath()
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

				// Emit InlineEditRequestMsg so the host replaces
				// this panel's BSP node with an ephemeral editor
				// in-place. In standalone mode the self-handling
				// fallback below (case embed.InlineEditRequestMsg)
				// runs instead.
				path := rulesPath
				return m, func() tea.Msg {
					return embed.InlineEditRequestMsg{Path: path}
				}
			}
		case key.Matches(msg, m.keys.SelectRules):
			if m.activePageName() == "rules" {
				// Embed the rules picker model in-process instead of
				// shelling out to `cx rules`. The picker speaks the
				// embed contract so we can route messages and editor
				// requests through this host.
				m.rulesTUI = rulestui.New(m.state.manager, m.cfg)
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

	case rulesFileChangedMsg:
		var cmds []tea.Cmd
		cmds = append(cmds, m.dispatchRefresh())
		if m.watcher != nil {
			cmds = append(cmds, m.watcher.NextEvent())
		}
		return m, tea.Batch(cmds...)

	case stateRefreshedMsg:
		if msg.seq < m.currentSeq {
			return m, nil // Discard stale refresh from earlier request
		}
		*m.state = msg.state
		// Update the watcher target if the rules path changed.
		if m.watcher != nil && msg.state.rulesPath != "" {
			m.watcher.SetTarget(msg.state.rulesPath)
		}
		// Forward to the pager so active pages can react to the new state.
		var pagerCmd tea.Cmd
		m.pager, pagerCmd = m.pager.Update(msg)
		return m, pagerCmd

	case refreshStateMsg, embed.EditFinishedMsg, embed.SplitEditorClosedMsg:
		m.state.loading = true
		return m, m.dispatchRefresh()

	case embed.EditRequestMsg:
		// Standalone fallback: when not hosted, EditRequestMsg is not
		// intercepted by WrapPanelCmd, so we self-handle by launching
		// the editor and returning EditFinishedMsg on completion.
		editor := os.Getenv("EDITOR")
		if editor == "" {
			editor = "vi"
		}
		editCmd := exec.Command(editor, msg.Path)
		return m, tea.ExecProcess(editCmd, func(err error) tea.Msg {
			return embed.EditFinishedMsg{Err: err}
		})

	case embed.InlineEditRequestMsg:
		// Standalone fallback: same as EditRequestMsg — no host to
		// perform an in-place BSP swap, so launch the editor directly.
		editor := os.Getenv("EDITOR")
		if editor == "" {
			editor = "vi"
		}
		editCmd := exec.Command(editor, msg.Path)
		return m, tea.ExecProcess(editCmd, func(err error) tea.Msg {
			return embed.EditFinishedMsg{Err: err}
		})
	}

	var pagerCmd tea.Cmd
	m.pager, pagerCmd = m.pager.Update(msg)
	cmds = append(cmds, pagerCmd)

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

	// Editor mode bypasses the pager's own View() because the
	// nvim viewport replaces the body entirely, but we still
	// want the pager's tab bar as context.
	if m.isEditing && m.editorModel != nil {
		bodyContent := m.pager.RenderTabBar() + "\n\n" + m.editorModel.View()
		return lipgloss.NewStyle().Padding(0, 2).Render(bodyContent)
	}

	// Build footer and delegate to pager which pins it at the
	// bottom of the pane. The pager's OuterPadding provides the
	// horizontal indent so no extra padding is needed here.
	m.pager.SetFooter(m.help.View())

	return m.pager.View()
}

// Close releases resources held by the model. Callers should invoke this
// when the Bubble Tea program exits to avoid leaking the file-watcher
// goroutine.
func (m *pagerModel) Close() {
	if m.watcher != nil {
		m.watcher.Close()
	}
}
