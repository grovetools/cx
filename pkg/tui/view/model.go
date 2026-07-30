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
	"github.com/grovetools/core/tui/keymap"
	coretheme "github.com/grovetools/core/tui/theme"

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
	m, _ := NewWithStartPage("tree", workDir, rulesFile, cfg, false)
	return m
}

// NewWithStartPage is like New but allows the caller to select the
// initial page. It returns an error if startPage is the deprecated
// "repo" page.
func NewWithStartPage(startPage, workDir, rulesFile string, cfg *config.Config, hosted bool) (Model, error) {
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
		NewSetRulesPage(state, hosted),
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
	mergedKeys := newViewKeyMap(cfg)
	p := pager.NewAt(pages, pager.KeyMapFromBase(keys.Base), activePage)
	p.SetConfig(pager.Config{
		OuterPadding: [4]int{1, 2, 1, 2},
		FooterHeight: 1,
	})
	var watcher *RulesWatcher
	watcher, _ = NewRulesWatcher() // best-effort; nil watcher is handled gracefully

	return &pagerModel{
		pager: p,
		state: state,
		keys:  keys,
		// The single container-level `?` overlay renders the merged,
		// page-grouped cx-view keymap so it matches the registry export
		// exactly (pages implement Keys() but the container owns help).
		help: help.New(mergedKeys),
		// The which-key chord host is container-level: the pages are separate
		// Bubble Tea models behind the pager, so a single seam in this Update
		// is the only place every keystroke passes through before dispatch.
		// It is seeded with the MERGED t… namespace and narrowed per keypress
		// to the active page's members (see activeNamespaces).
		whichKey:         keymap.NewWhichKeyHost(cfg, mergedKeys.Namespaces()...),
		ExitForNvimEdit:  false,
		NvimEditPath:     "",
		nvimEmbedEnabled: nvimEmbedEnabled,
		cfg:              cfg,
		watcher:          watcher,
		hosted:           hosted,
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
	// whichKey is the shared chord/which-key mixin (core/tui/keymap). It owns
	// the t… namespace arming, the popup show-delay, and the bottom-anchored
	// overlay. Its Namespaces slice is re-pointed at the active page's members
	// on every key event.
	whichKey keymap.WhichKeyHost

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

	hosted bool // True when running inside groveterm; use SplitEditorRequestMsg
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

// inputCapturingPage is implemented by a cx page that can hold the keyboard in
// a text field or a raw sub-mode (the tree's incremental search, its confirm
// prompt and ruleset picker; the list's filter). While one of those is open the
// page owns every rune, so a namespace prefix must not arm (sign-off E3).
type inputCapturingPage interface {
	capturingInput() bool
}

// activeNamespaces returns the which-key namespaces the CURRENTLY FOCUSED page
// can actually dispatch. cx-view is multi-page and its t… members are split
// across two distinct keymaps — the list page owns `ts`, the tree page owns
// th/tc/ti — so a single union would advertise chords the focused page silently
// drops. Pages with no members (rules, set-rules, stats, suggestions) return
// nil, which also means no prefix ever arms on them; that is what keeps the
// rules page's flat `s`=select rule set reachable.
func (m *pagerModel) activeNamespaces() []keymap.Namespace {
	switch m.activePageName() {
	case "list":
		return m.keys.Namespaces()
	case "tree":
		return treeKeys.Namespaces()
	}
	return nil
}

// namespaceArmable is the page/pane guard that runs BEFORE ProcessChord (E3,
// "namespaces arm top-level only"). A prefix may arm only when the focused page
// owns chord members and is not currently capturing raw input. Callers OR this
// with whichKey.Armed() so that once a prefix IS armed the continuation key
// still reaches the chord engine instead of being stolen back by the page.
func (m *pagerModel) namespaceArmable() bool {
	if len(m.activeNamespaces()) == 0 {
		return false
	}
	if p, ok := m.pager.Active().(inputCapturingPage); ok && p.capturingInput() {
		return false
	}
	return true
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
			if m.hosted {
				path := msg.Path
				return m, func() tea.Msg {
					return embed.SplitEditorRequestMsg{Path: path, Focus: false}
				}
			}
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

	// dispatchMsg is what the pager (and through it the active page) finally
	// sees. The chord seam below rewrites it into the completed chord's literal
	// key ("ts", "tc", …) so the page's own flat key.Matches resolves it; every
	// other message passes through untouched.
	dispatchMsg := msg

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

		// Chord seam. Point the host at the focused page's members first, then
		// run the E3 page guard, then hand the key to ProcessChord. A completed
		// chord is re-synthesized as its literal key ("ts", "tc", …) and falls
		// through — the chord-only invariant (E4) makes Keys()[0] the chord, so
		// the flat key.Matches switches below and inside the pages resolve it.
		m.whichKey.Namespaces = m.activeNamespaces()
		if m.namespaceArmable() || m.whichKey.Armed() {
			res, matched, chordCmd := m.whichKey.ProcessChord(msg)
			switch res {
			case keymap.ChordPending:
				// A t… prefix is armed; chordCmd is the show-delay tick that
				// re-renders with the popup.
				return m, chordCmd
			case keymap.ChordConsumed:
				// esc dismissed the popup, or a stray key closed an armed menu.
				return m, nil
			case keymap.ChordMatched:
				// Re-synthesize the completed chord's canonical key. The
				// !key.Matches guard matters only if a binding ever retains a
				// flat key alongside its chord: Keys()[0] would then be the
				// chord and blind rewriting would destroy the flat press.
				// Every member here is chord-only (E4), so it is belt-and-braces.
				if len(matched.Keys()) > 0 && !key.Matches(msg, matched) {
					msg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(matched.Keys()[0])}
					dispatchMsg = msg
				}
			case keymap.ChordNone:
				// Not a chord — fall through to the flat dispatch below.
			}
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

				if m.hosted {
					path := rulesPath
					return m, func() tea.Msg {
						return embed.SplitEditorRequestMsg{Path: path, Focus: false}
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
				m.rulesTUI = rulestui.New(m.state.manager, m.cfg, m.hosted)
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
	m.pager, pagerCmd = m.pager.Update(dispatchMsg)
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

	// Composite the bottom-anchored which-key popup onto the assembled frame
	// while a t… prefix is armed past the show-delay; returns the frame
	// unchanged otherwise. Bottom-anchored, never centered — the delayed
	// keymap.WhichKeyShowMsg tick forces the re-render that reveals it.
	frame := m.pager.View()
	return m.whichKey.RenderOverlayAvail(frame, lipgloss.Width(frame), m.height, *coretheme.DefaultTheme)
}

// Close releases resources held by the model. Callers should invoke this
// when the Bubble Tea program exits to avoid leaking the file-watcher
// goroutine.
func (m *pagerModel) Close() {
	if m.watcher != nil {
		m.watcher.Close()
	}
}
