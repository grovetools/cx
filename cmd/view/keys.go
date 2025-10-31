package view

import (
	"github.com/charmbracelet/bubbles/key"
	"github.com/mattsolo1/grove-core/tui/keymap"
)

// pagerKeyMap defines the key bindings for the main pager view.
type pagerKeyMap struct {
	keymap.Base
	NextPage    key.Binding
	PrevPage    key.Binding
	Edit        key.Binding
	SelectRules key.Binding
}

// ShortHelp returns keybindings to be shown in the footer.
func (k pagerKeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Help, k.NextPage, k.PrevPage, k.Edit, k.SelectRules, k.Quit}
}

// FullHelp returns keybindings for the expanded help view.
func (k pagerKeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{
			key.NewBinding(key.WithHelp("", "Navigation:")),
			k.NextPage,
			k.PrevPage,
			k.Edit,
			k.SelectRules,
			k.Quit,
			k.Help,
		},
	}
}

var pagerKeys = pagerKeyMap{
	Base: keymap.NewBase(),
	NextPage: key.NewBinding(
		key.WithKeys("tab"),
		key.WithHelp("tab", "next page"),
	),
	PrevPage: key.NewBinding(
		key.WithKeys("shift+tab"),
		key.WithHelp("shift+tab", "prev page"),
	),
	Edit: key.NewBinding(
		key.WithKeys("e"),
		key.WithHelp("e", "edit rules"),
	),
	SelectRules: key.NewBinding(
		key.WithKeys("s"),
		key.WithHelp("s", "select rule set"),
	),
}

// statsKeyMap defines the key bindings for the interactive stats page.
type statsKeyMap struct {
	keymap.Base
	SwitchFocus key.Binding
	Exclude     key.Binding
	Refresh     key.Binding
	GotoTop     key.Binding
	GotoBottom  key.Binding
	HalfPageUp  key.Binding
	HalfPageDown key.Binding
}

// ShortHelp returns keybindings to be shown in the footer.
func (k statsKeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Help, k.SwitchFocus, k.Exclude, k.Refresh, k.Quit}
}

// FullHelp is not used for this page but is required by the interface.
func (k statsKeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{}
}

var statsKeys = statsKeyMap{
	Base: keymap.NewBase(),
	SwitchFocus: key.NewBinding(
		key.WithKeys("s", "left", "right"),
		key.WithHelp("s/←/→", "switch list"),
	),
	Exclude: key.NewBinding(
		key.WithKeys("x"),
		key.WithHelp("x", "exclude"),
	),
	Refresh: key.NewBinding(
		key.WithKeys("r"),
		key.WithHelp("r", "refresh"),
	),
	GotoTop: key.NewBinding(
		key.WithKeys("g"),
		key.WithHelp("gg", "go to top"),
	),
	GotoBottom: key.NewBinding(
		key.WithKeys("G"),
		key.WithHelp("G", "go to bottom"),
	),
	HalfPageUp: key.NewBinding(
		key.WithKeys("ctrl+u"),
		key.WithHelp("ctrl-u", "half page up"),
	),
	HalfPageDown: key.NewBinding(
		key.WithKeys("ctrl+d"),
		key.WithHelp("ctrl-d", "half page down"),
	),
}

// --- Keymaps from old view.go (to be used in Job 2 & 3) ---

type treeViewKeyMap struct {
	keymap.Base
}

func (k treeViewKeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Help, key.NewBinding(key.WithHelp("tab", "repos")), k.Quit}
}

func (k treeViewKeyMap) FullHelp() [][]key.Binding {
	// Replicates the content from the old renderHelp function in a structured format
	return [][]key.Binding{
		{
			key.NewBinding(key.WithHelp("", "Navigation:")),
			key.NewBinding(key.WithKeys("up", "down", "j", "k"), key.WithHelp("↑/↓, j/k", "Move up/down")),
			key.NewBinding(key.WithKeys("enter", " "), key.WithHelp("enter, space", "Toggle expand")),
			key.NewBinding(key.WithKeys("g"), key.WithHelp("gg", "Go to top")),
			key.NewBinding(key.WithKeys("G"), key.WithHelp("G", "Go to bottom")),
			key.NewBinding(key.WithKeys("ctrl+d", "ctrl+u"), key.WithHelp("ctrl-d/u", "Page down/up")),
			key.NewBinding(key.WithHelp("", "Folding (vim-style):")),
			key.NewBinding(key.WithKeys("z"), key.WithHelp("za", "Toggle fold")),
			key.NewBinding(key.WithKeys("z"), key.WithHelp("zo/zc", "Open/close fold")),
			key.NewBinding(key.WithKeys("z"), key.WithHelp("zR/zM", "Open/close all")),
			key.NewBinding(key.WithHelp("", "Search:")),
			key.NewBinding(key.WithKeys("/"), key.WithHelp("/", "Search for files")),
			key.NewBinding(key.WithKeys("n", "N"), key.WithHelp("n/N", "Next/prev result")),
		},
		{
			key.NewBinding(key.WithHelp("", "Actions:")),
			key.NewBinding(key.WithKeys("h"), key.WithHelp("h", "Toggle hot context")),
			key.NewBinding(key.WithKeys("c"), key.WithHelp("c", "Toggle cold context")),
			key.NewBinding(key.WithKeys("x"), key.WithHelp("x", "Toggle exclude")),
			key.NewBinding(key.WithKeys("tab", "A"), key.WithHelp("tab/A", "Repository view")),
			key.NewBinding(key.WithKeys("p"), key.WithHelp("p", "Toggle pruning")),
			key.NewBinding(key.WithKeys("H"), key.WithHelp("H", "Toggle gitignored")),
			key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "Refresh view")),
			key.NewBinding(key.WithKeys("q"), key.WithHelp("q", "Quit")),
			key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "Toggle this help")),
		},
	}
}

type repoSelectKeyMap struct {
	keymap.Base
	FocusEcosystem key.Binding
	ClearFocus     key.Binding
	Hot            key.Binding
	Cold           key.Binding
	Exclude        key.Binding
	ViewInTree     key.Binding
}

func (k repoSelectKeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Help, key.NewBinding(key.WithHelp("tab", "tree")), k.Quit}
}

func (k repoSelectKeyMap) FullHelp() [][]key.Binding {
	// Replicates the content from the old renderRepoHelp function
	return [][]key.Binding{
		{
			key.NewBinding(key.WithHelp("", "Navigation:")),
			key.NewBinding(key.WithKeys("up", "down", "j", "k"), key.WithHelp("↑/↓, j/k", "Move up/down")),
			key.NewBinding(key.WithKeys("ctrl+u", "ctrl+d"), key.WithHelp("ctrl-u/d", "Half page up/down")),
			key.NewBinding(key.WithKeys("g", "G"), key.WithHelp("g/G", "Go to top/bottom")),
			key.NewBinding(key.WithKeys("/"), key.WithHelp("/", "Filter repositories")),
			key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "Clear filter")),
			key.NewBinding(key.WithHelp("", "Folding (vim-style):")),
			key.NewBinding(key.WithKeys("z"), key.WithHelp("zo/zc", "Open/close fold")),
			key.NewBinding(key.WithKeys("z"), key.WithHelp("zR/zM", "Open/close all")),
			key.NewBinding(key.WithHelp("", "Context Actions:")),
			key.NewBinding(key.WithKeys("h"), key.WithHelp("h", "Toggle hot context")),
			key.NewBinding(key.WithKeys("c"), key.WithHelp("c", "Toggle cold context")),
			key.NewBinding(key.WithKeys("x"), key.WithHelp("x", "Toggle exclude")),
			key.NewBinding(key.WithKeys("a"), key.WithHelp("a", "Add/remove from tree")),
		},
		{
			key.NewBinding(key.WithHelp("", "Repository Actions:")),
			key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "Refresh list")),
			key.NewBinding(key.WithKeys("A"), key.WithHelp("A", "Audit repository")),
			key.NewBinding(key.WithKeys("R"), key.WithHelp("R", "View audit report")),
			key.NewBinding(key.WithHelp("", "View Control:")),
			key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", "Switch to tree view")),
			key.NewBinding(key.WithKeys("q"), key.WithHelp("q", "Quit")),
			key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "Toggle this help")),
		},
	}
}

var (
	treeKeys = treeViewKeyMap{Base: keymap.NewBase()}
	repoKeys = repoSelectKeyMap{
		Base:           keymap.NewBase(),
		FocusEcosystem: key.NewBinding(key.WithKeys("@"), key.WithHelp("@", "focus ecosystem")),
		ClearFocus:     key.NewBinding(key.WithKeys("ctrl+g"), key.WithHelp("ctrl+g", "clear focus")),
		Hot:            key.NewBinding(key.WithKeys("h"), key.WithHelp("h", "toggle hot")),
		Cold:           key.NewBinding(key.WithKeys("c"), key.WithHelp("c", "toggle cold")),
		Exclude:        key.NewBinding(key.WithKeys("x"), key.WithHelp("x", "toggle exclude")),
		ViewInTree:     key.NewBinding(key.WithKeys("a"), key.WithHelp("a", "view in tree")),
	}
)
