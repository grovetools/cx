package dashboard

import (
	"github.com/charmbracelet/bubbles/key"
	"github.com/mattsolo1/grove-core/tui/keymap"
)

// Keymap for the dashboard TUI
type dashboardKeyMap struct {
	keymap.Base
	Refresh key.Binding
}

func (k dashboardKeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Refresh, k.Quit}
}

func (k dashboardKeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{
			key.NewBinding(key.WithHelp("", "Dashboard:")),
			key.NewBinding(key.WithHelp("Hot Context", "Files actively included in the primary context.")),
			key.NewBinding(key.WithHelp("Cold Context", "Files included for background or reference.")),
			key.NewBinding(key.WithHelp("Auto-Update", "Statistics refresh automatically on file changes.")),
		},
		{
			key.NewBinding(key.WithHelp("", "Actions:")),
			k.Refresh,
			k.Quit,
			k.Help,
		},
	}
}

var dashboardKeys = dashboardKeyMap{
	Base: keymap.NewBase(),
	Refresh: key.NewBinding(
		key.WithKeys("r"),
		key.WithHelp("r", "refresh"),
	),
}
