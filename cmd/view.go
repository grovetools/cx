package cmd

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/mattsolo1/grove-core/pkg/workspace"
	"github.com/mattsolo1/grove-core/tui/components/help"
	"github.com/mattsolo1/grove-core/tui/keymap"
	core_theme "github.com/mattsolo1/grove-core/tui/theme"
	grove_context "github.com/mattsolo1/grove-context/pkg/context"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

// --- Keymaps for Help View ---

type treeViewKeyMap struct {
	keymap.Base
}

func (k treeViewKeyMap) ShortHelp() []key.Binding {
	// Abridged help for the footer
	return []key.Binding{k.Help, key.NewBinding(key.WithHelp("tab", "repos")), k.Quit}
}

func (k treeViewKeyMap) FullHelp() [][]key.Binding {
	// Replicates the content from the old renderHelp function in a structured format
	return [][]key.Binding{
		{
			key.NewBinding(key.WithHelp("", "Navigation:")),
			key.NewBinding(key.WithKeys("up", "down", "j", "k"), key.WithHelp("↑/↓, j/k", "Move up/down")),
			key.NewBinding(key.WithKeys("enter", " "), key.WithHelp("enter, space", "Toggle expand")),
			key.NewBinding(key.WithKeys("g"), key.WithHelp("gg", "Go to top")),
			key.NewBinding(key.WithKeys("G"), key.WithHelp("G", "Go to bottom")),
			key.NewBinding(key.WithKeys("ctrl+d", "ctrl+u"), key.WithHelp("ctrl-d/u", "Page down/up")),
			key.NewBinding(key.WithHelp("", "Folding (vim-style):")),
			key.NewBinding(key.WithKeys("z"), key.WithHelp("za", "Toggle fold")),
			key.NewBinding(key.WithKeys("z"), key.WithHelp("zo/zc", "Open/close fold")),
			key.NewBinding(key.WithKeys("z"), key.WithHelp("zR/zM", "Open/close all")),
			key.NewBinding(key.WithHelp("", "Search:")),
			key.NewBinding(key.WithKeys("/"), key.WithHelp("/", "Search for files")),
			key.NewBinding(key.WithKeys("n", "N"), key.WithHelp("n/N", "Next/prev result")),
		},
		{
			key.NewBinding(key.WithHelp("", "Actions:")),
			key.NewBinding(key.WithKeys("h"), key.WithHelp("h", "Toggle hot context")),
			key.NewBinding(key.WithKeys("c"), key.WithHelp("c", "Toggle cold context")),
			key.NewBinding(key.WithKeys("x"), key.WithHelp("x", "Toggle exclude")),
			key.NewBinding(key.WithKeys("tab", "A"), key.WithHelp("tab/A", "Repository view")),
			key.NewBinding(key.WithKeys("p"), key.WithHelp("p", "Toggle pruning")),
			key.NewBinding(key.WithKeys("H"), key.WithHelp("H", "Toggle gitignored")),
			key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "Refresh view")),
			key.NewBinding(key.WithKeys("q"), key.WithHelp("q", "Quit")),
			key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "Toggle this help")),
		},
	}
}

type repoSelectKeyMap struct {
	keymap.Base
	FocusEcosystem key.Binding
	ClearFocus     key.Binding
}

func (k repoSelectKeyMap) ShortHelp() []key.Binding {
	// Abridged help for the footer
	return []key.Binding{k.Help, key.NewBinding(key.WithHelp("tab", "tree")), k.Quit}
}

func (k repoSelectKeyMap) FullHelp() [][]key.Binding {
	// Replicates the content from the old renderRepoHelp function
	return [][]key.Binding{
		{
			key.NewBinding(key.WithHelp("", "Navigation:")),
			key.NewBinding(key.WithKeys("up", "down", "j", "k"), key.WithHelp("↑/↓, j/k", "Move up/down")),
			key.NewBinding(key.WithKeys("ctrl+u", "ctrl+d"), key.WithHelp("ctrl-u/d", "Half page up/down")),
			key.NewBinding(key.WithKeys("g", "G"), key.WithHelp("g/G", "Go to top/bottom")),
			key.NewBinding(key.WithKeys("/"), key.WithHelp("/", "Filter repositories")),
			key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "Clear filter")),
			key.NewBinding(key.WithHelp("", "Folding (vim-style):")),
			key.NewBinding(key.WithKeys("z"), key.WithHelp("zo/zc", "Open/close fold")),
			key.NewBinding(key.WithKeys("z"), key.WithHelp("zR/zM", "Open/close all")),
			key.NewBinding(key.WithHelp("", "Context Actions:")),
			key.NewBinding(key.WithKeys("h"), key.WithHelp("h", "Toggle hot context")),
			key.NewBinding(key.WithKeys("c"), key.WithHelp("c", "Toggle cold context")),
			key.NewBinding(key.WithKeys("x"), key.WithHelp("x", "Toggle exclude")),
			key.NewBinding(key.WithKeys("a"), key.WithHelp("a", "Add/remove from tree")),
		},
		{
			key.NewBinding(key.WithHelp("", "Repository Actions:")),
			key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "Refresh list")),
			key.NewBinding(key.WithKeys("A"), key.WithHelp("A", "Audit repository")),
			key.NewBinding(key.WithKeys("R"), key.WithHelp("R", "View audit report")),
			key.NewBinding(key.WithHelp("", "View Control:")),
			key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", "Switch to tree view")),
			key.NewBinding(key.WithKeys("q"), key.WithHelp("q", "Quit")),
			key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "Toggle this help")),
		},
	}
}

var (
	treeKeys = treeViewKeyMap{Base: keymap.NewBase()}
	repoKeys = repoSelectKeyMap{
		Base: keymap.NewBase(),
		FocusEcosystem: key.NewBinding(
			key.WithKeys("@"),
			key.WithHelp("@", "focus ecosystem"),
		),
		ClearFocus: key.NewBinding(
			key.WithKeys("ctrl+g"),
			key.WithHelp("ctrl+g", "clear focus"),
		),
	}
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
			finalModel, err := p.Run()
			if err != nil {
				return err
			}
			
			// Check if we need to launch audit after exit
			if viewModel, ok := finalModel.(*viewModel); ok && viewModel.auditRepoURL != "" {
				prettyLog.InfoPretty(fmt.Sprintf("Launching audit for %s...", viewModel.auditRepoURL))
				// Execute cx repo audit command
				auditCmd := newRepoAuditCmd()
				auditCmd.SetArgs([]string{viewModel.auditRepoURL})
				return auditCmd.Execute()
			}
			
			return nil
		},
	}

	return cmd
}

// Message types
type treeLoadedMsg struct {
	tree *grove_context.FileNode
	err  error
}

type ruleChangeResultMsg struct {
	err           error
	successMsg    string
	refreshNeeded bool
}

type confirmActionMsg struct {
	action      string // "hot", "cold", "exclude"
	path        string
	isDirectory bool
	warning     string
}

type reposLoadedMsg struct {
	projects []*workspace.ProjectInfo
	err      error
}

type viewMode int

const (
	modeTree viewMode = iota
	modeRepoSelect
	modeReportView
)

// viewModel is the model for the interactive tree view
type viewModel struct {
	tree          *grove_context.FileNode
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
	help          help.Model
	rulesContent  string // Content of .grove/rules file
	showGitIgnored bool  // Whether to show gitignored files
	// Search functionality
	searchQuery   string // Current search query
	isSearching   bool   // Whether search mode is active
	searchResults []int  // Indices of matching visible nodes
	searchCursor  int    // Current position in search results
	// Confirmation state
	pendingConfirm *confirmActionMsg // Pending action awaiting confirmation
	// Statistics
	hotFiles     int
	hotTokens    int
	coldFiles    int
	coldTokens   int
	totalFiles   int
	totalTokens  int
	// Repository selection
	mode             viewMode
	projects         []*workspace.ProjectInfo // All discovered projects
	filteredProjects []*workspace.ProjectInfo // Filtered list of projects
	repoCursor       int                      // Current selection in repo list
	repoScrollOffset int                      // Scroll offset for repo list
	repoFilter       string                   // Filter string for repos
	repoFiltering    bool                     // Whether we're actively filtering
	workDir         string
	// Audit request
	auditRepoURL    string             // URL of repository to audit after exit
	// Report viewing
	reportContent   string             // Content of the audit report
	reportTitle     string             // Title/path of the report being viewed
	reportScrollOffset int            // Scroll offset for report viewing
	// Rules parsing
	hotRules        []string           // Rules in hot context section
	coldRules       []string           // Rules in cold context section
	viewPaths       []string           // Paths from @view directives
	// Worktree folding
	expandedRepos   map[string]bool    // Tracks which workspace repos have expanded worktrees
	// Focus mode
	ecosystemPickerMode bool                       // True when showing only ecosystems for selection
	focusedEcosystem    *workspace.ProjectInfo     // Currently focused ecosystem/worktree
}

// nodeWithLevel stores a node with its display level
type nodeWithLevel struct {
	node  *grove_context.FileNode
	level int
}

// newViewModel creates a new view model
func newViewModel() *viewModel {
	workDir, _ := os.Getwd()

	helpModel := help.NewBuilder().
		WithKeys(treeKeys).
		WithTitle("Grove Context Visualization - Help").
		Build()

	return &viewModel{
		expandedPaths:  make(map[string]bool),
		expandedRepos:  make(map[string]bool),
		loading:        true,
		pruning:        false,
		showGitIgnored: false,
		mode:           modeTree,
		workDir:        workDir,
		help:           helpModel,
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
		manager := grove_context.NewManager("")
		tree, err := manager.AnalyzeProjectTree(m.pruning, m.showGitIgnored)
		return treeLoadedMsg{tree: tree, err: err}
	}
}

// loadRulesContent loads the content of .grove/rules file and parses rules
func (m *viewModel) loadRulesContent() {
	m.loadAndParseRules()
}

// loadAndParseRules reads the .grove/rules file and populates hotRules and coldRules
func (m *viewModel) loadAndParseRules() {
	// Clear existing rules
	m.hotRules = []string{}
	m.coldRules = []string{}
	m.viewPaths = []string{}
	
	rulesPath := filepath.Join(".grove", "rules")
	content, err := os.ReadFile(rulesPath)
	if err != nil {
		if os.IsNotExist(err) {
			m.rulesContent = "# .grove/rules file not found\n# Rules will appear here when created"
		} else {
			m.rulesContent = fmt.Sprintf("# Error reading rules file:\n# %v", err)
		}
		return
	}
	
	m.rulesContent = string(content)
	if m.rulesContent == "" {
		m.rulesContent = "# Empty rules file"
		return
	}
	
	// Parse the rules
	lines := strings.Split(m.rulesContent, "\n")
	inColdSection := false
	
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		
		// Check for separator
		if trimmed == "---" {
			inColdSection = true
			continue
		}
		
		// Skip empty lines and comments, but process @view
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		if strings.HasPrefix(trimmed, "@view:") {
			path := strings.TrimSpace(strings.TrimPrefix(trimmed, "@view:"))
			if path != "" {
				m.viewPaths = append(m.viewPaths, path)
			}
			continue
		}
		if strings.HasPrefix(trimmed, "@") { // Skip other directives
			continue
		}
		
		// Add rule to appropriate section
		if inColdSection {
			m.coldRules = append(m.coldRules, trimmed)
		} else {
			m.hotRules = append(m.hotRules, trimmed)
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
func (m *viewModel) calculateNodeStats(node *grove_context.FileNode) {
	if node == nil {
		return
	}
	
	// Count files (not directories)
	if !node.IsDir {
		switch node.Status {
		case grove_context.StatusIncludedHot:
			m.hotFiles++
			m.hotTokens += node.TokenCount
			m.totalFiles++
			m.totalTokens += node.TokenCount
		case grove_context.StatusIncludedCold:
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

// performSearch finds all visible nodes that match the search query
func (m *viewModel) performSearch() {
	m.searchResults = []int{}
	if m.searchQuery == "" {
		return
	}
	
	query := strings.ToLower(m.searchQuery)
	for i, nl := range m.visibleNodes {
		if strings.Contains(strings.ToLower(nl.node.Name), query) {
			m.searchResults = append(m.searchResults, i)
		}
	}
}

// getRelativePath converts node path to relative path suitable for rules file
// For directories, it returns a glob pattern like "dirname/**" to include all contents
func (m *viewModel) getRelativePath(node *grove_context.FileNode) (string, error) {
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

// isPathPotentiallyDangerous checks if a path might be risky to add
func (m *viewModel) isPathPotentiallyDangerous(path string) (bool, string) {
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

// handleRuleAction checks if a path is dangerous and either prompts for confirmation or executes the action
func (m *viewModel) handleRuleAction(relPath string, action string, isDir bool) tea.Cmd {
	// Check if path is potentially dangerous
	isDangerous, warning := m.isPathPotentiallyDangerous(relPath)
	
	if isDangerous {
		// Set up confirmation
		m.pendingConfirm = &confirmActionMsg{
			action:      action,
			path:        relPath,
			isDirectory: isDir,
			warning:     warning,
		}
		m.statusMessage = fmt.Sprintf("⚠️  %s - Press 'y' to confirm, 'n' to cancel", warning)
		return nil
	}
	
	// Path is safe, proceed directly
	return m.toggleRuleCmd(relPath, action, isDir)
}

// discoverReposCmd returns a command to discover available repositories
func (m *viewModel) discoverReposCmd() tea.Cmd {
	return func() tea.Msg {
		logger := logrus.New()
		logger.SetOutput(io.Discard)
		discoverySvc := workspace.NewDiscoveryService(logger)

		result, err := discoverySvc.DiscoverAll()
		if err != nil {
			return reposLoadedMsg{err: fmt.Errorf("failed to run discovery: %w", err)}
		}

		projectInfos := workspace.TransformToProjectInfo(result)
		enrichOpts := &workspace.EnrichmentOptions{
			FetchGitStatus:      true,
			FetchClaudeSessions: true,
		}
		workspace.EnrichProjects(context.Background(), projectInfos, enrichOpts)

		return reposLoadedMsg{projects: projectInfos, err: nil}
	}
}

// filterRepos filters the repository list based on the current filter string and expansion state
// Organizes repos into sections: Ecosystem repos, Main ecosystem projects, Worktree projects, Cloned repos
// Supports ecosystem picker mode and focus mode
func (m *viewModel) filterRepos() {
	m.filteredProjects = []*workspace.ProjectInfo{}

	// Handle ecosystem picker mode - show only ecosystems with worktrees
	if m.ecosystemPickerMode {
		var ecosystemRepos []*workspace.ProjectInfo
		var worktrees []*workspace.ProjectInfo

		for _, proj := range m.projects {
			if proj.IsEcosystem && !proj.IsWorktree {
				ecosystemRepos = append(ecosystemRepos, proj)
			} else if proj.IsWorktree {
				worktrees = append(worktrees, proj)
			}
		}

		// Add ecosystems and their worktrees
		for _, eco := range ecosystemRepos {
			m.filteredProjects = append(m.filteredProjects, eco)

			// Add worktrees of this ecosystem
			for _, wt := range worktrees {
				if wt.ParentPath == eco.Path {
					m.filteredProjects = append(m.filteredProjects, wt)
				}
			}
		}
		return
	}

	// Determine which projects to filter from based on focus mode
	var projectsToFilter []*workspace.ProjectInfo
	if m.focusedEcosystem != nil {
		// Focus mode - show only the focused ecosystem and its children
		projectsToFilter = append(projectsToFilter, m.focusedEcosystem)

		// If focused on a worktree, include only projects in that worktree
		if m.focusedEcosystem.IsWorktree {
			// A worktree's name is what child projects will have as WorktreeName
			// Also need to find the parent ecosystem path
			parentEcoPath := m.focusedEcosystem.ParentPath
			for _, proj := range m.projects {
				// Find projects that belong to this worktree
				// They will have WorktreeName matching the worktree's Name
				if proj.WorktreeName == m.focusedEcosystem.Name &&
				   proj.ParentEcosystemPath == parentEcoPath &&
				   proj.Path != m.focusedEcosystem.Path {
					projectsToFilter = append(projectsToFilter, proj)
				}
			}
		} else if m.focusedEcosystem.IsEcosystem {
			// Focused on main ecosystem - include main projects and worktrees
			for _, proj := range m.projects {
				if proj.ParentEcosystemPath == m.focusedEcosystem.Path {
					projectsToFilter = append(projectsToFilter, proj)
				}
			}
		}
	} else {
		// No focus - use all projects
		projectsToFilter = m.projects
	}

	if m.repoFilter == "" {
		// No text filter - organize repos into hierarchical sections

		// Categorize all projects
		var ecosystemRepos []*workspace.ProjectInfo
		var worktrees []*workspace.ProjectInfo
		var mainEcosystemProjects []*workspace.ProjectInfo
		var worktreeProjects []*workspace.ProjectInfo
		var clonedRepos []*workspace.ProjectInfo

		for _, proj := range projectsToFilter {
			if proj.IsEcosystem {
				ecosystemRepos = append(ecosystemRepos, proj)
			} else if proj.IsWorktree {
				worktrees = append(worktrees, proj)
			} else if proj.WorktreeName != "" {
				// Project inside a worktree
				worktreeProjects = append(worktreeProjects, proj)
			} else if proj.ParentEcosystemPath != "" {
				// Project in main ecosystem repo
				mainEcosystemProjects = append(mainEcosystemProjects, proj)
			} else {
				// Cloned repo
				clonedRepos = append(clonedRepos, proj)
			}
		}

		// Build hierarchical structure:
		// Ecosystem -> Main projects + Worktrees -> Worktree projects
		for _, eco := range ecosystemRepos {
			m.filteredProjects = append(m.filteredProjects, eco)

			// If ecosystem is expanded, show its children
			if m.expandedRepos[eco.Path] {
				// First, add sub-projects from main ecosystem
				for _, proj := range mainEcosystemProjects {
					if proj.ParentEcosystemPath == eco.Path {
						m.filteredProjects = append(m.filteredProjects, proj)
					}
				}

				// Then, add worktrees of this ecosystem
				for _, wt := range worktrees {
					if wt.ParentPath == eco.Path {
						m.filteredProjects = append(m.filteredProjects, wt)

						// If this worktree is expanded, add its sub-projects
						if m.expandedRepos[wt.Path] {
							for _, wtProj := range worktreeProjects {
								if wtProj.WorktreeName == wt.WorktreeName && wtProj.ParentEcosystemPath == eco.Path {
									m.filteredProjects = append(m.filteredProjects, wtProj)
								}
							}
						}
					}
				}
			}
		}

		// Add cloned repos (not in focus mode)
		if m.focusedEcosystem == nil {
			m.filteredProjects = append(m.filteredProjects, clonedRepos...)
		}

	} else {
		// Apply filter (when filtering, show all matching repos regardless of expansion state)
		filter := strings.ToLower(m.repoFilter)

		for _, proj := range projectsToFilter {
			// Get branch from GitStatus if available
			branch := ""
			if proj.GitStatus != nil {
				if extStatus, ok := proj.GitStatus.(*workspace.ExtendedGitStatus); ok && extStatus.StatusInfo != nil {
					branch = extStatus.StatusInfo.Branch
				}
			}

			if strings.Contains(strings.ToLower(proj.Name), filter) ||
			   strings.Contains(strings.ToLower(proj.Path), filter) ||
			   strings.Contains(strings.ToLower(branch), filter) {
				m.filteredProjects = append(m.filteredProjects, proj)
			}
		}
	}

	// Reset cursor if it's out of bounds
	if m.repoCursor >= len(m.filteredProjects) {
		m.repoCursor = len(m.filteredProjects) - 1
		if m.repoCursor < 0 {
			m.repoCursor = 0
		}
	}
	m.ensureRepoCursorVisible()
}

// ensureRepoCursorVisible ensures the repo cursor is visible
func (m *viewModel) ensureRepoCursorVisible() {
	// Calculate the actual list height (must match renderRepoSelect)
	viewportHeight := m.height - 8 // Account for header, footer, and more margins
	listHeight := viewportHeight // Use full viewport like tree view
	
	if listHeight < 5 {
		listHeight = 5
	}
	
	if m.repoCursor < m.repoScrollOffset {
		m.repoScrollOffset = m.repoCursor
	} else if m.repoCursor >= m.repoScrollOffset + listHeight {
		m.repoScrollOffset = m.repoCursor - listHeight + 1
	}
	
	if m.repoScrollOffset < 0 {
		m.repoScrollOffset = 0
	}
	
	// Ensure scroll offset doesn't go beyond the list
	maxScrollOffset := len(m.filteredProjects) - listHeight
	if maxScrollOffset < 0 {
		maxScrollOffset = 0
	}
	if m.repoScrollOffset > maxScrollOffset {
		m.repoScrollOffset = maxScrollOffset
	}
}

// getPathForRule determines whether to use relative or absolute path for a repository
// isRepoInView checks if a repository is included via a @view directive.
func (m *viewModel) isRepoInView(proj *workspace.ProjectInfo) bool {
	repoPath := m.getPathForRule(proj.Path)
	for _, viewPath := range m.viewPaths {
		if viewPath == repoPath {
			return true
		}
	}
	return false
}

func (m *viewModel) getPathForRule(repoPath string) string {
	// Try to get relative path
	relPath, err := filepath.Rel(m.workDir, repoPath)
	if err != nil {
		// If we can't get relative path, use absolute
		return repoPath
	}
	
	// Count parent directory traversals
	traversalCount := strings.Count(relPath, "../")
	if traversalCount > 2 {
		// Too many traversals, use absolute path
		return repoPath
	}
	
	// Safe to use relative path
	return relPath
}

// getRepoStatus checks if a repository path matches any rule in hot or cold context
func (m *viewModel) getRepoStatus(proj *workspace.ProjectInfo) string {
	// Get the path we'll check against rules
	repoPath := m.getPathForRule(proj.Path)
	
	// Check hot rules
	for _, rule := range m.hotRules {
		// Clean the rule
		cleanRule := strings.TrimSpace(rule)
		
		// Skip exclusion rules
		if strings.HasPrefix(cleanRule, "!") {
			continue
		}
		
		// Check if this rule matches the repository path
		if m.ruleMatchesPath(cleanRule, repoPath) {
			return "hot"
		}
	}
	
	// Check cold rules
	for _, rule := range m.coldRules {
		// Clean the rule
		cleanRule := strings.TrimSpace(rule)
		
		// Skip exclusion rules
		if strings.HasPrefix(cleanRule, "!") {
			continue
		}
		
		// Check if this rule matches the repository path
		if m.ruleMatchesPath(cleanRule, repoPath) {
			return "cold"
		}
	}
	
	return "none"
}

// isRepoExcluded checks if a repository is explicitly excluded
func (m *viewModel) isRepoExcluded(proj *workspace.ProjectInfo) bool {
	repoPath := m.getPathForRule(proj.Path)
	
	// Check all rules for exclusion patterns
	allRules := append(m.hotRules, m.coldRules...)
	for _, rule := range allRules {
		if strings.HasPrefix(rule, "!") {
			excludePattern := strings.TrimPrefix(rule, "!")
			if m.ruleMatchesPath(excludePattern, repoPath) {
				return true
			}
		}
	}
	
	return false
}

// ruleMatchesPath checks if a rule pattern matches a repository path
func (m *viewModel) ruleMatchesPath(rule, path string) bool {
	// Normalize paths for comparison
	rule = strings.TrimSpace(rule)
	path = strings.TrimSpace(path)
	
	// Remove trailing slashes for consistent comparison
	rule = strings.TrimSuffix(rule, "/")
	path = strings.TrimSuffix(path, "/")
	
	// Direct match
	if rule == path {
		return true
	}
	
	// Check if rule with /** matches
	if strings.HasSuffix(rule, "/**") {
		baseRule := strings.TrimSuffix(rule, "/**")
		if baseRule == path {
			return true
		}
	}
	
	// Check if path matches a directory rule
	if rule+"/**" == path+"/**" {
		return true
	}
	
	// Check with ./ prefix variations
	if "./"+rule == path || rule == "./"+path {
		return true
	}
	
	return false
}

// getAuditStatusStyle returns the appropriate style for an audit status
func (m *viewModel) getAuditStatusStyle(status string) lipgloss.Style {
	theme := core_theme.DefaultTheme
	switch status {
	case "passed", "audited":
		return theme.Success
	case "failed":
		return theme.Error
	case "not_audited":
		return theme.Muted
	default:
		return theme.Warning
	}
}

// renderRepoSelect renders the repository selection view in compact tabular format
func (m *viewModel) renderRepoSelect() string {
	
	// Header - different based on mode
	var header string
	var subtitle string

	if m.ecosystemPickerMode {
		header = core_theme.DefaultTheme.Info.Render("[Select Ecosystem to Focus]")
		subtitle = core_theme.DefaultTheme.Muted.Render("Press Enter to focus on selected ecosystem, Esc to cancel")
	} else if m.focusedEcosystem != nil {
		focusIndicator := core_theme.DefaultTheme.Info.Render(fmt.Sprintf("[Focus: %s]", m.focusedEcosystem.Name))
		header = core_theme.DefaultTheme.Header.Render("Select Repository") + " " + focusIndicator
		subtitle = core_theme.DefaultTheme.Muted.Render("Press ctrl+g to clear focus")
	} else {
		header = core_theme.DefaultTheme.Header.Render("Select Repository")
		subtitle = core_theme.DefaultTheme.Muted.Render("Add repositories to rules file for further context refinement")
	}

	// Filter display
	if m.repoFilter != "" {
		filterStyle := core_theme.DefaultTheme.Muted
		header += filterStyle.Render(fmt.Sprintf(" (filter: %s)", m.repoFilter))
	}
	
	// Calculate dimensions (matching tree view layout exactly)
	viewportHeight := m.height - 12 // Account for header, subtitle, footer, and margins
	rulesWidth := int(float64(m.width) * 0.4)      // Right panel is 40% of width (was 33%)
	repoListWidth := m.width - rulesWidth - 2 // Left panel gets the rest
	
	// Split the right panel height for rules and stats (use fixed heights like main view)
	statsHeight := 8 // Fixed height for stats
	rulesHeight := viewportHeight - statsHeight - 1 // -1 for spacing
	
	// Build the repository list (left panel)
	var b strings.Builder
	
	// Styles for repo list
	selectedStyle := core_theme.DefaultTheme.Selected
	normalStyle := lipgloss.NewStyle().
		Foreground(core_theme.DefaultColors.LightText)
	// pathStyle := core_theme.DefaultTheme.Muted // Temporarily unused
	worktreeStyle := core_theme.DefaultTheme.Faint
	clonedStyle := lipgloss.NewStyle().
		Foreground(core_theme.DefaultColors.Orange)
	
	// Calculate visible range for repo list (use a smaller height for the actual list)
	listHeight := viewportHeight // Use full viewport like tree view
	visibleEnd := m.repoScrollOffset + listHeight
	if visibleEnd > len(m.filteredProjects) {
		visibleEnd = len(m.filteredProjects)
	}
	
	// Find max name width for alignment
	maxNameWidth := 20
	for _, repo := range m.filteredProjects {
		nameLen := len(repo.Name)
		if repo.IsWorktree {
			nameLen += 3 // Account for "└─ " prefix
		}
		if nameLen > maxNameWidth && nameLen < 40 {
			maxNameWidth = nameLen
		}
	}
	
	// Track current section for headers - now only 2 sections
	type repoSection int
	const (
		sectionEcosystem repoSection = iota
		sectionCloned
	)

	getSection := func(proj *workspace.ProjectInfo) repoSection {
		// Everything that's part of an ecosystem (ecosystem itself, worktrees, or sub-projects)
		if proj.IsEcosystem || proj.IsWorktree || proj.ParentEcosystemPath != "" || proj.WorktreeName != "" {
			return sectionEcosystem
		}
		return sectionCloned
	}

	currentSection := repoSection(-1) // Start with invalid section to trigger first header

	for i := m.repoScrollOffset; i < visibleEnd; i++ {
		repo := m.filteredProjects[i]
		section := getSection(repo)

		// Add section separator and header when transitioning to a new section
		if section != currentSection {
			// Add separator (except before the first section)
			if currentSection != repoSection(-1) {
				separatorLine := "  " + core_theme.DefaultTheme.Faint.
					Render("─────────────────────────────────────────────────")
				b.WriteString(separatorLine + "\n")
			}

			// Add section header
			headerStyle := core_theme.DefaultTheme.TableHeader
			switch section {
			case sectionEcosystem:
				b.WriteString("  " + headerStyle.Render("ECOSYSTEM REPOSITORIES") + "\n")
			case sectionCloned:
				b.WriteString("  " + headerStyle.Render("CLONED REPOSITORIES") + "\n")
				// For cloned repos, add column headers
				b.WriteString("  " + headerStyle.Render("URL                                      VERSION  COMMIT   STATUS       REPORT") + "\n")
			}
			currentSection = section
		}
		
		// Build the line
		var line string
		
		// Status indicator with colors matching main cx view
		status := m.getRepoStatus(repo)
		isExcluded := m.isRepoExcluded(repo)
		isInView := m.isRepoInView(repo)
		
		if isInView {
			// Blue eye icon for in view
			viewStyle := core_theme.DefaultTheme.Info
			line += viewStyle.Render("👁️") + " "
		} else if isExcluded {
			// Red exclude symbol
			excludeStyle := core_theme.DefaultTheme.Error
			line += excludeStyle.Render("🚫") + " "
		} else {
			switch status {
			case "hot":
				// Green checkmark for hot context
				hotStyle := core_theme.DefaultTheme.Success
				line += hotStyle.Render("✓") + " "
			case "cold":
				// Light blue snowflake for cold context
				coldStyle := lipgloss.NewStyle().Foreground(core_theme.DefaultColors.Blue)
				line += coldStyle.Render("❄️") + " "
			default:
				line += "  " // Empty space for none
			}
		}
		
		// Cursor indicator
		if i == m.repoCursor {
			line += "▶ "
		} else {
			line += "  "
		}

		if section == sectionEcosystem {
			// Ecosystem section - render as hierarchical tree
			name := repo.Name

			if repo.IsEcosystem {
				// Top-level ecosystem - check if it has children (worktrees or sub-projects)
				hasChildren := false
				for _, proj := range m.projects {
					if (proj.IsWorktree && proj.ParentPath == repo.Path) ||
					   (proj.ParentEcosystemPath == repo.Path && proj.WorktreeName == "") {
						hasChildren = true
						break
					}
				}
				if hasChildren {
					// Add fold indicator
					if m.expandedRepos[repo.Path] {
						name = "▾ " + name // Expanded indicator
					} else {
						name = "▸ " + name // Collapsed indicator
					}
				}
			} else if repo.IsWorktree {
				// Worktree under ecosystem - check if it has sub-projects
				hasChildren := false
				for _, proj := range m.projects {
					if proj.WorktreeName == repo.WorktreeName && proj.ParentEcosystemPath == repo.ParentEcosystemPath {
						hasChildren = true
						break
					}
				}
				if hasChildren {
					// Add fold indicator
					if m.expandedRepos[repo.Path] {
						name = "▾ └─ " + name // Expanded worktree
					} else {
						name = "▸ └─ " + name // Collapsed worktree
					}
				} else {
					name = "  └─ " + name // Worktree with no children
				}
			} else if repo.WorktreeName != "" {
				// Sub-project under a worktree (level 2)
				name = "    └─ " + name
			} else if repo.ParentEcosystemPath != "" {
				// Sub-project under main ecosystem (level 1)
				name = "  └─ " + name
			}

			var nameStyled string
			if i == m.repoCursor {
				nameStyled = selectedStyle.Render(fmt.Sprintf("%-*s", maxNameWidth, name))
			} else if repo.IsWorktree {
				nameStyled = worktreeStyle.Render(fmt.Sprintf("%-*s", maxNameWidth, name))
			} else {
				nameStyled = normalStyle.Render(fmt.Sprintf("%-*s", maxNameWidth, name))
			}
			line += nameStyled + "  "
			
			// Path (truncated if needed) - temporarily disabled
			// path := repo.Path
			// if repo.IsWorktree && repo.ParentPath != "" {
			// 	// Show relative path for worktrees
			// 	if rel, err := filepath.Rel(repo.ParentPath, repo.Path); err == nil {
			// 		path = "./" + rel
			// 	}
			// }
			
			// Temporarily remove path display to prevent wrapping
			// TODO: Re-enable with proper truncation
			// maxPathWidth := m.width - maxNameWidth - 10
			// if maxPathWidth < 20 {
			// 	maxPathWidth = 20
			// }
			// if len(path) > maxPathWidth {
			// 	path = "..." + path[len(path)-maxPathWidth+3:]
			// }
			// 
			// line += pathStyle.Render(path)
		} else {
			// Cloned repos - render in tabular format matching cx repo list
			url := repo.Name
			if len(url) > 40 {
				url = url[:37] + "..."
			}
			
			var urlStyled string
			if i == m.repoCursor {
				urlStyled = selectedStyle.Render(fmt.Sprintf("%-40s", url))
			} else {
				urlStyled = clonedStyle.Render(fmt.Sprintf("%-40s", url))
			}
			line += urlStyled + "  "
			
			// Version column
			version := repo.Version
			if version == "" {
				version = "default"
			}
			if len(version) > 7 {
				version = version[:7]
			}
			if i == m.repoCursor {
				line += selectedStyle.Render(fmt.Sprintf("%-7s", version)) + "  "
			} else {
				line += normalStyle.Render(fmt.Sprintf("%-7s", version)) + "  "
			}
			
			// Commit column
			commit := repo.Commit
			if commit == "" {
				commit = "-"
			}
			if i == m.repoCursor {
				line += selectedStyle.Render(fmt.Sprintf("%-7s", commit)) + "  "
			} else {
				line += normalStyle.Render(fmt.Sprintf("%-7s", commit)) + "  "
			}
			
			// Status column
			status := repo.AuditStatus
			if status == "" {
				status = "not_audited"
			}
			statusStyle := m.getAuditStatusStyle(status)
			if i == m.repoCursor {
				// When selected, use selected style
				statusStyled := selectedStyle.Render(fmt.Sprintf("%-11s", status))
				line += statusStyled + "  "
			} else {
				line += statusStyle.Render(fmt.Sprintf("%-11s", status)) + "  "
			}
			
			// Report column
			reportIndicator := ""
			if repo.ReportPath != "" {
				reportIndicator = "✓"
			}
			if i == m.repoCursor {
				line += selectedStyle.Render(fmt.Sprintf("%-6s", reportIndicator))
			} else if reportIndicator != "" {
				line += core_theme.DefaultTheme.Success.Render(reportIndicator)
			} else {
				line += fmt.Sprintf("%-6s", reportIndicator)
			}
		}
		
		b.WriteString(line + "\n")
	}
	
	// Show scroll indicator if needed
	if len(m.filteredProjects) > listHeight {
		scrollInfo := fmt.Sprintf("\n[%d-%d of %d]", 
			m.repoScrollOffset+1, 
			visibleEnd, 
			len(m.filteredProjects))
		b.WriteString(lipgloss.NewStyle().
			Foreground(core_theme.DefaultTheme.Muted.GetForeground()).
			Render(scrollInfo))
	}
	
	// Remove complex padding logic - let lipgloss handle it like tree view
	
	// Create scrollbar for the repository list
	scrollbar := ""
	if len(m.filteredProjects) > listHeight {
		scrollbar = m.renderScrollbar(
			len(m.filteredProjects),
			listHeight,
			m.repoScrollOffset,
			listHeight,
		)
	}
	
	// Create the repository list content (match tree view approach)
	repoListContent := lipgloss.NewStyle().
		Width(repoListWidth - 4). // Make room for scrollbar  
		MaxWidth(repoListWidth - 4). // Ensure no overflow
		Render(b.String())
	
	// Combine repo list with scrollbar
	var repoWithScrollbar string
	if scrollbar != "" {
		repoWithScrollbar = lipgloss.JoinHorizontal(
			lipgloss.Top,
			repoListContent,
			" ", // Small gap
			scrollbar,
		)
	} else {
		repoWithScrollbar = repoListContent
	}
	
	// Create the left panel (repository list with scrollbar) - with top padding
	repoPanel := lipgloss.NewStyle().
		Width(repoListWidth).
		Height(viewportHeight).
		Padding(1, 1). // Add 1 row top padding: top/bottom: 1, left/right: 1
		Render(repoWithScrollbar)
	
	// Create the right panel with rules and stats (exactly like tree view)
	// Rules panel (top part of right panel)
	rulesStyle := core_theme.DefaultTheme.Box.Copy().
		Width(rulesWidth).
		Height(rulesHeight).
		Padding(1, 2) // Add more padding for better spacing
	
	rulesPanel := rulesStyle.Render(m.renderRules(rulesWidth, rulesHeight))
	
	// Stats panel (bottom part of right panel)
	statsStyle := core_theme.DefaultTheme.Box.Copy().
		Width(rulesWidth).
		Height(statsHeight).
		Padding(1, 2) // Add more padding for better spacing
	
	statsPanel := statsStyle.Render(m.renderStats())
	
	// Combine rules and stats vertically for right panel
	rightPanel := lipgloss.JoinVertical(lipgloss.Left, rulesPanel, statsPanel)
	
	// Combine left and right panels horizontally
	mainContent := lipgloss.JoinHorizontal(lipgloss.Top, repoPanel, rightPanel)
	
	// Footer with help hint or status message
	var footer string
	if m.statusMessage != "" {
		statusStyle := core_theme.DefaultTheme.Success
		footer = statusStyle.Render(m.statusMessage)
	} else {
		footer = m.help.View()
	}
	
	// Combine all parts
	parts := []string{
		header,
		subtitle,
		mainContent,
		footer,
	}
	
	return lipgloss.JoinVertical(lipgloss.Left, parts...)
}

// renderReportView renders the audit report view
func (m *viewModel) renderReportView() string {
	var b strings.Builder
	
	// Header
	headerStyle := lipgloss.NewStyle().
		Foreground(core_theme.DefaultTheme.Info.GetForeground()).
		Bold(true)
	b.WriteString(headerStyle.Render(m.reportTitle))
	b.WriteString("\n")
	
	// Separator
	b.WriteString(strings.Repeat("─", m.width))
	b.WriteString("\n")
	
	// Calculate viewport
	viewportHeight := m.height - 5 // Leave room for header and help
	if viewportHeight < 5 {
		viewportHeight = 5
	}
	
	// Split content into lines
	lines := strings.Split(m.reportContent, "\n")
	
	// Display lines within viewport
	end := m.reportScrollOffset + viewportHeight
	if end > len(lines) {
		end = len(lines)
	}
	
	contentStyle := lipgloss.NewStyle().Foreground(core_theme.DefaultColors.LightText)
	for i := m.reportScrollOffset; i < end; i++ {
		b.WriteString(contentStyle.Render(lines[i]))
		b.WriteString("\n")
	}
	
	// Fill remaining space if needed
	displayedLines := end - m.reportScrollOffset
	for i := displayedLines; i < viewportHeight; i++ {
		b.WriteString("\n")
	}
	
	// Show scroll indicator if needed
	if len(lines) > viewportHeight {
		scrollInfo := fmt.Sprintf("[Lines %d-%d of %d]", 
			m.reportScrollOffset+1, 
			end, 
			len(lines))
		b.WriteString(lipgloss.NewStyle().
			Foreground(core_theme.DefaultTheme.Muted.GetForeground()).
			Render(scrollInfo))
		b.WriteString("\n")
	}
	
	// Help text
	helpStyle := lipgloss.NewStyle().Foreground(core_theme.DefaultTheme.Muted.GetForeground())
	help := "↑/↓ scroll • C-d/u half page • q/esc back • g/G top/bottom"
	b.WriteString(helpStyle.Render(help))
	
	return b.String()
}


// Update handles messages
func (m *viewModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		// Handle confirmation prompts first
		if m.pendingConfirm != nil {
			switch msg.String() {
			case "y", "Y":
				// User confirmed, execute the action
				action := m.pendingConfirm.action
				path := m.pendingConfirm.path
				isDir := m.pendingConfirm.isDirectory
				m.pendingConfirm = nil
				m.statusMessage = fmt.Sprintf("Adding %s to %s context...", path, action)
				return m, m.toggleRuleCmd(path, action, isDir)
			case "n", "N", "esc":
				// User cancelled
				m.pendingConfirm = nil
				m.statusMessage = "Action cancelled"
				return m, nil
			default:
				// Ignore other keys during confirmation
				return m, nil
			}
		}
		
		// Handle different modes
		switch m.mode {
		case modeRepoSelect:
			// Handle help popup first
			if m.help.ShowAll {
				// Handle keys to close help
				switch msg.String() {
				case "?", "q", "esc", "enter", " ":
					m.help.Toggle()
					return m, nil
				}
				// Ignore all other keys when help is shown
				return m, nil
			}

			// Handle ecosystem picker mode
			if m.ecosystemPickerMode {
				switch msg.String() {
				case "enter":
					// Select ecosystem to focus
					if m.repoCursor >= 0 && m.repoCursor < len(m.filteredProjects) {
						m.focusedEcosystem = m.filteredProjects[m.repoCursor]
						m.ecosystemPickerMode = false
						m.filterRepos()
						m.repoCursor = 0
						m.statusMessage = fmt.Sprintf("Focused on: %s", m.focusedEcosystem.Name)
					}
					return m, nil
				case "esc":
					// Cancel ecosystem picker
					m.ecosystemPickerMode = false
					m.filterRepos()
					return m, nil
				}
			}

			// Handle focus mode keys first (before string switch)
			if key.Matches(msg, repoKeys.ClearFocus) {
				if m.focusedEcosystem != nil {
					m.focusedEcosystem = nil
					m.filterRepos()
					m.repoCursor = 0
					m.statusMessage = "Cleared focus"
				}
				return m, nil
			}

			switch keypress := msg.String(); keypress {
			case "?":
				m.help.Toggle()
				return m, nil
			case "r":
				// Refresh repository list, rules, and stats
				m.statusMessage = "Refreshing..."
				// Reload rules
				m.loadAndParseRules()
				// Clear and rediscover repos
				m.projects = nil
				m.filteredProjects = nil
				m.repoFilter = ""
				m.repoFiltering = false
				// Trigger tree reload in background to update stats
				return m, tea.Batch(
					m.discoverReposCmd(),
					m.loadTreeCmd(),
				)
			case "a":
				// Add/remove repo from view
				if m.repoCursor >= 0 && m.repoCursor < len(m.filteredProjects) {
					repo := m.filteredProjects[m.repoCursor]
					path := m.getPathForRule(repo.Path)
					manager := grove_context.NewManager("")
					
					if err := manager.ToggleViewDirective(path); err != nil {
						m.statusMessage = fmt.Sprintf("Error: %v", err)
						return m, nil
					}
					
					// Check if it was added or removed to set status message
					wasInView := false
					for _, vp := range m.viewPaths {
						if vp == path {
							wasInView = true
							break
						}
					}
					if wasInView {
						m.statusMessage = fmt.Sprintf("Removed %s from view", repo.Name)
					} else {
						m.statusMessage = fmt.Sprintf("Added %s to view", repo.Name)
					}
					
					// Reload rules and refresh stats
					m.loadAndParseRules()
					return m, m.loadTreeCmd()
				}
			case "@":
				// Enter ecosystem picker mode
				m.ecosystemPickerMode = true
				m.filterRepos()
				m.repoCursor = 0
				return m, nil
			case "A":
				// Audit repository - only works for cloned repos
				if m.repoCursor >= 0 && m.repoCursor < len(m.filteredProjects) {
					repo := m.filteredProjects[m.repoCursor]
					// Check if it's a cloned repo (not a workspace repo)
					// Cloned repos are those without ParentEcosystemPath and not worktrees/ecosystems
					isCloned := repo.ParentEcosystemPath == "" && !repo.IsWorktree && !repo.IsEcosystem
					if isCloned {
						// Set the audit URL and quit to trigger audit
						m.auditRepoURL = repo.Name // repo.Name contains the URL for cloned repos
						return m, tea.Quit
					} else {
						m.statusMessage = "Audit is only available for cloned repositories"
					}
				}
			case "h":
				// Toggle hot context
				if m.repoCursor >= 0 && m.repoCursor < len(m.filteredProjects) {
					repo := m.filteredProjects[m.repoCursor]
					path := m.getPathForRule(repo.Path) + "/**" // Add /** for directory inclusion
					manager := grove_context.NewManager("")
					
					// Check current status
					status := m.getRepoStatus(repo)
					
					if status == "hot" {
						// Remove from hot context
						m.statusMessage = fmt.Sprintf("Removing %s from hot context...", repo.Name)
						if err := manager.RemoveRuleForPath(path); err != nil {
							m.statusMessage = fmt.Sprintf("Error: %v", err)
							return m, nil
						}
					} else {
						// Add to hot context
						m.statusMessage = fmt.Sprintf("Adding %s to hot context...", repo.Name)
						if err := manager.AppendRule(path, "hot"); err != nil {
							m.statusMessage = fmt.Sprintf("Error: %v", err)
							return m, nil
						}
					}
					
					// Reload rules to reflect changes
					m.loadAndParseRules()
					// Trigger background tree reload to update stats
					return m, m.loadTreeCmd()
				}
			case "x":
				// Toggle exclude - add exclusion rule or remove it
				if m.repoCursor >= 0 && m.repoCursor < len(m.filteredProjects) {
					repo := m.filteredProjects[m.repoCursor]
					path := m.getPathForRule(repo.Path) + "/**"
					manager := grove_context.NewManager("")
					
					// Check if already excluded
					isExcluded := false
					for _, rule := range m.hotRules {
						if rule == "!"+path || rule == "!"+m.getPathForRule(repo.Path) {
							isExcluded = true
							break
						}
					}
					if !isExcluded {
						for _, rule := range m.coldRules {
							if rule == "!"+path || rule == "!"+m.getPathForRule(repo.Path) {
								isExcluded = true
								break
							}
						}
					}
					
					if isExcluded {
						// Remove exclusion
						m.statusMessage = fmt.Sprintf("Removing exclusion for %s...", repo.Name)
						if err := manager.RemoveRuleForPath(path); err != nil {
							m.statusMessage = fmt.Sprintf("Error: %v", err)
							return m, nil
						}
					} else {
						// Add exclusion
						m.statusMessage = fmt.Sprintf("Excluding %s...", repo.Name)
						if err := manager.AppendRule(path, "exclude"); err != nil {
							m.statusMessage = fmt.Sprintf("Error: %v", err)
							return m, nil
						}
					}
					
					// Reload rules to reflect changes
					m.loadAndParseRules()
					// Trigger background tree reload to update stats
					return m, m.loadTreeCmd()
				}
			case "tab":
				// Toggle back to tree view
				m.mode = modeTree
				m.help.SetKeys(treeKeys)
				m.help.Title = "Grove Context Visualization - Help"
				m.statusMessage = ""
				m.repoFilter = ""
				m.repoFiltering = false
				// Reload rules and refresh tree in case they changed
				m.loadAndParseRules()
				// Trigger tree refresh to show updated context
				m.loading = true
				return m, m.loadTreeCmd()
			case "esc":
				// Handled in escape case above
			case "q":
				// Quit the entire cx view tool
				return m, tea.Quit
			case "up", "k":
				if m.repoCursor > 0 {
					m.repoCursor--
					m.ensureRepoCursorVisible()
				}
			case "down", "j":
				if m.repoCursor < len(m.filteredProjects)-1 {
					m.repoCursor++
					m.ensureRepoCursorVisible()
				}
			case "pgup":
				m.repoCursor -= 10
				if m.repoCursor < 0 {
					m.repoCursor = 0
				}
				m.ensureRepoCursorVisible()
			case "pgdown":
				m.repoCursor += 10
				if m.repoCursor >= len(m.filteredProjects) {
					m.repoCursor = len(m.filteredProjects) - 1
				}
				m.ensureRepoCursorVisible()
			case "ctrl+u":
				// Scroll up half page
				// Calculate the actual list height (must match renderRepoSelect)
				viewportHeight := m.height - 8
				listHeight := viewportHeight
				if listHeight < 5 {
					listHeight = 5
				}
				m.repoCursor -= listHeight / 2
				if m.repoCursor < 0 {
					m.repoCursor = 0
				}
				m.ensureRepoCursorVisible()
			case "ctrl+d":
				// Scroll down half page
				// Calculate the actual list height (must match renderRepoSelect)
				viewportHeight := m.height - 8
				listHeight := viewportHeight
				if listHeight < 5 {
					listHeight = 5
				}
				m.repoCursor += listHeight / 2
				if m.repoCursor >= len(m.filteredProjects) {
					m.repoCursor = len(m.filteredProjects) - 1
				}
				m.ensureRepoCursorVisible()
			case "home", "g":
				m.repoCursor = 0
				m.repoScrollOffset = 0
			case "end", "G":
				m.repoCursor = len(m.filteredProjects) - 1
				m.ensureRepoCursorVisible()
			case "/":
				// Start filtering
				m.repoFiltering = true
				m.repoFilter = ""
				m.statusMessage = "Type to filter repositories (ESC to clear)..."
			case "z":
				// Handle vim-style folding commands
				m.lastKey = "z"
				m.statusMessage = "Folding: o=open, c=close, R=open all, M=close all"
				return m, nil
			case "o":
				// Open fold at cursor (zo)
				if m.lastKey == "z" {
					m.lastKey = ""
					if m.repoCursor >= 0 && m.repoCursor < len(m.filteredProjects) {
						repo := m.filteredProjects[m.repoCursor]
						if !repo.IsWorktree {
							// It's a workspace repo, expand it
							m.expandedRepos[repo.Path] = true
							m.filterRepos()
							m.statusMessage = fmt.Sprintf("Expanded %s", repo.Name)
						} else {
							m.statusMessage = "Can only expand workspace repos, not worktrees"
						}
					}
				}
			case "c":
				// Close fold at cursor (zc) OR toggle cold context (c alone)
				if m.lastKey == "z" {
					m.lastKey = ""
					if m.repoCursor >= 0 && m.repoCursor < len(m.filteredProjects) {
						repo := m.filteredProjects[m.repoCursor]
						if !repo.IsWorktree {
							// It's a workspace repo, collapse it
							m.expandedRepos[repo.Path] = false
							m.filterRepos()
							m.statusMessage = fmt.Sprintf("Collapsed %s", repo.Name)
						} else {
							m.statusMessage = "Can only collapse workspace repos, not worktrees"
						}
					}
				} else {
					// This is the existing cold context toggle functionality
					// Toggle cold context
					if m.repoCursor >= 0 && m.repoCursor < len(m.filteredProjects) {
						repo := m.filteredProjects[m.repoCursor]
						path := m.getPathForRule(repo.Path) + "/**" // Add /** for directory inclusion
						manager := grove_context.NewManager("")

						// Check current status
						status := m.getRepoStatus(repo)

						if status == "cold" {
							// Remove from cold context
							m.statusMessage = fmt.Sprintf("Removing %s from cold context...", repo.Name)
							if err := manager.RemoveRuleForPath(path); err != nil {
								m.statusMessage = fmt.Sprintf("Error: %v", err)
								return m, nil
							}
						} else {
							// Add to cold context
							m.statusMessage = fmt.Sprintf("Adding %s to cold context...", repo.Name)
							if err := manager.AppendRule(path, "cold"); err != nil {
								m.statusMessage = fmt.Sprintf("Error: %v", err)
								return m, nil
							}
						}

						// Reload rules to reflect changes
						m.loadAndParseRules()
						// Trigger background tree reload to update stats
						return m, m.loadTreeCmd()
					}
				}
			case "R":
				// Open all folds (zR) OR view audit report (R alone)
				if m.lastKey == "z" {
					m.lastKey = ""
					// Expand all workspace repos
					for _, proj := range m.projects {
						isWorkspaceRepo := proj.ParentEcosystemPath != "" || proj.IsEcosystem
						if isWorkspaceRepo && !proj.IsWorktree {
							m.expandedRepos[proj.Path] = true
						}
					}
					m.filterRepos()
					// Keep cursor position but ensure it's visible
					m.ensureRepoCursorVisible()
					m.statusMessage = "Expanded all workspace repos"
				} else {
					// This is the existing audit report viewing functionality
					// View audit report - only works for audited repos
					if m.repoCursor >= 0 && m.repoCursor < len(m.filteredProjects) {
						repo := m.filteredProjects[m.repoCursor]
						if repo.ReportPath != "" {
							// Check if report file exists and load it
							content, err := os.ReadFile(repo.ReportPath)
							if err == nil {
								m.mode = modeReportView
								m.reportContent = string(content)
								m.reportTitle = fmt.Sprintf("Audit Report: %s", repo.Name)
								m.reportScrollOffset = 0
								return m, nil
							} else {
								m.statusMessage = "Could not read audit report file"
							}
						} else if repo.AuditStatus == "not_audited" || repo.AuditStatus == "" {
							m.statusMessage = "No audit report available - repository has not been audited"
						} else {
							m.statusMessage = "Audit report path not available"
						}
					}
				}
			case "M":
				// Close all folds (zM)
				if m.lastKey == "z" {
					m.lastKey = ""
					// Collapse all workspace repos
					for _, proj := range m.projects {
						isWorkspaceRepo := proj.ParentEcosystemPath != "" || proj.IsEcosystem
						if isWorkspaceRepo && !proj.IsWorktree {
							m.expandedRepos[proj.Path] = false
						}
					}
					m.filterRepos()
					// Reset cursor and scroll to top
					m.repoCursor = 0
					m.repoScrollOffset = 0
					m.statusMessage = "Collapsed all workspace repos"
				}
			case "backspace":
				if m.repoFiltering && len(m.repoFilter) > 0 {
					m.repoFilter = m.repoFilter[:len(m.repoFilter)-1]
					m.filterRepos()
					if m.repoFilter == "" {
						m.statusMessage = "Type to filter repositories (ESC to clear)..."
					}
				}
			case "escape":
				if m.repoFiltering {
					// Clear filter and exit filtering mode
					m.repoFiltering = false
					m.repoFilter = ""
					m.filterRepos()
					m.statusMessage = ""
				} else {
					// Normal escape behavior - return to tree view
					m.mode = modeTree
					m.help.SetKeys(treeKeys)
					m.help.Title = "Grove Context Visualization - Help"
					m.statusMessage = "Repository selection cancelled."
					m.repoFilter = ""
					m.repoFiltering = false
					return m, nil
				}
			default:
				// Only add character to filter if we're in filtering mode
				if m.repoFiltering && len(keypress) == 1 {
					m.repoFilter += keypress
					m.filterRepos()
					m.statusMessage = fmt.Sprintf("Filter: %s", m.repoFilter)
				}
			}
			return m, nil
		
		case modeReportView:
			// Handle report viewing mode
			switch keypress := msg.String(); keypress {
			case "q", "esc":
				// Return to repository selection
				m.mode = modeRepoSelect
				m.reportContent = ""
				m.reportTitle = ""
				m.reportScrollOffset = 0
				return m, nil
			case "up", "k":
				if m.reportScrollOffset > 0 {
					m.reportScrollOffset--
				}
			case "down", "j":
				lines := strings.Split(m.reportContent, "\n")
				maxScroll := len(lines) - (m.height - 5)
				if maxScroll < 0 {
					maxScroll = 0
				}
				if m.reportScrollOffset < maxScroll {
					m.reportScrollOffset++
				}
			case "ctrl+u":
				// Scroll up half page
				m.reportScrollOffset -= (m.height - 5) / 2
				if m.reportScrollOffset < 0 {
					m.reportScrollOffset = 0
				}
			case "ctrl+d":
				// Scroll down half page
				lines := strings.Split(m.reportContent, "\n")
				m.reportScrollOffset += (m.height - 5) / 2
				maxScroll := len(lines) - (m.height - 5)
				if maxScroll < 0 {
					maxScroll = 0
				}
				if m.reportScrollOffset > maxScroll {
					m.reportScrollOffset = maxScroll
				}
			case "pgup":
				m.reportScrollOffset -= m.height - 5
				if m.reportScrollOffset < 0 {
					m.reportScrollOffset = 0
				}
			case "pgdown":
				lines := strings.Split(m.reportContent, "\n")
				m.reportScrollOffset += m.height - 5
				maxScroll := len(lines) - (m.height - 5)
				if maxScroll < 0 {
					maxScroll = 0
				}
				if m.reportScrollOffset > maxScroll {
					m.reportScrollOffset = maxScroll
				}
			case "home", "g":
				m.reportScrollOffset = 0
			case "end", "G":
				lines := strings.Split(m.reportContent, "\n")
				maxScroll := len(lines) - (m.height - 5)
				if maxScroll < 0 {
					maxScroll = 0
				}
				m.reportScrollOffset = maxScroll
			}
			return m, nil
		
		case modeTree:
			// Handle search mode keys next
			if m.isSearching {
			switch msg.String() {
			case "enter":
				// Finish search and find results
				m.isSearching = false
				m.performSearch()
				if len(m.searchResults) > 0 {
					m.searchCursor = 0
					m.cursor = m.searchResults[0]
					m.ensureCursorVisible()
				}
				return m, nil
			case "esc":
				// Cancel search
				m.isSearching = false
				m.searchQuery = ""
				return m, nil
			case "backspace":
				// Remove last character from search query
				if len(m.searchQuery) > 0 {
					m.searchQuery = m.searchQuery[:len(m.searchQuery)-1]
				}
				return m, nil
			default:
				// Add character to search query
				if len(msg.String()) == 1 {
					m.searchQuery += msg.String()
				}
				return m, nil
			}
		}
		
		// Handle help popup keys next
		if m.help.ShowAll {
			// Handle keys to close help
			switch msg.String() {
			case "?", "q", "esc", "enter", " ":
				m.help.Toggle()
				return m, nil
			}
			// Ignore all other keys when help is shown
			return m, nil
		}
		
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "?":
			m.help.Toggle()
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
		case "z":
			m.lastKey = "z"
		case "R":
			if m.lastKey == "z" {
				// zR - expand all directories (vim-style)
				m.expandAll()
				m.lastKey = ""
			}
		case "M":
			if m.lastKey == "z" {
				// zM - collapse all directories (vim-style)
				m.collapseAll()
				m.lastKey = ""
			}
		case "o":
			if m.lastKey == "z" {
				// zo - open/expand current directory (vim-style)
				m.expandCurrent()
				m.lastKey = ""
			}
		case "c":
			if m.lastKey == "z" {
				// zc - close/collapse current directory (vim-style)
				m.collapseCurrent()
				m.lastKey = ""
			} else {
				// Regular 'c' - Toggle cold context
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
				m.statusMessage = fmt.Sprintf("Checking %s %s...", itemType, node.Name)
				return m, m.handleRuleAction(relPath, "cold", node.IsDir)
			}
		case "h":
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
			var itemType string
			if node.IsDir {
				itemType = "directory tree"
			} else {
				itemType = "file"
			}
			m.statusMessage = fmt.Sprintf("Checking %s %s...", itemType, node.Name)
			return m, m.handleRuleAction(relPath, "hot", node.IsDir)
		case "a":
			if m.lastKey == "z" {
				// za - toggle fold at cursor (vim-style)
				m.toggleExpanded()
				m.lastKey = ""
			} else {
				// Clear lastKey if 'a' pressed without 'z'
				m.lastKey = ""
			}
		case "tab":
			// Tab - Toggle to repository selection view
			m.mode = modeRepoSelect
			m.help.SetKeys(repoKeys)
			m.help.Title = "Repository Selection - Help"
			// Only discover repos if we haven't already
			if m.projects == nil {
				m.statusMessage = "Discovering repositories..."
				return m, m.discoverReposCmd()
			} else {
				// Repos already discovered, just switch view
				m.statusMessage = ""
				// Reload rules to ensure status indicators are current
				m.loadAndParseRules()
				return m, nil
			}
		case "A":
			// 'A' - Add repository (same as Tab but always rediscovers)
			m.mode = modeRepoSelect
			m.help.SetKeys(repoKeys)
			m.help.Title = "Repository Selection - Help"
			m.statusMessage = "Discovering repositories..."
			return m, m.discoverReposCmd()
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
			m.statusMessage = fmt.Sprintf("Checking %s %s...", itemType, node.Name)
			return m, m.handleRuleAction(relPath, "exclude", node.IsDir)
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
		case ".", "H":
			// Toggle gitignored files visibility
			m.showGitIgnored = !m.showGitIgnored
			if m.showGitIgnored {
				m.statusMessage = "Showing gitignored files"
			} else {
				m.statusMessage = "Hiding gitignored files"
			}
			m.loading = true
			return m, m.loadTreeCmd()
		case "r":
			// Refresh both tree and rules
			m.statusMessage = "Refreshing..."
			m.loadAndParseRules()
			m.loading = true
			return m, m.loadTreeCmd()
		case "/":
			// Enter search mode
			m.isSearching = true
			m.searchQuery = ""
			m.searchResults = nil
			m.searchCursor = 0
			return m, nil
		case "n":
			// Navigate to next search result
			if len(m.searchResults) > 0 {
				m.searchCursor = (m.searchCursor + 1) % len(m.searchResults)
				m.cursor = m.searchResults[m.searchCursor]
				m.ensureCursorVisible()
			}
			return m, nil
		case "N":
			// Navigate to previous search result
			if len(m.searchResults) > 0 {
				m.searchCursor--
				if m.searchCursor < 0 {
					m.searchCursor = len(m.searchResults) - 1
				}
				m.cursor = m.searchResults[m.searchCursor]
				m.ensureCursorVisible()
			}
			return m, nil
		default:
			// Clear lastKey for any other key that's not part of a combo
			if m.lastKey != "" && msg.String() != "g" && msg.String() != "z" {
				m.lastKey = ""
			}
		}
	}

	case reposLoadedMsg:
		if msg.err != nil {
			m.err = msg.err
			m.mode = modeTree
			return m, nil
		}
		m.projects = msg.projects
		m.statusMessage = ""

		// Initialize all workspace repos to collapsed state (zM style)
		// Workspace repos are those with ParentPath set (worktrees) or those that are ecosystems/part of ecosystems
		for _, proj := range m.projects {
			isWorkspaceRepo := proj.ParentEcosystemPath != "" || proj.IsEcosystem
			if isWorkspaceRepo && !proj.IsWorktree {
				m.expandedRepos[proj.Path] = false
			}
		}

		// Build filtered list (will respect collapsed state)
		m.filterRepos()

		if len(m.filteredProjects) == 0 {
			m.statusMessage = "No repositories found. Returning to tree view."
			m.mode = modeTree
			return m, nil
		}

		// Reset cursor and scroll
		m.repoCursor = 0
		m.repoScrollOffset = 0
		m.repoFilter = ""
		m.repoFiltering = false

		return m, nil

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
		// Load rules content (this also parses them)
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

// collapseAll collapses all directories in the tree
func (m *viewModel) collapseAll() {
	m.expandedPaths = make(map[string]bool)
	m.updateVisibleNodes()
}

// expandCurrent expands the currently selected directory
func (m *viewModel) expandCurrent() {
	if m.cursor >= len(m.visibleNodes) {
		return
	}
	node := m.visibleNodes[m.cursor].node
	if node.IsDir && len(node.Children) > 0 {
		m.expandedPaths[node.Path] = true
		m.updateVisibleNodes()
	}
}

// collapseCurrent collapses the currently selected directory
func (m *viewModel) collapseCurrent() {
	if m.cursor >= len(m.visibleNodes) {
		return
	}
	node := m.visibleNodes[m.cursor].node
	if node.IsDir {
		delete(m.expandedPaths, node.Path)
		m.updateVisibleNodes()
	}
}

// expandAllRecursive recursively marks all directories as expanded
func (m *viewModel) expandAllRecursive(node *grove_context.FileNode) {
	if node.IsDir && len(node.Children) > 0 {
		m.expandedPaths[node.Path] = true
		for _, child := range node.Children {
			m.expandAllRecursive(child)
		}
	}
}

// autoExpandToContent expands directories intelligently
func (m *viewModel) autoExpandToContent(node *grove_context.FileNode) {
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
func (m *viewModel) collectVisibleNodes(node *grove_context.FileNode, level int) {
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
			Foreground(core_theme.DefaultTheme.Error.GetForeground()).
			Render(fmt.Sprintf("Error: %v", m.err))
	}

	// Show help overlay if active
	if m.help.ShowAll {
		m.help.SetSize(m.width, m.height)
		return m.help.View()
	}

	// Show repository selection if active (check this first)
	if m.mode == modeRepoSelect {
		if m.projects == nil {
			return "Discovering repositories..."
		}
		return m.renderRepoSelect()
	}

	// Show report view if active
	if m.mode == modeReportView {
		return m.renderReportView()
	}

	// Header
	pruningIndicator := ""
	if m.pruning {
		pruningIndicator = " (Pruning)"
	}
	header := core_theme.DefaultTheme.Header.Render("Grove Context Visualization" + pruningIndicator)
	
	// Subtitle for file tree view
	treeSubtitle := core_theme.DefaultTheme.Muted.Render("Navigate files and directories to add/exclude from context")

	// Calculate split widths (60% for tree, 40% for rules)
	treeWidth := int(float64(m.width) * 0.6)
	rulesWidth := m.width - treeWidth - 3 // -3 for border and padding
	
	// Tree view
	viewportHeight := m.height - 15 // Account for header, subtitle, spacing, footer, and margins
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
	
	// Create scrollbar for the tree view
	treeScrollbar := ""
	if len(m.visibleNodes) > viewportHeight {
		treeScrollbar = m.renderScrollbar(
			len(m.visibleNodes),
			viewportHeight,
			m.scrollOffset,
			viewportHeight,
		)
	}

	// Status message
	statusMsg := ""
	if m.statusMessage != "" {
		statusMsg = core_theme.DefaultTheme.Success.Render(m.statusMessage)
	}

	// Calculate heights for rules and stats
	statsHeight := 8 // Fixed height for stats
	rulesHeight := viewportHeight - statsHeight - 1 // -1 for spacing
	
	// Rules panel
	rulesStyle := core_theme.DefaultTheme.Box.Copy().
		Width(rulesWidth).
		Height(rulesHeight).
		BorderForeground(core_theme.DefaultColors.Border)
	
	rulesPanel := rulesStyle.Render(m.renderRules(rulesWidth, rulesHeight))
	
	// Stats panel
	statsStyle := core_theme.DefaultTheme.Box.Copy().
		Width(rulesWidth).
		Height(statsHeight).
		BorderForeground(core_theme.DefaultColors.Border)
	
	statsPanel := statsStyle.Render(m.renderStats())
	
	// Combine rules and stats vertically
	rightPanel := lipgloss.JoinVertical(lipgloss.Left, rulesPanel, statsPanel)
	
	// Combine tree with scrollbar
	var treeWithScrollbar string
	if treeScrollbar != "" {
		treeContent := lipgloss.NewStyle().
			Width(treeWidth - 4). // Make room for scrollbar
			Render(tree)
		treeWithScrollbar = lipgloss.JoinHorizontal(
			lipgloss.Top,
			treeContent,
			" ", // Small gap
			treeScrollbar,
		)
	} else {
		treeWithScrollbar = tree
	}
	
	// Tree panel
	treeStyle := lipgloss.NewStyle().
		Width(treeWidth).
		Height(viewportHeight).
		Padding(0, 1) // top/bottom: 0, left/right: 1

	treePanel := treeStyle.Render(treeWithScrollbar)
	
	// Combine panels horizontally
	mainContent := lipgloss.JoinHorizontal(lipgloss.Top, treePanel, rightPanel)
	
	// Footer with help hint
	// Footer with search bar or help text
	var footer string
	if m.isSearching {
		searchStyle := lipgloss.NewStyle().
			Foreground(core_theme.DefaultTheme.Success.GetForeground()).
			Bold(true)
		footer = searchStyle.Render(fmt.Sprintf("/%s_", m.searchQuery))
	} else if len(m.searchResults) > 0 {
		resultsStyle := lipgloss.NewStyle().
			Foreground(core_theme.DefaultTheme.Success.GetForeground())
		footer = resultsStyle.Render(fmt.Sprintf("Found %d results (%d of %d) - n/N to navigate", 
			len(m.searchResults), m.searchCursor+1, len(m.searchResults)))
	} else {
		footer = m.help.View()
	}

	// Combine all parts
	parts := []string{
		header,
		treeSubtitle,
		"", // Add margin after subtitle
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
	
	// Highlight if this is a search match
	isSearchMatch := false
	for _, resultIndex := range m.searchResults {
		if resultIndex == index {
			isSearchMatch = true
			break
		}
	}
	if isSearchMatch && len(m.searchResults) > 0 {
		// Apply inverted style for search matches
		style = style.Reverse(true)
	}

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
	
	// Check if this path would be dangerous if added
	relPath, _ := m.getRelativePath(node)
	isDangerous, _ := m.isPathPotentiallyDangerous(relPath)
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

// getIcon returns the appropriate icon for a node
func (m *viewModel) getIcon(node *grove_context.FileNode) string {
	if node.IsDir {
		// Return blue-styled directory icon
		dirStyle := core_theme.DefaultTheme.Info // Blue
		return dirStyle.Render("📁")
	}
	return "📄"
}

// getStatusSymbol returns the status symbol for a node
func (m *viewModel) getStatusSymbol(node *grove_context.FileNode) string {
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

// getStyle returns the appropriate style for a status
func (m *viewModel) getStyle(node *grove_context.FileNode) lipgloss.Style {
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

// renderRepoHelp, renderHelp, and equalizeColumnHeights have been removed.
// Their logic has been migrated to the FullHelp methods of the new keymap structs.

// renderScrollbar creates a vertical scrollbar indicator
func (m *viewModel) renderScrollbar(totalItems, visibleItems, scrollOffset, height int) string {
	if totalItems <= visibleItems {
		// No need for scrollbar if all items fit
		return ""
	}
	
	// Calculate scrollbar metrics
	// The thumb size is proportional to the visible portion
	thumbSize := max(1, (height * visibleItems) / totalItems)
	if thumbSize > height {
		thumbSize = height
	}
	
	// Calculate thumb position
	maxScrollOffset := totalItems - visibleItems
	if maxScrollOffset <= 0 {
		maxScrollOffset = 1
	}
	thumbPosition := (scrollOffset * (height - thumbSize)) / maxScrollOffset
	
	// Build the scrollbar
	var scrollbar strings.Builder
	for i := 0; i < height; i++ {
		if i >= thumbPosition && i < thumbPosition+thumbSize {
			// Thumb
			scrollbar.WriteString("█")
		} else {
			// Track
			scrollbar.WriteString("│")
		}
		if i < height-1 {
			scrollbar.WriteString("\n")
		}
	}
	
	return lipgloss.NewStyle().
		Foreground(core_theme.DefaultTheme.Faint.GetForeground()).
		Render(scrollbar.String())
}

// Helper function for max
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// renderRules generates the rules panel content, truncated to fit the given dimensions
func (m *viewModel) renderRules(width, height int) string {
	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(core_theme.DefaultTheme.Info.GetForeground()).
		MarginBottom(1)
	
	rulesHeader := headerStyle.Render(".grove/rules")
	
	// Format rules content with line numbers and truncation
	rulesLines := strings.Split(m.rulesContent, "\n")
	var numberedLines []string
	maxLines := height - 3 // Account for header, margin, and borders
	
	// Cap at 10 lines maximum
	if maxLines > 10 {
		maxLines = 10
	}
	
	for i, line := range rulesLines {
		if i >= maxLines && maxLines > 0 {
			// Add indicator that there are more lines
			remaining := len(rulesLines) - i
			moreStyle := lipgloss.NewStyle().
				Foreground(core_theme.DefaultTheme.Muted.GetForeground()).
				Italic(true)
			numberedLines = append(numberedLines, moreStyle.Render(fmt.Sprintf("... (%d more lines)", remaining)))
			break
		}
		
		lineNum := lipgloss.NewStyle().
			Foreground(core_theme.DefaultTheme.Muted.GetForeground()).
			Width(3).
			Align(lipgloss.Right).
			Render(fmt.Sprintf("%d", i+1))
		
		// Truncate line if too long, showing the end (file path)
		maxLineWidth := (width - 6) * 2 / 3 // Cut max width by 1/3 and account for line numbers and padding
		if len(line) > maxLineWidth && maxLineWidth > 0 {
			// Show the last part of the path with "..." at the beginning
			line = "..." + line[len(line)-(maxLineWidth-3):]
		}
		
		numberedLines = append(numberedLines, fmt.Sprintf("%s  %s", lineNum, line))
	}
	
	rulesContentFormatted := strings.Join(numberedLines, "\n")
	return lipgloss.JoinVertical(lipgloss.Left, rulesHeader, rulesContentFormatted)
}

// renderStats renders the context statistics
func (m *viewModel) renderStats() string {
	statsStyle := lipgloss.NewStyle().
		Foreground(core_theme.DefaultColors.LightText).
		Padding(0, 1)
	
	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(core_theme.DefaultTheme.Info.GetForeground())
	
	greenStyle := lipgloss.NewStyle().
		Foreground(core_theme.DefaultTheme.Success.GetForeground())
		
	blueStyle := lipgloss.NewStyle().
		Foreground(core_theme.DefaultTheme.Accent.GetForeground())
	
	// Format token counts
	hotTokensStr := grove_context.FormatTokenCount(m.hotTokens)
	coldTokensStr := grove_context.FormatTokenCount(m.coldTokens)
	totalTokensStr := grove_context.FormatTokenCount(m.totalTokens)
	
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
		Foreground(core_theme.DefaultTheme.Muted.GetForeground()).
		Render(strings.Join(legendItems, "\n"))
}
