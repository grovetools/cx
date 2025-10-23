package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/fsnotify/fsnotify"
	"github.com/mattsolo1/grove-core/tui/components/help"
	"github.com/mattsolo1/grove-core/tui/keymap"
	core_theme "github.com/mattsolo1/grove-core/tui/theme"
	"github.com/spf13/cobra"
	"github.com/mattsolo1/grove-context/pkg/context"
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

// NewDashboardCmd creates the dashboard command
func NewDashboardCmd() *cobra.Command {
	var horizontal bool
	var plain bool
	
	cmd := &cobra.Command{
		Use:   "dashboard",
		Short: "Display a live dashboard of context statistics",
		Long:  `Launch an interactive terminal UI to display real-time hot and cold context statistics that update automatically when files change.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if plain {
				// Plain output mode
				return outputPlainStats()
			}
			
			// Interactive TUI mode
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
	cmd.Flags().BoolVarP(&plain, "plain", "p", false, "Output plain text statistics (no TUI)")
	
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
	help        help.Model
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

	helpModel := help.NewBuilder().
		WithKeys(dashboardKeys).
		WithTitle("Grove Context Dashboard - Help").
		Build()

	return &dashboardModel{
		watcher:    watcher,
		lastUpdate: time.Now(),
		horizontal: horizontal,
		help:       helpModel,
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
			   name == "dist" || name == "build" || name == ".grove-context" || name == ".grove-worktrees" {
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

// View renders the UI
func (m dashboardModel) View() string {
	theme := core_theme.DefaultTheme

	// If help is visible, show it and return
	if m.help.ShowAll {
		m.help.SetSize(m.width, m.height)
		return m.help.View()
	}

	// Header
	header := theme.Header.Render("Grove Context Dashboard")

	// Status line
	statusStyle := theme.Muted.Copy().MarginBottom(1)

	updateIndicator := ""
	if m.isUpdating {
		updateIndicator = " (updating...)"
	}

	status := statusStyle.Render(fmt.Sprintf("Last update: %s%s",
		m.lastUpdate.Format("15:04:05"),
		updateIndicator))

	// Error display
	if m.err != nil {
		errorStyle := theme.Error.Copy().MarginTop(1).MarginBottom(1)

		return header + "\n" + status + "\n" +
			errorStyle.Render(fmt.Sprintf("Error: %v", m.err)) + "\n\n" +
			statusStyle.Render("Press 'r' to retry, 'q' to quit")
	}

	// Hot context box
	hotBox := renderSummaryBox("Hot Context Statistics", m.hotStats, m.horizontal)

	// Cold context box
	coldBox := renderSummaryBox("Cold Context Statistics", m.coldStats, m.horizontal)

	// Help text
	help := m.help.View()

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
	theme := core_theme.DefaultTheme

	// Box style - adjust width for horizontal layout
	boxWidth := 50
	if horizontal {
		boxWidth = 25 // Half width for side-by-side display
	}

	boxStyle := theme.Box.Copy().
		BorderForeground(theme.Colors.Cyan).
		Width(boxWidth)

	// Title style - use bold for emphasis without explicit color
	titleStyle := theme.Bold.Copy().
		MarginBottom(1)

	// Content
	content := titleStyle.Render(title) + "\n\n"

	// Stats - use terminal defaults for regular text
	labelStyle := lipgloss.NewStyle()
	valueStyle := theme.Bold

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
	theme := core_theme.DefaultTheme
	boxWidth := 50
	if horizontal {
		boxWidth = 25 // Half width for side-by-side display
	}

	boxStyle := theme.Box.Copy().
		BorderForeground(theme.Colors.MutedText).
		Width(boxWidth)

	titleStyle := theme.Muted.Copy().Bold(true).MarginBottom(1)

	emptyStyle := theme.Muted.Copy().Italic(true)

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
		   part == "__pycache__" || part == ".pytest_cache" || part == ".grove-worktrees" {
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

// outputPlainStats outputs statistics in plain text format
func outputPlainStats() error {
	mgr := context.NewManager(".")
	
	// Get hot context files
	hotFiles, err := mgr.ResolveFilesFromRules()
	if err != nil {
		return fmt.Errorf("error resolving hot context: %w", err)
	}

	// Get cold context files
	coldFiles, err := mgr.ResolveColdContextFiles()
	if err != nil {
		return fmt.Errorf("error resolving cold context: %w", err)
	}

	// Get stats for both
	hotStats, err := mgr.GetStats("hot", hotFiles, 10)
	if err != nil {
		return fmt.Errorf("error getting hot stats: %w", err)
	}
	
	coldStats, err := mgr.GetStats("cold", coldFiles, 10)
	if err != nil {
		return fmt.Errorf("error getting cold stats: %w", err)
	}

	// Render boxes without TUI (plain text)
	hotBox := renderPlainBox("Hot Context Statistics", hotStats)
	coldBox := renderPlainBox("Cold Context Statistics", coldStats)
	
	// Split into lines for side-by-side display
	hotLines := strings.Split(hotBox, "\n")
	coldLines := strings.Split(coldBox, "\n")
	
	// Ensure both have same number of lines
	maxLines := len(hotLines)
	if len(coldLines) > maxLines {
		maxLines = len(coldLines)
	}
	
	// Pad shorter one with empty lines
	for len(hotLines) < maxLines {
		hotLines = append(hotLines, strings.Repeat(" ", 27))
	}
	for len(coldLines) < maxLines {
		coldLines = append(coldLines, strings.Repeat(" ", 27))
	}
	
	// Print header
	fmt.Println("Grove Context Dashboard")
	fmt.Printf("Last update: %s\n\n", time.Now().Format("15:04:05"))
	
	// Print side by side
	for i := 0; i < maxLines; i++ {
		fmt.Printf("%s  %s\n", hotLines[i], coldLines[i])
	}
	
	return nil
}

// renderPlainBox renders a statistics box without lipgloss
func renderPlainBox(title string, stats *context.ContextStats) string {
	var b strings.Builder
	
	// Top border
	b.WriteString("╭─────────────────────────╮\n")
	
	// Empty line
	b.WriteString("│                         │\n")
	
	// Title lines - split title into two lines
	titleLines := []string{"Hot Context", "Statistics"}
	if strings.Contains(title, "Cold") {
		titleLines[0] = "Cold Context"
	}
	
	// Center each line
	for _, line := range titleLines {
		padding := (25 - len(line)) / 2
		b.WriteString(fmt.Sprintf("│%*s%s%*s│\n", padding, "", line, 25-padding-len(line), ""))
	}
	
	// Empty lines
	b.WriteString("│                         │\n")
	b.WriteString("│                         │\n")
	
	if stats != nil {
		// Total Files
		filesLine := fmt.Sprintf("  Total Files:     %-6d", stats.TotalFiles)
		b.WriteString(fmt.Sprintf("│%-25s│\n", filesLine))
		
		// Total Tokens
		b.WriteString("│  Total Tokens:          │\n")
		tokenStr := "  ~" + context.FormatTokenCount(stats.TotalTokens)
		b.WriteString(fmt.Sprintf("│%-25s│\n", tokenStr))
		
		// Total Size
		sizeStr := context.FormatBytes(int(stats.TotalSize))
		// Handle size display
		sizeParts := strings.Fields(sizeStr)
		if len(sizeParts) == 2 && len(sizeParts[0]) <= 6 {
			// Normal case: "123.4 KB"
			b.WriteString(fmt.Sprintf("│  Total Size:      %-6s│\n", sizeParts[0]))
			b.WriteString(fmt.Sprintf("│  %-23s│\n", sizeParts[1]))
		} else if len(sizeStr) <= 6 {
			// Short case: fits on one line
			b.WriteString(fmt.Sprintf("│  Total Size:      %-6s│\n", sizeStr))
			b.WriteString("│                         │\n")
		} else {
			// Long case: split at 6 chars
			b.WriteString(fmt.Sprintf("│  Total Size:      %-6s│\n", sizeStr[:6]))
			b.WriteString(fmt.Sprintf("│  %-23s│\n", sizeStr[6:]))
		}
	} else {
		// No data
		b.WriteString("│  No data available      │\n")
		b.WriteString("│                         │\n")
		b.WriteString("│                         │\n")
		b.WriteString("│                         │\n")
		b.WriteString("│                         │\n")
	}
	
	// Empty line
	b.WriteString("│                         │\n")
	
	// Bottom border
	b.WriteString("╰─────────────────────────╯")
	
	return b.String()
}