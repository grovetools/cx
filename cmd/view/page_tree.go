package view

import (
	"fmt"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	core_theme "github.com/mattsolo1/grove-core/tui/theme"
	grove_context "github.com/mattsolo1/grove-context/pkg/context"
)

// --- Page Implementation ---

type treePage struct {
	sharedState *sharedState

	// Tree view state
	tree          *grove_context.FileNode
	cursor        int
	visibleNodes  []*nodeWithLevel
	expandedPaths map[string]bool
	scrollOffset  int
	pruning       bool
	showGitIgnored bool

	// Search state
	searchQuery   string
	isSearching   bool
	searchResults []int
	searchCursor  int

	// Other state
	width, height int
	lastKey       string
	statusMessage string

	// Confirmation state
	pendingConfirm *confirmActionMsg
}

// --- Messages ---

type treeLoadedMsg struct {
	tree *grove_context.FileNode
	err  error
}

type nodeWithLevel struct {
	node  *grove_context.FileNode
	level int
}

type confirmActionMsg struct {
	action      string // "hot", "cold", "exclude"
	path        string
	isDirectory bool
	warning     string
}

type ruleChangeResultMsg struct {
	err           error
	successMsg    string
	refreshNeeded bool
}

// --- Constructor ---

func NewTreePage(state *sharedState) Page {
	return &treePage{
		sharedState:    state,
		expandedPaths:  make(map[string]bool),
		pruning:        false,
		showGitIgnored: false,
	}
}

// --- Page Interface ---

func (p *treePage) Name() string { return "tree" }

func (p *treePage) Keys() interface{} {
	return treeKeys
}

func (p *treePage) Init() tea.Cmd {
	return p.loadTreeCmd()
}

func (p *treePage) Focus() tea.Cmd {
	// When focused, refresh the tree view in case rules have changed
	p.statusMessage = "Refreshing tree..."
	return p.loadTreeCmd()
}

func (p *treePage) Blur() {
	p.isSearching = false
	p.searchQuery = ""
	p.searchResults = nil
	p.pendingConfirm = nil
}

func (p *treePage) SetSize(width, height int) {
	p.width, p.height = width, height
	p.ensureCursorVisible()
}

// --- Commands ---

func (p *treePage) loadTreeCmd() tea.Cmd {
	return func() tea.Msg {
		manager := grove_context.NewManager("")
		tree, err := manager.AnalyzeProjectTree(p.pruning, p.showGitIgnored)
		return treeLoadedMsg{tree: tree, err: err}
	}
}

func (p *treePage) toggleRuleCmd(path, targetType string, isDirectory bool) tea.Cmd {
	return func() tea.Msg {
		manager := grove_context.NewManager("")

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
			case grove_context.RuleNotFound, grove_context.RuleCold, grove_context.RuleExcluded:
				// Remove existing rule first if it exists
				if currentStatus != grove_context.RuleNotFound {
					if removeErr := manager.RemoveRule(path); removeErr != nil {
						return ruleChangeResultMsg{err: removeErr, refreshNeeded: false}
					}
				}
				// Add to hot context
				err = manager.AppendRule(path, "hot")
				successMsg = fmt.Sprintf("Added %s to hot context: %s", itemType, path)
			case grove_context.RuleHot:
				// Remove from hot context
				err = manager.RemoveRule(path)
				successMsg = fmt.Sprintf("Removed %s from hot context: %s", itemType, path)
			}
		case "cold":
			switch currentStatus {
			case grove_context.RuleNotFound, grove_context.RuleHot, grove_context.RuleExcluded:
				// Remove existing rule first if it exists
				if currentStatus != grove_context.RuleNotFound {
					if removeErr := manager.RemoveRule(path); removeErr != nil {
						return ruleChangeResultMsg{err: removeErr, refreshNeeded: false}
					}
				}
				// Add to cold context
				err = manager.AppendRule(path, "cold")
				successMsg = fmt.Sprintf("Added %s to cold context: %s", itemType, path)
			case grove_context.RuleCold:
				// Remove from cold context
				err = manager.RemoveRule(path)
				successMsg = fmt.Sprintf("Removed %s from cold context: %s", itemType, path)
			}
		case "exclude":
			switch currentStatus {
			case grove_context.RuleNotFound, grove_context.RuleHot, grove_context.RuleCold:
				// Remove existing rule first if it exists
				if currentStatus != grove_context.RuleNotFound {
					if removeErr := manager.RemoveRule(path); removeErr != nil {
						return ruleChangeResultMsg{err: removeErr, refreshNeeded: false}
					}
				}
				// Add exclusion rule
				// If the item is currently in cold context, add exclusion to cold section
				if currentStatus == grove_context.RuleCold {
					err = manager.AppendRule(path, "exclude-cold")
				} else {
					err = manager.AppendRule(path, "exclude")
				}
				successMsg = fmt.Sprintf("Excluded %s: %s", itemType, path)
			case grove_context.RuleExcluded:
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

// --- Update ---

func (p *treePage) Update(msg tea.Msg) (Page, tea.Cmd) {
	switch msg := msg.(type) {
	case treeLoadedMsg:
		p.statusMessage = ""
		if msg.err != nil {
			p.sharedState.err = msg.err
			return p, nil
		}
		p.tree = msg.tree
		if p.tree != nil {
			p.expandedPaths[p.tree.Path] = true
			p.autoExpandToContent(p.tree)
		}
		p.updateVisibleNodes()
		return p, nil

	case ruleChangeResultMsg:
		if msg.err != nil {
			p.statusMessage = fmt.Sprintf("Error: %v", msg.err)
			return p, nil
		}
		p.statusMessage = msg.successMsg
		if msg.refreshNeeded {
			// Reload shared state and tree
			return p, tea.Batch(refreshSharedStateCmd(), p.loadTreeCmd())
		}
		return p, nil

	case tea.KeyMsg:
		// Handle confirmation prompts first
		if p.pendingConfirm != nil {
			switch msg.String() {
			case "y", "Y":
				// User confirmed, execute the action
				action := p.pendingConfirm.action
				path := p.pendingConfirm.path
				isDir := p.pendingConfirm.isDirectory
				p.pendingConfirm = nil
				p.statusMessage = fmt.Sprintf("Adding %s to %s context...", path, action)
				return p, p.toggleRuleCmd(path, action, isDir)
			case "n", "N", "esc":
				// User cancelled
				p.pendingConfirm = nil
				p.statusMessage = "Action cancelled"
				return p, nil
			default:
				// Ignore other keys during confirmation
				return p, nil
			}
		}

		// Handle search mode keys
		if p.isSearching {
			switch msg.String() {
			case "enter":
				// Finish search and find results
				p.isSearching = false
				p.performSearch()
				if len(p.searchResults) > 0 {
					p.searchCursor = 0
					p.cursor = p.searchResults[0]
					p.ensureCursorVisible()
				}
				return p, nil
			case "esc":
				// Cancel search
				p.isSearching = false
				p.searchQuery = ""
				return p, nil
			case "backspace":
				// Remove last character from search query
				if len(p.searchQuery) > 0 {
					p.searchQuery = p.searchQuery[:len(p.searchQuery)-1]
				}
				return p, nil
			default:
				// Add character to search query
				if len(msg.String()) == 1 {
					p.searchQuery += msg.String()
				}
				return p, nil
			}
		}

		// Normal mode logic
		switch msg.String() {
		case "up", "k":
			if p.cursor > 0 {
				p.cursor--
				p.ensureCursorVisible()
			}
		case "down", "j":
			if p.cursor < len(p.visibleNodes)-1 {
				p.cursor++
				p.ensureCursorVisible()
			}
		case "enter", " ":
			p.toggleExpanded()
		case "pgup":
			p.cursor -= 10
			if p.cursor < 0 {
				p.cursor = 0
			}
			p.ensureCursorVisible()
		case "pgdown":
			p.cursor += 10
			if p.cursor >= len(p.visibleNodes) {
				p.cursor = len(p.visibleNodes) - 1
			}
			p.ensureCursorVisible()
		case "home":
			p.cursor = 0
			p.scrollOffset = 0
		case "end":
			p.cursor = len(p.visibleNodes) - 1
			p.ensureCursorVisible()
		case "ctrl+d":
			// Scroll down half a page
			viewportHeight := p.height - 4
			if viewportHeight < 1 {
				viewportHeight = 1
			}
			p.cursor += viewportHeight / 2
			if p.cursor >= len(p.visibleNodes) {
				p.cursor = len(p.visibleNodes) - 1
			}
			p.ensureCursorVisible()
		case "ctrl+u":
			// Scroll up half a page
			viewportHeight := p.height - 4
			if viewportHeight < 1 {
				viewportHeight = 1
			}
			p.cursor -= viewportHeight / 2
			if p.cursor < 0 {
				p.cursor = 0
			}
			p.ensureCursorVisible()
		case "z":
			p.lastKey = "z"
		case "R":
			if p.lastKey == "z" {
				// zR - expand all directories (vim-style)
				p.expandAll()
				p.lastKey = ""
			}
		case "M":
			if p.lastKey == "z" {
				// zM - collapse all directories (vim-style)
				p.collapseAll()
				p.lastKey = ""
			}
		case "o":
			if p.lastKey == "z" {
				// zo - open/expand current directory (vim-style)
				p.expandCurrent()
				p.lastKey = ""
			}
		case "c":
			if p.lastKey == "z" {
				// zc - close/collapse current directory (vim-style)
				p.collapseCurrent()
				p.lastKey = ""
			} else {
				// Regular 'c' - Toggle cold context
				if p.cursor >= len(p.visibleNodes) {
					break
				}
				node := p.visibleNodes[p.cursor].node
				relPath, err := p.getRelativePath(node)
				if err != nil {
					p.statusMessage = fmt.Sprintf("Error: %v", err)
					break
				}
				// Create appropriate status message for directories vs files
				var itemType string
				if node.IsDir {
					itemType = "directory tree"
				} else {
					itemType = "file"
				}
				p.statusMessage = fmt.Sprintf("Checking %s %s...", itemType, node.Name)
				return p, p.handleRuleAction(relPath, "cold", node.IsDir)
			}
		case "h":
			// Toggle hot context
			if p.cursor >= len(p.visibleNodes) {
				break
			}
			node := p.visibleNodes[p.cursor].node
			relPath, err := p.getRelativePath(node)
			if err != nil {
				p.statusMessage = fmt.Sprintf("Error: %v", err)
				break
			}
			var itemType string
			if node.IsDir {
				itemType = "directory tree"
			} else {
				itemType = "file"
			}
			p.statusMessage = fmt.Sprintf("Checking %s %s...", itemType, node.Name)
			return p, p.handleRuleAction(relPath, "hot", node.IsDir)
		case "a":
			if p.lastKey == "z" {
				// za - toggle fold at cursor (vim-style)
				p.toggleExpanded()
				p.lastKey = ""
			} else {
				// Clear lastKey if 'a' pressed without 'z'
				p.lastKey = ""
			}
		case "x":
			// Toggle exclude
			if p.cursor >= len(p.visibleNodes) {
				break
			}
			node := p.visibleNodes[p.cursor].node
			relPath, err := p.getRelativePath(node)
			if err != nil {
				p.statusMessage = fmt.Sprintf("Error: %v", err)
				break
			}
			// Create appropriate status message for directories vs files
			var itemType string
			if node.IsDir {
				itemType = "directory tree"
			} else {
				itemType = "file"
			}
			p.statusMessage = fmt.Sprintf("Checking %s %s...", itemType, node.Name)
			return p, p.handleRuleAction(relPath, "exclude", node.IsDir)
		case "g":
			if p.lastKey == "g" {
				// gg - go to top
				p.cursor = 0
				p.scrollOffset = 0
				p.lastKey = ""
			} else {
				p.lastKey = "g"
			}
		case "G":
			// Go to bottom
			p.cursor = len(p.visibleNodes) - 1
			p.ensureCursorVisible()
		case "p":
			// Toggle pruning mode
			p.pruning = !p.pruning
			if p.pruning {
				p.statusMessage = "Pruning mode enabled - showing only directories with context files"
			} else {
				p.statusMessage = "Pruning mode disabled - showing all directories"
			}
			return p, p.loadTreeCmd()
		case ".", "H":
			// Toggle gitignored files visibility
			p.showGitIgnored = !p.showGitIgnored
			if p.showGitIgnored {
				p.statusMessage = "Showing gitignored files"
			} else {
				p.statusMessage = "Hiding gitignored files"
			}
			return p, p.loadTreeCmd()
		case "r":
			// Refresh both tree and rules
			p.statusMessage = "Refreshing..."
			return p, tea.Batch(refreshSharedStateCmd(), p.loadTreeCmd())
		case "/":
			// Enter search mode
			p.isSearching = true
			p.searchQuery = ""
			p.searchResults = nil
			p.searchCursor = 0
			return p, nil
		case "n":
			// Navigate to next search result
			if len(p.searchResults) > 0 {
				p.searchCursor = (p.searchCursor + 1) % len(p.searchResults)
				p.cursor = p.searchResults[p.searchCursor]
				p.ensureCursorVisible()
			}
			return p, nil
		case "N":
			// Navigate to previous search result
			if len(p.searchResults) > 0 {
				p.searchCursor--
				if p.searchCursor < 0 {
					p.searchCursor = len(p.searchResults) - 1
				}
				p.cursor = p.searchResults[p.searchCursor]
				p.ensureCursorVisible()
			}
			return p, nil
		default:
			p.lastKey = ""
		}
	}

	return p, nil
}

// --- View ---

func (p *treePage) View() string {
	if p.tree == nil {
		return "Loading tree..."
	}

	var b strings.Builder

	// Calculate viewport height
	viewportHeight := p.height - 4 // Room for header/footer
	if viewportHeight < 1 {
		viewportHeight = 1
	}

	// Render visible nodes
	for i := p.scrollOffset; i < len(p.visibleNodes) && i < p.scrollOffset+viewportHeight; i++ {
		line := p.renderNode(i)
		b.WriteString(line + "\n")
	}

	content := b.String()

	// Show status message if present
	if p.statusMessage != "" {
		statusStyle := core_theme.DefaultTheme.Success
		if strings.Contains(p.statusMessage, "Error") {
			statusStyle = core_theme.DefaultTheme.Error
		}
		content += "\n" + statusStyle.Render(p.statusMessage)
	}

	// Show search bar if searching
	if p.isSearching {
		searchStyle := lipgloss.NewStyle().
			Foreground(core_theme.DefaultTheme.Success.GetForeground()).
			Bold(true)
		content += "\n" + searchStyle.Render(fmt.Sprintf("/%s_", p.searchQuery))
	} else if len(p.searchResults) > 0 {
		resultsStyle := lipgloss.NewStyle().
			Foreground(core_theme.DefaultTheme.Success.GetForeground())
		content += "\n" + resultsStyle.Render(fmt.Sprintf("Found %d results (%d of %d) - n/N to navigate",
			len(p.searchResults), p.searchCursor+1, len(p.searchResults)))
	}

	// Show confirmation prompt if present
	if p.pendingConfirm != nil {
		confirmStyle := core_theme.DefaultTheme.Warning
		content += "\n" + confirmStyle.Render(fmt.Sprintf("⚠️  %s - Press 'y' to confirm, 'n' to cancel", p.pendingConfirm.warning))
	}

	return content
}

// --- Helper Functions ---

func (p *treePage) updateVisibleNodes() {
	p.visibleNodes = nil
	if p.tree != nil {
		// Start with the children of the root, not the root itself
		for _, child := range p.tree.Children {
			p.collectVisibleNodes(child, 0)
		}
	}

	// Ensure cursor is within bounds
	if p.cursor >= len(p.visibleNodes) {
		p.cursor = len(p.visibleNodes) - 1
	}
	if p.cursor < 0 {
		p.cursor = 0
	}
}

func (p *treePage) collectVisibleNodes(node *grove_context.FileNode, level int) {
	p.visibleNodes = append(p.visibleNodes, &nodeWithLevel{
		node:  node,
		level: level,
	})

	if node.IsDir && p.expandedPaths[node.Path] {
		for _, child := range node.Children {
			p.collectVisibleNodes(child, level+1)
		}
	}
}

func (p *treePage) ensureCursorVisible() {
	viewportHeight := p.height - 4
	if viewportHeight < 1 {
		viewportHeight = 1
	}

	if p.cursor < p.scrollOffset {
		p.scrollOffset = p.cursor
	} else if p.cursor >= p.scrollOffset+viewportHeight {
		p.scrollOffset = p.cursor - viewportHeight + 1
	}
}

func (p *treePage) toggleExpanded() {
	if p.cursor >= len(p.visibleNodes) {
		return
	}

	current := p.visibleNodes[p.cursor]
	if current.node.IsDir && len(current.node.Children) > 0 {
		if p.expandedPaths[current.node.Path] {
			delete(p.expandedPaths, current.node.Path)
		} else {
			p.expandedPaths[current.node.Path] = true
		}
		p.updateVisibleNodes()
	}
}

func (p *treePage) expandAll() {
	if p.tree == nil {
		return
	}
	p.expandAllRecursive(p.tree)
	p.updateVisibleNodes()
}

func (p *treePage) collapseAll() {
	p.expandedPaths = make(map[string]bool)
	// Keep root expanded
	if p.tree != nil {
		p.expandedPaths[p.tree.Path] = true
	}
	p.updateVisibleNodes()
}

func (p *treePage) expandCurrent() {
	if p.cursor >= len(p.visibleNodes) {
		return
	}
	node := p.visibleNodes[p.cursor].node
	if node.IsDir && len(node.Children) > 0 {
		p.expandedPaths[node.Path] = true
		p.updateVisibleNodes()
	}
}

func (p *treePage) collapseCurrent() {
	if p.cursor >= len(p.visibleNodes) {
		return
	}
	node := p.visibleNodes[p.cursor].node
	if node.IsDir {
		delete(p.expandedPaths, node.Path)
		p.updateVisibleNodes()
	}
}

func (p *treePage) expandAllRecursive(node *grove_context.FileNode) {
	if node.IsDir && len(node.Children) > 0 {
		p.expandedPaths[node.Path] = true
		for _, child := range node.Children {
			p.expandAllRecursive(child)
		}
	}
}

func (p *treePage) autoExpandToContent(node *grove_context.FileNode) {
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
		p.expandedPaths[node.Path] = true
		// Continue expanding single-child chains
		if len(node.Children) == 1 && dirCount == 1 {
			for _, child := range node.Children {
				if child.IsDir {
					p.autoExpandToContent(child)
				}
			}
		}
	}
}

func (p *treePage) performSearch() {
	p.searchResults = []int{}
	if p.searchQuery == "" {
		return
	}

	query := strings.ToLower(p.searchQuery)
	for i, nl := range p.visibleNodes {
		if strings.Contains(strings.ToLower(nl.node.Name), query) {
			p.searchResults = append(p.searchResults, i)
		}
	}
}

func (p *treePage) renderNode(index int) string {
	if index >= len(p.visibleNodes) {
		return ""
	}

	nl := p.visibleNodes[index]
	node := nl.node
	level := nl.level

	// Indentation
	indent := strings.Repeat("  ", level)

	// Cursor indicator
	cursor := "  "
	if index == p.cursor {
		cursor = "> "
	}

	// Icon and name
	icon := p.getIcon(node)
	name := node.Name

	// Style based on status
	style := p.getStyle(node)

	// Highlight if this is a search match
	isSearchMatch := false
	for _, resultIndex := range p.searchResults {
		if resultIndex == index {
			isSearchMatch = true
			break
		}
	}
	if isSearchMatch && len(p.searchResults) > 0 {
		// Apply inverted style for search matches
		style = style.Reverse(true)
	}

	// Directory expansion indicator
	expandIndicator := ""
	if node.IsDir && len(node.Children) > 0 {
		if p.expandedPaths[node.Path] {
			expandIndicator = "▼ "
		} else {
			expandIndicator = "▶ "
		}
	} else if node.IsDir {
		expandIndicator = "  "
	}

	// Status symbol
	statusSymbol := p.getStatusSymbol(node)

	// Check if this path would be dangerous if added
	relPath, _ := p.getRelativePath(node)
	isDangerous, _ := p.isPathPotentiallyDangerous(relPath)
	dangerSymbol := ""
	if isDangerous && node.Status == grove_context.StatusOmittedNoMatch {
		dangerSymbol = " ⚠️"
	}

	// Token count with color based on size
	tokenStr := ""
	if node.TokenCount > 0 {
		var tokenStyle lipgloss.Style
		if node.TokenCount >= 100000 {
			tokenStyle = core_theme.DefaultTheme.Error // Red for 100K+
		} else if node.TokenCount >= 50000 {
			tokenStyle = core_theme.DefaultTheme.Warning // Orange for 50K+
		} else if node.TokenCount >= 10000 {
			tokenStyle = core_theme.DefaultTheme.Highlight // Yellow for 10K+
		} else {
			tokenStyle = core_theme.DefaultTheme.Faint // Dim gray for < 10K
		}
		tokenStr = tokenStyle.Render(fmt.Sprintf(" (%s)", grove_context.FormatTokenCount(node.TokenCount)))
	}

	// Combine all parts
	line := fmt.Sprintf("%s%s%s%s %s%s%s%s", cursor, indent, expandIndicator, icon, name, statusSymbol, dangerSymbol, tokenStr)
	return style.Render(line)
}

func (p *treePage) getIcon(node *grove_context.FileNode) string {
	if node.IsDir {
		// Return blue-styled directory icon
		dirStyle := core_theme.DefaultTheme.Info // Blue
		return dirStyle.Render("📁")
	}
	return "📄"
}

func (p *treePage) getStatusSymbol(node *grove_context.FileNode) string {
	switch node.Status {
	case grove_context.StatusIncludedHot:
		greenStyle := core_theme.DefaultTheme.Success // Green
		return greenStyle.Render(" ✓")
	case grove_context.StatusIncludedCold:
		lightBlueStyle := core_theme.DefaultTheme.Accent // Light blue
		return lightBlueStyle.Render(" ❄️")
	case grove_context.StatusExcludedByRule:
		return " 🚫"
	case grove_context.StatusIgnoredByGit:
		return " 🙈" // Git ignored
	default:
		return ""
	}
}

func (p *treePage) getStyle(node *grove_context.FileNode) lipgloss.Style {
	theme := core_theme.DefaultTheme
	// Base style based on status
	var style lipgloss.Style
	switch node.Status {
	case grove_context.StatusIncludedHot:
		style = lipgloss.NewStyle().Foreground(theme.Colors.Green)
	case grove_context.StatusIncludedCold:
		style = lipgloss.NewStyle().Foreground(theme.Colors.Cyan)
	case grove_context.StatusExcludedByRule:
		style = lipgloss.NewStyle().Foreground(theme.Colors.Red)
	case grove_context.StatusOmittedNoMatch:
		style = lipgloss.NewStyle().Foreground(theme.Colors.MutedText)
	case grove_context.StatusDirectory:
		style = lipgloss.NewStyle().Foreground(theme.Colors.LightText)
	case grove_context.StatusIgnoredByGit:
		style = lipgloss.NewStyle().Foreground(core_theme.DefaultTheme.Muted.GetForeground()) // Very dark grey for gitignored
	default:
		style = lipgloss.NewStyle()
	}

	// Make directories bold
	if node.IsDir {
		style = style.Bold(true)
	}

	return style
}

func (p *treePage) getRelativePath(node *grove_context.FileNode) (string, error) {
	manager := grove_context.NewManager("")
	workDir := manager.GetWorkDir()

	// Get relative path from current working directory
	relPath, err := filepath.Rel(workDir, node.Path)

	var basePath string
	if err == nil && !strings.HasPrefix(relPath, "..") {
		// Path is inside the project, use relative path for portability
		basePath = filepath.ToSlash(relPath)
	} else {
		// Path is outside the project, use absolute path to avoid "../" issues
		basePath = filepath.ToSlash(node.Path)
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

func (p *treePage) isPathPotentiallyDangerous(path string) (bool, string) {
	// Count parent traversals
	traversalCount := strings.Count(path, "../")
	if traversalCount > 1 {
		return true, fmt.Sprintf("Path goes up %d directories", traversalCount)
	}

	// Check for overly broad patterns
	if strings.HasPrefix(path, "../**") || strings.HasPrefix(path, "../../**") {
		return true, "Pattern could include many unintended files"
	}

	// Check if it's outside the current directory tree
	if strings.HasPrefix(path, "..") {
		return true, "Path is outside current project directory"
	}

	return false, ""
}

func (p *treePage) handleRuleAction(relPath string, action string, isDir bool) tea.Cmd {
	// Check if path is potentially dangerous
	isDangerous, warning := p.isPathPotentiallyDangerous(relPath)

	if isDangerous {
		// Set up confirmation
		p.pendingConfirm = &confirmActionMsg{
			action:      action,
			path:        relPath,
			isDirectory: isDir,
			warning:     warning,
		}
		p.statusMessage = fmt.Sprintf("⚠️  %s - Press 'y' to confirm, 'n' to cancel", warning)
		return nil
	}

	// Path is safe, proceed directly
	return p.toggleRuleCmd(relPath, action, isDir)
}
