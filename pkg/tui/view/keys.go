package view

import (
	"github.com/charmbracelet/bubbles/key"
	"github.com/grovetools/core/config"
	"github.com/grovetools/core/tui/keymap"
)

// disable turns off a set of bindings in place. A disabled binding is skipped
// by the keymap audit, never rendered in help, and never matched by
// key.Matches — so it drops cleanly out of a TUI's advertised interface.
func disable(bindings ...*key.Binding) {
	for _, b := range bindings {
		b.SetEnabled(false)
	}
}

// pagerKeyMap defines the key bindings for the main pager view (the list page)
// and the container-level actions (Edit/SelectRules/Help/Quit). Navigation,
// folding, refresh, and tab-cycling all come from the embedded Base so the keys
// are truthful chords (gg, zo/zc/za/zR/zM) routed through keymap.SequenceState.
type pagerKeyMap struct {
	keymap.Base
	Edit        key.Binding
	SelectRules key.Binding
	Exclude     key.Binding
	ExcludeDir  key.Binding
	ToggleSort  key.Binding
}

// ShortHelp returns keybindings to be shown in the footer.
func (k pagerKeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Help, k.ToggleSort, k.NextTab, k.PrevTab, k.Edit, k.SelectRules, k.Quit}
}

// Compile-time guard: satisfies the sectioned help/audit contract (value receiver).
var _ keymap.SectionedKeyMap = pagerKeyMap{}

// Sections returns the grouped key bindings the pager/list page actually
// implements. Nav/Fold/System/Pages come from Base; the pager component owns
// tab-cycling via NextTab/PrevTab/Tab1..9 (see model.go pager.KeyMapFromBase).
func (k pagerKeyMap) Sections() []keymap.Section {
	return []keymap.Section{
		keymap.NavigationSection(k.Up, k.Down, k.PageUp, k.PageDown, k.Top, k.Bottom),
		keymap.NewSection("Pages", k.NextTab, k.PrevTab, k.Tab1, k.Tab2, k.Tab3, k.Tab4, k.Tab5, k.Tab6, k.Tab7, k.Tab8, k.Tab9),
		keymap.NewSection(keymap.SectionRules, k.Edit, k.SelectRules, k.Exclude, k.ExcludeDir, k.Base.Refresh),
		keymap.NewSection("Display", k.ToggleSort),
		k.Base.FoldSection(),
		k.Base.SystemSection(),
	}
}

func newPagerKeyMap(cfg *config.Config) pagerKeyMap {
	km := pagerKeyMap{
		Base: keymap.Load(cfg, "cx.view"),
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
	}
	keymap.ApplyTUIOverrides(cfg, "cx", "view", &km)

	// Disable every Base binding this view does not honor. Kept enabled:
	// Up/Down/PageUp/PageDown/Top/Bottom (nav), Refresh (ctrl+r), Help, Quit,
	// NextTab/PrevTab/Tab1..9 (pager tab nav, consumed via KeyMapFromBase),
	// and the five Fold* chords (routed through the sequence engine).
	disable(
		&km.Base.Left, &km.Base.Right, &km.Base.Home, &km.Base.End,
		&km.Base.Confirm, &km.Base.Cancel, &km.Base.Back, &km.Base.Edit,
		&km.Base.Delete, &km.Base.Yank, &km.Base.Rename, &km.Base.CopyPath,
		&km.Base.Search, &km.Base.SearchNext, &km.Base.SearchPrev,
		&km.Base.ClearSearch, &km.Base.Grep,
		&km.Base.SwitchView, &km.Base.FocusNext, &km.Base.FocusPrev,
		&km.Base.TogglePreview,
		&km.Base.Select, &km.Base.SelectAll, &km.Base.SelectNone,
	)
	return km
}

var pagerKeys = func() pagerKeyMap {
	cfg, _ := config.LoadDefault()
	return newPagerKeyMap(cfg)
}()

// statsKeyMap defines the key bindings for the interactive stats page.
type statsKeyMap struct {
	keymap.Base
	SwitchFocus key.Binding
	Exclude     key.Binding
}

// ShortHelp returns keybindings to be shown in the footer.
func (k statsKeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Help, k.SwitchFocus, k.Exclude, k.Base.Refresh, k.Quit}
}

// Compile-time guard: satisfies the sectioned help/audit contract (value receiver).
var _ keymap.SectionedKeyMap = statsKeyMap{}

// Sections returns the grouped key bindings the stats page actually implements.
func (k statsKeyMap) Sections() []keymap.Section {
	return []keymap.Section{
		keymap.NavigationSection(k.Up, k.Down, k.SwitchFocus, k.PageUp, k.PageDown, k.Top, k.Bottom),
		keymap.ActionsSection(k.Exclude, k.Base.Refresh),
		k.Base.SystemSection(),
	}
}

func newStatsKeyMap(cfg *config.Config) statsKeyMap {
	km := statsKeyMap{
		Base: keymap.Load(cfg, "cx.view"),
		SwitchFocus: key.NewBinding(
			key.WithKeys("s", "left", "right"),
			key.WithHelp("s/←/→", "switch list"),
		),
		Exclude: key.NewBinding(
			key.WithKeys("x"),
			key.WithHelp("x", "exclude"),
		),
	}
	keymap.ApplyTUIOverrides(cfg, "cx", "view", &km)

	// Kept enabled: Up/Down/PageUp/PageDown/Top/Bottom (nav), Refresh, Help,
	// Quit. The stats page has no folds, tabs, or search of its own.
	disable(
		&km.Base.Left, &km.Base.Right, &km.Base.Home, &km.Base.End,
		&km.Base.Confirm, &km.Base.Cancel, &km.Base.Back, &km.Base.Edit,
		&km.Base.Delete, &km.Base.Yank, &km.Base.Rename, &km.Base.CopyPath,
		&km.Base.Search, &km.Base.SearchNext, &km.Base.SearchPrev,
		&km.Base.ClearSearch, &km.Base.Grep,
		&km.Base.SwitchView, &km.Base.NextTab, &km.Base.PrevTab,
		&km.Base.FocusNext, &km.Base.FocusPrev, &km.Base.TogglePreview,
		&km.Base.Tab1, &km.Base.Tab2, &km.Base.Tab3, &km.Base.Tab4, &km.Base.Tab5,
		&km.Base.Tab6, &km.Base.Tab7, &km.Base.Tab8, &km.Base.Tab9,
		&km.Base.Select, &km.Base.SelectAll, &km.Base.SelectNone,
		&km.Base.FoldOpen, &km.Base.FoldClose, &km.Base.FoldToggle,
		&km.Base.FoldOpenAll, &km.Base.FoldCloseAll,
	)
	return km
}

var statsKeys = func() statsKeyMap {
	cfg, _ := config.LoadDefault()
	return newStatsKeyMap(cfg)
}()

// treeViewKeyMap defines the key bindings for the tree page.
type treeViewKeyMap struct {
	keymap.Base
	ToggleExpand  key.Binding
	ToggleHot     key.Binding
	ToggleCold    key.Binding
	ToggleExclude key.Binding
	ToggleIgnored key.Binding
	Refresh       key.Binding
}

func (k treeViewKeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Help, k.Quit}
}

// Compile-time guard: satisfies the sectioned help/audit contract (value receiver).
var _ keymap.SectionedKeyMap = treeViewKeyMap{}

// Sections returns the grouped key bindings the tree page actually implements.
func (k treeViewKeyMap) Sections() []keymap.Section {
	return []keymap.Section{
		keymap.NavigationSection(k.Up, k.Down, k.Top, k.Bottom, k.PageUp, k.PageDown),
		keymap.NewSection("Tree", k.ToggleExpand),
		keymap.NewSection(keymap.SectionContext, k.ToggleHot, k.ToggleCold, k.ToggleExclude, k.ToggleIgnored),
		keymap.SearchSection(k.Search, k.SearchNext, k.SearchPrev),
		k.Base.FoldSection(),
		keymap.NewSection("Other", k.Refresh),
		k.Base.SystemSection(),
	}
}

func newTreeKeyMap(cfg *config.Config) treeViewKeyMap {
	km := treeViewKeyMap{
		Base: keymap.Load(cfg, "cx.view"),
		ToggleExpand: key.NewBinding(
			key.WithKeys("enter", " "),
			key.WithHelp("enter/space", "toggle expand"),
		),
		ToggleHot: key.NewBinding(
			key.WithKeys("h"),
			key.WithHelp("h", "toggle hot"),
		),
		ToggleCold: key.NewBinding(
			key.WithKeys("c"),
			key.WithHelp("c", "toggle cold"),
		),
		ToggleExclude: key.NewBinding(
			key.WithKeys("x"),
			key.WithHelp("x", "toggle exclude"),
		),
		ToggleIgnored: key.NewBinding(
			key.WithKeys("H", "."),
			key.WithHelp("H/.", "toggle gitignored"),
		),
		// r stays canonical for the tree page; ctrl+r added as the ecosystem
		// alias (Decision 3). Adding ctrl+r lets Base.Refresh be disabled
		// below without losing the key.
		Refresh: key.NewBinding(
			key.WithKeys("r", "ctrl+r"),
			key.WithHelp("r", "refresh"),
		),
	}
	keymap.ApplyTUIOverrides(cfg, "cx", "view", &km)

	// Kept enabled: Up/Down/PageUp/PageDown/Top/Bottom (nav), Search/SearchNext/
	// SearchPrev, Help, Quit, and the five Fold* chords. The tree's own Refresh
	// carries ctrl+r, so the duplicate Base.Refresh is disabled.
	disable(
		&km.Base.Left, &km.Base.Right, &km.Base.Home, &km.Base.End,
		&km.Base.Confirm, &km.Base.Cancel, &km.Base.Back, &km.Base.Edit,
		&km.Base.Delete, &km.Base.Yank, &km.Base.Rename, &km.Base.Refresh,
		&km.Base.CopyPath, &km.Base.ClearSearch, &km.Base.Grep,
		&km.Base.SwitchView, &km.Base.NextTab, &km.Base.PrevTab,
		&km.Base.FocusNext, &km.Base.FocusPrev, &km.Base.TogglePreview,
		&km.Base.Tab1, &km.Base.Tab2, &km.Base.Tab3, &km.Base.Tab4, &km.Base.Tab5,
		&km.Base.Tab6, &km.Base.Tab7, &km.Base.Tab8, &km.Base.Tab9,
		&km.Base.Select, &km.Base.SelectAll, &km.Base.SelectNone,
	)
	return km
}

var treeKeys = func() treeViewKeyMap {
	cfg, _ := config.LoadDefault()
	return newTreeKeyMap(cfg)
}()

// viewKeyMap is the merged, page-grouped keymap for the whole cx view meta-panel.
// It composes the three page keymaps so the container's single `?` overlay and
// the keys registry advertise one truthful, page-labeled export under id
// "cx-view". MakeTUIInfo/AuditCoverage recurse into the nested keymaps and
// collapse duplicate Base signatures across them.
type viewKeyMap struct {
	Pager pagerKeyMap
	Stats statsKeyMap
	Tree  treeViewKeyMap
}

func newViewKeyMap(cfg *config.Config) viewKeyMap {
	km := viewKeyMap{
		Pager: newPagerKeyMap(cfg),
		Stats: newStatsKeyMap(cfg),
		Tree:  newTreeKeyMap(cfg),
	}
	// The merged export/help view carries a single `refresh` (Tree.Refresh,
	// r+ctrl+r) and a single `exclude` (Pager.Exclude, x). Disable the shadowed
	// duplicates so Sections() and AuditCoverage agree and the keys registry
	// gets one ConfigKey each. This only affects the merged export/help; the
	// per-page runtime keymaps keep their own refresh/exclude.
	disable(&km.Pager.Base.Refresh, &km.Stats.Base.Refresh, &km.Stats.Exclude)
	return km
}

// Compile-time guard: satisfies the sectioned help/audit contract (value receiver).
var _ keymap.SectionedKeyMap = viewKeyMap{}

// Sections returns the page-grouped key bindings for the merged cx-view export.
// Each enabled binding across the three page keymaps is covered exactly once by
// signature; duplicate Base bindings (nav, folds, system) collapse to a single
// appearance.
func (k viewKeyMap) Sections() []keymap.Section {
	return []keymap.Section{
		keymap.NavigationSection(k.Pager.Up, k.Pager.Down, k.Pager.PageUp, k.Pager.PageDown, k.Pager.Top, k.Pager.Bottom),
		keymap.NewSection("Pages", k.Pager.NextTab, k.Pager.PrevTab, k.Pager.Tab1, k.Pager.Tab2, k.Pager.Tab3, k.Pager.Tab4, k.Pager.Tab5, k.Pager.Tab6, k.Pager.Tab7, k.Pager.Tab8, k.Pager.Tab9),
		keymap.NewSection(keymap.SectionRules, k.Pager.Edit, k.Pager.SelectRules, k.Pager.Exclude, k.Pager.ExcludeDir),
		keymap.NewSection("List", k.Pager.ToggleSort),
		// Tree.Refresh (r, ctrl+r) is the single merged refresh: its ctrl+r key
		// also represents the pager/stats Base.Refresh, so those are omitted here
		// to keep one `refresh` ConfigKey in the merged export (page keymaps still
		// carry their own refresh at runtime).
		keymap.NewSection("Tree", k.Tree.ToggleExpand, k.Tree.ToggleHot, k.Tree.ToggleCold, k.Tree.ToggleExclude, k.Tree.ToggleIgnored, k.Tree.Refresh, k.Tree.Search, k.Tree.SearchNext, k.Tree.SearchPrev),
		// k.Pager.Exclude already carries x=exclude for the merged export; the
		// stats page's identical x=exclude is omitted to avoid a duplicate
		// `exclude` ConfigKey.
		keymap.NewSection("Stats", k.Stats.SwitchFocus),
		k.Pager.Base.FoldSection(),
		k.Pager.Base.SystemSection(),
	}
}

// KeymapInfo returns the keymap metadata for the cx view TUI.
// Used by the grove keys registry generator to aggregate all TUI keybindings.
func KeymapInfo() keymap.TUIInfo {
	return keymap.MakeTUIInfo(
		"cx-view",
		"cx",
		"Context viewer with tree, stats, and list pages",
		newViewKeyMap(nil),
	)
}
