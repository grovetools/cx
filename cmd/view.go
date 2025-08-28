package cmd

import (
	"fmt"
	"os"
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
	pruning       bool   // Whether to show only directories with context files
	showHelp      bool   // Whether to show help popup
	rulesContent  string // Content of .grove/rules file
	// Statistics
	hotFiles     int
	hotTokens    int
	coldFiles    int
	coldTokens   int
	totalFiles   int
	totalTokens  int
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
		pruning:       false,
	}
}

// Init initializes the model
func (m *viewModel) Init() tea.Cmd {
	// Load rules content initially
	m.loadRulesContent()
	return m.loadTreeCmd()
}

// loadTreeCmd returns a command to load the project tree analysis
func (m *viewModel) loadTreeCmd() tea.Cmd {
	return func() tea.Msg {
		manager := context.NewManager("")
		tree, err := manager.AnalyzeProjectTree(m.pruning)
		return treeLoadedMsg{tree: tree, err: err}
	}
}

// loadRulesContent loads the content of .grove/rules file
func (m *viewModel) loadRulesContent() {
	rulesPath := filepath.Join(".grove", "rules")
	content, err := os.ReadFile(rulesPath)
	if err != nil {
		if os.IsNotExist(err) {
			m.rulesContent = "# .grove/rules file not found\n# Rules will appear here when created"
		} else {
			m.rulesContent = fmt.Sprintf("# Error reading rules file:\n# %v", err)
		}
	} else {
		m.rulesContent = string(content)
		if m.rulesContent == "" {
			m.rulesContent = "# Empty rules file"
		}
	}
}

// calculateStats recursively calculates statistics from the tree
func (m *viewModel) calculateStats() {
	// Reset stats
	m.hotFiles = 0
	m.hotTokens = 0
	m.coldFiles = 0
	m.coldTokens = 0
	m.totalFiles = 0
	m.totalTokens = 0
	
	if m.tree != nil {
		m.calculateNodeStats(m.tree)
	}
}

// calculateNodeStats recursively calculates stats for a node and its children
func (m *viewModel) calculateNodeStats(node *context.FileNode) {
	if node == nil {
		return
	}
	
	// Count files (not directories)
	if !node.IsDir {
		switch node.Status {
		case context.StatusIncludedHot:
			m.hotFiles++
			m.hotTokens += node.TokenCount
			m.totalFiles++
			m.totalTokens += node.TokenCount
		case context.StatusIncludedCold:
			m.coldFiles++
			m.coldTokens += node.TokenCount
			m.totalFiles++
			m.totalTokens += node.TokenCount
		}
	}
	
	// Recurse through children
	for _, child := range node.Children {
		m.calculateNodeStats(child)
	}
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
		// Handle help popup keys first
		if m.showHelp {
			switch msg.String() {
			case "?", "q", "esc", "ctrl+c":
				m.showHelp = false
				return m, nil
			default:
				return m, nil
			}
		}
		
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "?":
			m.showHelp = true
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
			viewportHeight := m.height - 10
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
			viewportHeight := m.height - 10
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
		case "p":
			// Toggle pruning mode
			m.pruning = !m.pruning
			if m.pruning {
				m.statusMessage = "Pruning mode enabled - showing only directories with context files"
			} else {
				m.statusMessage = "Pruning mode disabled - showing all directories"
			}
			m.loading = true
			return m, m.loadTreeCmd()
		case "r":
			// Refresh both tree and rules
			m.statusMessage = "Refreshing..."
			m.loadRulesContent()
			m.loading = true
			return m, m.loadTreeCmd()
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
				// Reload rules content to show the changes
				m.loadRulesContent()
				// Refresh the tree to show the updated context
				m.loading = true
				return m, m.loadTreeCmd()
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
		// Load rules content
		m.loadRulesContent()
		// Calculate statistics
		m.calculateStats()
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
	viewportHeight := m.height - 10 // Account for header and footer - match View() calculation
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

	// Show help popup if active
	if m.showHelp {
		helpView := m.renderHelp()
		return lipgloss.NewStyle().
			Width(m.width).
			Height(m.height).
			Align(lipgloss.Center, lipgloss.Center).
			Render(helpView)
	}

	// Header
	pruningIndicator := ""
	if m.pruning {
		pruningIndicator = " (Pruning)"
	}
	header := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("12")).
		MarginBottom(1).
		Render("Grove Context Visualization" + pruningIndicator)

	// Calculate split widths (60% for tree, 40% for rules)
	treeWidth := int(float64(m.width) * 0.6)
	rulesWidth := m.width - treeWidth - 3 // -3 for border and padding
	
	// Tree view
	viewportHeight := m.height - 10 // Account for header, footer, and margins - add more padding
	if viewportHeight < 1 {
		viewportHeight = 1
	}

	var treeLines []string
	for i := m.scrollOffset; i < len(m.visibleNodes) && i < m.scrollOffset+viewportHeight; i++ {
		line := m.renderNode(i)
		// Truncate line if too long for tree panel
		if lipgloss.Width(line) > treeWidth-2 {
			line = lipgloss.NewStyle().MaxWidth(treeWidth-2).Render(line)
		}
		treeLines = append(treeLines, line)
	}

	tree := strings.Join(treeLines, "\n")

	// Status message
	statusMsg := ""
	if m.statusMessage != "" {
		statusMsg = lipgloss.NewStyle().
			Foreground(lipgloss.Color("34")).
			Bold(true).
			Render(m.statusMessage)
	}

	// Calculate heights for rules and stats
	statsHeight := 8 // Fixed height for stats
	rulesHeight := viewportHeight - statsHeight - 1 // -1 for spacing
	
	// Rules panel
	rulesStyle := lipgloss.NewStyle().
		Width(rulesWidth).
		Height(rulesHeight).
		Border(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("241")).
		Padding(0, 1)
	
	rulesHeaderStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("12")).
		MarginBottom(1)
	
	rulesHeader := rulesHeaderStyle.Render(".grove/rules")
	
	// Format rules content with line numbers
	rulesLines := strings.Split(m.rulesContent, "\n")
	var numberedLines []string
	for i, line := range rulesLines {
		if i >= rulesHeight-3 { // Account for header and borders
			break
		}
		lineNum := lipgloss.NewStyle().
			Foreground(lipgloss.Color("241")).
			Width(3).
			Align(lipgloss.Right).
			Render(fmt.Sprintf("%d", i+1))
		
		// Truncate line if too long
		maxLineWidth := rulesWidth - 6 // Account for line numbers and padding
		if len(line) > maxLineWidth && maxLineWidth > 0 {
			line = line[:maxLineWidth-1] + "…"
		}
		
		numberedLines = append(numberedLines, fmt.Sprintf("%s  %s", lineNum, line))
	}
	
	rulesContentFormatted := strings.Join(numberedLines, "\n")
	rulesPanel := rulesStyle.Render(lipgloss.JoinVertical(lipgloss.Left, rulesHeader, rulesContentFormatted))
	
	// Stats panel
	statsStyle := lipgloss.NewStyle().
		Width(rulesWidth).
		Height(statsHeight).
		Border(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("241")).
		Padding(0, 1)
	
	statsPanel := statsStyle.Render(m.renderStats())
	
	// Combine rules and stats vertically
	rightPanel := lipgloss.JoinVertical(lipgloss.Left, rulesPanel, statsPanel)
	
	// Tree panel
	treeStyle := lipgloss.NewStyle().
		Width(treeWidth).
		Height(viewportHeight).
		Padding(0, 1) // top/bottom: 0, left/right: 1
	
	treePanel := treeStyle.Render(tree)
	
	// Combine panels horizontally
	mainContent := lipgloss.JoinHorizontal(lipgloss.Top, treePanel, rightPanel)
	
	// Footer with help hint
	footer := lipgloss.NewStyle().
		Foreground(lipgloss.Color("241")).
		Render("Press ? for help")

	// Combine all parts
	parts := []string{
		header,
		mainContent,
		"",
	}
	
	if statusMsg != "" {
		parts = append(parts, statusMsg, "")
	}
	
	parts = append(parts, footer)
	
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
	style := m.getStyle(node)

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

	// Status symbol
	statusSymbol := m.getStatusSymbol(node)

	// Token count
	tokenStr := ""
	if node.TokenCount > 0 {
		tokenStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("244")) // Dim gray
		tokenStr = tokenStyle.Render(fmt.Sprintf(" (%s)", context.FormatTokenCount(node.TokenCount)))
	}

	// Combine all parts
	line := fmt.Sprintf("%s%s%s%s %s%s%s", cursor, indent, expandIndicator, icon, name, statusSymbol, tokenStr)
	return style.Render(line)
}

// getIcon returns the appropriate icon for a node
func (m *viewModel) getIcon(node *context.FileNode) string {
	if node.IsDir {
		// Return blue-styled directory icon
		dirStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("39")) // Blue
		return dirStyle.Render("📁")
	}
	return "📄"
}

// getStatusSymbol returns the status symbol for a node
func (m *viewModel) getStatusSymbol(node *context.FileNode) string {
	switch node.Status {
	case context.StatusIncludedHot:
		greenStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("34")) // Green
		return greenStyle.Render(" ✓")
	case context.StatusIncludedCold:
		lightBlueStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("117")) // Light blue
		return lightBlueStyle.Render(" ❄️")
	case context.StatusExcludedByRule:
		return " 🚫"
	default:
		return ""
	}
}

// getStyle returns the appropriate style for a status
func (m *viewModel) getStyle(node *context.FileNode) lipgloss.Style {
	// Base style based on status
	var style lipgloss.Style
	switch node.Status {
	case context.StatusIncludedHot:
		style = lipgloss.NewStyle().Foreground(lipgloss.Color("65")) // Dark greenish-grey for hot
	case context.StatusIncludedCold:
		style = lipgloss.NewStyle().Foreground(lipgloss.Color("67")) // Dark bluish-grey for cold
	case context.StatusExcludedByRule:
		style = lipgloss.NewStyle().Foreground(lipgloss.Color("95")) // Dark reddish-grey for excluded
	case context.StatusOmittedNoMatch:
		style = lipgloss.NewStyle().Foreground(lipgloss.Color("240")) // Darker grey for omitted
	case context.StatusDirectory:
		style = lipgloss.NewStyle().Foreground(lipgloss.Color("252")) // Light gray for plain directories
	default:
		style = lipgloss.NewStyle()
	}
	
	// Make directories bold
	if node.IsDir {
		style = style.Bold(true)
	}
	
	return style
}

// renderHelp renders the help popup with legend and navigation
func (m *viewModel) renderHelp() string {
	// Create styles
	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("241")).
		Padding(2, 3).
		Width(70).
		Align(lipgloss.Center)
	
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("12")).
		MarginBottom(1)
	
	columnStyle := lipgloss.NewStyle().
		Width(30).
		MarginRight(4)
	
	keyStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("34")).
		Bold(true)
	
	// Navigation column
	navItems := []string{
		"Navigation:",
		"",
		keyStyle.Render("↑/↓, j/k") + " - Move up/down",
		keyStyle.Render("Enter, Space") + " - Toggle expand",
		keyStyle.Render("gg") + " - Go to top",
		keyStyle.Render("G") + " - Go to bottom",
		keyStyle.Render("Ctrl-D") + " - Page down",
		keyStyle.Render("Ctrl-U") + " - Page up",
		"",
		"Actions:",
		"",
		keyStyle.Render("a") + " - Toggle hot context",
		keyStyle.Render("c") + " - Toggle cold context",
		keyStyle.Render("x") + " - Toggle exclude",
		keyStyle.Render("A") + " - Expand all",
		keyStyle.Render("p") + " - Toggle pruning",
		keyStyle.Render("r") + " - Refresh view",
		"",
		keyStyle.Render("q") + " - Quit",
		keyStyle.Render("?") + " - Toggle this help",
	}
	
	// Legend column
	legendItems := []string{
		"Legend:",
		"",
		"File Types:",
		"  📁 - Directory",
		"  📄 - File",
		"",
		"Context Status:",
		"  " + keyStyle.Render("✓") + " - Hot Context",
		"  " + keyStyle.Render("❄️") + " - Cold Context",  
		"  " + keyStyle.Render("🚫") + " - Excluded",
		"  (none) - Omitted",
		"",
		"Colors:",
		"  Bold - Directories",
		"  Green-grey - Hot items",
		"  Blue-grey - Cold items",
		"  Red-grey - Excluded items",
		"  Grey - Omitted items",
	}
	
	// Render columns
	navColumn := columnStyle.Render(strings.Join(navItems, "\n"))
	legendColumn := columnStyle.Copy().MarginRight(0).Render(strings.Join(legendItems, "\n"))
	
	// Combine columns
	content := lipgloss.JoinHorizontal(lipgloss.Top, navColumn, legendColumn)
	
	// Add title and wrap in box
	title := titleStyle.Render("Grove Context View - Help")
	fullContent := lipgloss.JoinVertical(lipgloss.Center, title, content)
	
	return boxStyle.Render(fullContent)
}

// renderStats renders the context statistics
func (m *viewModel) renderStats() string {
	statsStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("252")).
		Padding(0, 1)
	
	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("12"))
	
	greenStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("34"))
		
	blueStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("117"))
	
	// Format token counts
	hotTokensStr := context.FormatTokenCount(m.hotTokens)
	coldTokensStr := context.FormatTokenCount(m.coldTokens)
	totalTokensStr := context.FormatTokenCount(m.totalTokens)
	
	stats := []string{
		headerStyle.Render("Context Statistics"),
		"",
		fmt.Sprintf("Hot:   %s %d files, %s tokens", 
			greenStyle.Render("✓"), m.hotFiles, greenStyle.Render(hotTokensStr)),
		fmt.Sprintf("Cold:  %s %d files, %s tokens", 
			blueStyle.Render("❄️"), m.coldFiles, blueStyle.Render(coldTokensStr)),
		"",
		fmt.Sprintf("Total: %d files, %s tokens", m.totalFiles, totalTokensStr),
	}
	
	return statsStyle.Render(strings.Join(stats, "\n"))
}

// renderLegend renders the legend (kept for backwards compatibility)
func (m *viewModel) renderLegend() string {
	legendItems := []string{
		"Legend:",
		"  📁/📄 Directory/File",
		"  ✓ Hot Context",
		"  ❄️  Cold Context",
		"  🚫 Excluded by Rule",
		"  (no symbol) Omitted",
	}

	return lipgloss.NewStyle().
		Foreground(lipgloss.Color("241")).
		Render(strings.Join(legendItems, "\n"))
}
