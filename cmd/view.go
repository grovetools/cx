package cmd

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/mattsolo1/grove-context/pkg/context"
	"github.com/spf13/cobra"
)

// NewViewCmd creates the view command
func NewViewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "view",
		Short: "Display an interactive visualization of context composition",
		Long:  `Launch an interactive terminal UI that shows which files are included, excluded, or ignored in your context based on rules and git ignore patterns.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			m := newViewModel()
			p := tea.NewProgram(m, tea.WithAltScreen())
			_, err := p.Run()
			return err
		},
	}

	return cmd
}

// Message types
type treeLoadedMsg struct {
	tree *context.FileNode
	err  error
}

// viewModel is the model for the interactive tree view
type viewModel struct {
	tree          *context.FileNode
	cursor        int
	visibleNodes  []*nodeWithLevel
	expandedPaths map[string]bool
	width         int
	height        int
	scrollOffset  int
	loading       bool
	err           error
}

// nodeWithLevel stores a node with its display level
type nodeWithLevel struct {
	node  *context.FileNode
	level int
}

// newViewModel creates a new view model
func newViewModel() *viewModel {
	return &viewModel{
		expandedPaths: make(map[string]bool),
		loading:       true,
	}
}

// Init initializes the model
func (m *viewModel) Init() tea.Cmd {
	return loadTree
}

// loadTree loads the project tree analysis
func loadTree() tea.Msg {
	manager := context.NewManager("")
	tree, err := manager.AnalyzeProjectTree()
	return treeLoadedMsg{tree: tree, err: err}
}

// Update handles messages
func (m *viewModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
				m.ensureCursorVisible()
			}
		case "down", "j":
			if m.cursor < len(m.visibleNodes)-1 {
				m.cursor++
				m.ensureCursorVisible()
			}
		case "enter", " ":
			m.toggleExpanded()
		case "pgup":
			m.cursor -= 10
			if m.cursor < 0 {
				m.cursor = 0
			}
			m.ensureCursorVisible()
		case "pgdown":
			m.cursor += 10
			if m.cursor >= len(m.visibleNodes) {
				m.cursor = len(m.visibleNodes) - 1
			}
			m.ensureCursorVisible()
		case "home":
			m.cursor = 0
			m.scrollOffset = 0
		case "end":
			m.cursor = len(m.visibleNodes) - 1
			m.ensureCursorVisible()
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.ensureCursorVisible()

	case treeLoadedMsg:
		m.loading = false
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		m.tree = msg.tree
		// Expand root by default
		m.expandedPaths[m.tree.Path] = true
		m.updateVisibleNodes()
	}

	return m, nil
}

// toggleExpanded toggles the expanded state of the current directory
func (m *viewModel) toggleExpanded() {
	if m.cursor >= len(m.visibleNodes) {
		return
	}

	current := m.visibleNodes[m.cursor]
	if current.node.IsDir && len(current.node.Children) > 0 {
		if m.expandedPaths[current.node.Path] {
			delete(m.expandedPaths, current.node.Path)
		} else {
			m.expandedPaths[current.node.Path] = true
		}
		m.updateVisibleNodes()
	}
}

// updateVisibleNodes updates the list of visible nodes based on expansion state
func (m *viewModel) updateVisibleNodes() {
	m.visibleNodes = nil
	if m.tree != nil {
		// Start with the children of the root, not the root itself
		for _, child := range m.tree.Children {
			m.collectVisibleNodes(child, 0)
		}
	}

	// Ensure cursor is within bounds
	if m.cursor >= len(m.visibleNodes) {
		m.cursor = len(m.visibleNodes) - 1
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
}

// collectVisibleNodes recursively collects visible nodes
func (m *viewModel) collectVisibleNodes(node *context.FileNode, level int) {
	m.visibleNodes = append(m.visibleNodes, &nodeWithLevel{
		node:  node,
		level: level,
	})

	if node.IsDir && m.expandedPaths[node.Path] {
		for _, child := range node.Children {
			m.collectVisibleNodes(child, level+1)
		}
	}
}

// ensureCursorVisible ensures the cursor is visible in the viewport
func (m *viewModel) ensureCursorVisible() {
	viewportHeight := m.height - 6 // Account for header and footer
	if viewportHeight < 1 {
		viewportHeight = 1
	}

	if m.cursor < m.scrollOffset {
		m.scrollOffset = m.cursor
	} else if m.cursor >= m.scrollOffset+viewportHeight {
		m.scrollOffset = m.cursor - viewportHeight + 1
	}
}

// View renders the UI
func (m *viewModel) View() string {
	if m.loading {
		return lipgloss.NewStyle().
			Width(m.width).
			Height(m.height).
			Align(lipgloss.Center, lipgloss.Center).
			Render("Loading project tree...")
	}

	if m.err != nil {
		return lipgloss.NewStyle().
			Width(m.width).
			Height(m.height).
			Align(lipgloss.Center, lipgloss.Center).
			Foreground(lipgloss.Color("1")).
			Render(fmt.Sprintf("Error: %v", m.err))
	}

	// Header
	header := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("12")).
		MarginBottom(1).
		Render("Grove Context Visualization")

	// Tree view
	viewportHeight := m.height - 6 // Account for header, footer, and margins
	if viewportHeight < 1 {
		viewportHeight = 1
	}

	var treeLines []string
	for i := m.scrollOffset; i < len(m.visibleNodes) && i < m.scrollOffset+viewportHeight; i++ {
		line := m.renderNode(i)
		treeLines = append(treeLines, line)
	}

	tree := strings.Join(treeLines, "\n")

	// Legend
	legend := m.renderLegend()

	// Controls
	controls := lipgloss.NewStyle().
		Foreground(lipgloss.Color("241")).
		Render("Use ↑/↓/j/k to navigate, Enter/Space to toggle directories, q to quit")

	// Combine all parts
	return lipgloss.JoinVertical(
		lipgloss.Left,
		header,
		tree,
		"",
		legend,
		controls,
	)
}

// renderNode renders a single node
func (m *viewModel) renderNode(index int) string {
	if index >= len(m.visibleNodes) {
		return ""
	}

	nl := m.visibleNodes[index]
	node := nl.node
	level := nl.level

	// Indentation
	indent := strings.Repeat("  ", level)

	// Cursor indicator
	cursor := "  "
	if index == m.cursor {
		cursor = "> "
	}

	// Icon and name
	icon := m.getIcon(node)
	name := node.Name

	// Style based on status
	style := m.getStyle(node.Status)

	// Directory expansion indicator
	expandIndicator := ""
	if node.IsDir && len(node.Children) > 0 {
		if m.expandedPaths[node.Path] {
			expandIndicator = "▼ "
		} else {
			expandIndicator = "▶ "
		}
	} else if node.IsDir {
		expandIndicator = "  "
	}

	// Combine all parts
	line := fmt.Sprintf("%s%s%s%s %s", cursor, indent, expandIndicator, icon, name)
	return style.Render(line)
}

// getIcon returns the appropriate icon for a node
func (m *viewModel) getIcon(node *context.FileNode) string {
	if node.IsDir {
		return "📁"
	}

	switch node.Status {
	case context.StatusIncludedHot:
		return "🔥"
	case context.StatusIncludedCold:
		return "❄️"
	case context.StatusExcludedByRule:
		return "🚫"
	case context.StatusIgnoredByGit:
		return "🔒"
	default:
		return "📄"
	}
}

// getStyle returns the appropriate style for a status
func (m *viewModel) getStyle(status context.NodeStatus) lipgloss.Style {
	switch status {
	case context.StatusIncludedHot:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("202")) // Orange/red
	case context.StatusIncludedCold:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("51")) // Cyan
	case context.StatusExcludedByRule:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("196")) // Red
	case context.StatusIgnoredByGit:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("242")) // Dark gray
	case context.StatusOmittedNoMatch:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("245")) // Gray
	case context.StatusDirectory:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("39")) // Blue
	default:
		return lipgloss.NewStyle()
	}
}

// renderLegend renders the legend
func (m *viewModel) renderLegend() string {
	legendItems := []string{
		"Legend:",
		"🔥 Hot Context",
		"❄️  Cold Context",
		"🚫 Excluded by Rule",
		"📄 Omitted (No Match)",
		"🔒 Ignored by Git",
	}

	return lipgloss.NewStyle().
		Foreground(lipgloss.Color("241")).
		Render(strings.Join(legendItems, "   "))
}
