package dashboard

import tea "github.com/charmbracelet/bubbletea"

// Run starts the dashboard TUI.
func Run(horizontal bool) error {
	m, err := newDashboardModel(horizontal)
	if err != nil {
		return err
	}
	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err = p.Run()
	return err
}
