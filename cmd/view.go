package cmd

import (
	"fmt"
	"path/filepath"
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

type ruleChangeResultMsg struct {
	err           error
	successMsg    string
	refreshNeeded bool
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
	lastKey       string // Track last key for multi-key commands like "gg"
	statusMessage string // Status message for user feedback
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

// toggleRuleCmd creates a command to toggle rules
func (m *viewModel) toggleRuleCmd(path, targetType string, isDirectory bool) tea.Cmd {
	return func() tea.Msg {
		manager := context.NewManager("")
		
		// Check current status
		currentStatus := manager.GetRuleStatus(path)
		
		var err error
		var successMsg string
		
		// Determine item type for messages
		itemType := "file"
		if isDirectory {
			itemType = "directory tree"
		}
		
		// Toggle logic based on current state and target
		switch targetType {
		case "hot":
			switch currentStatus {
			case context.RuleNotFound, context.RuleCold, context.RuleExcluded:
				// Remove existing rule first if it exists
				if currentStatus != context.RuleNotFound {
					if removeErr := manager.RemoveRule(path); removeErr != nil {
						return ruleChangeResultMsg{err: removeErr, refreshNeeded: false}
					}
				}
				// Add to hot context
				err = manager.AppendRule(path, "hot")
				successMsg = fmt.Sprintf("Added %s to hot context: %s", itemType, path)
			case context.RuleHot:
				// Remove from hot context
				err = manager.RemoveRule(path)
				successMsg = fmt.Sprintf("Removed %s from hot context: %s", itemType, path)
			}
		case "cold":
			switch currentStatus {
			case context.RuleNotFound, context.RuleHot, context.RuleExcluded:
				// Remove existing rule first if it exists
				if currentStatus != context.RuleNotFound {
					if removeErr := manager.RemoveRule(path); removeErr != nil {
						return ruleChangeResultMsg{err: removeErr, refreshNeeded: false}
					}
				}
				// Add to cold context
				err = manager.AppendRule(path, "cold")
				successMsg = fmt.Sprintf("Added %s to cold context: %s", itemType, path)
			case context.RuleCold:
				// Remove from cold context
				err = manager.RemoveRule(path)
				successMsg = fmt.Sprintf("Removed %s from cold context: %s", itemType, path)
			}
		case "exclude":
			switch currentStatus {
			case context.RuleNotFound, context.RuleHot, context.RuleCold:
				// Remove existing rule first if it exists
				if currentStatus != context.RuleNotFound {
					if removeErr := manager.RemoveRule(path); removeErr != nil {
						return ruleChangeResultMsg{err: removeErr, refreshNeeded: false}
					}
				}
				// Add exclusion rule
				err = manager.AppendRule(path, "exclude")
				successMsg = fmt.Sprintf("Excluded %s: %s", itemType, path)
			case context.RuleExcluded:
				// Remove exclusion
				err = manager.RemoveRule(path)
				successMsg = fmt.Sprintf("Removed exclusion for %s: %s", itemType, path)
			}
		}
		
		if err != nil {
			return ruleChangeResultMsg{err: err, refreshNeeded: false}
		}
		
		return ruleChangeResultMsg{err: nil, successMsg: successMsg, refreshNeeded: true}
	}
}

// getRelativePath converts node path to relative path suitable for rules file
// For directories, it returns a glob pattern like "dirname/**" to include all contents
func (m *viewModel) getRelativePath(node *context.FileNode) (string, error) {
	manager := context.NewManager("")
	
	// Get relative path from current working directory
	relPath, err := filepath.Rel(manager.GetWorkDir(), node.Path)
	if err != nil {
		return "", err
	}
	
	// If path starts with "..", it's outside workdir, use absolute path
	var basePath string
	if strings.HasPrefix(relPath, "..") {
		basePath = node.Path
	} else {
		basePath = relPath
	}
	
	// For directories, append /** to include all contents recursively
	if node.IsDir {
		// Ensure we don't double-slash
		if strings.HasSuffix(basePath, "/") {
			return basePath + "**", nil
		} else {
			return basePath + "/**", nil
		}
	}
	
	return basePath, nil
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
		case "ctrl+d":
			// Scroll down half a page
			viewportHeight := m.height - 6
			if viewportHeight < 1 {
				viewportHeight = 1
			}
			m.cursor += viewportHeight / 2
			if m.cursor >= len(m.visibleNodes) {
				m.cursor = len(m.visibleNodes) - 1
			}
			m.ensureCursorVisible()
		case "ctrl+u":
			// Scroll up half a page
			viewportHeight := m.height - 6
			if viewportHeight < 1 {
				viewportHeight = 1
			}
			m.cursor -= viewportHeight / 2
			if m.cursor < 0 {
				m.cursor = 0
			}
			m.ensureCursorVisible()
		case "A":
			// Auto-expand all directories
			m.expandAll()
		case "a":
			// Toggle hot context
			if m.cursor >= len(m.visibleNodes) {
				break
			}
			node := m.visibleNodes[m.cursor].node
			relPath, err := m.getRelativePath(node)
			if err != nil {
				m.statusMessage = fmt.Sprintf("Error: %v", err)
				break
			}
			// Create appropriate status message for directories vs files
			var itemType string
			if node.IsDir {
				itemType = "directory tree"
			} else {
				itemType = "file"
			}
			m.statusMessage = fmt.Sprintf("Toggling hot context for %s %s...", itemType, node.Name)
			return m, m.toggleRuleCmd(relPath, "hot", node.IsDir)
		case "c":
			// Toggle cold context
			if m.cursor >= len(m.visibleNodes) {
				break
			}
			node := m.visibleNodes[m.cursor].node
			relPath, err := m.getRelativePath(node)
			if err != nil {
				m.statusMessage = fmt.Sprintf("Error: %v", err)
				break
			}
			// Create appropriate status message for directories vs files
			var itemType string
			if node.IsDir {
				itemType = "directory tree"
			} else {
				itemType = "file"
			}
			m.statusMessage = fmt.Sprintf("Toggling cold context for %s %s...", itemType, node.Name)
			return m, m.toggleRuleCmd(relPath, "cold", node.IsDir)
		case "x":
			// Toggle exclude
			if m.cursor >= len(m.visibleNodes) {
				break
			}
			node := m.visibleNodes[m.cursor].node
			relPath, err := m.getRelativePath(node)
			if err != nil {
				m.statusMessage = fmt.Sprintf("Error: %v", err)
				break
			}
			// Create appropriate status message for directories vs files
			var itemType string
			if node.IsDir {
				itemType = "directory tree"
			} else {
				itemType = "file"
			}
			m.statusMessage = fmt.Sprintf("Toggling exclusion for %s %s...", itemType, node.Name)
			return m, m.toggleRuleCmd(relPath, "exclude", node.IsDir)
		case "g":
			if m.lastKey == "g" {
				// gg - go to top
				m.cursor = 0
				m.scrollOffset = 0
				m.lastKey = ""
			} else {
				m.lastKey = "g"
			}
		case "G":
			// Go to bottom
			m.cursor = len(m.visibleNodes) - 1
			m.ensureCursorVisible()
		default:
			// Clear lastKey for any other key that's not part of a combo
			if m.lastKey != "" && msg.String() != "g" {
				m.lastKey = ""
			}
		}

	case ruleChangeResultMsg:
		if msg.err != nil {
			m.statusMessage = fmt.Sprintf("Error: %v", msg.err)
		} else {
			m.statusMessage = msg.successMsg
			if msg.refreshNeeded {
				// Refresh the tree to show the updated context
				m.loading = true
				return m, loadTree
			}
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
		// Auto-expand paths to content
		m.autoExpandToContent(m.tree)
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

// expandAll expands all directories in the tree
func (m *viewModel) expandAll() {
	if m.tree == nil {
		return
	}
	m.expandAllRecursive(m.tree)
	m.updateVisibleNodes()
}

// expandAllRecursive recursively marks all directories as expanded
func (m *viewModel) expandAllRecursive(node *context.FileNode) {
	if node.IsDir && len(node.Children) > 0 {
		m.expandedPaths[node.Path] = true
		for _, child := range node.Children {
			m.expandAllRecursive(child)
		}
	}
}

// autoExpandToContent expands directories intelligently
func (m *viewModel) autoExpandToContent(node *context.FileNode) {
	if !node.IsDir {
		return
	}
	
	// Count how many directory children this node has
	dirCount := 0
	var hasProjectDirs bool
	
	for _, child := range node.Children {
		if child.IsDir {
			dirCount++
			// Check if any child contains "(CWD)" or looks like a project
			if strings.Contains(child.Name, "(CWD)") || 
			   strings.Contains(child.Name, "grove-") {
				hasProjectDirs = true
			}
		}
	}
	
	// Auto-expand if:
	// 1. This directory has only one child directory (single-child chain)
	// 2. OR this directory contains project directories (to show the project list)
	if (len(node.Children) == 1 && dirCount == 1) || hasProjectDirs {
		m.expandedPaths[node.Path] = true
		// Continue expanding single-child chains
		if len(node.Children) == 1 && dirCount == 1 {
			for _, child := range node.Children {
				if child.IsDir {
					m.autoExpandToContent(child)
				}
			}
		}
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

	// Status message
	statusMsg := ""
	if m.statusMessage != "" {
		statusMsg = lipgloss.NewStyle().
			Foreground(lipgloss.Color("34")).
			Bold(true).
			Render(m.statusMessage)
	}

	// Controls
	controls := lipgloss.NewStyle().
		Foreground(lipgloss.Color("241")).
		Render("↑/↓/j/k: navigate | gg/G: top/bottom | Enter/Space: toggle | A: expand all | a: toggle hot | c: toggle cold | x: toggle exclude (dirs include all contents) | Ctrl-D/U: page down/up | q: quit")

	// Combine all parts
	parts := []string{
		header,
		tree,
		"",
		legend,
	}
	
	if statusMsg != "" {
		parts = append(parts, statusMsg)
	}
	
	parts = append(parts, controls)
	
	return lipgloss.JoinVertical(lipgloss.Left, parts...)
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
	}

	return lipgloss.NewStyle().
		Foreground(lipgloss.Color("241")).
		Render(strings.Join(legendItems, "   "))
}
