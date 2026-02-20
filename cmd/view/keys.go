package view

import (
	"github.com/charmbracelet/bubbles/key"
	"github.com/grovetools/core/tui/keymap"
)

// pagerKeyMap defines the key bindings for the main pager view.
type pagerKeyMap struct {
	keymap.Base
	NextPage     key.Binding
	PrevPage     key.Binding
	Edit         key.Binding
	SelectRules  key.Binding
	Exclude      key.Binding
	ExcludeDir   key.Binding
	ToggleSort   key.Binding
	Refresh      key.Binding
	GotoTop      key.Binding
	GotoBottom   key.Binding
	HalfPageUp   key.Binding
	HalfPageDown key.Binding
	FoldPrefix   key.Binding // 'z' prefix for fold commands
}

// ShortHelp returns keybindings to be shown in the footer.
func (k pagerKeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Help, k.ToggleSort, k.NextPage, k.PrevPage, k.Edit, k.SelectRules, k.Quit}
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

// Sections returns grouped sections of key bindings for the full help view.
// Only includes sections that the pager actually implements.
func (k pagerKeyMap) Sections() []keymap.Section {
	// Start with base sections we actually use
	nav := k.Base.NavigationSection()
	// Customize navigation to only show bindings we implement
	nav.Bindings = []key.Binding{k.Up, k.Down, k.GotoTop, k.GotoBottom, k.HalfPageUp, k.HalfPageDown}

	return []keymap.Section{
		nav,
		{
			Name:     "Pages",
			Bindings: []key.Binding{k.NextPage, k.PrevPage},
		},
		{
			Name:     "Rules",
			Bindings: []key.Binding{k.Edit, k.SelectRules, k.Exclude, k.ExcludeDir, k.Refresh},
		},
		{
			Name:     "Display",
			Bindings: []key.Binding{k.ToggleSort},
		},
		k.Base.FoldSection(),
		k.Base.SystemSection(),
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
	ToggleSort: key.NewBinding(
		key.WithKeys("t"),
		key.WithHelp("t", "toggle sort"),
	),
	Exclude: key.NewBinding(
		key.WithKeys("x"),
		key.WithHelp("x", "exclude file"),
	),
	ExcludeDir: key.NewBinding(
		key.WithKeys("X"),
		key.WithHelp("X", "exclude dir"),
	),
	Refresh: key.NewBinding(
		key.WithKeys("r"),
		key.WithHelp("r", "refresh"),
	),
	FoldPrefix: key.NewBinding(
		key.WithKeys("z"),
		key.WithHelp("za/zo/zc/zR/zM", "fold operations"),
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

// Sections returns grouped sections of key bindings for the full help view.
// Only includes sections that the stats page actually implements.
func (k statsKeyMap) Sections() []keymap.Section {
	return []keymap.Section{
		{
			Name:     "Navigation",
			Bindings: []key.Binding{k.Up, k.Down, k.SwitchFocus, k.GotoTop, k.GotoBottom, k.HalfPageUp, k.HalfPageDown},
		},
		{
			Name:     "Actions",
			Bindings: []key.Binding{k.Exclude, k.Refresh},
		},
		k.Base.SystemSection(),
	}
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
	return []key.Binding{k.Help, k.Quit}
}

func (k treeViewKeyMap) FullHelp() [][]key.Binding {
	// Replicates the content from the old renderHelp function in a structured format
	return [][]key.Binding{
		{
			key.NewBinding(key.WithHelp("", "Navigation:")),
			key.NewBinding(key.WithKeys("up", "down", "j", "k"), key.WithHelp("up/down, j/k", "Move up/down")),
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
			key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", "Next Page")),
			key.NewBinding(key.WithKeys("H"), key.WithHelp("H", "Toggle gitignored")),
			key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "Refresh view")),
			key.NewBinding(key.WithKeys("q"), key.WithHelp("q", "Quit")),
			key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "Toggle this help")),
		},
	}
}

// Sections returns grouped sections of key bindings for the full help view.
// Only includes sections that the tree view actually implements.
func (k treeViewKeyMap) Sections() []keymap.Section {
	// Customize navigation for tree view
	nav := k.Base.NavigationSection()
	nav.Bindings = []key.Binding{k.Up, k.Down, k.Top, k.Bottom, k.PageUp, k.PageDown}

	return []keymap.Section{
		nav,
		{
			Name: "Tree",
			Bindings: []key.Binding{
				key.NewBinding(key.WithKeys("enter", "space"), key.WithHelp("enter/space", "toggle expand")),
			},
		},
		{
			Name: "Context",
			Bindings: []key.Binding{
				key.NewBinding(key.WithKeys("h"), key.WithHelp("h", "toggle hot")),
				key.NewBinding(key.WithKeys("c"), key.WithHelp("c", "toggle cold")),
				key.NewBinding(key.WithKeys("x"), key.WithHelp("x", "toggle exclude")),
				key.NewBinding(key.WithKeys("H"), key.WithHelp("H", "toggle gitignored")),
			},
		},
		k.Base.SearchSection(),
		k.Base.FoldSection(),
		{
			Name: "Other",
			Bindings: []key.Binding{
				key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "refresh")),
				key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", "next page")),
			},
		},
		k.Base.SystemSection(),
	}
}

var (
	treeKeys = treeViewKeyMap{Base: keymap.NewBase()}
)
