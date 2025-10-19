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
	provider     *workspace.Provider
	providerOnce sync.Once
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

// initProvider performs the workspace discovery process once and initializes the provider.
func (r *AliasResolver) initProvider() {
	r.providerOnce.Do(func() {
		logger := logrus.New()
		logger.SetLevel(logrus.WarnLevel)

		discoveryService := workspace.NewDiscoveryService(logger)
		if r.configPath != "" {
			discoveryService = discoveryService.WithConfigPath(r.configPath)
		}

		discoveryResult, err := discoveryService.DiscoverAll()
		if err != nil {
			r.discoverErr = fmt.Errorf("failed to discover workspaces: %w", err)
			return
		}

		r.provider = workspace.NewProvider(discoveryResult)
	})
}

// Resolve translates a pure alias string (e.g., "ecosystem:repo") into an absolute path.
func (r *AliasResolver) Resolve(alias string) (string, error) {
	r.initProvider()
	if r.discoverErr != nil {
		return "", r.discoverErr
	}
	if r.provider == nil {
		return "", fmt.Errorf("workspace provider not initialized")
	}

	allNodes := r.provider.All()
	components := strings.Split(alias, ":")

	// Context-aware resolution for single-component aliases
	if len(components) == 1 {
		name := components[0]
		if r.workDir != "" {
			// Normalize the workDir to handle macOS /private/var symlink
			normalizedWorkDir := r.workDir
			// On macOS, /private/var is symlinked to /var, but EvalSymlinks doesn't always resolve it
			// Try stripping /private prefix if it exists
			if strings.HasPrefix(normalizedWorkDir, "/private/") {
				normalizedWorkDir = strings.TrimPrefix(normalizedWorkDir, "/private")
			}

			currentNode := r.provider.FindByPath(normalizedWorkDir)
			// If not found with /private stripped, try the original path
			if currentNode == nil && normalizedWorkDir != r.workDir {
				currentNode = r.provider.FindByPath(r.workDir)
			}

			if currentNode != nil {
				// Priority 1: If the current node is an ecosystem (e.g., an ecosystem worktree root),
				// prioritize finding a direct child project with the alias name.
				if currentNode.IsEcosystem() {
					for _, node := range allNodes {
						if node.Name == name && node.ParentEcosystemPath == currentNode.Path {
							return node.Path, nil // Found a direct child.
						}
					}
				}

				// Priority 2: If the current node is a project within an ecosystem, prioritize finding a sibling project.
				// This handles resolving aliases between projects in the same ecosystem or ecosystem worktree.
				if currentNode.ParentEcosystemPath != "" {
					for _, node := range allNodes {
						if node.Name == name && node.ParentEcosystemPath == currentNode.ParentEcosystemPath {
							return node.Path, nil // Found a sibling.
						}
					}
				}
			}
		}

		// Fallback for single-component alias: find a top-level project or best match
		var topLevelMatch *workspace.WorkspaceNode
		var shallowerMatch *workspace.WorkspaceNode
		var anyMatch *workspace.WorkspaceNode
		for _, node := range allNodes {
			if node.Name == name {
				depth := node.Depth // Use pre-calculated depth
				if depth == 0 { // Top-level nodes (standalone projects, ecosystems)
					if topLevelMatch == nil {
						topLevelMatch = node
					}
				}
				// Prefer shallower nodes (e.g., ecosystem sub-projects over worktree sub-projects)
				if shallowerMatch == nil || node.Depth < shallowerMatch.Depth {
					shallowerMatch = node
				}
				if anyMatch == nil {
					anyMatch = node
				}
			}
		}
		if topLevelMatch != nil {
			return topLevelMatch.Path, nil
		}
		if shallowerMatch != nil {
			return shallowerMatch.Path, nil
		}
		if anyMatch != nil {
			return anyMatch.Path, nil
		}
		return "", fmt.Errorf("alias not found: '%s'", alias)
	}

	// Resolution for multi-component aliases
	for _, node := range allNodes {
		switch len(components) {
		case 2: // ecosystem:repo OR repo:worktree OR eco-worktree:project
			comp1 := components[0]
			comp2 := components[1]
			// ecosystem:repo
			if node.Kind == workspace.KindEcosystemSubProject && filepath.Base(node.ParentEcosystemPath) == comp1 && node.Name == comp2 {
				return node.Path, nil
			}
			// repo:worktree
			if node.IsWorktree() && node.ParentProjectPath != "" && filepath.Base(node.ParentProjectPath) == comp1 && node.Name == comp2 {
				return node.Path, nil
			}
			// eco-worktree:project (e.g., general-refactoring:grove-core)
			if node.ParentEcosystemPath != "" && filepath.Base(node.ParentEcosystemPath) == comp1 && node.Name == comp2 {
				return node.Path, nil
			}

		case 3: // ecosystem:repo:worktree OR root-eco:eco-worktree:project
			comp1 := components[0]
			comp2 := components[1]
			comp3 := components[2]
			// ecosystem:repo:worktree
			if node.IsWorktree() && node.ParentProjectPath != "" && node.ParentEcosystemPath != "" &&
				filepath.Base(node.ParentEcosystemPath) == comp1 &&
				filepath.Base(node.ParentProjectPath) == comp2 &&
				node.Name == comp3 {
				return node.Path, nil
			}
			// root-eco:eco-worktree:project (e.g., grove-ecosystem:general-refactoring:grove-core)
			if node.ParentEcosystemPath != "" && node.Name == comp3 && filepath.Base(node.ParentEcosystemPath) == comp2 {
				// Check for root ecosystem name by traversing up from the parent ecosystem path
				// ParentEcosystemPath is like /path/to/root-eco/.grove-worktrees/eco-worktree
				ecoWorktreeParentDir := filepath.Dir(node.ParentEcosystemPath)
				if filepath.Base(ecoWorktreeParentDir) == ".grove-worktrees" {
					rootEcoPath := filepath.Dir(ecoWorktreeParentDir)
					if filepath.Base(rootEcoPath) == comp1 {
						return node.Path, nil
					}
				}
			}
		default:
			return "", fmt.Errorf("invalid alias format '%s', must have 1 to 3 components separated by ':'", alias)
		}
	}

	return "", fmt.Errorf("alias not found: '%s'", alias)
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
	var finalPath string
	if strings.HasPrefix(parts.Pattern, " @") {
		// Pattern is a directive like " @grep: \"query\""
		// If the resolved path is a bare directory (no glob), append /** before the directive
		if !strings.Contains(resolvedPath, "*") && !strings.Contains(resolvedPath, "?") {
			finalPath = resolvedPath + "/**" + parts.Pattern
		} else {
			finalPath = resolvedPath + parts.Pattern
		}
	} else if parts.Pattern == "" {
		// No pattern, just the alias - append /** to match all files
		finalPath = resolvedPath + "/**"
	} else {
		// Pattern is a file path like "/pkg/**"
		// Use filepath.Join
		finalPath = filepath.Join(resolvedPath, parts.Pattern)
	}

	parts.ResolvedLine = strings.TrimSpace(parts.Prefix) + finalPath
	// Normalize short forms to full forms in output
	if strings.Contains(parts.Prefix, "@view:") || strings.Contains(parts.Prefix, "@v:") {
		parts.ResolvedLine = "@view: " + finalPath
	}

	return parts.ResolvedLine, nil
}

// parseAliasLine deconstructs a rule line into its prefix, alias, and pattern.
func (r *AliasResolver) parseAliasLine(line string) (*aliasLineParts, error) {
	// Regex to find @alias: (or @a:) and capture prefix, alias, and optional pattern/directives.
	// It handles prefixes like '!', '@view:' (or '@v:'), and combinations.
	// Supports short forms: @a: for @alias:, @v: for @view:
	// Pattern can be:
	//   - /path/pattern (traditional)
	//   - @directive: "query" (search directives)
	//   - (nothing)
	re := regexp.MustCompile(`^(?P<prefix>!?(?:\s*@(?:view|v):\s*)?)?\s*@(?:alias|a):(?P<alias>[^/\s@]+)(?P<pattern>/.+|\s+@(?:find|grep):.+)?$`)
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
