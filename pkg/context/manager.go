package context

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

// Constants for context file paths
const (
	GroveDir        = ".grove"
	ContextFile     = ".grove/context"
	FilesListFile   = ".grove/context-files"
	RulesFile       = ".grovectx"
	SnapshotsDir    = ".grove/context-snapshots"
)

// Manager handles context operations
type Manager struct {
	workDir string
}

// NewManager creates a new context manager
func NewManager(workDir string) *Manager {
	if workDir == "" {
		workDir, _ = os.Getwd()
	}
	return &Manager{workDir: workDir}
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

// GetContextInfo returns information about the context
func (m *Manager) GetContextInfo() (fileCount int, tokenCount int, size int, err error) {
	// Check if file exists
	if _, err := os.Stat(ContextFile); os.IsNotExist(err) {
		return 0, 0, 0, fmt.Errorf("%s file not found. Run 'grove cx generate' to create it", ContextFile)
	}
	
	// Read file content for token count
	content, err := os.ReadFile(ContextFile)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("error reading %s: %w", ContextFile, err)
	}
	
	// Count files in .grove-ctx-files
	files, err := m.ReadFilesList(FilesListFile)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("error reading %s: %w", FilesListFile, err)
	}
	
	// Approximate token count (roughly 4 characters per token)
	tokenCount = len(content) / 4
	fileCount = len(files)
	size = len(content)
	
	return fileCount, tokenCount, size, nil
}

// GenerateContext creates the context file from the files list
func (m *Manager) GenerateContext(useXMLFormat bool) error {
	// Ensure .grove directory exists
	groveDir := filepath.Join(m.workDir, GroveDir)
	if err := os.MkdirAll(groveDir, 0755); err != nil {
		return fmt.Errorf("error creating %s directory: %w", groveDir, err)
	}
	
	// Read file list
	filesListPath := filepath.Join(m.workDir, FilesListFile)
	filesToInclude, err := m.ReadFilesList(filesListPath)
	if err != nil {
		return fmt.Errorf("error reading %s: %w", filesListPath, err)
	}
	
	if len(filesToInclude) == 0 {
		return fmt.Errorf("%s is empty or not found", filesListPath)
	}
	
	// Create context file
	contextPath := filepath.Join(m.workDir, ContextFile)
	ctxFile, err := os.Create(contextPath)
	if err != nil {
		return fmt.Errorf("error creating %s: %w", contextPath, err)
	}
	defer ctxFile.Close()
	
	// Write concatenated content
	for _, file := range filesToInclude {
		if useXMLFormat {
			// XML-style delimiters (often better for LLMs)
			fmt.Fprintf(ctxFile, "<file path=\"%s\">\n", file)
			
			// Read and write file content
			filePath := filepath.Join(m.workDir, file)
			content, err := os.ReadFile(filePath)
			if err != nil {
				fmt.Fprintf(ctxFile, "<error>%v</error>\n", err)
				fmt.Fprintf(ctxFile, "</file>\n\n")
				continue
			}
			
			ctxFile.Write(content)
			fmt.Fprintf(ctxFile, "\n</file>\n\n")
		} else {
			// Classic delimiter style
			fmt.Fprintf(ctxFile, "=== FILE: %s ===\n", file)
			
			// Read and write file content
			filePath := filepath.Join(m.workDir, file)
			content, err := os.ReadFile(filePath)
			if err != nil {
				fmt.Fprintf(ctxFile, "Error reading file: %v\n", err)
				fmt.Fprintf(ctxFile, "=== END FILE: %s ===\n\n", file)
				continue
			}
			
			ctxFile.Write(content)
			
			// Write end marker
			fmt.Fprintf(ctxFile, "\n=== END FILE: %s ===\n\n", file)
		}
	}
	
	fmt.Printf("Generated %s with %d files\n", ContextFile, len(filesToInclude))
	return nil
}

// getGitIgnoredFiles returns a set of all gitignored files for the repository
// containing the given directory. It returns a map of absolute paths for efficient lookup.
func (m *Manager) getGitIgnoredFiles(forDir string) (map[string]bool, error) {
	ignoredFiles := make(map[string]bool)

	// Ensure the provided path is absolute
	absForDir, err := filepath.Abs(forDir)
	if err != nil {
		return ignoredFiles, err
	}

	// Find the root of the git repository for the given directory.
	gitRootCmd := exec.Command("git", "-C", absForDir, "rev-parse", "--show-toplevel")
	gitRootOutput, err := gitRootCmd.Output()
	if err != nil {
		// This directory is not in a git repository, so no files are gitignored.
		return ignoredFiles, nil
	}
	gitRootPath := strings.TrimSpace(string(gitRootOutput))

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

	return ignoredFiles, nil
}

// UpdateFromRules updates the files list based on rules file patterns
func (m *Manager) UpdateFromRules() error {
	// Read rules from .grovectx
	rulesPath := filepath.Join(m.workDir, RulesFile)
	patterns, err := m.ReadFilesList(rulesPath)
	if err != nil {
		if os.IsNotExist(err) {
			// Prompt user to create .grovectx
			fmt.Printf(".grovectx not found. Would you like to create one with '*' (include all files)? [Y/n]: ")
			var response string
			fmt.Scanln(&response)
			
			if response == "" || strings.ToLower(response) == "y" || strings.ToLower(response) == "yes" {
				// Create .grovectx with "*"
				if err := m.WriteFilesList(rulesPath, []string{"*"}); err != nil {
					return fmt.Errorf("error creating %s: %w", RulesFile, err)
				}
				fmt.Printf("Created %s with '*' pattern\n", RulesFile)
				
				// Read the newly created file
				patterns, err = m.ReadFilesList(rulesPath)
				if err != nil {
					return fmt.Errorf("error reading %s: %w", rulesPath, err)
				}
			} else {
				return fmt.Errorf("%s not found. Create it with patterns to include", RulesFile)
			}
		} else {
			return fmt.Errorf("error reading %s: %w", rulesPath, err)
		}
	}

	if len(patterns) == 0 {
		return fmt.Errorf("%s is empty or not found", RulesFile)
	}
	
	// Get gitignored files for the current working directory for handling relative patterns.
	gitIgnoredForCWD, err := m.getGitIgnoredFiles(m.workDir)
	if err != nil {
		fmt.Printf("Warning: could not get gitignored files for current directory: %v\n", err)
		gitIgnoredForCWD = make(map[string]bool)
	}

	// This map will store the final list of files to include.
	uniqueFiles := make(map[string]bool)

	// Separate patterns into relative and absolute paths
	var relativePatterns []string
	absolutePaths := make(map[string][]string) // map[absolutePath]patterns
	var deferredExclusions []string // Store exclusion patterns to process after inclusions
	
	// First pass: process inclusion patterns
	for _, pattern := range patterns {
		cleanPattern := pattern
		isExclude := strings.HasPrefix(pattern, "!")
		if isExclude {
			cleanPattern = strings.TrimPrefix(pattern, "!")
			// Defer exclusion patterns for second pass
			if filepath.IsAbs(cleanPattern) {
				deferredExclusions = append(deferredExclusions, pattern)
			} else {
				relativePatterns = append(relativePatterns, pattern)
			}
			continue
		}
		
		// Check if this is an absolute path
		if filepath.IsAbs(cleanPattern) {
			// For absolute paths, we'll walk them separately
			// Store the patterns that apply to this absolute path
			basePath := cleanPattern
			
			// For inclusion patterns, determine the base path
			if strings.Contains(cleanPattern, "*") || strings.Contains(cleanPattern, "?") {
				// Pattern contains wildcards - use the directory part as base
				basePath = filepath.Dir(cleanPattern)
				// Keep going up until we find a path without wildcards
				for strings.Contains(basePath, "*") || strings.Contains(basePath, "?") {
					basePath = filepath.Dir(basePath)
				}
			} else if strings.HasSuffix(cleanPattern, "/") {
				// Directory pattern - remove trailing slash
				basePath = strings.TrimSuffix(cleanPattern, "/")
			} else {
				// Could be a file or directory - check what it is
				if info, err := os.Stat(cleanPattern); err == nil && info.IsDir() {
					basePath = cleanPattern
				} else {
					// File pattern or non-existent path - use directory part
					basePath = filepath.Dir(cleanPattern)
				}
			}
			
			if _, exists := absolutePaths[basePath]; !exists {
				absolutePaths[basePath] = []string{}
			}
			absolutePaths[basePath] = append(absolutePaths[basePath], pattern)
		} else {
			// Relative pattern for current working directory
			relativePatterns = append(relativePatterns, pattern)
		}
	}
	
	// Second pass: add exclusion patterns to relevant base paths
	for _, pattern := range deferredExclusions {
		cleanPattern := strings.TrimPrefix(pattern, "!")
		// Add the exclusion pattern to all base paths it might affect
		for basePath := range absolutePaths {
			if strings.HasPrefix(cleanPattern, basePath) || strings.HasPrefix(basePath, cleanPattern) {
				absolutePaths[basePath] = append(absolutePaths[basePath], pattern)
			}
		}
	}

	// Process relative patterns using the CWD's gitignore rules.
	if len(relativePatterns) > 0 {
		err = m.walkAndMatchPatterns(m.workDir, relativePatterns, gitIgnoredForCWD, uniqueFiles, true)
		if err != nil {
			return fmt.Errorf("error walking working directory: %w", err)
		}
	}

	// Process each absolute path with its own specific gitignore rules.
	for absPath, pathPatterns := range absolutePaths {
		// Get gitignore rules for the repository containing this specific absolute path.
		gitIgnoredForAbsPath, err := m.getGitIgnoredFiles(absPath)
		if err != nil {
			fmt.Printf("Warning: could not get gitignored files for %s: %v\n", absPath, err)
			gitIgnoredForAbsPath = make(map[string]bool)
		}

		// Walk the path and apply its patterns and gitignore rules.
		err = m.walkAndMatchPatterns(absPath, pathPatterns, gitIgnoredForAbsPath, uniqueFiles, false)
		if err != nil {
			fmt.Printf("Warning: error walking absolute path %s: %v\n", absPath, err)
		}
	}

	// Convert map to a sorted slice for consistent output.
	filesToInclude := make([]string, 0, len(uniqueFiles))
	for file := range uniqueFiles {
		filesToInclude = append(filesToInclude, file)
	}
	sort.Strings(filesToInclude)

	// Ensure .grove directory exists relative to workDir
	groveDir := filepath.Join(m.workDir, GroveDir)
	if err := os.MkdirAll(groveDir, 0755); err != nil {
		return fmt.Errorf("error creating %s directory: %w", groveDir, err)
	}

	// Write the filtered file list to context-files
	filesPath := filepath.Join(m.workDir, FilesListFile)
	return m.WriteFilesList(filesPath, filesToInclude)
}

// walkAndMatchPatterns walks a directory and matches files against patterns
func (m *Manager) walkAndMatchPatterns(rootPath string, patterns []string, gitIgnoredFiles map[string]bool, uniqueFiles map[string]bool, useRelativePaths bool) error {
	// Walk the directory tree from the specified root path.
	return filepath.WalkDir(rootPath, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// First, check if the file or directory is ignored by git. This is the most efficient check.
		// The `path` from WalkDir is absolute if the root is absolute, which it always will be.
		if gitIgnoredFiles[path] {
			if d.IsDir() {
				return filepath.SkipDir // Prune the walk for ignored directories.
			}
			return nil // Skip ignored files.
		}

		// Always prune .git and .grove directories from the walk.
		if d.IsDir() {
			if d.Name() == ".git" || d.Name() == ".grove" {
				return filepath.SkipDir
			}
			return nil // Continue walking into subdirectories.
		}

		// Get path relative to the walk root for pattern matching.
		relPath, err := filepath.Rel(rootPath, path)
		if err != nil {
			return err
		}
		// Always use forward slashes for cross-platform pattern matching consistency.
		relPath = filepath.ToSlash(relPath)

		// Skip the root directory itself.
		if relPath == "." {
			return nil
		}

		// --- Gitignore-style matching logic ---
		// Default to not included. A file must match an include pattern.
		isIncluded := false
		for _, pattern := range patterns {
			isExclude := strings.HasPrefix(pattern, "!")
			if isExclude {
				pattern = strings.TrimPrefix(pattern, "!")
			}

			// Gitignore-style matching logic
			match := false
			pattern = filepath.ToSlash(pattern) // Ensure pattern uses slashes
			
			// For absolute path patterns, we need special handling
			if filepath.IsAbs(pattern) {
				// For absolute patterns, match against the full absolute path
				absPath := filepath.ToSlash(path)
				pattern = filepath.ToSlash(pattern)
				
				// Check if pattern matches the absolute path
				if pattern == absPath {
					// Exact match
					match = true
				} else if strings.HasSuffix(pattern, "/") {
					// Directory pattern
					dirPattern := strings.TrimSuffix(pattern, "/")
					if absPath == dirPattern || strings.HasPrefix(absPath, dirPattern+"/") {
						match = true
					}
				} else if matched, _ := filepath.Match(pattern, absPath); matched {
					// Wildcard pattern match
					match = true
				}
			}

			if !match {
				// Handle ** patterns
				if strings.Contains(pattern, "**") {
					match = matchDoubleStarPattern(pattern, relPath)
				} else if strings.HasSuffix(pattern, "/") {
					// Directory pattern: 'demos/' should match 'demos/main.go'
					dirPattern := strings.TrimSuffix(pattern, "/")
					if relPath == dirPattern || strings.HasPrefix(relPath, dirPattern+"/") {
						match = true
					}
				} else if matched, _ := filepath.Match(pattern, relPath); matched {
					// Full path pattern: 'internal/cli/agent.go'
					match = true
				} else if !strings.Contains(pattern, "/") {
					// Basename pattern if no slashes: '*.go' should match 'a.go' and 'cli/a.go'
					if matched, _ := filepath.Match(pattern, filepath.Base(relPath)); matched {
						match = true
					}
				}
			}

			// The last matching pattern wins.
			if match {
				isIncluded = !isExclude
			}
		}

		if isIncluded {
			// Determine the final path to store
			var finalPath string
			if useRelativePaths {
				// For relative patterns, store path relative to workDir
				finalPath, _ = filepath.Rel(m.workDir, path)
			} else {
				// For absolute patterns, check if the path is within workDir
				if relPath, err := filepath.Rel(m.workDir, path); err == nil && !strings.HasPrefix(relPath, "..") {
					// Path is within workDir, use relative path for consistency
					finalPath = relPath
				} else {
					// Path is outside workDir, use absolute path
					finalPath = path
				}
			}
			
			uniqueFiles[finalPath] = true
		}

		return nil
	})
}

// SaveSnapshot saves the current context files list as a snapshot
func (m *Manager) SaveSnapshot(name, description string) error {
	// Ensure .grove directory and snapshots subdirectory exist
	if err := os.MkdirAll(SnapshotsDir, 0755); err != nil {
		return fmt.Errorf("error creating snapshots directory: %w", err)
	}
	
	// Read current files list
	content, err := os.ReadFile(FilesListFile)
	if err != nil {
		return fmt.Errorf("error reading %s: %w", FilesListFile, err)
	}
	
	// Save to snapshot
	snapshotPath := filepath.Join(SnapshotsDir, name)
	if err := os.WriteFile(snapshotPath, content, 0644); err != nil {
		return fmt.Errorf("error saving snapshot: %w", err)
	}
	
	// Save description if provided
	if description != "" {
		descPath := filepath.Join(SnapshotsDir, name+".desc")
		if err := os.WriteFile(descPath, []byte(description), 0644); err != nil {
			// Non-fatal error
			fmt.Printf("Warning: could not save description: %v\n", err)
		}
	}
	
	fmt.Printf("Saved snapshot to %s\n", snapshotPath)
	return nil
}

// LoadSnapshot loads a snapshot into the current context files list
func (m *Manager) LoadSnapshot(name string) error {
	snapshotPath := filepath.Join(SnapshotsDir, name)
	
	// Read snapshot
	content, err := os.ReadFile(snapshotPath)
	if err != nil {
		return fmt.Errorf("error reading snapshot: %w", err)
	}
	
	// Write to current files list
	if err := os.WriteFile(FilesListFile, content, 0644); err != nil {
		return fmt.Errorf("error writing %s: %w", FilesListFile, err)
	}
	
	fmt.Printf("Loaded snapshot from %s\n", snapshotPath)
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
	files, err := m.ReadFilesList(FilesListFile)
	if err != nil {
		return nil, fmt.Errorf("error reading %s: %w", FilesListFile, err)
	}
	
	// Convert to absolute paths
	var absPaths []string
	for _, file := range files {
		absPath, err := filepath.Abs(file)
		if err != nil {
			absPaths = append(absPaths, file + " (error getting absolute path)")
		} else {
			absPaths = append(absPaths, absPath)
		}
	}
	
	return absPaths, nil
}

// matchDoubleStarPattern handles patterns with ** for recursive matching
func matchDoubleStarPattern(pattern, path string) bool {
	// Split pattern at **
	parts := strings.Split(pattern, "**")
	
	if len(parts) == 2 {
		prefix := strings.TrimSuffix(parts[0], "/")
		suffix := strings.TrimPrefix(parts[1], "/")
		
		// Check prefix match
		if prefix != "" && !strings.HasPrefix(path, prefix) {
			return false
		}
		
		// Check suffix match
		if suffix != "" {
			// For patterns like "**/*.go", match against the filename
			if !strings.Contains(suffix, "/") {
				matched, _ := filepath.Match(suffix, filepath.Base(path))
				return matched
			}
			// For patterns with paths in suffix, do a full suffix match
			// This is a simplified version - full gitignore would be more complex
			pathAfterPrefix := strings.TrimPrefix(path, prefix)
			pathAfterPrefix = strings.TrimPrefix(pathAfterPrefix, "/")
			matched, _ := filepath.Match(suffix, pathAfterPrefix)
			return matched
		}
		
		// If only prefix is specified, it matches
		return true
	}
	
	// Fallback to simple match
	matched, _ := filepath.Match(pattern, path)
	return matched
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