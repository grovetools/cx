package dashboard

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// Init initializes the model
func (m dashboardModel) Init() tea.Cmd {
	return tea.Batch(
		fetchStatsCmd(),
		m.waitForActivityCmd(),
		tickCmd(),
	)
}

// Update handles messages
func (m dashboardModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case statsUpdatedMsg:
		m.hotStats = msg.hotStats
		m.coldStats = msg.coldStats
		m.err = msg.err
		m.lastUpdate = time.Now()
		m.isUpdating = false
		return m, nil

	case fileChangeMsg:
		// A file change was detected
		if !m.isUpdating {
			m.isUpdating = true
			return m, tea.Batch(fetchStatsCmd(), m.waitForActivityCmd())
		}
		// If already updating, keep listening for more changes
		return m, m.waitForActivityCmd()

	case errorMsg:
		m.err = msg.err
		return m, m.waitForActivityCmd()

	case tickMsg:
		// Continue listening for changes
		return m, tickCmd()

	case tea.KeyMsg:
		// If help is visible, it consumes all key presses
		if m.help.ShowAll {
			m.help.Toggle() // Any key closes help
			return m, nil
		}

		switch msg.String() {
		case "q", "esc", "ctrl+c":
			m.watcher.Close()
			return m, tea.Quit
		case "r":
			// Manual refresh
			if !m.isUpdating {
				m.isUpdating = true
				return m, fetchStatsCmd()
			}
		case "?":
			m.help.Toggle()
		}
	}

	return m, nil
}
