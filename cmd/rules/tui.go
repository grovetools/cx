package rules

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
)

// Run launches the interactive rule set selector TUI.
func Run() error {
	m := newRulesPickerModel()
	p := tea.NewProgram(m, tea.WithAltScreen())
	finalModel, err := p.Run()
	if err != nil {
		return fmt.Errorf("error running TUI: %w", err)
	}
	// Check for errors that occurred within the model
	if finalModel.(*rulesPickerModel).err != nil {
		return finalModel.(*rulesPickerModel).err
	}
	return nil
}
