package rules

import (
	"github.com/charmbracelet/bubbles/key"
	"github.com/mattsolo1/grove-core/tui/keymap"
)

// --- TUI Keymap ---

type pickerKeyMap struct {
	keymap.Base
	Select key.Binding
	Load   key.Binding
	Edit   key.Binding
	Save   key.Binding
	Delete key.Binding
}

func (k pickerKeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Help, k.Quit}
}

func (k pickerKeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Up, k.Down, k.Select, k.Load},
		{k.Save, k.Edit, k.Delete, k.Help, k.Quit},
	}
}

var defaultPickerKeyMap = pickerKeyMap{
	Base: keymap.NewBase(),
	Select: key.NewBinding(
		key.WithKeys("enter"),
		key.WithHelp("enter", "select"),
	),
	Load: key.NewBinding(
		key.WithKeys("l"),
		key.WithHelp("l", "load to .grove/rules"),
	),
	Edit: key.NewBinding(
		key.WithKeys("e"),
		key.WithHelp("e", "edit"),
	),
	Save: key.NewBinding(
		key.WithKeys("s"),
		key.WithHelp("s", "save"),
	),
	Delete: key.NewBinding(
		key.WithKeys("d"),
		key.WithHelp("d", "delete"),
	),
}
