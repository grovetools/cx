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
	"github.com/mattsolo1/grove-core/util/pathutil"
)

// Constants for context file paths
const (
	GroveDir                   = ".grove"
	ContextFile                = ".grove/context"
	FilesListFile              = ".grove/context-files"
	RulesFile                  = ".grovectx"
	ActiveRulesFile            = ".grove/rules"
	SnapshotsDir               = ".grove/context-snapshots"
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

	// 3. Construct the path to the ruleset file
	rulesFilePath := filepath.Join(projectPath, ".cx", rulesetName+".rules")

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
	cacheKey := strings.ToLower(gitRootPath)

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

				// Resolve symlinks first, then lowercase for case-insensitive lookup
				if evalPath, err := filepath.EvalSymlinks(absolutePath); err == nil {
					absolutePath = evalPath
				}
				normalizedPath := strings.ToLower(absolutePath)
				ignoredFiles[normalizedPath] = true
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

// SaveSnapshot saves the current rules as a snapshot
func (m *Manager) SaveSnapshot(name, description string) error {
	// Ensure .grove directory and snapshots subdirectory exist
	snapshotsDir := filepath.Join(m.workDir, SnapshotsDir)
	if err := os.MkdirAll(snapshotsDir, 0755); err != nil {
		return fmt.Errorf("error creating snapshots directory: %w", err)
	}

	// Read current rules file
	activeRulesPath := filepath.Join(m.workDir, ActiveRulesFile)
	if _, err := os.Stat(activeRulesPath); os.IsNotExist(err) {
		// Try old .grovectx file
		activeRulesPath = filepath.Join(m.workDir, RulesFile)
		if _, err := os.Stat(activeRulesPath); os.IsNotExist(err) {
			return fmt.Errorf("no rules file found. Create %s with patterns to include", ActiveRulesFile)
		}
	}

	content, err := os.ReadFile(activeRulesPath)
	if err != nil {
		return fmt.Errorf("error reading rules file: %w", err)
	}

	// Save to snapshot with .rules extension
	snapshotPath := filepath.Join(snapshotsDir, name+".rules")
	if err := os.WriteFile(snapshotPath, content, 0644); err != nil {
		return fmt.Errorf("error saving snapshot: %w", err)
	}

	// Save description if provided
	if description != "" {
		descPath := filepath.Join(snapshotsDir, name+".rules.desc")
		if err := os.WriteFile(descPath, []byte(description), 0644); err != nil {
			// Non-fatal error
			fmt.Printf("Warning: could not save description: %v\n", err)
		}
	}

	fmt.Printf("Saved rules snapshot to %s\n", snapshotPath)
	return nil
}

// LoadSnapshot loads a snapshot into the current rules file
func (m *Manager) LoadSnapshot(name string) error {
	snapshotsDir := filepath.Join(m.workDir, SnapshotsDir)

	// Try with .rules extension first
	snapshotPath := filepath.Join(snapshotsDir, name+".rules")
	if _, err := os.Stat(snapshotPath); os.IsNotExist(err) {
		// Try without extension for backward compatibility
		snapshotPath = filepath.Join(snapshotsDir, name)
		if _, err := os.Stat(snapshotPath); os.IsNotExist(err) {
			return fmt.Errorf("snapshot '%s' not found", name)
		}
	}

	// Read snapshot
	content, err := os.ReadFile(snapshotPath)
	if err != nil {
		return fmt.Errorf("error reading snapshot: %w", err)
	}

	// Ensure .grove directory exists
	groveDir := filepath.Join(m.workDir, GroveDir)
	if err := os.MkdirAll(groveDir, 0755); err != nil {
		return fmt.Errorf("error creating %s directory: %w", groveDir, err)
	}

	// Write to active rules file
	activeRulesPath := filepath.Join(m.workDir, ActiveRulesFile)
	if err := os.WriteFile(activeRulesPath, content, 0644); err != nil {
		return fmt.Errorf("error writing rules: %w", err)
	}

	fmt.Printf("Loaded rules snapshot from %s\n", snapshotPath)
	return nil
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
					canonicalPath, err := filepath.EvalSymlinks(node.Path)
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
					canonicalPath, err := filepath.EvalSymlinks(node.Path)
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
			canonicalGroveHome, err := filepath.EvalSymlinks(groveHome)
			if err != nil {
				canonicalGroveHome = groveHome
			}
			allowed = append(allowed, canonicalGroveHome)
		}

		// Also add notebook root directories to allowed paths
		if mergedCfg != nil && mergedCfg.Notebooks != nil {
			for notebookName, notebook := range mergedCfg.Notebooks {
				if notebook.RootDir != "" {
					notebookRootDir, err := pathutil.Expand(notebook.RootDir)
					if err != nil {
						fmt.Fprintf(os.Stderr, "Warning: could not expand notebook '%s' root_dir '%s': %v\n", notebookName, notebook.RootDir, err)
					} else {
						// Canonicalize notebook root path
						canonicalNotebookRoot, err := filepath.EvalSymlinks(notebookRootDir)
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

			// Resolve symlinks for canonical path
			canonicalPath, err := filepath.EvalSymlinks(absPath)
			if err != nil {
				// Fallback to absolute path if symlink resolution fails
				fmt.Fprintf(os.Stderr, "Warning: could not resolve symlinks for allowed_path '%s': %v\n", absPath, err)
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

	// Resolve symlinks to get the canonical path for comparison
	canonicalPath, err := filepath.EvalSymlinks(absPath)
	if err != nil {
		// If symlink resolution fails, fall back to the non-symlinked absolute path for the check.
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
				nodePath := node.Path
				if evalPath, err := filepath.EvalSymlinks(node.Path); err == nil {
					nodePath = evalPath
				}
				// Normalize for case-insensitive filesystems
				normalizedNodePath := strings.ToLower(nodePath)
				normalizedCanonicalPath := strings.ToLower(canonicalPath)

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
