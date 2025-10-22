package view

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/mattsolo1/grove-core/pkg/workspace"
	"github.com/mattsolo1/grove-core/tui/components/navigator"
	"github.com/mattsolo1/grove-core/tui/components/table"
	"github.com/mattsolo1/grove-core/tui/theme"
	"github.com/mattsolo1/grove-context/pkg/context"
)

// --- Page Implementation ---

type repoPage struct {
	sharedState  *sharedState
	nav          navigator.Model
	width        int
	height       int
	scrollOffset int
}

// --- Constructor ---

func NewRepoPage(state *sharedState) Page {
	// Initial empty config; projects will be loaded in Focus()
	navCfg := navigator.Config{
		Projects: []workspace.WorkspaceNode{},
	}
	nav := navigator.New(navCfg)

	p := &repoPage{
		sharedState: state,
		nav:         nav,
	}

	// Set custom key handler for context actions
	p.nav.CustomKeyHandler = p.handleKeypress

	return p
}

// handleKeypress implements the custom key handler for the navigator.
func (p *repoPage) handleKeypress(m navigator.Model, msg tea.KeyMsg) (navigator.Model, tea.Cmd) {
	filtered := m.GetFiltered()
	if len(filtered) == 0 {
		return m, nil
	}

	cursor := m.GetCursor()
	if cursor >= len(filtered) {
		return m, nil
	}

	selectedNode := filtered[cursor]

	var cmd tea.Cmd
	var ruleAction string

	switch {
	case key.Matches(msg, repoKeys.Hot): // 'h' for hot
		ruleAction = "hot"
	case key.Matches(msg, repoKeys.Cold): // 'c' for cold
		ruleAction = "cold"
	case key.Matches(msg, repoKeys.Exclude): // 'x' for exclude
		ruleAction = "exclude"
	case key.Matches(msg, repoKeys.ViewInTree): // 'a' to toggle view
		// Toggle @view: directive for this project
		cmd = p.toggleViewCmd(selectedNode.Path)
		return m, cmd
	}

	if ruleAction != "" {
		// Rule is path to project + /** to include all files
		rule := filepath.Join(selectedNode.Path, "**")
		cmd = p.toggleRuleCmd(rule, ruleAction)
	}

	return m, cmd
}

// toggleRuleCmd creates a command to add/remove a rule and refresh the state.
func (p *repoPage) toggleRuleCmd(path, action string) tea.Cmd {
	return func() tea.Msg {
		mgr := context.NewManager("")
		// This is a simplified toggle. A full implementation would check current status.
		// For now, we remove any existing rule for the path and add the new one.
		_ = mgr.RemoveRule(path) // Ignore error if it doesn't exist
		if err := mgr.AppendRule(path, action); err != nil {
			return tea.Quit // Or an error message
		}
		// Return a message to tell the main view model to refresh all state.
		return refreshStateMsg{}
	}
}

// toggleViewCmd toggles the @view: directive for the selected project
func (p *repoPage) toggleViewCmd(path string) tea.Cmd {
	return func() tea.Msg {
		mgr := context.NewManager("")
		// Toggle @view: directive for this path
		if err := mgr.ToggleViewDirective(path); err != nil {
			return tea.Quit // Or an error message
		}
		// Return a message to tell the main view model to refresh all state.
		return refreshStateMsg{}
	}
}

// --- Page Interface ---

func (p *repoPage) Name() string { return "repo" }

func (p *repoPage) Keys() interface{} {
	return repoKeys
}

func (p *repoPage) Init() tea.Cmd {
	return p.nav.Init()
}

func (p *repoPage) Focus() tea.Cmd {
	// When the page gets focus, send a message to the navigator with the latest projects.
	return func() tea.Msg {
		// Convert []*WorkspaceNode to []WorkspaceNode for the navigator
		projectValues := make([]workspace.WorkspaceNode, len(p.sharedState.projects))
		for i, ptr := range p.sharedState.projects {
			if ptr != nil {
				projectValues[i] = *ptr
			}
		}
		return navigator.ProjectsLoadedMsg{Projects: projectValues}
	}
}

func (p *repoPage) Blur() {
	// You can add logic here if the page needs to do something when it loses focus.
}

func (p *repoPage) SetSize(width, height int) {
	p.width, p.height = width, height
	// Create a new model to pass to the navigator's update function to set size.
	// This is how bubbletea components often handle size changes.
	m, _ := p.nav.Update(tea.WindowSizeMsg{Width: width, Height: height})
	p.nav = m.(navigator.Model)
}

// --- Update ---

func (p *repoPage) Update(msg tea.Msg) (Page, tea.Cmd) {
	var cmd tea.Cmd
	// Delegate update to the navigator component
	m, cmd := p.nav.Update(msg)
	p.nav = m.(navigator.Model)
	return p, cmd
}

// --- View ---

func (p *repoPage) View() string {
	// Override navigator's default view with custom table-based hierarchical rendering
	return p.buildCustomView()
}

// buildCustomView creates a table-based hierarchical view for cx view repo
func (p *repoPage) buildCustomView() string {
	width := p.width
	height := p.height

	// Handle very small terminal sizes
	if width < 40 || height < 10 {
		return "Terminal too small. Please resize."
	}

	// Calculate viewport height (leave room for tabs at top and help footer at bottom)
	// Match the pattern from other pages: height - 5
	viewportHeight := height - 5
	if viewportHeight < 5 {
		viewportHeight = 5
	}

	// Calculate available table height (minus our own footer line)
	tableHeight := viewportHeight - 3
	if tableHeight < 3 {
		tableHeight = 3
	}

	// Build the table view
	mainContent := p.buildTableView(tableHeight)

	// Build footer with filter info
	var footerInfo string
	filterVal := p.nav.GetFilterInput()
	if filterVal != "" {
		footerInfo = fmt.Sprintf("Filter: %s", filterVal)
	} else {
		footerInfo = "/ filter • h/c/x hot/cold/exclude • a toggle view"
	}

	content := mainContent + "\n" + lipgloss.NewStyle().Faint(true).Render(footerInfo)

	// Wrap in a style with constrained height to match other pages
	return lipgloss.NewStyle().
		Width(width).
		Height(viewportHeight).
		Render(content)
}

// buildTableView constructs and renders the main table of workspaces.
func (p *repoPage) buildTableView(availableHeight int) string {
	// Get filtered projects from navigator - they're already hierarchically ordered
	// with pre-calculated TreePrefix from BuildWorkspaceTree
	filtered := p.nav.GetFiltered()
	if len(filtered) == 0 {
		return "No workspaces discovered.\n\nTip: Configure search_paths in ~/.grove/config.yml"
	}

	// Convert to pointers for buildTableRows
	filteredPtrs := make([]*workspace.WorkspaceNode, len(filtered))
	for i := range filtered {
		filteredPtrs[i] = &filtered[i]
	}

	// Build table rows (no need for additional hierarchical grouping)
	allRows := p.buildTableRows(filteredPtrs)

	// Guard against empty results
	if len(allRows) == 0 {
		return "No matching workspaces found."
	}

	cursor := p.nav.GetCursor()

	// Calculate visible rows based on scroll offset
	startIdx := p.scrollOffset
	endIdx := startIdx + availableHeight
	if endIdx > len(allRows) {
		endIdx = len(allRows)
	}
	if startIdx >= len(allRows) {
		startIdx = 0
		endIdx = len(allRows)
		if endIdx > availableHeight {
			endIdx = availableHeight
		}
	}

	// Ensure cursor is visible
	if cursor < startIdx {
		p.scrollOffset = cursor
		startIdx = cursor
		endIdx = startIdx + availableHeight
		if endIdx > len(allRows) {
			endIdx = len(allRows)
		}
	}
	if cursor >= endIdx {
		endIdx = cursor + 1
		startIdx = endIdx - availableHeight
		if startIdx < 0 {
			startIdx = 0
		}
		p.scrollOffset = startIdx
	}

	visibleRows := allRows[startIdx:endIdx]

	// Adjust cursor to be relative to the visible window
	relativeCursor := cursor - p.scrollOffset
	if relativeCursor < 0 {
		relativeCursor = 0
	}
	if relativeCursor >= len(visibleRows) {
		relativeCursor = len(visibleRows) - 1
	}

	// Use the selectable table component for rendering (3 columns: checkmark, WORKSPACE, and PATH)
	mainContent := table.SelectableTableWithOptions(
		[]string{"", "WORKSPACE", "PATH"},
		visibleRows,
		relativeCursor,
		table.SelectableTableOptions{HighlightColumn: 1},
	)

	// Add scroll indicator if there are more items
	if len(allRows) > availableHeight {
		mainContent += "\n" + lipgloss.NewStyle().Faint(true).Render(
			fmt.Sprintf("Showing %d-%d of %d workspaces", startIdx+1, endIdx, len(allRows)),
		)
	}

	return mainContent
}

// buildTableRows creates the data rows for the workspace table.
// Tree structure is pre-calculated in the TreePrefix field by BuildWorkspaceTree.
func (p *repoPage) buildTableRows(projects []*workspace.WorkspaceNode) [][]string {
	var rows [][]string

	for _, proj := range projects {
		// Apply styling based on workspace type
		var nameStyle lipgloss.Style
		if proj.IsWorktree() {
			// Worktrees: info style (blue/teal)
			nameStyle = theme.DefaultTheme.Info
		} else if proj.IsEcosystem() {
			// Ecosystems: bold header style
			nameStyle = theme.DefaultTheme.Header
		} else {
			// Primary workspaces: info style
			nameStyle = theme.DefaultTheme.Info
		}

		// Use pre-calculated TreePrefix for hierarchical display
		name := nameStyle.Render(proj.TreePrefix + proj.Name)

		path := shortenPath(proj.Path)

		// Check if this project is included via @view
		isChecked := " "
		for _, viewPath := range p.sharedState.viewPaths {
			// A view path might be a glob, so trim it for comparison with the project path.
			cleanViewPath := strings.TrimSuffix(viewPath, "/**")
			cleanViewPath = strings.TrimSuffix(cleanViewPath, "/*")

			if proj.Path == cleanViewPath {
				isChecked = "✓"
				break
			}
		}

		rows = append(rows, []string{
			isChecked, // New checkmark column
			name,
			lipgloss.NewStyle().Faint(true).Render(path),
		})
	}
	return rows
}

// shortenPath replaces the home directory prefix with a tilde (~).
func shortenPath(path string) string {
	home, err := os.UserHomeDir()
	if err != nil {
		return path // Fallback to original path on error
	}

	if strings.HasPrefix(path, home) {
		return filepath.Join("~", strings.TrimPrefix(path, home))
	}

	return path
}
