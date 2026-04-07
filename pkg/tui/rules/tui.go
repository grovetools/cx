package rules

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
)

// Run launches the interactive rule set selector TUI as a standalone program.
// workDir overrides the working directory for context resolution (empty uses CWD).
//
// Deprecated: callers should construct the model via New and host it via
// embed.RunStandalone. This helper exists for backward compatibility during
// the embeddable-TUI refactor.
func Run(workDir string) error {
	m := New(workDir, nil)
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
