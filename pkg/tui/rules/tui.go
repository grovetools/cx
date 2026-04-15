package rules

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/grovetools/core/tui/embed"
)

// Run launches the interactive rule set selector TUI as a standalone program
// by wrapping the model in an embed.StandaloneHost. workDir overrides the
// working directory for context resolution (empty uses CWD).
//
// This is a thin convenience wrapper for cobra entrypoints; embedding hosts
// (such as cx view or grove-terminal) should call New directly and route
// messages themselves rather than going through Run.
func Run(workDir string) error {
	m := New(workDir, "", nil)
	if _, err := embed.RunStandalone(m, tea.WithAltScreen()); err != nil {
		return fmt.Errorf("error running TUI: %w", err)
	}
	if m.err != nil {
		return m.err
	}
	return nil
}
