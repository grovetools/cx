package view

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/grovetools/core/config"
	core_theme "github.com/grovetools/core/tui/theme"
	"github.com/grovetools/cx/pkg/context"
	"github.com/grovetools/cx/pkg/context/tree"
)

// --- Page Implementation ---

type treePage struct {
	sharedState *sharedState

	// Tree view state
	tree           *tree.FileNode
	cursor         int
	visibleNodes   []*nodeWithLevel
	expandedPaths  map[string]bool
	scrollOffset   int
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

	// Ruleset selection state
	rulesetSelector *rulesetSelectorState

	// Cursor restoration state
	pathToRestore string
}

// --- Messages ---

type treeLoadedMsg struct {
	tree *tree.FileNode
	err  error
}

type nodeWithLevel struct {
	node  *tree.FileNode
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

type rulesetSelectorState struct {
	node      *tree.FileNode
	action    string // "hot", "cold", "exclude"
	rulesets  []string
	cursor    int
	wsPath    string // workspace path where rulesets were found
	alias     string // the alias prefix (e.g., "grove-ecosystem:grove-core")
}

// --- Constructor ---

func NewTreePage(state *sharedState) Page {
	return &treePage{
		sharedState:    state,
		expandedPaths:  make(map[string]bool),
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
	p.rulesetSelector = nil
}

func (p *treePage) SetSize(width, height int) {
	p.width, p.height = width, height
	p.ensureCursorVisible()
}

// --- Commands ---

func (p *treePage) loadTreeCmd() tea.Cmd {
	return func() tea.Msg {
		manager := context.NewManager("")
		projectTree, err := tree.AnalyzeProjectTree(manager, p.showGitIgnored)
		return treeLoadedMsg{tree: projectTree, err: err}
	}
}

func (p *treePage) toggleRuleCmd(path, targetType string, isDirectory bool) tea.Cmd {
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
				// If the item is currently in cold context, add exclusion to cold section
				if currentStatus == context.RuleCold {
					err = manager.AppendRule(path, "exclude-cold")
				} else {
					err = manager.AppendRule(path, "exclude")
				}
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

func (p *treePage) getAliasForRule(node *tree.FileNode) (string, bool) {
	if p.sharedState.projectProvider == nil {
		return "", false
	}

	// Find the workspace node containing the file/dir.
	wsNode := p.sharedState.projectProvider.FindByPath(node.Path)
	if wsNode == nil || wsNode.RootEcosystemPath == "" || wsNode.IsEcosystem() {
		return "", false // Not part of a known ecosystem or is an ecosystem itself.
	}

	// Get the root ecosystem to build the alias prefix.
	rootEcoNode := p.sharedState.projectProvider.FindByPath(wsNode.RootEcosystemPath)
	if rootEcoNode == nil {
		return "", false
	}
	rootEcoName := rootEcoNode.Name

	var alias string

	// Case 1: Sub-project inside an ecosystem worktree (e.g., root-eco:eco-worktree:project)
	if wsNode.ParentEcosystemPath != "" && wsNode.ParentEcosystemPath != wsNode.RootEcosystemPath {
		ecoWorktreeNode := p.sharedState.projectProvider.FindByPath(wsNode.ParentEcosystemPath)
		if ecoWorktreeNode != nil {
			alias = fmt.Sprintf("%s:%s:%s", rootEcoName, ecoWorktreeNode.Name, wsNode.Name)
		}
	} else if wsNode.IsWorktree() && wsNode.ParentProjectPath != "" {
		// Case 2: Worktree of a sub-project in the root ecosystem (e.g., root-eco:project:worktree)
		parentProjNode := p.sharedState.projectProvider.FindByPath(wsNode.ParentProjectPath)
		if parentProjNode != nil {
			alias = fmt.Sprintf("%s:%s:%s", rootEcoName, parentProjNode.Name, wsNode.Name)
		}
	} else if wsNode.Name != rootEcoName {
		// Case 3: Simple sub-project in the root ecosystem (e.g., root-eco:project)
		// Only create alias if the project name is different from the ecosystem name
		alias = fmt.Sprintf("%s:%s", rootEcoName, wsNode.Name)
	}

	if alias == "" {
		return "", false
	}

	// Calculate the file's path relative to the workspace's root
	relPath, err := filepath.Rel(wsNode.Path, node.Path)
	if err != nil {
		return "", false
	}

	var fullRule strings.Builder
	fullRule.WriteString("@a:")
	fullRule.WriteString(alias)

	// If adding the entire workspace (relPath == "."), try to use the default ruleset
	if relPath == "." && node.IsDir {
		// Try to load the workspace's grove.yml to get the default ruleset
		if cfg, err := config.LoadFrom(wsNode.Path); err == nil && cfg != nil {
			var contextConfig struct {
				DefaultRulesPath string `yaml:"default_rules_path"`
			}
			if err := cfg.UnmarshalExtension("context", &contextConfig); err == nil && contextConfig.DefaultRulesPath != "" {
				// Extract just the filename without extension
				base := filepath.Base(contextConfig.DefaultRulesPath)
				// Remove .rules extension if present
				rulesetName := strings.TrimSuffix(base, ".rules")
				fullRule.WriteString("::")
				fullRule.WriteString(rulesetName)
				return fullRule.String(), true
			}
		}
		// Fallback to /** if no default ruleset found
		fullRule.WriteString("/**")
		return fullRule.String(), true
	}

	// For specific files or subdirectories, add the path
	if relPath != "." {
		fullRule.WriteString("/")
		fullRule.WriteString(filepath.ToSlash(relPath))
	}

	// If the node being added is a directory, append `/**` to include its contents recursively.
	if node.IsDir {
		if !strings.HasSuffix(fullRule.String(), "/**") {
			fullRule.WriteString("/**")
		}
	}

	return fullRule.String(), true
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

		if p.pathToRestore != "" {
			p.restoreCursorPosition()
			p.pathToRestore = "" // Reset for the next action
		}
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
		// Handle ruleset selector first
		if p.rulesetSelector != nil {
			switch msg.String() {
			case "up", "k":
				if p.rulesetSelector.cursor > 0 {
					p.rulesetSelector.cursor--
				}
				return p, nil
			case "down", "j":
				if p.rulesetSelector.cursor < len(p.rulesetSelector.rulesets)-1 {
					p.rulesetSelector.cursor++
				}
				return p, nil
			case "enter":
				// User selected a ruleset
				if len(p.rulesetSelector.rulesets) == 0 {
					// No rulesets available, use /** instead
					rule := fmt.Sprintf("@a:%s/**", p.rulesetSelector.alias)
					action := p.rulesetSelector.action
					node := p.rulesetSelector.node
					p.rulesetSelector = nil
					p.statusMessage = fmt.Sprintf("Adding %s...", rule)
					return p, p.handleRuleAction(rule, action, node.IsDir)
				}
				// Get selected ruleset
				selectedRuleset := p.rulesetSelector.rulesets[p.rulesetSelector.cursor]
				rule := fmt.Sprintf("@a:%s::%s", p.rulesetSelector.alias, selectedRuleset)
				action := p.rulesetSelector.action
				node := p.rulesetSelector.node
				p.rulesetSelector = nil
				p.statusMessage = fmt.Sprintf("Adding %s...", rule)
				return p, p.handleRuleAction(rule, action, node.IsDir)
			case "/":
				// User chose to use /** instead
				rule := fmt.Sprintf("@a:%s/**", p.rulesetSelector.alias)
				action := p.rulesetSelector.action
				node := p.rulesetSelector.node
				p.rulesetSelector = nil
				p.statusMessage = fmt.Sprintf("Adding %s...", rule)
				return p, p.handleRuleAction(rule, action, node.IsDir)
			case "esc":
				// User cancelled
				p.rulesetSelector = nil
				p.statusMessage = "Action cancelled"
				return p, nil
			default:
				// Ignore other keys during selection
				return p, nil
			}
		}

		// Handle confirmation prompts
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
			viewportHeight := p.height
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
			viewportHeight := p.height
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
				p.pathToRestore = node.Path // Store the current item's path

				// Check if we should show ruleset selector for workspace roots
				if p.tryShowRulesetSelector(node, "cold") {
					return p, nil
				}

				rule, err := p.getRulePath(node)
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
				return p, p.handleRuleAction(rule, "cold", node.IsDir)
			}
		case "h":
			// Toggle hot context
			if p.cursor >= len(p.visibleNodes) {
				break
			}
			node := p.visibleNodes[p.cursor].node
			p.pathToRestore = node.Path // Store the current item's path

			// Check if we should show ruleset selector for workspace roots
			if p.tryShowRulesetSelector(node, "hot") {
				return p, nil
			}

			rule, err := p.getRulePath(node)
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
			return p, p.handleRuleAction(rule, "hot", node.IsDir)
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
			p.pathToRestore = node.Path // Store the current item's path

			// Check if we should show ruleset selector for workspace roots
			if p.tryShowRulesetSelector(node, "exclude") {
				return p, nil
			}

			rule, err := p.getRulePath(node)
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
			return p, p.handleRuleAction(rule, "exclude", node.IsDir)
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

	// Calculate viewport height. p.height is the available space from the pager.
	viewportHeight := p.height
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
		searchStyle := core_theme.DefaultTheme.Bold
		content += "\n" + searchStyle.Render(fmt.Sprintf("/%s_", p.searchQuery))
	} else if len(p.searchResults) > 0 {
		resultsStyle := core_theme.DefaultTheme.Muted
		content += "\n" + resultsStyle.Render(fmt.Sprintf("Found %d results (%d of %d) - n/N to navigate",
			len(p.searchResults), p.searchCursor+1, len(p.searchResults)))
	}

	// Show confirmation prompt if present
	if p.pendingConfirm != nil {
		confirmStyle := core_theme.DefaultTheme.Warning
		content += "\n" + confirmStyle.Render(fmt.Sprintf("%s - Press 'y' to confirm, 'n' to cancel", p.pendingConfirm.warning))
	}

	// Show ruleset selector overlay if active
	if p.rulesetSelector != nil {
		content = p.renderRulesetSelector(content)
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

func (p *treePage) collectVisibleNodes(node *tree.FileNode, level int) {
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
	viewportHeight := p.height
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

func (p *treePage) expandAllRecursive(node *tree.FileNode) {
	if node.IsDir && len(node.Children) > 0 {
		p.expandedPaths[node.Path] = true
		for _, child := range node.Children {
			p.expandAllRecursive(child)
		}
	}
}

func (p *treePage) autoExpandToContent(node *tree.FileNode) {
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
		cursor = core_theme.IconArrowRightBold + " "
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

	// Status symbol
	statusSymbol := p.getStatusSymbol(node)

	// Check if this path would be dangerous if added
	relPath, _ := p.getFilePathRule(node)
	isDangerous, _ := p.isPathPotentiallyDangerous(relPath)
	dangerSymbol := ""
	if isDangerous && node.Status == context.StatusOmittedNoMatch {
		dangerSymbol = " [!]"
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
			tokenStyle = core_theme.DefaultTheme.Muted // Dim gray for < 10K
		}
		tokenStr = tokenStyle.Render(fmt.Sprintf(" (%s)", context.FormatTokenCount(node.TokenCount)))
	}

	// Combine all parts (no expansion indicator - folder icon shows open/closed state)
	line := fmt.Sprintf("%s%s%s %s%s%s%s", cursor, indent, icon, name, statusSymbol, dangerSymbol, tokenStr)
	return style.Render(line)
}

// restoreCursorPosition finds the new index for the cursor after a refresh.
// It first tries to find an exact match for the previously selected path.
// If not found, it walks up the directory tree to find the closest visible parent.
func (p *treePage) restoreCursorPosition() {
	if p.pathToRestore == "" || len(p.visibleNodes) == 0 {
		return
	}

	// 1. Try to find an exact match.
	for i, vn := range p.visibleNodes {
		if vn.node.Path == p.pathToRestore {
			p.cursor = i
			p.ensureCursorVisible()
			return
		}
	}

	// 2. If not found, walk up the path to find the nearest visible parent.
	parentPath := filepath.Dir(p.pathToRestore)
	for {
		for i, vn := range p.visibleNodes {
			if vn.node.Path == parentPath {
				p.cursor = i
				p.ensureCursorVisible()
				return
			}
		}
		newParent := filepath.Dir(parentPath)
		if newParent == parentPath || newParent == "." || newParent == "/" {
			break // Reached the filesystem root.
		}
		parentPath = newParent
	}
}

func (p *treePage) getIcon(node *tree.FileNode) string {
	if node.IsDir {
		// Show tree icon for the root node
		if p.tree != nil && node.Path == p.tree.Path {
			return core_theme.IconFolderTree
		}
		// Show remove variant for excluded items
		if node.Status == context.StatusExcludedByRule {
			return core_theme.IconFolderRemove
		}
		// Show plus variant for hot context items
		if node.Status == context.StatusIncludedHot {
			return core_theme.IconFolderPlus
		}
		// Show open folder if expanded, closed folder otherwise
		if p.expandedPaths[node.Path] {
			return core_theme.IconFolderOpen
		}
		return core_theme.IconFolder
	}
	// Show cancel variant for excluded files
	if node.Status == context.StatusExcludedByRule {
		return core_theme.IconFileCancel
	}
	// Show plus variant for hot context files
	if node.Status == context.StatusIncludedHot {
		return core_theme.IconFilePlus
	}
	return core_theme.IconFile
}

func (p *treePage) getStatusSymbol(node *tree.FileNode) string {
	switch node.Status {
	case context.StatusIncludedHot:
		greenStyle := core_theme.DefaultTheme.Success // Green
		return greenStyle.Render(" " + core_theme.IconSuccess)
	case context.StatusIncludedCold:
		lightBlueStyle := core_theme.DefaultTheme.Accent // Light blue
		return lightBlueStyle.Render(" " + core_theme.IconPineTreeBox) // Using PineTreeBox for "cold"
	case context.StatusExcludedByRule:
		return " " + core_theme.IconStatusBlocked
	case context.StatusIgnoredByGit:
		return " " + core_theme.IconGit // Using Git icon for gitignored
	default:
		return ""
	}
}

func (p *treePage) getStyle(node *tree.FileNode) lipgloss.Style {
	theme := core_theme.DefaultTheme
	// Base style based on status
	var style lipgloss.Style
	switch node.Status {
	case context.StatusIncludedHot:
		style = theme.Success
	case context.StatusIncludedCold:
		style = theme.Info
	case context.StatusExcludedByRule:
		style = theme.Error
	case context.StatusOmittedNoMatch:
		style = theme.Muted
	case context.StatusDirectory:
		// Directories use bold for emphasis without explicit color
		style = lipgloss.NewStyle().Bold(true)
	case context.StatusIgnoredByGit:
		style = theme.Muted
	default:
		style = lipgloss.NewStyle()
	}

	// Make directories bold
	if node.IsDir {
		style = style.Bold(true)
	}

	return style
}

// getFilePathRule generates a context rule using a relative or absolute file path.
// This is the fallback when an alias cannot be generated.
func (p *treePage) getFilePathRule(node *tree.FileNode) (string, error) {
	manager := context.NewManager("")
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

// getRulePath generates the most appropriate context rule for a file or directory.
// It prefers generating an alias-based rule and falls back to a file path-based rule.
func (p *treePage) getRulePath(node *tree.FileNode) (string, error) {
	if alias, ok := p.getAliasForRule(node); ok {
		return alias, nil
	}
	return p.getFilePathRule(node)
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
		p.statusMessage = fmt.Sprintf("%s - Press 'y' to confirm, 'n' to cancel", warning)
		return nil
	}

	// Path is safe, proceed directly
	return p.toggleRuleCmd(relPath, action, isDir)
}

// tryShowRulesetSelector checks if we should show the ruleset selector for this node.
// Returns true if selector was shown, false if normal action should proceed.
func (p *treePage) tryShowRulesetSelector(node *tree.FileNode, action string) bool {
	// Only show selector for workspace roots
	if p.sharedState.projectProvider == nil {
		return false
	}

	wsNode := p.sharedState.projectProvider.FindByPath(node.Path)
	if wsNode == nil || wsNode.RootEcosystemPath == "" || wsNode.IsEcosystem() {
		return false
	}

	// Check if this is the workspace root
	relPath, err := filepath.Rel(wsNode.Path, node.Path)
	if err != nil || relPath != "." {
		return false // Not the workspace root
	}

	// Get the alias for this workspace
	rootEcoNode := p.sharedState.projectProvider.FindByPath(wsNode.RootEcosystemPath)
	if rootEcoNode == nil {
		return false
	}
	rootEcoName := rootEcoNode.Name

	var alias string
	if wsNode.ParentEcosystemPath != "" && wsNode.ParentEcosystemPath != wsNode.RootEcosystemPath {
		ecoWorktreeNode := p.sharedState.projectProvider.FindByPath(wsNode.ParentEcosystemPath)
		if ecoWorktreeNode != nil {
			alias = fmt.Sprintf("%s:%s:%s", rootEcoName, ecoWorktreeNode.Name, wsNode.Name)
		}
	} else if wsNode.IsWorktree() && wsNode.ParentProjectPath != "" {
		parentProjNode := p.sharedState.projectProvider.FindByPath(wsNode.ParentProjectPath)
		if parentProjNode != nil {
			alias = fmt.Sprintf("%s:%s:%s", rootEcoName, parentProjNode.Name, wsNode.Name)
		}
	} else if wsNode.Name != rootEcoName {
		alias = fmt.Sprintf("%s:%s", rootEcoName, wsNode.Name)
	}

	if alias == "" {
		return false
	}

	// Discover available rulesets
	rulesets := p.discoverRulesets(wsNode.Path)

	// Show the selector
	p.rulesetSelector = &rulesetSelectorState{
		node:     node,
		action:   action,
		rulesets: rulesets,
		cursor:   0,
		wsPath:   wsNode.Path,
		alias:    alias,
	}

	p.statusMessage = "Select a ruleset..."
	return true
}

// discoverRulesets finds all available .rules files in .cx/ and .grove/ directories
func (p *treePage) discoverRulesets(wsPath string) []string {
	var rulesets []string

	// Check both .cx/ and .grove/ directories
	for _, dir := range []string{".cx", ".grove"} {
		rulesDir := filepath.Join(wsPath, dir)
		entries, err := os.ReadDir(rulesDir)
		if err != nil {
			continue
		}

		for _, entry := range entries {
			if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".rules") {
				// Extract ruleset name (remove .rules extension)
				name := strings.TrimSuffix(entry.Name(), ".rules")
				rulesets = append(rulesets, name)
			}
		}
	}

	return rulesets
}

// renderRulesetSelector renders the ruleset selection popup to the right of the tree
func (p *treePage) renderRulesetSelector(content string) string {
	if p.rulesetSelector == nil {
		return content
	}

	theme := core_theme.DefaultTheme

	// Build the popup content
	var popup strings.Builder

	title := fmt.Sprintf("Select Ruleset for @a:%s", p.rulesetSelector.alias)
	popup.WriteString(theme.Bold.Render(title) + "\n\n")

	if len(p.rulesetSelector.rulesets) == 0 {
		popup.WriteString(theme.Muted.Render("No rulesets found.") + "\n")
		popup.WriteString(theme.Muted.Render("Using /** instead.") + "\n\n")
		popup.WriteString(theme.Muted.Render("Press any key to continue..."))
	} else {
		for i, ruleset := range p.rulesetSelector.rulesets {
			cursor := "  "
			if i == p.rulesetSelector.cursor {
				cursor = "> "
			}

			line := fmt.Sprintf("%s%s", cursor, ruleset)
			if i == p.rulesetSelector.cursor {
				popup.WriteString(theme.Highlight.Render(line) + "\n")
			} else {
				popup.WriteString(line + "\n")
			}
		}

		popup.WriteString("\n")
		popup.WriteString(theme.Muted.Render("↑/↓ Navigate") + "\n")
		popup.WriteString(theme.Muted.Render("Enter Select") + "\n")
		popup.WriteString(theme.Muted.Render("/ Use /**") + "\n")
		popup.WriteString(theme.Muted.Render("Esc Cancel"))
	}

	// Create a box around the popup
	popupContent := popup.String()
	popupStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("62")).
		Padding(1, 2).
		Width(45).
		Height(p.height - 4) // Match the tree height

	renderedPopup := popupStyle.Render(popupContent)

	// Join the tree content and popup side-by-side
	return lipgloss.JoinHorizontal(lipgloss.Top, content, "  ", renderedPopup)
}
