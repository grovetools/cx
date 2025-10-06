package view

import tea "github.com/charmbracelet/bubbletea"

// Page is the interface for a full-screen view in the TUI.
type Page interface {
	// Name returns the identifier for the page (e.g., "tree", "repo").
	Name() string
	// Init initializes the page model.
	Init() tea.Cmd
	// Update handles messages for the page.
	Update(tea.Msg) (Page, tea.Cmd)
	// View renders the page's UI.
	View() string
	// Focus is called when the page becomes active.
	Focus() tea.Cmd
	// Blur is called when the page loses focus.
	Blur()
	// SetSize sets the dimensions for the page.
	SetSize(width, height int)
}
