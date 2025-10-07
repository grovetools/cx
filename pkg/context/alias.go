package context

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"github.com/mattsolo1/grove-core/pkg/workspace"
	"github.com/sirupsen/logrus"
)

// aliasLineParts holds the parsed components of a rule line containing an alias.
type aliasLineParts struct {
	// The full original line.
	OriginalLine string
	// Any prefix before the alias directive (e.g., "!", "@view: ").
	Prefix string
	// The core alias string (e.g., "ecosystem:repo").
	Alias string
	// Any path or glob pattern appended to the alias (e.g., "/src/**/*.go").
	Pattern string
	// The full resolved path pattern (e.g., "!/path/to/project/src/**/*.go").
	ResolvedLine string
}

// AliasResolver discovers available workspaces and resolves aliases to their absolute paths.
type AliasResolver struct {
	projects     []*workspace.ProjectInfo
	discoverOnce sync.Once
	discoverErr  error
	configPath   string // Optional: custom config path for testing
	workDir      string // Current working directory for context-aware resolution
}

// NewAliasResolver creates a new, uninitialized alias resolver.
// Discovery happens lazily on first use.
func NewAliasResolver() *AliasResolver {
	return &AliasResolver{}
}

// NewAliasResolverWithWorkDir creates a new alias resolver with a working directory for context-aware resolution.
func NewAliasResolverWithWorkDir(workDir string) *AliasResolver {
	return &AliasResolver{workDir: workDir}
}

// NewAliasResolverWithConfig creates a new alias resolver with a custom config path for testing.
func NewAliasResolverWithConfig(configPath string) *AliasResolver {
	return &AliasResolver{configPath: configPath}
}

// discoverWorkspaces performs the workspace discovery process once.
func (r *AliasResolver) discoverWorkspaces() {
	r.discoverOnce.Do(func() {
		// We use a temporary logger as this is a background process.
		logger := logrus.New()
		logger.SetLevel(logrus.WarnLevel)

		discoveryService := workspace.NewDiscoveryService(logger)

		// If a custom config path is set (for testing), use it
		if r.configPath != "" {
			discoveryService = discoveryService.WithConfigPath(r.configPath)
		}

		result, err := discoveryService.DiscoverAll()
		if err != nil {
			r.discoverErr = fmt.Errorf("failed to discover workspaces: %w", err)
			return
		}

		r.projects = workspace.TransformToProjectInfo(result)
	})
}

// Resolve translates a pure alias string (e.g., "ecosystem:repo") into an absolute path.
func (r *AliasResolver) Resolve(alias string) (string, error) {
	r.discoverWorkspaces()
	if r.discoverErr != nil {
		return "", r.discoverErr
	}

	components := strings.Split(alias, ":")
	switch len(components) {
	case 1:
		return r.matchSingle(components[0])
	case 2:
		return r.matchDouble(components[0], components[1])
	case 3:
		return r.matchTriple(components[0], components[1], components[2])
	default:
		return "", fmt.Errorf("invalid alias format '%s', must have 1 to 3 components separated by ':'", alias)
	}
}

// matchSingle finds a project by its top-level name.
// Context-aware priority:
// 1. Sibling in same worktree directory (if current project has WorktreeName)
// 2. Projects NOT inside worktree directories (top-level)
// 3. Any other match
func (r *AliasResolver) matchSingle(name string) (string, error) {
	var currentProject *workspace.ProjectInfo
	var topLevelMatch *workspace.ProjectInfo
	var fallbackMatch *workspace.ProjectInfo

	// If we have a working directory, try to get current project context
	if r.workDir != "" {
		currentProject, _ = workspace.GetProjectByPath(r.workDir)
	}

	for _, p := range r.projects {
		if !p.IsWorktree && p.Name == name {
			// Priority 1: If current project is in a worktree, prefer siblings in same worktree
			if currentProject != nil && currentProject.WorktreeName != "" {
				if p.WorktreeName == currentProject.WorktreeName {
					return p.Path, nil
				}
			}

			// Priority 2: Top-level projects (not in any worktree)
			if p.WorktreeName == "" {
				if topLevelMatch == nil {
					topLevelMatch = p
				}
			} else {
				// Priority 3: Any match in a worktree
				if fallbackMatch == nil {
					fallbackMatch = p
				}
			}
		}
	}

	if topLevelMatch != nil {
		return topLevelMatch.Path, nil
	}

	if fallbackMatch != nil {
		return fallbackMatch.Path, nil
	}

	return "", fmt.Errorf("alias not found: '%s'", name)
}

// matchDouble finds a project by ecosystem:repo or repo:worktree.
func (r *AliasResolver) matchDouble(first, second string) (string, error) {
	// Try ecosystem:repo match
	// For a valid ecosystem:repo match, the project must be a direct child of the ecosystem,
	// NOT inside any worktree directory. Use WorktreeName == "" to verify this.
	for _, p := range r.projects {
		if p.ParentEcosystemPath != "" &&
		   filepath.Base(p.ParentEcosystemPath) == first &&
		   p.Name == second &&
		   !p.IsWorktree &&
		   p.WorktreeName == "" { // Ensures it's not inside a worktree directory
			return p.Path, nil
		}
	}

	// Try repo:worktree match
	for _, p := range r.projects {
		if p.IsWorktree && p.ParentPath != "" && filepath.Base(p.ParentPath) == first && p.Name == second {
			return p.Path, nil
		}
	}

	return "", fmt.Errorf("alias not found: '%s:%s'", first, second)
}

// matchTriple finds a project by ecosystem:repo:worktree.
func (r *AliasResolver) matchTriple(eco, repo, worktree string) (string, error) {
	for _, p := range r.projects {
		if p.IsWorktree && p.ParentEcosystemPath != "" && p.ParentPath != "" &&
			filepath.Base(p.ParentEcosystemPath) == eco &&
			filepath.Base(p.ParentPath) == repo &&
			p.Name == worktree {
			return p.Path, nil
		}
	}
	return "", fmt.Errorf("alias not found: '%s:%s:%s'", eco, repo, worktree)
}

// ResolveLine parses a full rule line, resolves the alias, and reconstructs the line with an absolute path.
func (r *AliasResolver) ResolveLine(line string) (string, error) {
	parts, err := r.parseAliasLine(line)
	if err != nil {
		return "", err
	}

	resolvedPath, err := r.Resolve(parts.Alias)
	if err != nil {
		// TODO: Add suggestions for similar aliases.
		return "", fmt.Errorf("on line '%s': %w", line, err)
	}

	// Reconstruct the line.
	finalPath := filepath.Join(resolvedPath, parts.Pattern)
	parts.ResolvedLine = strings.TrimSpace(parts.Prefix) + finalPath
	// Normalize short forms to full forms in output
	if strings.Contains(parts.Prefix, "@view:") || strings.Contains(parts.Prefix, "@v:") {
		parts.ResolvedLine = "@view: " + finalPath
	}

	return parts.ResolvedLine, nil
}

// parseAliasLine deconstructs a rule line into its prefix, alias, and pattern.
func (r *AliasResolver) parseAliasLine(line string) (*aliasLineParts, error) {
	// Regex to find @alias: (or @a:) and capture prefix, alias, and optional pattern.
	// It handles prefixes like '!', '@view:' (or '@v:'), and combinations.
	// Supports short forms: @a: for @alias:, @v: for @view:
	re := regexp.MustCompile(`^(?P<prefix>!?(?:\s*@(?:view|v):\s*)?)?\s*@(?:alias|a):(?P<alias>[^/\s]+)(?P<pattern>/.*)?$`)
	matches := re.FindStringSubmatch(line)
	if matches == nil {
		return nil, fmt.Errorf("invalid alias format in line: '%s'", line)
	}

	parts := &aliasLineParts{OriginalLine: line}
	for i, name := range re.SubexpNames() {
		if i > 0 && i <= len(matches) {
			switch name {
			case "prefix":
				parts.Prefix = matches[i]
			case "alias":
				parts.Alias = matches[i]
			case "pattern":
				parts.Pattern = matches[i]
			}
		}
	}
	return parts, nil
}
