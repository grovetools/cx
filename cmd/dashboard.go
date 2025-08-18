package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/fsnotify/fsnotify"
	"github.com/spf13/cobra"
	"github.com/mattsolo1/grove-context/pkg/context"
)

// NewDashboardCmd creates the dashboard command
func NewDashboardCmd() *cobra.Command {
	var horizontal bool
	
	cmd := &cobra.Command{
		Use:   "dashboard",
		Short: "Display a live dashboard of context statistics",
		Long:  `Launch an interactive terminal UI to display real-time hot and cold context statistics that update automatically when files change.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			m, err := newDashboardModel(horizontal)
			if err != nil {
				return err
			}
			p := tea.NewProgram(m, tea.WithAltScreen())
			_, err = p.Run()
			return err
		},
	}
	
	cmd.Flags().BoolVarP(&horizontal, "horizontal", "H", false, "Display statistics side by side")
	
	return cmd
}

// Message types
type statsUpdatedMsg struct {
	hotStats  *context.ContextStats
	coldStats *context.ContextStats
	err       error
}

type fileChangeMsg struct{}

type errorMsg struct {
	err error
}

type tickMsg time.Time

// dashboardModel is the model for the interactive dashboard
type dashboardModel struct {
	hotStats    *context.ContextStats
	coldStats   *context.ContextStats
	watcher     *fsnotify.Watcher
	width       int
	height      int
	err         error
	lastUpdate  time.Time
	isUpdating  bool
	horizontal  bool
}

// newDashboardModel creates a new dashboard model
func newDashboardModel(horizontal bool) (*dashboardModel, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("failed to create file watcher: %w", err)
	}

	// Watch the current directory recursively
	if err := watchDirectory(".", watcher); err != nil {
		watcher.Close()
		return nil, fmt.Errorf("failed to watch directory: %w", err)
	}

	return &dashboardModel{
		watcher:    watcher,
		lastUpdate: time.Now(),
		horizontal: horizontal,
	}, nil
}

// watchDirectory recursively adds directories to the watcher
func watchDirectory(path string, watcher *fsnotify.Watcher) error {
	// Add the directory itself
	if err := watcher.Add(path); err != nil {
		return err
	}
	
	// Walk through subdirectories
	entries, err := os.ReadDir(path)
	if err != nil {
		return err
	}
	
	for _, entry := range entries {
		if entry.IsDir() {
			// Skip common directories that shouldn't be watched
			name := entry.Name()
			if name == ".git" || name == "node_modules" || name == "vendor" || 
			   name == "dist" || name == "build" || name == ".grove-context" {
				continue
			}
			
			subPath := filepath.Join(path, name)
			if err := watchDirectory(subPath, watcher); err != nil {
				// Ignore errors for individual subdirectories
				continue
			}
		}
	}
	
	return nil
}

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
		}
	}

	return m, nil
}

// View renders the UI
func (m dashboardModel) View() string {
	// Header
	headerStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#4ecdc4")).
		Bold(true).
		MarginBottom(1)

	header := headerStyle.Render("Grove Context Dashboard")

	// Status line
	statusStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#808080")).
		MarginBottom(1)

	updateIndicator := ""
	if m.isUpdating {
		updateIndicator = " (updating...)"
	}

	status := statusStyle.Render(fmt.Sprintf("Last update: %s%s", 
		m.lastUpdate.Format("15:04:05"),
		updateIndicator))

	// Error display
	if m.err != nil {
		errorStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#ff4444")).
			Bold(true).
			MarginTop(1).
			MarginBottom(1)
		
		return header + "\n" + status + "\n" + 
			errorStyle.Render(fmt.Sprintf("Error: %v", m.err)) + "\n\n" +
			statusStyle.Render("Press 'r' to retry, 'q' to quit")
	}

	// Hot context box
	hotBox := renderSummaryBox("Hot Context Statistics", m.hotStats, m.horizontal)

	// Cold context box
	coldBox := renderSummaryBox("Cold Context Statistics", m.coldStats, m.horizontal)

	// Help text
	helpStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#808080")).
		MarginTop(1)

	layoutIndicator := ""
	if m.horizontal {
		layoutIndicator = " (horizontal layout)"
	} else {
		layoutIndicator = " (vertical layout)"
	}
	
	help := helpStyle.Render("Watching for file changes... Press 'r' to refresh manually, 'q' to quit" + layoutIndicator)

	// Combine all elements based on layout
	if m.horizontal {
		// Horizontal layout
		statsRow := lipgloss.JoinHorizontal(
			lipgloss.Top,
			hotBox,
			"  ", // spacing between boxes
			coldBox,
		)
		
		return lipgloss.JoinVertical(
			lipgloss.Left,
			header,
			status,
			"",
			statsRow,
			"",
			help,
		)
	} else {
		// Vertical layout
		return lipgloss.JoinVertical(
			lipgloss.Left,
			header,
			status,
			"",
			hotBox,
			"",
			coldBox,
			"",
			help,
		)
	}
}

// renderSummaryBox renders a statistics summary box
func renderSummaryBox(title string, stats *context.ContextStats, horizontal bool) string {
	if stats == nil {
		return renderEmptyBox(title, horizontal)
	}

	// Box style - adjust width for horizontal layout
	boxWidth := 50
	if horizontal {
		boxWidth = 25 // Half width for side-by-side display
	}
	
	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#4ecdc4")).
		Padding(1, 2).
		Width(boxWidth)

	// Title style
	titleStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#4ecdc4")).
		Bold(true).
		MarginBottom(1)

	// Content
	content := titleStyle.Render(title) + "\n\n"

	// Stats
	labelStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#95e1d3"))
	
	valueStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#ffffff")).
		Bold(true)

	// Format file count
	fileCount := fmt.Sprintf("%-16s %s", "Total Files:", valueStyle.Render(fmt.Sprintf("%d", stats.TotalFiles)))
	content += labelStyle.Render(fileCount) + "\n"

	// Format token count
	tokenStr := "~" + context.FormatTokenCount(stats.TotalTokens)
	tokenCount := fmt.Sprintf("%-16s %s", "Total Tokens:", valueStyle.Render(tokenStr))
	content += labelStyle.Render(tokenCount) + "\n"

	// Format size
	sizeStr := context.FormatBytes(int(stats.TotalSize))
	size := fmt.Sprintf("%-16s %s", "Total Size:", valueStyle.Render(sizeStr))
	content += labelStyle.Render(size)

	return boxStyle.Render(content)
}

// renderEmptyBox renders an empty statistics box
func renderEmptyBox(title string, horizontal bool) string {
	boxWidth := 50
	if horizontal {
		boxWidth = 25 // Half width for side-by-side display
	}
	
	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#808080")).
		Padding(1, 2).
		Width(boxWidth)

	titleStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#808080")).
		Bold(true).
		MarginBottom(1)

	emptyStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#808080")).
		Italic(true)

	content := titleStyle.Render(title) + "\n\n" +
		emptyStyle.Render("No data available")

	return boxStyle.Render(content)
}

// fetchStatsCmd fetches context statistics
func fetchStatsCmd() tea.Cmd {
	return func() tea.Msg {
		mgr := context.NewManager(".")
		
		// Get hot context files
		hotFiles, err := mgr.ResolveFilesFromRules()
		if err != nil {
			return statsUpdatedMsg{err: err}
		}

		// Get cold context files
		coldFiles, err := mgr.ResolveColdContextFiles()
		if err != nil {
			return statsUpdatedMsg{err: err}
		}

		// Get stats for both
		hotStats, err := mgr.GetStats("hot", hotFiles, 10)
		if err != nil {
			return statsUpdatedMsg{err: err}
		}
		
		coldStats, err := mgr.GetStats("cold", coldFiles, 10)
		if err != nil {
			return statsUpdatedMsg{err: err}
		}

		return statsUpdatedMsg{
			hotStats:  hotStats,
			coldStats: coldStats,
		}
	}
}

// waitForActivityCmd waits for file system activity
func (m *dashboardModel) waitForActivityCmd() tea.Cmd {
	return func() tea.Msg {
		select {
		case event, ok := <-m.watcher.Events:
			if !ok {
				return errorMsg{err: fmt.Errorf("watcher closed")}
			}
			
			// Ignore certain events and files
			if event.Op&fsnotify.Chmod == fsnotify.Chmod {
				return m.waitForActivityCmd()() // Continue waiting
			}
			
			// Check if it's a relevant file
			if isRelevantFile(event.Name) {
				// Add a small delay to ensure file write is complete
				time.Sleep(50 * time.Millisecond)
				return fileChangeMsg{}
			}
			
			return m.waitForActivityCmd()() // Continue waiting
			
		case err, ok := <-m.watcher.Errors:
			if !ok {
				return errorMsg{err: fmt.Errorf("watcher error channel closed")}
			}
			return errorMsg{err: err}
		}
	}
}

// tickCmd sends periodic tick messages
func tickCmd() tea.Cmd {
	return tea.Tick(100*time.Millisecond, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

// isRelevantFile checks if a file change is relevant to context
func isRelevantFile(path string) bool {
	// Clean the path
	path = filepath.Clean(path)
	base := filepath.Base(path)
	
	// Always relevant files
	if base == ".grove-context.yaml" || base == "CLAUDE.md" {
		return true
	}
	
	// Check if it's in .grove-context directory
	if strings.Contains(path, ".grove-context/") {
		return false
	}
	
	// Ignore hidden files and directories
	if strings.HasPrefix(base, ".") {
		return false
	}
	
	// Ignore common non-relevant directories
	parts := strings.Split(path, string(filepath.Separator))
	for _, part := range parts {
		if part == "node_modules" || part == "vendor" || part == ".git" ||
		   part == "dist" || part == "build" || part == "target" ||
		   part == "__pycache__" || part == ".pytest_cache" {
			return false
		}
	}
	
	// Ignore common binary and temporary files
	ext := strings.ToLower(filepath.Ext(path))
	ignoredExts := []string{".pyc", ".pyo", ".so", ".dylib", ".dll", ".exe", ".bin", ".o", ".a", ".log", ".tmp", ".swp", ".swo", ".DS_Store"}
	for _, ignored := range ignoredExts {
		if ext == ignored {
			return false
		}
	}
	
	// Consider other files relevant
	return true
}