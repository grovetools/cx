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

type repoPage struct {
	sharedState *sharedState

	// Repo view state
	projects         []*workspace.ProjectInfo
	filteredProjects []*workspace.ProjectInfo
	repoCursor       int
	repoScrollOffset int
	repoFilter       string
	repoFiltering    bool
	expandedRepos    map[string]bool

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

type reposLoadedMsg struct {
	projects []*workspace.ProjectInfo
	err      error
}

// --- Constructor ---

func NewRepoPage(state *sharedState) Page {
	workDir, _ := os.Getwd()
	return &repoPage{
		sharedState:   state,
		expandedRepos: make(map[string]bool),
		workDir:       workDir,
	}
}

// --- Page Interface ---

func (p *repoPage) Name() string { return "repo" }

func (p *repoPage) Init() tea.Cmd {
	// Projects are loaded on focus to avoid loading if the page is never viewed
	return nil
}

func (p *repoPage) Focus() tea.Cmd {
	if p.projects == nil {
		p.statusMessage = "Discovering repositories..."
		return p.discoverReposCmd()
	}
	// If already loaded, just refresh the view based on current rules
	p.filterRepos()
	return nil
}

func (p *repoPage) Blur() {
	p.repoFiltering = false
	p.repoFilter = ""
}

func (p *repoPage) SetSize(width, height int) {
	p.width, p.height = width, height
}

// --- Commands ---

func (p *repoPage) discoverReposCmd() tea.Cmd {
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

// --- Update ---

func (p *repoPage) Update(msg tea.Msg) (Page, tea.Cmd) {
	switch msg := msg.(type) {
	case reposLoadedMsg:
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
		p.filterRepos()
		return p, nil

	case tea.KeyMsg:
		// Handle ecosystem picker mode
		if p.ecosystemPickerMode {
			switch msg.String() {
			case "enter":
				// Select ecosystem to focus
				if p.repoCursor >= 0 && p.repoCursor < len(p.filteredProjects) {
					p.focusedEcosystem = p.filteredProjects[p.repoCursor]
					p.ecosystemPickerMode = false
					p.filterRepos()
					p.repoCursor = 0
					p.statusMessage = fmt.Sprintf("Focused on: %s", p.focusedEcosystem.Name)
				}
				return p, nil
			case "esc":
				// Cancel ecosystem picker
				p.ecosystemPickerMode = false
				p.filterRepos()
				return p, nil
			}
		}

		// Focus mode keys
		if msg.String() == "ctrl+g" {
			if p.focusedEcosystem != nil {
				p.focusedEcosystem = nil
				p.filterRepos()
				p.repoCursor = 0
				p.statusMessage = "Cleared focus"
			}
			return p, nil
		}

		// Main key handling
		switch keypress := msg.String(); keypress {
		case "r":
			// Refresh repository list, rules, and stats
			p.statusMessage = "Refreshing..."
			// Clear and rediscover repos
			p.projects = nil
			p.filteredProjects = nil
			p.repoFilter = ""
			p.repoFiltering = false
			return p, tea.Batch(p.discoverReposCmd(), refreshSharedStateCmd())
		case "a":
			// Add/remove repo from view
			if p.repoCursor >= 0 && p.repoCursor < len(p.filteredProjects) {
				repo := p.filteredProjects[p.repoCursor]
				path := p.getPathForRule(repo.Path)
				manager := grove_context.NewManager("")

				if err := manager.ToggleViewDirective(path); err != nil {
					p.statusMessage = fmt.Sprintf("Error: %v", err)
					return p, nil
				}

				// Check if it was added or removed to set status message
				wasInView := p.isRepoInView(repo)
				if wasInView {
					p.statusMessage = fmt.Sprintf("Removed %s from view", repo.Name)
				} else {
					p.statusMessage = fmt.Sprintf("Added %s to view", repo.Name)
				}

				return p, refreshSharedStateCmd()
			}
		case "@":
			// Enter ecosystem picker mode
			p.ecosystemPickerMode = true
			p.filterRepos()
			p.repoCursor = 0
			return p, nil
		case "h":
			// Toggle hot context
			if p.repoCursor >= 0 && p.repoCursor < len(p.filteredProjects) {
				repo := p.filteredProjects[p.repoCursor]
				path := p.getPathForRule(repo.Path) + "/**"
				manager := grove_context.NewManager("")

				// Check current status
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
			// Toggle exclude
			if p.repoCursor >= 0 && p.repoCursor < len(p.filteredProjects) {
				repo := p.filteredProjects[p.repoCursor]
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
		case "up", "k":
			if p.repoCursor > 0 {
				p.repoCursor--
				p.ensureRepoCursorVisible()
			}
		case "down", "j":
			if p.repoCursor < len(p.filteredProjects)-1 {
				p.repoCursor++
				p.ensureRepoCursorVisible()
			}
		case "pgup":
			p.repoCursor -= 10
			if p.repoCursor < 0 {
				p.repoCursor = 0
			}
			p.ensureRepoCursorVisible()
		case "pgdown":
			p.repoCursor += 10
			if p.repoCursor >= len(p.filteredProjects) {
				p.repoCursor = len(p.filteredProjects) - 1
			}
			p.ensureRepoCursorVisible()
		case "ctrl+u":
			viewportHeight := p.height - 8
			listHeight := viewportHeight
			if listHeight < 5 {
				listHeight = 5
			}
			p.repoCursor -= listHeight / 2
			if p.repoCursor < 0 {
				p.repoCursor = 0
			}
			p.ensureRepoCursorVisible()
		case "ctrl+d":
			viewportHeight := p.height - 8
			listHeight := viewportHeight
			if listHeight < 5 {
				listHeight = 5
			}
			p.repoCursor += listHeight / 2
			if p.repoCursor >= len(p.filteredProjects) {
				p.repoCursor = len(p.filteredProjects) - 1
			}
			p.ensureRepoCursorVisible()
		case "home", "g":
			p.repoCursor = 0
			p.repoScrollOffset = 0
		case "end", "G":
			p.repoCursor = len(p.filteredProjects) - 1
			p.ensureRepoCursorVisible()
		case "/":
			p.repoFiltering = true
			p.repoFilter = ""
			p.statusMessage = "Type to filter repositories (ESC to clear)..."
		case "z":
			p.lastKey = "z"
			p.statusMessage = "Folding: o=open, c=close, R=open all, M=close all"
			return p, nil
		case "o":
			if p.lastKey == "z" {
				p.lastKey = ""
				if p.repoCursor >= 0 && p.repoCursor < len(p.filteredProjects) {
					repo := p.filteredProjects[p.repoCursor]
					if repo.IsEcosystem || repo.IsWorktree {
						p.expandedRepos[repo.Path] = true
						p.filterRepos()
						p.statusMessage = fmt.Sprintf("Expanded %s", repo.Name)
					}
				}
			}
		case "c":
			if p.lastKey == "z" {
				p.lastKey = ""
				if p.repoCursor >= 0 && p.repoCursor < len(p.filteredProjects) {
					repo := p.filteredProjects[p.repoCursor]
					if repo.IsEcosystem || repo.IsWorktree {
						p.expandedRepos[repo.Path] = false
						p.filterRepos()
						p.statusMessage = fmt.Sprintf("Collapsed %s", repo.Name)
					}
				}
			} else {
				// Toggle cold context
				if p.repoCursor >= 0 && p.repoCursor < len(p.filteredProjects) {
					repo := p.filteredProjects[p.repoCursor]
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
		case "R":
			if p.lastKey == "z" {
				p.lastKey = ""
				for _, proj := range p.projects {
					isWorkspaceRepo := proj.ParentEcosystemPath != "" || proj.IsEcosystem
					if isWorkspaceRepo && !proj.IsWorktree {
						p.expandedRepos[proj.Path] = true
					}
				}
				p.filterRepos()
				p.ensureRepoCursorVisible()
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
				p.filterRepos()
				p.repoCursor = 0
				p.repoScrollOffset = 0
				p.statusMessage = "Collapsed all workspace repos"
			}
		case "backspace":
			if p.repoFiltering && len(p.repoFilter) > 0 {
				p.repoFilter = p.repoFilter[:len(p.repoFilter)-1]
				p.filterRepos()
				if p.repoFilter == "" {
					p.statusMessage = "Type to filter repositories (ESC to clear)..."
				}
			}
		case "escape":
			if p.repoFiltering {
				p.repoFiltering = false
				p.repoFilter = ""
				p.filterRepos()
				p.statusMessage = ""
			}
		default:
			if p.repoFiltering && len(keypress) == 1 {
				p.repoFilter += keypress
				p.filterRepos()
				p.statusMessage = fmt.Sprintf("Filter: %s", p.repoFilter)
			} else {
				p.lastKey = ""
			}
		}
	}
	return p, nil
}

// --- View ---

func (p *repoPage) View() string {
	if p.projects == nil {
		return "Discovering repositories..."
	}

	// Header - different based on mode
	var header string
	var subtitle string

	if p.ecosystemPickerMode {
		header = core_theme.DefaultTheme.Info.Render("[Select Ecosystem to Focus]")
		subtitle = core_theme.DefaultTheme.Muted.Render("Press Enter to focus on selected ecosystem, Esc to cancel")
	} else if p.focusedEcosystem != nil {
		focusIndicator := core_theme.DefaultTheme.Info.Render(fmt.Sprintf("[Focus: %s]", p.focusedEcosystem.Name))
		header = core_theme.DefaultTheme.Header.Render("Select Repository") + " " + focusIndicator
		subtitle = core_theme.DefaultTheme.Muted.Render("Press ctrl+g to clear focus")
	} else {
		header = core_theme.DefaultTheme.Header.Render("Select Repository")
		subtitle = core_theme.DefaultTheme.Muted.Render("Add repositories to rules file for further context refinement")
	}

	// Filter display
	if p.repoFilter != "" {
		filterStyle := core_theme.DefaultTheme.Muted
		header += filterStyle.Render(fmt.Sprintf(" (filter: %s)", p.repoFilter))
	}

	var b strings.Builder

	// Styles for repo list
	selectedStyle := core_theme.DefaultTheme.Selected
	normalStyle := lipgloss.NewStyle().Foreground(core_theme.DefaultColors.LightText)
	worktreeStyle := core_theme.DefaultTheme.Faint
	clonedStyle := lipgloss.NewStyle().Foreground(core_theme.DefaultColors.Orange)

	// Calculate visible range
	viewportHeight := p.height - 4
	if viewportHeight < 1 {
		viewportHeight = 1
	}
	visibleEnd := p.repoScrollOffset + viewportHeight
	if visibleEnd > len(p.filteredProjects) {
		visibleEnd = len(p.filteredProjects)
	}

	// Find max name width for alignment
	maxNameWidth := 20
	for _, repo := range p.filteredProjects {
		nameLen := len(repo.Name)
		if repo.IsWorktree {
			nameLen += 3
		}
		if nameLen > maxNameWidth && nameLen < 40 {
			maxNameWidth = nameLen
		}
	}

	// Track sections
	type repoSection int
	const (
		sectionEcosystem repoSection = iota
		sectionCloned
	)

	getSection := func(proj *workspace.ProjectInfo) repoSection {
		if proj.IsEcosystem || proj.IsWorktree || proj.ParentEcosystemPath != "" || proj.WorktreeName != "" {
			return sectionEcosystem
		}
		return sectionCloned
	}

	currentSection := repoSection(-1)

	for i := p.repoScrollOffset; i < visibleEnd; i++ {
		repo := p.filteredProjects[i]
		section := getSection(repo)

		// Add section headers
		if section != currentSection {
			if currentSection != repoSection(-1) {
				separatorLine := "  " + core_theme.DefaultTheme.Faint.Render("─────────────────────────────────────────────────")
				b.WriteString(separatorLine + "\n")
			}

			headerStyle := core_theme.DefaultTheme.TableHeader
			switch section {
			case sectionEcosystem:
				b.WriteString("  " + headerStyle.Render("ECOSYSTEM REPOSITORIES") + "\n")
			case sectionCloned:
				b.WriteString("  " + headerStyle.Render("CLONED REPOSITORIES") + "\n")
				b.WriteString("  " + headerStyle.Render("URL                                      VERSION  COMMIT   STATUS       REPORT") + "\n")
			}
			currentSection = section
		}

		// Build the line
		var line string

		// Status indicator
		status := p.getRepoStatus(repo)
		isExcluded := p.isRepoExcluded(repo)
		isInView := p.isRepoInView(repo)

		if isInView {
			viewStyle := core_theme.DefaultTheme.Info
			line += viewStyle.Render("👁️") + " "
		} else if isExcluded {
			excludeStyle := core_theme.DefaultTheme.Error
			line += excludeStyle.Render("🚫") + " "
		} else {
			switch status {
			case "hot":
				hotStyle := core_theme.DefaultTheme.Success
				line += hotStyle.Render("✓") + " "
			case "cold":
				coldStyle := lipgloss.NewStyle().Foreground(core_theme.DefaultColors.Blue)
				line += coldStyle.Render("❄️") + " "
			default:
				line += "  "
			}
		}

		// Cursor indicator
		if i == p.repoCursor {
			line += "▶ "
		} else {
			line += "  "
		}

		if section == sectionEcosystem {
			// Ecosystem section - hierarchical tree
			name := repo.Name

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
					name = "  └─ " + name
				}
			} else if repo.WorktreeName != "" {
				name = "    └─ " + name
			} else if repo.ParentEcosystemPath != "" {
				name = "  └─ " + name
			}

			var nameStyled string
			if i == p.repoCursor {
				nameStyled = selectedStyle.Render(fmt.Sprintf("%-*s", maxNameWidth, name))
			} else if repo.IsWorktree {
				nameStyled = worktreeStyle.Render(fmt.Sprintf("%-*s", maxNameWidth, name))
			} else {
				nameStyled = normalStyle.Render(fmt.Sprintf("%-*s", maxNameWidth, name))
			}
			line += nameStyled + "  "

		} else {
			// Cloned repos - tabular format
			url := repo.Name
			if len(url) > 40 {
				url = url[:37] + "..."
			}

			var urlStyled string
			if i == p.repoCursor {
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
			if i == p.repoCursor {
				line += selectedStyle.Render(fmt.Sprintf("%-7s", version)) + "  "
			} else {
				line += normalStyle.Render(fmt.Sprintf("%-7s", version)) + "  "
			}

			// Commit column
			commit := repo.Commit
			if commit == "" {
				commit = "-"
			}
			if i == p.repoCursor {
				line += selectedStyle.Render(fmt.Sprintf("%-7s", commit)) + "  "
			} else {
				line += normalStyle.Render(fmt.Sprintf("%-7s", commit)) + "  "
			}

			// Status column
			auditStatus := repo.AuditStatus
			if auditStatus == "" {
				auditStatus = "not_audited"
			}
			statusStyle := p.getAuditStatusStyle(auditStatus)
			if i == p.repoCursor {
				statusStyled := selectedStyle.Render(fmt.Sprintf("%-11s", auditStatus))
				line += statusStyled + "  "
			} else {
				line += statusStyle.Render(fmt.Sprintf("%-11s", auditStatus)) + "  "
			}

			// Report column
			reportIndicator := ""
			if repo.ReportPath != "" {
				reportIndicator = "✓"
			}
			if i == p.repoCursor {
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
	if len(p.filteredProjects) > viewportHeight {
		scrollInfo := fmt.Sprintf("\n[%d-%d of %d]",
			p.repoScrollOffset+1,
			visibleEnd,
			len(p.filteredProjects))
		b.WriteString(lipgloss.NewStyle().
			Foreground(core_theme.DefaultTheme.Muted.GetForeground()).
			Render(scrollInfo))
	}

	content := b.String()

	// Footer with status message
	if p.statusMessage != "" {
		statusStyle := core_theme.DefaultTheme.Success
		if strings.Contains(p.statusMessage, "Error") {
			statusStyle = core_theme.DefaultTheme.Error
		}
		content += "\n" + statusStyle.Render(p.statusMessage)
	}

	// Combine all parts
	parts := []string{
		header,
		subtitle,
		content,
	}

	return lipgloss.JoinVertical(lipgloss.Left, parts...)
}

// --- Helper Functions ---

func (p *repoPage) filterRepos() {
	p.filteredProjects = []*workspace.ProjectInfo{}

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
			p.filteredProjects = append(p.filteredProjects, eco)
			for _, wt := range worktrees {
				if wt.ParentPath == eco.Path {
					p.filteredProjects = append(p.filteredProjects, wt)
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

	if p.repoFilter == "" {
		// No text filter - organize repos hierarchically
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
			p.filteredProjects = append(p.filteredProjects, eco)

			if p.expandedRepos[eco.Path] {
				for _, proj := range mainEcosystemProjects {
					if proj.ParentEcosystemPath == eco.Path {
						p.filteredProjects = append(p.filteredProjects, proj)
					}
				}

				for _, wt := range worktrees {
					if wt.ParentPath == eco.Path {
						p.filteredProjects = append(p.filteredProjects, wt)

						if p.expandedRepos[wt.Path] {
							for _, wtProj := range worktreeProjects {
								if wtProj.WorktreeName == wt.WorktreeName && wtProj.ParentEcosystemPath == eco.Path {
									p.filteredProjects = append(p.filteredProjects, wtProj)
								}
							}
						}
					}
				}
			}
		}

		if p.focusedEcosystem == nil {
			p.filteredProjects = append(p.filteredProjects, clonedRepos...)
		}

	} else {
		// Apply filter
		filter := strings.ToLower(p.repoFilter)

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
				p.filteredProjects = append(p.filteredProjects, proj)
			}
		}
	}

	// Reset cursor if out of bounds
	if p.repoCursor >= len(p.filteredProjects) {
		p.repoCursor = len(p.filteredProjects) - 1
		if p.repoCursor < 0 {
			p.repoCursor = 0
		}
	}
	p.ensureRepoCursorVisible()
}

func (p *repoPage) ensureRepoCursorVisible() {
	viewportHeight := p.height - 8
	listHeight := viewportHeight

	if listHeight < 5 {
		listHeight = 5
	}

	if p.repoCursor < p.repoScrollOffset {
		p.repoScrollOffset = p.repoCursor
	} else if p.repoCursor >= p.repoScrollOffset+listHeight {
		p.repoScrollOffset = p.repoCursor - listHeight + 1
	}
}

func (p *repoPage) getPathForRule(repoPath string) string {
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

func (p *repoPage) getRepoStatus(proj *workspace.ProjectInfo) string {
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

func (p *repoPage) isRepoExcluded(proj *workspace.ProjectInfo) bool {
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

func (p *repoPage) isRepoInView(proj *workspace.ProjectInfo) bool {
	repoPath := p.getPathForRule(proj.Path)
	for _, viewPath := range p.sharedState.viewPaths {
		if viewPath == repoPath {
			return true
		}
	}
	return false
}

func (p *repoPage) ruleMatchesPath(rule, path string) bool {
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

func (p *repoPage) getAuditStatusStyle(status string) lipgloss.Style {
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
