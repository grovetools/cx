package context

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
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
)

// Manager handles context operations
type Manager struct {
	workDir         string
	gitIgnoredCache map[string]map[string]bool // Cache for gitignored files by repository root
	aliasResolver   *AliasResolver             // Lazily initialized alias resolver
}

// NewManager creates a new context manager
func NewManager(workDir string) *Manager {
	if workDir == "" {
		workDir, _ = os.Getwd()
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
	return m.aliasResolver
}

// ResolveLineForRulePreview resolves a single rule line, handling aliases.
// It is intended for use by tools like the Neovim plugin's rule previewer.
func (m *Manager) ResolveLineForRulePreview(line string) (string, error) {
	resolver := m.getAliasResolver()
	if resolver != nil && (strings.Contains(line, "@alias:") || strings.Contains(line, "@a:")) {
		return resolver.ResolveLine(line)
	}
	// If not an alias line, return the original line.
	return line, nil
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

	// Check if we have a cached result for this repository
	if cachedResult, found := m.gitIgnoredCache[gitRootPath]; found {
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
				ignoredFiles[absolutePath] = true
			}
		}
	}

	// Cache the result before returning
	m.gitIgnoredCache[gitRootPath] = ignoredFiles

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

// parseGitRule checks if a rule is a Git URL and extracts the URL and optional version
func (m *Manager) parseGitRule(rule string) (isGitURL bool, repoURL, version string) {
	// Remove exclusion prefix if present
	if strings.HasPrefix(rule, "!") {
		rule = strings.TrimPrefix(rule, "!")
	}

	// Check for common Git URL patterns
	gitURLPattern := regexp.MustCompile(`^(https?://|git@|github\.com/|gitlab\.com/|bitbucket\.org/)`)
	if !gitURLPattern.MatchString(rule) {
		return false, "", ""
	}

	// Parse version if present (format: url@version)
	parts := strings.SplitN(rule, "@", 2)
	repoURL = parts[0]

	// Ensure proper URL format
	if !strings.HasPrefix(repoURL, "http://") && !strings.HasPrefix(repoURL, "https://") {
		if strings.HasPrefix(repoURL, "github.com/") {
			repoURL = "https://" + repoURL
		} else if strings.HasPrefix(repoURL, "gitlab.com/") {
			repoURL = "https://" + repoURL
		} else if strings.HasPrefix(repoURL, "bitbucket.org/") {
			repoURL = "https://" + repoURL
		} else if strings.HasPrefix(repoURL, "git@") {
			// Convert SSH URL to HTTPS
			repoURL = strings.Replace(repoURL, "git@github.com:", "https://github.com/", 1)
			repoURL = strings.Replace(repoURL, "git@gitlab.com:", "https://gitlab.com/", 1)
			repoURL = strings.Replace(repoURL, "git@bitbucket.org:", "https://bitbucket.org/", 1)
		}
	}

	if len(parts) > 1 {
		version = parts[1]
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
