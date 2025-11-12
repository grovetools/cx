package context

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"github.com/mattsolo1/grove-core/config"
	"github.com/mattsolo1/grove-core/state"
	"github.com/mattsolo1/grove-core/util/pathutil"
)

// Constants for context file paths
const (
	GroveDir                   = ".grove"
	ContextFile                = ".grove/context"
	FilesListFile              = ".grove/context-files"
	RulesFile                  = ".grovectx"
	ActiveRulesFile            = ".grove/rules"
	CachedContextFilesListFile = ".grove/cached-context-files"
	CachedContextFile          = ".grove/cached-context"
	RulesDir                   = ".cx"
	RulesWorkDir               = ".cx.work"
	RulesExt                   = ".rules"
)

// ContextConfig defines configuration specific to grove-context.
// This is intended to be nested under the "context" key in grove.yml Extensions.
type ContextConfig struct {
	// IncludedWorkspaces is a strict allowlist: if set, only these workspaces are scanned for context.
	IncludedWorkspaces []string `yaml:"included_workspaces,omitempty"`
	// ExcludedWorkspaces is a denylist: these workspaces are excluded from context scanning.
	// Ignored if IncludedWorkspaces is set.
	ExcludedWorkspaces []string `yaml:"excluded_workspaces,omitempty"`
	// AllowedPaths is a list of additional paths that can be included in context,
	// regardless of workspace boundaries. Paths can be absolute or use ~/ for home directory.
	AllowedPaths []string `yaml:"allowed_paths,omitempty"`
}

// Manager handles context operations
type Manager struct {
	workDir         string
	gitIgnoredCache map[string]map[string]bool // Cache for gitignored files by repository root
	aliasResolver   *AliasResolver             // Lazily initialized alias resolver
	allowedRoots    []string
	allowedRootsErr error
	rootsOnce       sync.Once
	skippedRules    []SkippedRule // Rules that were skipped during parsing with reasons
	skippedMutex    sync.Mutex    // Protects skippedRules
}

// NewManager creates a new context manager
func NewManager(workDir string) *Manager {
	if workDir == "" {
		workDir, _ = os.Getwd()
	}
	// Always ensure workDir is an absolute path
	absWorkDir, err := filepath.Abs(workDir)
	if err == nil {
		workDir = absWorkDir
	}
	return &Manager{
		workDir:         workDir,
		gitIgnoredCache: make(map[string]map[string]bool),
		aliasResolver:   nil, // Lazily initialized
	}
}

// GetWorkDir returns the current working directory
func (m *Manager) GetWorkDir() string {
	return m.workDir
}

// getAliasResolver lazily initializes and returns the AliasResolver.
func (m *Manager) getAliasResolver() *AliasResolver {
	if m.aliasResolver == nil {
		m.aliasResolver = NewAliasResolverWithWorkDir(m.workDir)
	}
	m.aliasResolver.InitProvider() // This is idempotent
	return m.aliasResolver
}

// ResolveLineForRulePreview resolves a single rule line, handling aliases.
// It is intended for use by tools like the Neovim plugin's rule previewer.
func (m *Manager) ResolveLineForRulePreview(line string) (string, error) {
	resolver := m.getAliasResolver()
	trimmedLine := strings.TrimSpace(line)

	// 1. Check for ruleset import syntax (::)
	if strings.Contains(trimmedLine, "::") && (strings.Contains(trimmedLine, "@a:") || strings.Contains(trimmedLine, "@alias:")) {
		patterns, err := m.resolvePatternsFromRulesetImport(trimmedLine)
		if err != nil {
			return "", err
		}
		// Return patterns as a multi-line string for the command to split
		return strings.Join(patterns, "\n"), nil
	}

	// 2. Handle simple aliases or plain patterns
	if resolver != nil && (strings.Contains(trimmedLine, "@alias:") || strings.Contains(trimmedLine, "@a:")) {
		return resolver.ResolveLine(trimmedLine)
	}

	// 3. If not an alias or import, return the original line.
	return line, nil
}

// resolvePatternsFromRulesetImport resolves a ruleset import string (e.g., "proj::rules") into a slice of patterns.
func (m *Manager) resolvePatternsFromRulesetImport(importRule string) ([]string, error) {
	// 1. Parse the import string
	// The line might have a prefix like "!", so trim it first for parsing.
	isExclude := strings.HasPrefix(importRule, "!")
	if isExclude {
		importRule = strings.TrimPrefix(importRule, "!")
	}

	prefix := "@a:"
	if strings.HasPrefix(importRule, "@alias:") {
		prefix = "@alias:"
	}
	trimmedRule := strings.TrimPrefix(importRule, prefix)

	parts := strings.SplitN(trimmedRule, "::", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid ruleset import format: %s", importRule)
	}
	projectAlias, rulesetName := parts[0], parts[1]

	// 2. Resolve the project alias to its absolute path
	projectPath, err := m.getAliasResolver().Resolve(projectAlias)
	if err != nil {
		return nil, fmt.Errorf("could not resolve project alias '%s': %w", projectAlias, err)
	}

	// 3. Find the ruleset file - check both .cx/ and .cx.work/
	rulesFilePath, err := FindRulesetFile(projectPath, rulesetName)
	if err != nil {
		return nil, fmt.Errorf("could not find ruleset '%s' in project '%s': %w", rulesetName, projectAlias, err)
	}

	// 4. Expand all rules from that file
	hotRules, coldRules, _, err := m.expandAllRules(rulesFilePath, make(map[string]bool), 0)
	if err != nil {
		return nil, fmt.Errorf("could not expand ruleset '%s' from project '%s': %w", rulesetName, projectAlias, err)
	}

	// 5. Collect all patterns and prefix them with the project path
	var resolvedPatterns []string
	allRules := append(hotRules, coldRules...)
	for _, rule := range allRules {
		pattern := rule.Pattern
		if !filepath.IsAbs(pattern) {
			pattern = filepath.Join(projectPath, pattern)
		}
		if rule.IsExclude || isExclude { // The rule itself or the import line can be an exclusion
			resolvedPatterns = append(resolvedPatterns, "!"+pattern)
		} else {
			resolvedPatterns = append(resolvedPatterns, pattern)
		}
	}

	return resolvedPatterns, nil
}

// ResolveFilesFromPatterns exposes the internal file resolution logic for external use.
// It resolves files from a given set of patterns.
func (m *Manager) ResolveFilesFromPatterns(patterns []string) ([]string, error) {
	return m.resolveFilesFromPatterns(patterns)
}

// ReadFilesList reads the list of files from a file
func (m *Manager) ReadFilesList(filename string) ([]string, error) {
	// Ensure we use the full path relative to workDir
	fullPath := filename
	if !filepath.IsAbs(filename) {
		fullPath = filepath.Join(m.workDir, filename)
	}

	file, err := os.Open(fullPath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var files []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		// Skip empty lines and comments
		if line != "" && !strings.HasPrefix(line, "#") {
			files = append(files, line)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return files, nil
}

// WriteFilesList writes a list of files to a file
func (m *Manager) WriteFilesList(filename string, files []string) error {
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	for _, f := range files {
		fmt.Fprintln(file, f)
	}

	return nil
}

// getGitIgnoredFiles returns a set of all gitignored files for the repository
// containing the given directory. It returns a map of absolute paths for efficient lookup.
func (m *Manager) getGitIgnoredFiles(forDir string) (map[string]bool, error) {
	// Ensure the provided path is absolute
	absForDir, err := filepath.Abs(forDir)
	if err != nil {
		return make(map[string]bool), err
	}

	// Find the root of the git repository for the given directory.
	gitRootCmd := exec.Command("git", "-C", absForDir, "rev-parse", "--show-toplevel")
	gitRootOutput, err := gitRootCmd.Output()
	if err != nil {
		// This directory is not in a git repository, so no files are gitignored.
		return make(map[string]bool), nil
	}
	gitRootPath := strings.TrimSpace(string(gitRootOutput))

	// Normalize the cache key for case-insensitive filesystems
	cacheKey, err := pathutil.NormalizeForLookup(gitRootPath)
	if err != nil {
		// If normalization fails, use original path
		cacheKey = gitRootPath
	}

	// Check if we have a cached result for this repository
	if cachedResult, found := m.gitIgnoredCache[cacheKey]; found {
		return cachedResult, nil
	}

	// If not cached, proceed with the original logic
	ignoredFiles := make(map[string]bool)

	// Get all tracked files to correctly handle cases where an ignored file is explicitly tracked.
	trackedCmd := exec.Command("git", "ls-files")
	trackedCmd.Dir = gitRootPath
	trackedOutput, _ := trackedCmd.Output()
	trackedFiles := make(map[string]bool)
	for _, line := range strings.Split(string(trackedOutput), "\n") {
		if line = strings.TrimSpace(line); line != "" {
			// Store relative to the git root for lookup.
			trackedFiles[line] = true
		}
	}

	// Use `git ls-files` to get a list of all individual files that are ignored.
	// We avoid the `--directory` flag to get a complete file list, which simplifies our logic.
	cmd := exec.Command("git", "ls-files", "--others", "--ignored", "--exclude-standard")
	cmd.Dir = gitRootPath

	output, err := cmd.Output()
	if err != nil {
		// If git command fails, return an empty map.
		return ignoredFiles, nil
	}

	// Process each ignored file path.
	scanner := bufio.NewScanner(strings.NewReader(string(output)))
	for scanner.Scan() {
		relativePath := scanner.Text()
		if relativePath != "" {
			// An ignored file is only truly ignored if it's not tracked.
			if !trackedFiles[relativePath] {
				// Store the full absolute path for consistent and easy lookup later.
				absolutePath := filepath.Join(gitRootPath, relativePath)

				// Normalize path for case-insensitive filesystems
				normalizedPath, err := pathutil.NormalizeForLookup(absolutePath)
				if err == nil {
					ignoredFiles[normalizedPath] = true
				}
			}
		}
	}

	// Cache the result before returning (use normalized key)
	m.gitIgnoredCache[cacheKey] = ignoredFiles

	return ignoredFiles, nil
}

// UpdateFromRules updates the files list based on rules file patterns (deprecated - kept for compatibility)
func (m *Manager) UpdateFromRules() error {
	// Get the resolved file list
	filesToInclude, err := m.ResolveFilesFromRules()
	if err != nil {
		// Handle the special case where neither file exists
		if strings.Contains(err.Error(), "no rules file found") {
			// Prompt user to create .grovectx for backward compatibility
			fmt.Printf(".grovectx not found. Would you like to create one with '*' (include all files)? [Y/n]: ")
			var response string
			fmt.Scanln(&response)

			if response == "" || strings.ToLower(response) == "y" || strings.ToLower(response) == "yes" {
				// Create .grovectx with "*"
				rulesPath := filepath.Join(m.workDir, RulesFile)
				if err := m.WriteFilesList(rulesPath, []string{"*"}); err != nil {
					return fmt.Errorf("error creating %s: %w", RulesFile, err)
				}
				fmt.Printf("Created %s with '*' pattern\n", RulesFile)

				// Try again
				filesToInclude, err = m.ResolveFilesFromRules()
				if err != nil {
					return err
				}
			} else {
				return fmt.Errorf("%s not found. Create it with patterns to include", RulesFile)
			}
		} else {
			return err
		}
	}

	// Ensure .grove directory exists relative to workDir
	groveDir := filepath.Join(m.workDir, GroveDir)
	if err := os.MkdirAll(groveDir, 0755); err != nil {
		return fmt.Errorf("error creating %s directory: %w", groveDir, err)
	}

	// Write the filtered file list to context-files
	filesPath := filepath.Join(m.workDir, FilesListFile)
	return m.WriteFilesList(filesPath, filesToInclude)
}

// ParseGitRule checks if a rule is a Git URL and extracts the URL and optional version
func (m *Manager) ParseGitRule(rule string) (isGitURL bool, repoURL, version string) {
	// Remove exclusion prefix if present
	if strings.HasPrefix(rule, "!") {
		rule = strings.TrimPrefix(rule, "!")
	}

	// Check for common Git URL patterns
	gitURLPattern := regexp.MustCompile(`^(https?://|git@|github\.com/|gitlab\.com/|bitbucket\.org/)`)
	if !gitURLPattern.MatchString(rule) {
		return false, "", ""
	}

	// Ensure proper URL format first
	if !strings.HasPrefix(rule, "http://") && !strings.HasPrefix(rule, "https://") {
		if strings.HasPrefix(rule, "github.com/") {
			rule = "https://" + rule
		} else if strings.HasPrefix(rule, "gitlab.com/") {
			rule = "https://" + rule
		} else if strings.HasPrefix(rule, "bitbucket.org/") {
			rule = "https://" + rule
		} else if strings.HasPrefix(rule, "git@") {
			// Convert SSH URL to HTTPS
			rule = strings.Replace(rule, "git@github.com:", "https://github.com/", 1)
			rule = strings.Replace(rule, "git@gitlab.com:", "https://gitlab.com/", 1)
			rule = strings.Replace(rule, "git@bitbucket.org:", "https://bitbucket.org/", 1)
		}
	}

	// Now parse the URL to extract repo and version
	// Format: https://github.com/owner/repo[@version][/path/pattern]
	// We need to find where the repo ends and the version/path begins

	// The pattern is: protocol://domain/owner/repo[@version]
	// Anything after the repo (with optional version) is a path pattern that should be ignored for repo resolution

	// First, extract protocol://domain/ part
	protoEnd := strings.Index(rule, "://")
	if protoEnd == -1 {
		return false, "", ""
	}
	protoEnd += 3 // Move past ://

	// Find the next / after domain
	domainEnd := strings.Index(rule[protoEnd:], "/")
	if domainEnd == -1 {
		// No path at all, just domain
		return true, rule, ""
	}
	domainEnd += protoEnd

	// Now we're at: protocol://domain/owner/repo[@version][/path]
	// Split the remaining path by /
	pathPart := rule[domainEnd+1:] // Skip the / after domain
	pathSegments := strings.Split(pathPart, "/")

	if len(pathSegments) < 2 {
		// Need at least owner/repo
		return true, rule, ""
	}

	// owner is pathSegments[0]
	// repo[@version] is pathSegments[1]
	owner := pathSegments[0]
	repoWithVersion := pathSegments[1]

	// Check if repo part contains @ for version
	if atIndex := strings.Index(repoWithVersion, "@"); atIndex != -1 {
		// Has version
		repoName := repoWithVersion[:atIndex]
		version = repoWithVersion[atIndex+1:]
		repoURL = rule[:domainEnd+1] + owner + "/" + repoName
	} else {
		// No version
		repoURL = rule[:domainEnd+1] + owner + "/" + repoWithVersion
		version = ""
	}

	return true, repoURL, version
}

// ShowContext outputs the context file content
func (m *Manager) ShowContext() error {
	content, err := os.ReadFile(ContextFile)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("%s file not found. Run 'grove cx generate' to create it", ContextFile)
		}
		return fmt.Errorf("error reading %s: %w", ContextFile, err)
	}

	fmt.Print(string(content))
	return nil
}

// ListFiles returns the list of files in the context
func (m *Manager) ListFiles() ([]string, error) {
	// Dynamically resolve files from rules
	files, err := m.ResolveFilesFromRules()
	if err != nil {
		return nil, fmt.Errorf("error resolving files from rules: %w", err)
	}

	// Convert to absolute paths and deduplicate based on canonical paths
	// Use a map to track seen paths (normalized for case-insensitive filesystems)
	seenPaths := make(map[string]string) // normalized -> original
	var absPaths []string

	for _, file := range files {
		absPath, err := filepath.Abs(file)
		if err != nil {
			absPaths = append(absPaths, file+" (error getting absolute path)")
			continue
		}

		// On case-insensitive filesystems, normalize to lowercase for comparison
		normalizedKey := strings.ToLower(absPath)

		// Only add if we haven't seen this normalized path before
		if _, seen := seenPaths[normalizedKey]; !seen {
			seenPaths[normalizedKey] = absPath
			absPaths = append(absPaths, absPath)
		}
	}

	return absPaths, nil
}

// Utility functions for formatting

// FormatTokenCount formats a token count for display
func FormatTokenCount(tokens int) string {
	if tokens < 1000 {
		return fmt.Sprintf("%d", tokens)
	} else if tokens < 1000000 {
		return fmt.Sprintf("%.1fk", float64(tokens)/1000)
	} else {
		return fmt.Sprintf("%.1fM", float64(tokens)/1000000)
	}
}

// FormatBytes formats byte count for display
func FormatBytes(bytes int) string {
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
	)

	if bytes < KB {
		return fmt.Sprintf("%d bytes", bytes)
	} else if bytes < MB {
		return fmt.Sprintf("%.1f KB", float64(bytes)/KB)
	} else if bytes < GB {
		return fmt.Sprintf("%.1f MB", float64(bytes)/MB)
	} else {
		return fmt.Sprintf("%.1f GB", float64(bytes)/GB)
	}
}

// initAllowedRoots discovers all workspaces and filters them based on
// included_workspaces and excluded_workspaces configurations. The result is
// a cached list of allowed root directories for context scanning.
func (m *Manager) initAllowedRoots() {
	m.rootsOnce.Do(func() {
		resolver := m.getAliasResolver()
		if resolver.DiscoverErr != nil {
			m.allowedRootsErr = resolver.DiscoverErr
			return
		}
		if resolver.Provider == nil {
			m.allowedRootsErr = fmt.Errorf("workspace provider could not be initialized")
			return
		}

		allProjects := resolver.Provider.All()

		mergedCfg, err := config.LoadFrom(m.workDir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: could not load grove configuration to apply workspace filters: %v\n", err)
		}

		// Read context-specific configuration from the extension
		var ctxCfg ContextConfig
		if mergedCfg != nil {
			if err := mergedCfg.UnmarshalExtension("context", &ctxCfg); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to read context configuration: %v\n", err)
			}
		}

		var allowed []string

		if len(ctxCfg.IncludedWorkspaces) > 0 {
			// --- ALLOWLIST MODE ---
			includedNames := make(map[string]bool)
			for _, name := range ctxCfg.IncludedWorkspaces {
				includedNames[name] = true
			}
			for _, node := range allProjects {
				if includedNames[node.Name] {
					// Canonicalize workspace paths
					canonicalPath, err := pathutil.NormalizeForLookup(node.Path)
					if err != nil {
						canonicalPath = node.Path
					}
					allowed = append(allowed, canonicalPath)
				}
			}
		} else {
			// --- DENYLIST MODE (Default) ---
			excludedNames := make(map[string]bool)
			for _, name := range ctxCfg.ExcludedWorkspaces {
				excludedNames[name] = true
			}
			for _, node := range allProjects {
				if !excludedNames[node.Name] {
					// Canonicalize workspace paths
					canonicalPath, err := pathutil.NormalizeForLookup(node.Path)
					if err != nil {
						canonicalPath = node.Path
					}
					allowed = append(allowed, canonicalPath)
				}
			}
		}

		// Add ~/.grove as an explicit exception
		homeDir, err := os.UserHomeDir()
		if err == nil {
			groveHome := filepath.Join(homeDir, ".grove")
			// Canonicalize grove home path
			canonicalGroveHome, err := pathutil.NormalizeForLookup(groveHome)
			if err != nil {
				canonicalGroveHome = groveHome
			}
			allowed = append(allowed, canonicalGroveHome)
		}

		// Also add notebook root directories to allowed paths
		if mergedCfg != nil && mergedCfg.Notebooks != nil && mergedCfg.Notebooks.Definitions != nil {
			for notebookName, notebook := range mergedCfg.Notebooks.Definitions {
				if notebook.RootDir != "" {
					notebookRootDir, err := pathutil.Expand(notebook.RootDir)
					if err != nil {
						fmt.Fprintf(os.Stderr, "Warning: could not expand notebook '%s' root_dir '%s': %v\n", notebookName, notebook.RootDir, err)
					} else {
						// Canonicalize notebook root path
						canonicalNotebookRoot, err := pathutil.NormalizeForLookup(notebookRootDir)
						if err != nil {
							canonicalNotebookRoot = notebookRootDir
						}
						// Add the notebook root to the list of allowed paths if not already present.
						isAlreadyAllowed := false
						for _, root := range allowed {
							if root == canonicalNotebookRoot {
								isAlreadyAllowed = true
								break
							}
						}
						if !isAlreadyAllowed {
							allowed = append(allowed, canonicalNotebookRoot)
						}
					}
				}
			}
		}

		// Add paths from context.allowed_paths configuration
		for _, allowedPath := range ctxCfg.AllowedPaths {
			// Expand ~ and environment variables
			expandedPath, err := pathutil.Expand(allowedPath)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Warning: could not expand allowed_path '%s': %v\n", allowedPath, err)
				continue
			}

			// Convert to absolute path
			absPath, err := filepath.Abs(expandedPath)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Warning: could not resolve absolute path for '%s': %v\n", expandedPath, err)
				continue
			}

			// Resolve symlinks and normalize for canonical path
			canonicalPath, err := pathutil.NormalizeForLookup(absPath)
			if err != nil {
				// Fallback to absolute path if normalization fails
				fmt.Fprintf(os.Stderr, "Warning: could not normalize allowed_path '%s': %v\n", absPath, err)
				canonicalPath = absPath
			}

			// Check if already present
			isAlreadyAllowed := false
			for _, root := range allowed {
				if root == canonicalPath {
					isAlreadyAllowed = true
					break
				}
			}
			if !isAlreadyAllowed {
				allowed = append(allowed, canonicalPath)
			}
		}

		m.allowedRoots = allowed
	})
}

// GetAllowedRoots returns the list of sandboxed root directories for scanning.
func (m *Manager) GetAllowedRoots() ([]string, error) {
	m.initAllowedRoots()
	return m.allowedRoots, m.allowedRootsErr
}

// findGitRoot finds the root directory of the git repository by walking up from the manager's working directory.
func (m *Manager) findGitRoot() string {
	// Try using git rev-parse first for efficiency.
	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
	cmd.Dir = m.workDir
	output, err := cmd.Output()
	if err == nil {
		return strings.TrimSpace(string(output))
	}

	// Fallback: walk up the directory tree looking for .git
	dir := m.workDir
	for {
		if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
			return dir
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			// Reached the root
			break
		}
		dir = parent
	}

	return "" // Not in a git repository
}

// FindRulesetFile searches for a ruleset file by name in both .cx/ and .cx.work/ directories.
// It returns the absolute path to the file if found, or an error if not found.
// This function checks .cx/ first, then .cx.work/ as a fallback.
func FindRulesetFile(projectPath, rulesetName string) (string, error) {
	// Check .cx/ directory first
	rulesFilePath := filepath.Join(projectPath, RulesDir, rulesetName+RulesExt)
	if _, err := os.Stat(rulesFilePath); err == nil {
		return rulesFilePath, nil
	}

	// Try .cx.work/ as fallback
	rulesFilePath = filepath.Join(projectPath, RulesWorkDir, rulesetName+RulesExt)
	if _, err := os.Stat(rulesFilePath); err == nil {
		return rulesFilePath, nil
	}

	return "", fmt.Errorf("ruleset '%s' not found in %s/ or %s/", rulesetName, RulesDir, RulesWorkDir)
}

// GetSkippedRules returns the list of rules that were skipped during the last parsing operation
func (m *Manager) GetSkippedRules() []SkippedRule {
	m.skippedMutex.Lock()
	defer m.skippedMutex.Unlock()
	// Return a copy to avoid concurrent modification
	result := make([]SkippedRule, len(m.skippedRules))
	copy(result, m.skippedRules)
	return result
}

// ClearSkippedRules clears the list of skipped rules
func (m *Manager) ClearSkippedRules() {
	m.skippedMutex.Lock()
	defer m.skippedMutex.Unlock()
	m.skippedRules = nil
}

// AddSkippedRule adds a skipped rule to the list
func (m *Manager) addSkippedRule(lineNum int, rule string, reason string) {
	m.skippedMutex.Lock()
	defer m.skippedMutex.Unlock()
	m.skippedRules = append(m.skippedRules, SkippedRule{
		LineNum: lineNum,
		Rule:    rule,
		Reason:  reason,
	})
}

// EnsureAndGetRulesPath finds the active rules file, creates it with boilerplate if it doesn't exist,
// and returns its absolute path. This is useful for integrations that need to open the file directly.
func (m *Manager) EnsureAndGetRulesPath() (string, error) {
	rulesContent, rulesPath, err := m.LoadRulesContent()
	if err != nil {
		return "", err
	}

	// If LoadRulesContent didn't return a path, determine where to create the file
	if rulesPath == "" {
		// Check if there's an active rule set in state
		activeSource, _ := state.GetString(StateSourceKey)
		if activeSource != "" {
			// Use the active source path from state
			rulesPath = filepath.Join(m.workDir, activeSource)
		} else {
			// Default to .grove/rules
			rulesPath = filepath.Join(m.workDir, ActiveRulesFile)
		}
	}

	// If the rules file doesn't exist, create it
	if _, err := os.Stat(rulesPath); os.IsNotExist(err) {
		// Ensure parent directory exists
		groveDir := filepath.Dir(rulesPath)
		if err := os.MkdirAll(groveDir, 0755); err != nil {
			return "", fmt.Errorf("error creating %s directory: %w", groveDir, err)
		}

		// Use default content if available, otherwise use boilerplate
		if rulesContent == nil {
			rulesContent = []byte("# Context rules file\n# Add patterns to include files, one per line\n# Use ! prefix to exclude\n# Examples:\n#   *.go\n#   !*_test.go\n#   src/**/*.js\n\n*\n")
		}

		if err := os.WriteFile(rulesPath, rulesContent, 0644); err != nil {
			return "", fmt.Errorf("error creating %s: %w", rulesPath, err)
		}
	}

	// Get absolute path to rules file
	absRulesPath, err := filepath.Abs(rulesPath)
	if err != nil {
		return "", fmt.Errorf("error getting absolute path: %w", err)
	}

	return absRulesPath, nil
}

// EditRulesCmd prepares an *exec.Cmd to open the active rules file in an editor.
// It handles finding/creating the rules file, determining the editor, and setting the
// working directory to the git root for a consistent editing experience.
func (m *Manager) EditRulesCmd() (*exec.Cmd, error) {
	absRulesPath, err := m.EnsureAndGetRulesPath()
	if err != nil {
		return nil, err
	}

	// Get editor from environment
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "vim" // A reasonable default
	}

	// Prepare the command
	editorCmd := exec.Command(editor, absRulesPath)
	editorCmd.Stdin = os.Stdin
	editorCmd.Stdout = os.Stdout
	editorCmd.Stderr = os.Stderr

	// Set the command's working directory to the git root for consistency
	if gitRoot := m.findGitRoot(); gitRoot != "" {
		editorCmd.Dir = gitRoot
	}

	return editorCmd, nil
}

// IsPathAllowed checks if a given path is within one of the allowed workspace roots.
// It returns true if allowed, or false and a reason string if not.
func (m *Manager) IsPathAllowed(path string) (bool, string) {
	m.initAllowedRoots()
	if m.allowedRootsErr != nil {
		return false, fmt.Sprintf("cannot validate path, workspace discovery failed: %v", m.allowedRootsErr)
	}

	absPath, err := filepath.Abs(path)
	if err != nil {
		return false, fmt.Sprintf("could not resolve absolute path for '%s': %v", path, err)
	}

	// Normalize path for case-insensitive filesystems and resolve symlinks
	canonicalPath, err := pathutil.NormalizeForLookup(absPath)
	if err != nil {
		// If normalization fails, fall back to the absolute path for the check.
		// This might happen for paths that don't exist yet but are part of a pattern.
		canonicalPath = absPath
	}

	// FIRST, check if this path or any containing directory is an excluded workspace
	// (higher priority than parent allowance)
	resolver := m.getAliasResolver()
	if resolver.Provider != nil {
		// Load config to check exclusions
		mergedCfg, _ := config.LoadFrom(m.workDir)
		var ctxCfg ContextConfig
		if mergedCfg != nil {
			mergedCfg.UnmarshalExtension("context", &ctxCfg)
		}

		// Check if the given path is within or equals any excluded workspace
		if len(ctxCfg.ExcludedWorkspaces) > 0 {
			excludedNames := make(map[string]bool)
			for _, name := range ctxCfg.ExcludedWorkspaces {
				excludedNames[name] = true
			}

			allNodes := resolver.Provider.All()
			for _, node := range allNodes {
				// Check if this is an excluded workspace
				if !excludedNames[node.Name] {
					continue
				}

				// Canonicalize the workspace path for comparison
				normalizedNodePath, err := pathutil.NormalizeForLookup(node.Path)
				if err != nil {
					continue // Skip if normalization fails
				}
				normalizedCanonicalPath, err := pathutil.NormalizeForLookup(canonicalPath)
				if err != nil {
					continue // Skip if normalization fails
				}

				// Check if the canonicalPath is equal to or within this excluded workspace
				if normalizedCanonicalPath == normalizedNodePath ||
					strings.HasPrefix(normalizedCanonicalPath, normalizedNodePath+string(filepath.Separator)) {
					return false, fmt.Sprintf("workspace '%s' containing path '%s' is in your 'excluded_workspaces' list", node.Name, path)
				}
			}
		}
	}

	// THEN check if it's under an allowed root
	for _, root := range m.allowedRoots {
		// Check if canonicalPath is equal to or a subdirectory of root.
		if canonicalPath == root || strings.HasPrefix(canonicalPath, root+string(filepath.Separator)) {
			return true, ""
		}
	}

	// If not found in allowed roots, return error
	return false, fmt.Sprintf("path '%s' is outside of any known or allowed workspace", path)
}

// ClassifyAllProjectFiles is the unified, deterministic classification engine that
// resolves and classifies all files based on context rules. It returns a map of file
// paths to their NodeStatus. This method ensures consistency across all views (tree, stats, list).
func (m *Manager) ClassifyAllProjectFiles(showGitIgnored bool) (map[string]NodeStatus, error) {
	result := make(map[string]NodeStatus)

	// Step 1: Get the definitive sets of hot and cold files using the existing stable methods
	hotFiles, err := m.ResolveFilesFromRules()
	if err != nil {
		return nil, fmt.Errorf("failed to resolve hot files: %w", err)
	}

	coldFiles, err := m.ResolveColdContextFiles()
	if err != nil {
		return nil, fmt.Errorf("failed to resolve cold files: %w", err)
	}

	// Step 2: Load rules content for attribution
	rulesContent, _, err := m.LoadRulesContent()
	if err != nil {
		return nil, fmt.Errorf("failed to load rules content: %w", err)
	}

	// Get attribution to identify explicitly excluded files
	_, _, exclusionResult, _, err := m.ResolveFilesWithAttribution(string(rulesContent))
	if err != nil {
		return nil, fmt.Errorf("failed to resolve attribution: %w", err)
	}

	// Step 3: Create maps for efficient lookup and convert relative paths to absolute
	// Normalize paths for case-insensitive filesystems and symlink resolution
	hotFilesMap := make(map[string]bool)
	for _, f := range hotFiles {
		absPath := f
		if !filepath.IsAbs(f) {
			absPath = filepath.Join(m.workDir, f)
		}
		// Normalize the path to handle case-insensitive filesystems
		if normalizedPath, err := pathutil.NormalizeForLookup(absPath); err == nil {
			absPath = normalizedPath
		}
		hotFilesMap[absPath] = true
	}

	coldFilesMap := make(map[string]bool)
	for _, f := range coldFiles {
		absPath := f
		if !filepath.IsAbs(f) {
			absPath = filepath.Join(m.workDir, f)
		}
		// Normalize the path to handle case-insensitive filesystems
		if normalizedPath, err := pathutil.NormalizeForLookup(absPath); err == nil {
			absPath = normalizedPath
		}
		coldFilesMap[absPath] = true
	}

	// Extract excluded files from exclusionResult (which is a map[int][]string)
	excludedFilesMap := make(map[string]bool)
	for _, files := range exclusionResult {
		for _, f := range files {
			absPath := f
			if !filepath.IsAbs(f) {
				absPath = filepath.Join(m.workDir, f)
			}
			// Normalize the path to handle case-insensitive filesystems
			if normalizedPath, err := pathutil.NormalizeForLookup(absPath); err == nil {
				absPath = normalizedPath
			}
			excludedFilesMap[absPath] = true
		}
	}

	// Step 4: Classify the resolved files (now all with absolute paths)
	// Hot files (but not if they're also in cold - cold takes precedence)
	for file := range hotFilesMap {
		if !coldFilesMap[file] {
			result[file] = StatusIncludedHot
		}
	}

	// Cold files
	for file := range coldFilesMap {
		result[file] = StatusIncludedCold
	}

	// Excluded files
	for file := range excludedFilesMap {
		result[file] = StatusExcludedByRule
	}

	// Step 5: Extract root paths from the rules to know what directories to walk
	// We need to walk these to find all files for the tree view
	activeRulesFile := m.findActiveRulesFile()
	if activeRulesFile == "" {
		// If no rules file, check for defaults
		_, defaultRulesFile := m.LoadDefaultRulesContent()
		if _, err := os.Stat(defaultRulesFile); !os.IsNotExist(err) {
			activeRulesFile = defaultRulesFile
		} else {
			// No active or default rules found - just return what we have
			return result, nil
		}
	}

	hotRules, coldRules, _, err := m.expandAllRules(activeRulesFile, make(map[string]bool), 0)
	if err != nil {
		return nil, fmt.Errorf("failed to expand rules: %w", err)
	}

	// Extract patterns for root path discovery
	var allPatterns []string
	for _, rule := range hotRules {
		pattern := rule.Pattern
		if rule.Directive != "" {
			pattern = pattern + "|||" + rule.Directive + "|||" + rule.DirectiveQuery
		}
		if rule.IsExclude {
			allPatterns = append(allPatterns, "!"+pattern)
		} else {
			allPatterns = append(allPatterns, pattern)
		}
	}
	for _, rule := range coldRules {
		pattern := rule.Pattern
		if rule.Directive != "" {
			pattern = pattern + "|||" + rule.Directive + "|||" + rule.DirectiveQuery
		}
		if rule.IsExclude {
			allPatterns = append(allPatterns, "!"+pattern)
		} else {
			allPatterns = append(allPatterns, pattern)
		}
	}

	// Pre-process patterns
	allPatterns = m.preProcessPatterns(allPatterns)
	rootPaths := m.extractRootPaths(allPatterns)

	// Ensure working directory is in result (canonicalized)
	workDirCanonical := m.workDir
	if wd, err := filepath.EvalSymlinks(m.workDir); err == nil {
		workDirCanonical = wd
	}
	result[workDirCanonical] = StatusDirectory

	// Step 6: Walk each root path to discover all files and directories
	for _, rootPath := range rootPaths {
		// Canonicalize the root path for consistent comparisons
		rootPathCanonical := rootPath
		if rp, err := filepath.EvalSymlinks(rootPath); err == nil {
			rootPathCanonical = rp
		}

		gitIgnoredFiles := make(map[string]bool)
		if showGitIgnored {
			gitIgnoredFiles, err = m.getGitIgnoredFiles(rootPath)
			if err != nil {
				// Non-fatal, continue without gitignore info
				gitIgnoredFiles = make(map[string]bool)
			}
		}

		// Ensure all parent directories up to workDir exist in result (canonicalized)
		if rootPathCanonical != workDirCanonical {
			current := rootPathCanonical
			for current != workDirCanonical && current != "/" && current != "." {
				if _, exists := result[current]; !exists {
					result[current] = StatusDirectory
				}
				parent := filepath.Dir(current)
				if parent == current {
					break
				}
				current = parent
			}
		}

		// Walk the directory tree
		err = filepath.WalkDir(rootPath, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}

			// Canonicalize the path to match how files were stored in result
			canonicalPath := path
			if cp, err := filepath.EvalSymlinks(path); err == nil {
				canonicalPath = cp
			}

			// Skip the root itself
			if canonicalPath == rootPathCanonical {
				return nil
			}

			// Skip .git and .grove directories BEFORE processing
			if d.IsDir() && (d.Name() == ".git" || d.Name() == ".grove") {
				return filepath.SkipDir
			}

			// Check if already classified
			if _, exists := result[canonicalPath]; exists {
				return nil
			}

			// Classify based on what we know
			if d.IsDir() {
				if showGitIgnored && gitIgnoredFiles[path] {
					result[canonicalPath] = StatusIgnoredByGit
				} else {
					result[canonicalPath] = StatusDirectory
				}
			} else {
				// It's a file
				if showGitIgnored && gitIgnoredFiles[path] {
					result[canonicalPath] = StatusIgnoredByGit
				} else {
					// Not in any of our resolved sets, so it's omitted
					result[canonicalPath] = StatusOmittedNoMatch
				}
			}

			return nil
		})

		if err != nil {
			return nil, fmt.Errorf("failed to walk directory %s: %w", rootPath, err)
		}
	}

	// Step 7: Ensure all parent directories of classified files exist in result
	for path := range result {
		if path == m.workDir {
			continue
		}

		parent := filepath.Dir(path)
		for parent != path && parent != "/" && parent != "." {
			if _, exists := result[parent]; !exists {
				result[parent] = StatusDirectory
			}
			path = parent
			parent = filepath.Dir(path)
		}
	}

	// Step 8: Filter tree nodes to remove empty directories
	result = m.filterTreeNodes(result)

	return result, nil
}

// filterTreeNodes filters the file tree to show all directories containing any non-git-ignored files
func (m *Manager) filterTreeNodes(fileStatuses map[string]NodeStatus) map[string]NodeStatus {
	// Build a map of directories to their children
	dirChildren := make(map[string][]string)

	// First pass: identify all parent-child relationships
	for path, status := range fileStatuses {
		if status == StatusIgnoredByGit {
			continue // Skip ignored files
		}

		// Add this path to its parent's children
		parent := filepath.Dir(path)
		if parent != path { // Not the root
			dirChildren[parent] = append(dirChildren[parent], path)
		}
	}

	// Second pass: identify directories with included content
	dirsWithContent := make(map[string]bool)

	var markDirWithContent func(dirPath string)
	markDirWithContent = func(dirPath string) {
		if dirsWithContent[dirPath] {
			return // Already marked
		}
		dirsWithContent[dirPath] = true

		// Mark all parent directories as having content
		parent := filepath.Dir(dirPath)
		if parent != dirPath && parent != "/" && parent != "." {
			markDirWithContent(parent)
		}
	}

	// Mark directories that contain included files
	for path, status := range fileStatuses {
		// Any non-directory and non-git-ignored file counts as content
		hasContent := status != StatusDirectory && status != StatusIgnoredByGit

		// Also mark excluded directories themselves as having content so they show up
		if status == StatusExcludedByRule {
			hasContent = true
			dirsWithContent[path] = true
		}

		if hasContent {
			// This is a file with content - mark its parent directory
			parent := filepath.Dir(path)
			markDirWithContent(parent)
		}
	}

	// Third pass: create the filtered result
	filtered := make(map[string]NodeStatus)
	for path, status := range fileStatuses {
		if status == StatusDirectory || status == StatusExcludedByRule {
			// Include directories that have content or are explicitly excluded
			if dirsWithContent[path] || status == StatusExcludedByRule {
				filtered[path] = status
			}
		} else if status != StatusIgnoredByGit {
			// For files, check if their parent directory has content
			parent := filepath.Dir(path)
			if dirsWithContent[parent] {
				// Include all non-ignored files whose parent directory has content
				filtered[path] = status
			}
		}
	}

	return filtered
}

// extractRootPaths extracts all unique root paths from patterns
func (m *Manager) extractRootPaths(patterns []string) []string {
	rootsMap := make(map[string]bool)
	rootsMap[m.workDir] = true // Always include working directory

	for _, pattern := range patterns {
		if strings.HasPrefix(pattern, "!") {
			pattern = strings.TrimPrefix(pattern, "!")
		}

		// Extract base path from pattern
		if filepath.IsAbs(pattern) {
			// For absolute paths, find the non-glob base
			basePath := pattern
			for i, part := range strings.Split(pattern, string(filepath.Separator)) {
				if strings.ContainsAny(part, "*?[") {
					basePath = strings.Join(strings.Split(pattern, string(filepath.Separator))[:i], string(filepath.Separator))
					break
				}
			}

			// If pattern is a file path (no glob), use its directory
			if basePath == pattern && !strings.ContainsAny(pattern, "*?[") {
				if stat, err := os.Stat(pattern); err == nil && !stat.IsDir() {
					basePath = filepath.Dir(pattern)
				}
			}

			if basePath != "" {
				// Check if the path exists and is a directory before adding
				if stat, err := os.Stat(basePath); err == nil && stat.IsDir() {
					rootsMap[basePath] = true
				}
			}
		} else if strings.HasPrefix(pattern, "../") {
			// For relative external paths like ../grove-flow/**/*.go
			parts := strings.Split(pattern, "/")
			nonGlobParts := []string{}
			for _, part := range parts {
				if strings.ContainsAny(part, "*?[") {
					break
				}
				nonGlobParts = append(nonGlobParts, part)
			}
			if len(nonGlobParts) > 0 {
				relBase := strings.Join(nonGlobParts, "/")
				absBase := filepath.Join(m.workDir, relBase)
				absBase = filepath.Clean(absBase)
				if stat, err := os.Stat(absBase); err == nil && stat.IsDir() {
					rootsMap[absBase] = true
				}
			}
		}
	}

	// Convert map to slice
	var roots []string
	for root := range rootsMap {
		roots = append(roots, root)
	}
	return roots
}
