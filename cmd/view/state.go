package view

import (
	"io"
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/grovetools/core/pkg/workspace"
	"github.com/grovetools/core/util/pathutil"
	"github.com/grovetools/cx/pkg/context"
	"github.com/sirupsen/logrus"
)

// sharedState holds all the data that is shared across different pages of the TUI.
type sharedState struct {
	loading         bool
	err             error
	hotFiles        []string
	coldFiles       []string
	rulesContent    string
	rulesPath       string // Path to the active rules file
	hotStats        *context.ContextStats
	coldStats       *context.ContextStats
	projects        []*workspace.WorkspaceNode
	projectProvider *workspace.Provider
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

		// Load projects using full discovery to get a provider
		logger := logrus.New()
		logger.SetOutput(io.Discard) // Discard logs to not mess with TUI
		discoveryService := workspace.NewDiscoveryService(logger)
		discoveryResult, err := discoveryService.DiscoverAll()
		if err != nil {
			newState.err = err
			// Continue, don't fail the whole TUI
		} else {
			provider := workspace.NewProvider(discoveryResult)
			newState.projectProvider = provider
			newState.projects = provider.All() // Get all nodes from the provider
		}

		return stateRefreshedMsg{state: newState}
	}
}

// displayPathInfo holds information about how to display a file path
type displayPathInfo struct {
	ecosystem string // Ecosystem context (e.g., "grove-ecosystem/")
	repo      string // Repository name (e.g., "grove-core/")
	path      string // File path relative to repo (e.g., "pkg/workspace/types.go")
}

// getDisplayPathInfo converts an absolute file path to a display-friendly format with ecosystem context.
// Returns ecosystem, repo, and path separately for colored rendering.
func (s *sharedState) getDisplayPathInfo(filePath string) displayPathInfo {
	if s.projectProvider == nil {
		return displayPathInfo{path: filePath}
	}

	// Normalize the file path to get a canonical representation
	// that handles symlinks and case-insensitivity, matching what the provider stores.
	canonicalPath, err := pathutil.NormalizeForLookup(filePath)
	if err != nil {
		// Fallback to absolute path if normalization fails
		var absErr error
		canonicalPath, absErr = filepath.Abs(filePath)
		if absErr != nil {
			// If even that fails, use the original path
			canonicalPath = filePath
		}
	}

	node := s.projectProvider.FindByPath(canonicalPath)
	if node != nil {
		projectName := node.Name
		projectPath := node.Path

		// If it's a project worktree, use its parent's name for context,
		// but calculate the relative path from the worktree's own root.
		if node.IsProjectWorktreeChild() {
			if parentNode := s.projectProvider.FindByPath(node.ParentProjectPath); parentNode != nil {
				projectName = parentNode.Name
				projectPath = node.Path
			}
		}

		// Normalize projectPath to match the canonicalPath normalization
		// This ensures filepath.Rel works correctly when paths have been lowercased
		normalizedProjectPath, err := pathutil.NormalizeForLookup(projectPath)
		if err != nil {
			// Fallback to original projectPath if normalization fails
			normalizedProjectPath = projectPath
		}

		relPath, err := filepath.Rel(normalizedProjectPath, canonicalPath)
		if err == nil {
			// Add ecosystem context if this is part of an ecosystem
			var ecosystemName string
			if node.RootEcosystemPath != "" {
				// Find the ecosystem node to get its name
				ecosystemNode := s.projectProvider.FindByPath(node.RootEcosystemPath)
				if ecosystemNode != nil {
					ecosystemName = ecosystemNode.Name + "/"
				}
			}

			return displayPathInfo{
				ecosystem: ecosystemName,
				repo:      projectName + "/",
				path:      relPath,
			}
		}
	}

	// As a fallback, try to make the path relative to the current working directory.
	cwd, err := os.Getwd()
	if err == nil {
		// Normalize cwd to match canonicalPath normalization
		normalizedCwd, cwdErr := pathutil.NormalizeForLookup(cwd)
		if cwdErr == nil {
			cwd = normalizedCwd
		}
		if relPath, err := filepath.Rel(cwd, canonicalPath); err == nil && !strings.HasPrefix(relPath, "..") {
			return displayPathInfo{path: relPath}
		}
	}

	return displayPathInfo{path: filePath}
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
