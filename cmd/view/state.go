package view

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/mattsolo1/grove-core/pkg/workspace"
	"github.com/mattsolo1/grove-context/pkg/context"
)

// sharedState holds all the data that is shared across different pages of the TUI.
type sharedState struct {
	loading      bool
	err          error
	hotFiles     []string
	coldFiles    []string
	rulesContent string
	rulesPath    string // Path to the active rules file
	hotStats     *context.ContextStats
	coldStats    *context.ContextStats
	projects     []*workspace.WorkspaceNode
	// Parsed rules
	hotRules  []string
	coldRules []string
	viewPaths []string
}

// stateRefreshedMsg is sent when the sharedState has been updated.
type stateRefreshedMsg struct {
	state sharedState
}

// refreshStateMsg is a command to trigger a state refresh.
type refreshStateMsg struct{}

// refreshSharedStateCmd fetches all context data and returns it in a stateRefreshedMsg.
func refreshSharedStateCmd() tea.Cmd {
	return func() tea.Msg {
		mgr := context.NewManager("")
		newState := sharedState{loading: false}

		// Load rules content
		rulesBytes, rulesPath, err := mgr.LoadRulesContent()
		if err != nil {
			newState.err = err
			return stateRefreshedMsg{state: newState}
		}
		newState.rulesContent = string(rulesBytes)
		newState.rulesPath = rulesPath

		// Parse rules (pass the manager to resolve aliases)
		parseRules(&newState, string(rulesBytes), mgr)

		// Resolve hot and cold files
		hotFiles, err := mgr.ResolveFilesFromRules()
		if err != nil {
			newState.err = err
			return stateRefreshedMsg{state: newState}
		}
		newState.hotFiles = hotFiles

		coldFiles, err := mgr.ResolveColdContextFiles()
		if err != nil {
			newState.err = err
			return stateRefreshedMsg{state: newState}
		}
		newState.coldFiles = coldFiles

		// Get stats for both
		if len(hotFiles) > 0 {
			hotStats, err := mgr.GetStats("hot", hotFiles, 10)
			if err != nil {
				newState.err = err
				return stateRefreshedMsg{state: newState}
			}
			newState.hotStats = hotStats
		}
		if len(coldFiles) > 0 {
			coldStats, err := mgr.GetStats("cold", coldFiles, 10)
			if err != nil {
				newState.err = err
				return stateRefreshedMsg{state: newState}
			}
			newState.coldStats = coldStats
		}

		// Load projects
		projects, err := workspace.GetProjects(nil)
		if err != nil {
			newState.err = err
			// Continue, don't fail the whole TUI
		} else {
			newState.projects = projects
		}

		return stateRefreshedMsg{state: newState}
	}
}

// parseRules parses the rules content and populates hotRules, coldRules, and viewPaths.
func parseRules(state *sharedState, content string, mgr *context.Manager) {
	state.hotRules = []string{}
	state.coldRules = []string{}
	state.viewPaths = []string{}

	lines := strings.Split(content, "\n")
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
		if strings.HasPrefix(trimmed, "@view:") || strings.HasPrefix(trimmed, "@v:") {
			path := ""
			if strings.HasPrefix(trimmed, "@view:") {
				path = strings.TrimSpace(strings.TrimPrefix(trimmed, "@view:"))
			} else {
				path = strings.TrimSpace(strings.TrimPrefix(trimmed, "@v:"))
			}

			if path != "" {
				// Use the manager to resolve any potential alias in the line.
				resolvedLine, err := mgr.ResolveLineForRulePreview(trimmed)
				if err == nil {
					resolvedPath := resolvedLine
					if strings.HasPrefix(resolvedPath, "@view:") {
						resolvedPath = strings.TrimSpace(strings.TrimPrefix(resolvedPath, "@view:"))
					} else if strings.HasPrefix(resolvedPath, "@v:") {
						resolvedPath = strings.TrimSpace(strings.TrimPrefix(resolvedPath, "@v:"))
					}
					state.viewPaths = append(state.viewPaths, resolvedPath)
				} else {
					// Fallback to unresolved path if alias resolution fails
					state.viewPaths = append(state.viewPaths, path)
				}
			}
			continue
		}
		if strings.HasPrefix(trimmed, "@") { // Skip other directives
			continue
		}

		// Add rule to appropriate section
		if inColdSection {
			state.coldRules = append(state.coldRules, trimmed)
		} else {
			state.hotRules = append(state.hotRules, trimmed)
		}
	}
}
