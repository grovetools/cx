package view

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/mattsolo1/grove-core/pkg/workspace"
	core_theme "github.com/mattsolo1/grove-core/tui/theme"
	grove_context "github.com/mattsolo1/grove-context/pkg/context"
	"github.com/sirupsen/logrus"
)

// --- Page Implementation ---

type repoPageV2 struct {
	sharedState *sharedState

	// Repo view state
	projects      []*workspace.ProjectInfo
	visibleRepos  []*workspace.ProjectInfo // Flattened list of visible repos after filtering
	cursor        int
	scrollOffset  int
	filter        string
	filtering     bool
	expandedRepos map[string]bool

	// Focus mode
	ecosystemPickerMode bool
	focusedEcosystem    *workspace.ProjectInfo

	// Other state
	width, height int
	lastKey       string
	statusMessage string
	workDir       string
}

// --- Messages ---

type reposLoadedMsgV2 struct {
	projects []*workspace.ProjectInfo
	err      error
}

// --- Constructor ---

func NewRepoPageV2(state *sharedState) Page {
	workDir, _ := os.Getwd()
	return &repoPageV2{
		sharedState:   state,
		expandedRepos: make(map[string]bool),
		workDir:       workDir,
	}
}

// --- Page Interface ---

func (p *repoPageV2) Name() string { return "repo" }

func (p *repoPageV2) Init() tea.Cmd {
	return nil
}

func (p *repoPageV2) Focus() tea.Cmd {
	if p.projects == nil {
		p.statusMessage = "Discovering repositories..."
		return p.discoverReposCmd()
	}
	p.updateVisibleRepos()
	return nil
}

func (p *repoPageV2) Blur() {
	p.filtering = false
	p.filter = ""
}

func (p *repoPageV2) SetSize(width, height int) {
	p.width, p.height = width, height
	p.ensureCursorVisible()
}

// --- Commands ---

func (p *repoPageV2) discoverReposCmd() tea.Cmd {
	return func() tea.Msg {
		logger := logrus.New()
		logger.SetOutput(io.Discard)
		discoverySvc := workspace.NewDiscoveryService(logger)

		result, err := discoverySvc.DiscoverAll()
		if err != nil {
			return reposLoadedMsgV2{err: fmt.Errorf("failed to run discovery: %w", err)}
		}

		projectInfos := workspace.TransformToProjectInfo(result)
		enrichOpts := &workspace.EnrichmentOptions{
			FetchGitStatus:      true,
			FetchClaudeSessions: true,
		}
		workspace.EnrichProjects(context.Background(), projectInfos, enrichOpts)

		return reposLoadedMsgV2{projects: projectInfos, err: nil}
	}
}

// --- Update ---

func (p *repoPageV2) Update(msg tea.Msg) (Page, tea.Cmd) {
	switch msg := msg.(type) {
	case reposLoadedMsgV2:
		p.statusMessage = ""
		if msg.err != nil {
			p.sharedState.err = msg.err
			return p, nil
		}
		p.projects = msg.projects
		for _, proj := range p.projects {
			if proj.IsEcosystem && !proj.IsWorktree {
				p.expandedRepos[proj.Path] = false
			}
		}
		p.updateVisibleRepos()
		return p, nil

	case tea.KeyMsg:
		// Handle ecosystem picker mode
		if p.ecosystemPickerMode {
			switch msg.String() {
			case "enter":
				if p.cursor >= 0 && p.cursor < len(p.visibleRepos) {
					p.focusedEcosystem = p.visibleRepos[p.cursor]
					p.ecosystemPickerMode = false
					p.updateVisibleRepos()
					p.cursor = 0
					p.statusMessage = fmt.Sprintf("Focused on: %s", p.focusedEcosystem.Name)
				}
				return p, nil
			case "esc":
				p.ecosystemPickerMode = false
				p.updateVisibleRepos()
				return p, nil
			}
		}

		// Focus mode keys
		if msg.String() == "ctrl+g" {
			if p.focusedEcosystem != nil {
				p.focusedEcosystem = nil
				p.updateVisibleRepos()
				p.cursor = 0
				p.statusMessage = "Cleared focus"
			}
			return p, nil
		}

		// Main key handling
		switch keypress := msg.String(); keypress {
		case "r":
			p.statusMessage = "Refreshing..."
			p.projects = nil
			p.visibleRepos = nil
			p.filter = ""
			p.filtering = false
			return p, tea.Batch(p.discoverReposCmd(), refreshSharedStateCmd())
		case "a":
			if p.cursor >= 0 && p.cursor < len(p.visibleRepos) {
				repo := p.visibleRepos[p.cursor]
				path := p.getPathForRule(repo.Path)
				manager := grove_context.NewManager("")

				if err := manager.ToggleViewDirective(path); err != nil {
					p.statusMessage = fmt.Sprintf("Error: %v", err)
					return p, nil
				}

				wasInView := p.isRepoInView(repo)
				if wasInView {
					p.statusMessage = fmt.Sprintf("Removed %s from view", repo.Name)
				} else {
					p.statusMessage = fmt.Sprintf("Added %s to view", repo.Name)
				}

				return p, refreshSharedStateCmd()
			}
		case "@":
			p.ecosystemPickerMode = true
			p.updateVisibleRepos()
			p.cursor = 0
			return p, nil
		case "h":
			if p.cursor >= 0 && p.cursor < len(p.visibleRepos) {
				repo := p.visibleRepos[p.cursor]
				path := p.getPathForRule(repo.Path) + "/**"
				manager := grove_context.NewManager("")

				status := p.getRepoStatus(repo)

				if status == "hot" {
					p.statusMessage = fmt.Sprintf("Removing %s from hot context...", repo.Name)
					if err := manager.RemoveRuleForPath(path); err != nil {
						p.statusMessage = fmt.Sprintf("Error: %v", err)
						return p, nil
					}
				} else {
					p.statusMessage = fmt.Sprintf("Adding %s to hot context...", repo.Name)
					if err := manager.AppendRule(path, "hot"); err != nil {
						p.statusMessage = fmt.Sprintf("Error: %v", err)
						return p, nil
					}
				}

				return p, refreshSharedStateCmd()
			}
		case "x":
			if p.cursor >= 0 && p.cursor < len(p.visibleRepos) {
				repo := p.visibleRepos[p.cursor]
				path := p.getPathForRule(repo.Path) + "/**"
				manager := grove_context.NewManager("")

				isExcluded := p.isRepoExcluded(repo)

				if isExcluded {
					p.statusMessage = fmt.Sprintf("Removing exclusion for %s...", repo.Name)
					if err := manager.RemoveRuleForPath(path); err != nil {
						p.statusMessage = fmt.Sprintf("Error: %v", err)
						return p, nil
					}
				} else {
					p.statusMessage = fmt.Sprintf("Excluding %s...", repo.Name)
					if err := manager.AppendRule(path, "exclude"); err != nil {
						p.statusMessage = fmt.Sprintf("Error: %v", err)
						return p, nil
					}
				}

				return p, refreshSharedStateCmd()
			}
		case "c":
			if p.lastKey == "z" {
				p.lastKey = ""
				if p.cursor >= 0 && p.cursor < len(p.visibleRepos) {
					repo := p.visibleRepos[p.cursor]
					if repo.IsEcosystem || repo.IsWorktree {
						p.expandedRepos[repo.Path] = false
						p.updateVisibleRepos()
						p.statusMessage = fmt.Sprintf("Collapsed %s", repo.Name)
					}
				}
			} else {
				// Toggle cold context
				if p.cursor >= 0 && p.cursor < len(p.visibleRepos) {
					repo := p.visibleRepos[p.cursor]
					path := p.getPathForRule(repo.Path) + "/**"
					manager := grove_context.NewManager("")

					status := p.getRepoStatus(repo)

					if status == "cold" {
						p.statusMessage = fmt.Sprintf("Removing %s from cold context...", repo.Name)
						if err := manager.RemoveRuleForPath(path); err != nil {
							p.statusMessage = fmt.Sprintf("Error: %v", err)
							return p, nil
						}
					} else {
						p.statusMessage = fmt.Sprintf("Adding %s to cold context...", repo.Name)
						if err := manager.AppendRule(path, "cold"); err != nil {
							p.statusMessage = fmt.Sprintf("Error: %v", err)
							return p, nil
						}
					}

					return p, refreshSharedStateCmd()
				}
			}
		case "up", "k":
			if p.cursor > 0 {
				p.cursor--
				p.ensureCursorVisible()
			}
		case "down", "j":
			if p.cursor < len(p.visibleRepos)-1 {
				p.cursor++
				p.ensureCursorVisible()
			}
		case "pgup":
			p.cursor -= 10
			if p.cursor < 0 {
				p.cursor = 0
			}
			p.ensureCursorVisible()
		case "pgdown":
			p.cursor += 10
			if p.cursor >= len(p.visibleRepos) {
				p.cursor = len(p.visibleRepos) - 1
			}
			p.ensureCursorVisible()
		case "ctrl+u":
			viewportHeight := p.height - 4
			p.cursor -= viewportHeight / 2
			if p.cursor < 0 {
				p.cursor = 0
			}
			p.ensureCursorVisible()
		case "ctrl+d":
			viewportHeight := p.height - 4
			p.cursor += viewportHeight / 2
			if p.cursor >= len(p.visibleRepos) {
				p.cursor = len(p.visibleRepos) - 1
			}
			p.ensureCursorVisible()
		case "home", "g":
			p.cursor = 0
			p.scrollOffset = 0
		case "end", "G":
			p.cursor = len(p.visibleRepos) - 1
			p.ensureCursorVisible()
		case "/":
			p.filtering = true
			p.filter = ""
			p.statusMessage = "Type to filter repositories (ESC to clear)..."
		case "z":
			p.lastKey = "z"
			p.statusMessage = "Folding: o=open, c=close, R=open all, M=close all"
			return p, nil
		case "o":
			if p.lastKey == "z" {
				p.lastKey = ""
				if p.cursor >= 0 && p.cursor < len(p.visibleRepos) {
					repo := p.visibleRepos[p.cursor]
					if repo.IsEcosystem || repo.IsWorktree {
						p.expandedRepos[repo.Path] = true
						p.updateVisibleRepos()
						p.statusMessage = fmt.Sprintf("Expanded %s", repo.Name)
					}
				}
			}
		case "R":
			if p.lastKey == "z" {
				p.lastKey = ""
				for _, proj := range p.projects {
					isWorkspaceRepo := proj.ParentEcosystemPath != "" || proj.IsEcosystem
					if isWorkspaceRepo && !proj.IsWorktree {
						p.expandedRepos[proj.Path] = true
					}
				}
				p.updateVisibleRepos()
				p.ensureCursorVisible()
				p.statusMessage = "Expanded all workspace repos"
			}
		case "M":
			if p.lastKey == "z" {
				p.lastKey = ""
				for _, proj := range p.projects {
					isWorkspaceRepo := proj.ParentEcosystemPath != "" || proj.IsEcosystem
					if isWorkspaceRepo && !proj.IsWorktree {
						p.expandedRepos[proj.Path] = false
					}
				}
				p.updateVisibleRepos()
				p.cursor = 0
				p.scrollOffset = 0
				p.statusMessage = "Collapsed all workspace repos"
			}
		case "backspace":
			if p.filtering && len(p.filter) > 0 {
				p.filter = p.filter[:len(p.filter)-1]
				p.updateVisibleRepos()
				if p.filter == "" {
					p.statusMessage = "Type to filter repositories (ESC to clear)..."
				}
			}
		case "escape":
			if p.filtering {
				p.filtering = false
				p.filter = ""
				p.updateVisibleRepos()
				p.statusMessage = ""
			}
		default:
			if p.filtering && len(keypress) == 1 {
				p.filter += keypress
				p.updateVisibleRepos()
				p.statusMessage = fmt.Sprintf("Filter: %s", p.filter)
			} else {
				p.lastKey = ""
			}
		}
	}
	return p, nil
}

// --- View ---

func (p *repoPageV2) View() string {
	if p.projects == nil {
		return "Discovering repositories..."
	}

	var b strings.Builder

	// Calculate viewport height (room for tab header and help footer managed by pager)
	viewportHeight := p.height - 4
	if viewportHeight < 1 {
		viewportHeight = 1
	}

	// Render visible repos
	for i := p.scrollOffset; i < len(p.visibleRepos) && i < p.scrollOffset+viewportHeight; i++ {
		line := p.renderRepo(i)
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

	return content
}

func (p *repoPageV2) renderRepo(index int) string {
	if index >= len(p.visibleRepos) {
		return ""
	}

	repo := p.visibleRepos[index]
	var line string

	// Status indicator
	status := p.getRepoStatus(repo)
	isExcluded := p.isRepoExcluded(repo)
	isInView := p.isRepoInView(repo)

	if isInView {
		line += core_theme.DefaultTheme.Info.Render("👁️ ")
	} else if isExcluded {
		line += core_theme.DefaultTheme.Error.Render("🚫 ")
	} else {
		switch status {
		case "hot":
			line += core_theme.DefaultTheme.Success.Render("✓ ")
		case "cold":
			line += lipgloss.NewStyle().Foreground(core_theme.DefaultColors.Blue).Render("❄️ ")
		default:
			line += "  "
		}
	}

	// Cursor indicator
	if index == p.cursor {
		line += "▶ "
	} else {
		line += "  "
	}

	// Build hierarchical name with indentation
	name := repo.Name
	indent := ""

	if repo.IsEcosystem {
		hasChildren := false
		for _, proj := range p.projects {
			if (proj.IsWorktree && proj.ParentPath == repo.Path) ||
				(proj.ParentEcosystemPath == repo.Path && proj.WorktreeName == "") {
				hasChildren = true
				break
			}
		}
		if hasChildren {
			if p.expandedRepos[repo.Path] {
				name = "▾ " + name
			} else {
				name = "▸ " + name
			}
		}
	} else if repo.IsWorktree {
		indent = "  "
		hasChildren := false
		for _, proj := range p.projects {
			if proj.WorktreeName == repo.WorktreeName && proj.ParentEcosystemPath == repo.ParentEcosystemPath {
				hasChildren = true
				break
			}
		}
		if hasChildren {
			if p.expandedRepos[repo.Path] {
				name = "▾ └─ " + name
			} else {
				name = "▸ └─ " + name
			}
		} else {
			name = "└─ " + name
		}
	} else if repo.WorktreeName != "" {
		indent = "    "
		name = "└─ " + name
	} else if repo.ParentEcosystemPath != "" {
		indent = "  "
		name = "└─ " + name
	}

	line += indent

	// Style the name
	nameStyle := lipgloss.NewStyle().Foreground(core_theme.DefaultColors.LightText)
	if index == p.cursor {
		nameStyle = core_theme.DefaultTheme.Selected
	} else if repo.IsWorktree {
		nameStyle = core_theme.DefaultTheme.Faint
	}

	line += nameStyle.Render(name)

	// Add path info
	pathInfo := fmt.Sprintf(" (%s)", repo.Path)
	line += core_theme.DefaultTheme.Muted.Render(pathInfo)

	return line
}

// --- Helper Functions ---

func (p *repoPageV2) updateVisibleRepos() {
	p.visibleRepos = nil

	// Handle ecosystem picker mode
	if p.ecosystemPickerMode {
		var ecosystemRepos []*workspace.ProjectInfo
		var worktrees []*workspace.ProjectInfo

		for _, proj := range p.projects {
			if proj.IsEcosystem && !proj.IsWorktree {
				ecosystemRepos = append(ecosystemRepos, proj)
			} else if proj.IsWorktree {
				worktrees = append(worktrees, proj)
			}
		}

		for _, eco := range ecosystemRepos {
			p.visibleRepos = append(p.visibleRepos, eco)
			for _, wt := range worktrees {
				if wt.ParentPath == eco.Path {
					p.visibleRepos = append(p.visibleRepos, wt)
				}
			}
		}
		return
	}

	// Determine which projects to filter from based on focus mode
	var projectsToFilter []*workspace.ProjectInfo
	if p.focusedEcosystem != nil {
		projectsToFilter = append(projectsToFilter, p.focusedEcosystem)

		if p.focusedEcosystem.IsWorktree {
			parentEcoPath := p.focusedEcosystem.ParentPath
			for _, proj := range p.projects {
				if proj.WorktreeName == p.focusedEcosystem.Name &&
					proj.ParentEcosystemPath == parentEcoPath &&
					proj.Path != p.focusedEcosystem.Path {
					projectsToFilter = append(projectsToFilter, proj)
				}
			}
		} else if p.focusedEcosystem.IsEcosystem {
			for _, proj := range p.projects {
				if proj.ParentEcosystemPath == p.focusedEcosystem.Path {
					projectsToFilter = append(projectsToFilter, proj)
				}
			}
		}
	} else {
		projectsToFilter = p.projects
	}

	// Apply text filter if active
	if p.filter != "" {
		filter := strings.ToLower(p.filter)
		var filtered []*workspace.ProjectInfo
		for _, proj := range projectsToFilter {
			branch := ""
			if proj.GitStatus != nil {
				if extStatus, ok := proj.GitStatus.(*workspace.ExtendedGitStatus); ok && extStatus.StatusInfo != nil {
					branch = extStatus.StatusInfo.Branch
				}
			}

			if strings.Contains(strings.ToLower(proj.Name), filter) ||
				strings.Contains(strings.ToLower(proj.Path), filter) ||
				strings.Contains(strings.ToLower(branch), filter) {
				filtered = append(filtered, proj)
			}
		}
		projectsToFilter = filtered
	}

	// Build hierarchical visible list
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
			worktreeProjects = append(worktreeProjects, proj)
		} else if proj.ParentEcosystemPath != "" {
			mainEcosystemProjects = append(mainEcosystemProjects, proj)
		} else {
			clonedRepos = append(clonedRepos, proj)
		}
	}

	// Build hierarchical structure
	for _, eco := range ecosystemRepos {
		p.visibleRepos = append(p.visibleRepos, eco)

		if p.expandedRepos[eco.Path] {
			for _, proj := range mainEcosystemProjects {
				if proj.ParentEcosystemPath == eco.Path {
					p.visibleRepos = append(p.visibleRepos, proj)
				}
			}

			for _, wt := range worktrees {
				if wt.ParentPath == eco.Path {
					p.visibleRepos = append(p.visibleRepos, wt)

					if p.expandedRepos[wt.Path] {
						for _, wtProj := range worktreeProjects {
							if wtProj.WorktreeName == wt.WorktreeName && wtProj.ParentEcosystemPath == eco.Path {
								p.visibleRepos = append(p.visibleRepos, wtProj)
							}
						}
					}
				}
			}
		}
	}

	if p.focusedEcosystem == nil {
		p.visibleRepos = append(p.visibleRepos, clonedRepos...)
	}

	// Ensure cursor is within bounds
	if p.cursor >= len(p.visibleRepos) {
		p.cursor = len(p.visibleRepos) - 1
		if p.cursor < 0 {
			p.cursor = 0
		}
	}
	p.ensureCursorVisible()
}

func (p *repoPageV2) ensureCursorVisible() {
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

func (p *repoPageV2) getPathForRule(repoPath string) string {
	relPath, err := filepath.Rel(p.workDir, repoPath)
	if err != nil {
		return repoPath
	}

	traversalCount := strings.Count(relPath, "../")
	if traversalCount > 2 {
		return repoPath
	}

	return relPath
}

func (p *repoPageV2) getRepoStatus(proj *workspace.ProjectInfo) string {
	repoPath := p.getPathForRule(proj.Path)

	// Check hot rules
	for _, rule := range p.sharedState.hotRules {
		cleanRule := strings.TrimSpace(rule)
		if strings.HasPrefix(cleanRule, "!") {
			continue
		}
		if p.ruleMatchesPath(cleanRule, repoPath) {
			return "hot"
		}
	}

	// Check cold rules
	for _, rule := range p.sharedState.coldRules {
		cleanRule := strings.TrimSpace(rule)
		if strings.HasPrefix(cleanRule, "!") {
			continue
		}
		if p.ruleMatchesPath(cleanRule, repoPath) {
			return "cold"
		}
	}

	return ""
}

func (p *repoPageV2) isRepoExcluded(proj *workspace.ProjectInfo) bool {
	repoPath := p.getPathForRule(proj.Path)

	allRules := append(p.sharedState.hotRules, p.sharedState.coldRules...)
	for _, rule := range allRules {
		if strings.HasPrefix(rule, "!") {
			excludePattern := strings.TrimPrefix(rule, "!")
			if p.ruleMatchesPath(excludePattern, repoPath) {
				return true
			}
		}
	}

	return false
}

func (p *repoPageV2) isRepoInView(proj *workspace.ProjectInfo) bool {
	repoPath := p.getPathForRule(proj.Path)
	for _, viewPath := range p.sharedState.viewPaths {
		if viewPath == repoPath {
			return true
		}
	}
	return false
}

func (p *repoPageV2) ruleMatchesPath(rule, path string) bool {
	rule = strings.TrimSpace(rule)
	path = strings.TrimSpace(path)

	// Remove trailing /** from rule for comparison
	cleanRule := strings.TrimSuffix(rule, "/**")
	cleanRule = strings.TrimSuffix(cleanRule, "/*")

	// Check exact match
	if cleanRule == path {
		return true
	}

	// Check if rule is a parent directory of the path
	if strings.HasPrefix(path+"/", cleanRule+"/") {
		return true
	}

	return false
}
