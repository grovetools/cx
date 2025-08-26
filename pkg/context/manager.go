package context

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// NodeStatus represents the classification of a file in the context
type NodeStatus int

const (
	StatusIncludedHot    NodeStatus = iota // In hot context
	StatusIncludedCold                     // In cold context
	StatusExcludedByRule                   // Matched an include rule, but then an exclude rule
	StatusOmittedNoMatch                   // Not matched by any include rule
	StatusIgnoredByGit                     // Ignored by .gitignore (not used in final result)
	StatusDirectory                        // A directory containing other nodes
)

// Constants for context file paths
const (
	GroveDir                  = ".grove"
	ContextFile               = ".grove/context"
	FilesListFile             = ".grove/context-files"
	RulesFile                 = ".grovectx"
	ActiveRulesFile           = ".grove/rules"
	SnapshotsDir              = ".grove/context-snapshots"
	CachedContextFilesListFile = ".grove/cached-context-files"
	CachedContextFile          = ".grove/cached-context"
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

// GetWorkDir returns the current working directory
func (m *Manager) GetWorkDir() string {
	return m.workDir
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


// findActiveRulesFile returns the path to the active rules file if it exists
func (m *Manager) findActiveRulesFile() string {
	// Check for new rules file location first
	activeRulesPath := filepath.Join(m.workDir, ActiveRulesFile)
	if _, err := os.Stat(activeRulesPath); err == nil {
		return activeRulesPath
	}
	
	// Check for old .grovectx file for backward compatibility
	oldRulesPath := filepath.Join(m.workDir, RulesFile)
	if _, err := os.Stat(oldRulesPath); err == nil {
		return oldRulesPath
	}
	
	return ""
}

// GenerateContext creates the context file from the files list
func (m *Manager) GenerateContext(useXMLFormat bool) error {
	// Ensure .grove directory exists
	groveDir := filepath.Join(m.workDir, GroveDir)
	if err := os.MkdirAll(groveDir, 0755); err != nil {
		return fmt.Errorf("error creating %s directory: %w", groveDir, err)
	}
	
	// Dynamically resolve file list from rules
	filesToInclude, err := m.ResolveFilesFromRules()
	if err != nil {
		return fmt.Errorf("error resolving files from rules: %w", err)
	}
	
	// Handle case where no rules file exists
	if len(filesToInclude) == 0 {
		// Print visible warning to stderr
		fmt.Fprintf(os.Stderr, "\n⚠️  WARNING: No rules file found!\n")
		fmt.Fprintf(os.Stderr, "⚠️  Create %s with patterns to include files in the context.\n", ActiveRulesFile)
		fmt.Fprintf(os.Stderr, "⚠️  Generating empty context file.\n\n")
	}
	
	// Create context file
	contextPath := filepath.Join(m.workDir, ContextFile)
	ctxFile, err := os.Create(contextPath)
	if err != nil {
		return fmt.Errorf("error creating %s: %w", contextPath, err)
	}
	defer ctxFile.Close()
	
	// If no files to include, write a comment explaining why
	if len(filesToInclude) == 0 {
		if useXMLFormat {
			fmt.Fprintf(ctxFile, "<!-- No rules file found. Create %s with patterns to include files. -->\n", ActiveRulesFile)
		} else {
			fmt.Fprintf(ctxFile, "# No rules file found. Create %s with patterns to include files.\n", ActiveRulesFile)
		}
	}
	
	// Write concatenated content
	for _, file := range filesToInclude {
		if useXMLFormat {
			// XML-style delimiters (often better for LLMs)
			fmt.Fprintf(ctxFile, "<file path=\"%s\">\n", file)
			
			// Read and write file content
			filePath := file
			if !filepath.IsAbs(file) {
				filePath = filepath.Join(m.workDir, file)
			}
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
			filePath := file
			if !filepath.IsAbs(file) {
				filePath = filepath.Join(m.workDir, file)
			}
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
	
	// Generate cached context file list
	activeRulesPath := m.findActiveRulesFile()
	if activeRulesPath == "" {
		// If no rules file, ensure any old cached list is removed
		os.Remove(filepath.Join(m.workDir, CachedContextFilesListFile))
		return nil
	}

	_, coldPatterns, _, _, _, _, err := m.parseRulesFile(activeRulesPath)
	if err != nil {
		return fmt.Errorf("error parsing cold context rules: %w", err)
	}

	coldFiles, err := m.resolveFilesFromPatterns(coldPatterns)
	if err != nil {
		return fmt.Errorf("error resolving cold context files: %w", err)
	}

	// Write the list to .grove/cached-context-files
	cachedListPath := filepath.Join(m.workDir, CachedContextFilesListFile)
	if err := m.WriteFilesList(cachedListPath, coldFiles); err != nil {
		return err
	}

	// Provide user feedback
	if len(coldFiles) > 0 {
		fmt.Printf("Generated %s with %d files for cached context\n", CachedContextFilesListFile, len(coldFiles))
	}
	
	return nil
}

// GenerateCachedContext generates .grove/cached-context with only the cold context files.
func (m *Manager) GenerateCachedContext() error {
	// Ensure .grove directory exists
	groveDir := filepath.Join(m.workDir, GroveDir)
	if err := os.MkdirAll(groveDir, 0755); err != nil {
		return fmt.Errorf("error creating %s directory: %w", groveDir, err)
	}
	
	// Get ONLY cold context files
	coldFiles, err := m.ResolveColdContextFiles()
	if err != nil {
		return fmt.Errorf("error resolving cold context files: %w", err)
	}
	
	// Create cached context file
	cachedPath := filepath.Join(m.workDir, CachedContextFile)
	cachedFile, err := os.Create(cachedPath)
	if err != nil {
		return fmt.Errorf("error creating %s: %w", cachedPath, err)
	}
	defer cachedFile.Close()
	
	// If no cold files, we can just create an empty file or a small XML structure.
	// Let's keep the structure for consistency.
	fmt.Fprintf(cachedFile, "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n")
	fmt.Fprintf(cachedFile, "<context>\n")
	fmt.Fprintf(cachedFile, "  <cold-context files=\"%d\">\n", len(coldFiles))
	
	// Write cold context files
	for _, file := range coldFiles {
		if err := m.writeFileToXML(cachedFile, file, "    "); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: error writing file %s: %v\n", file, err)
		}
	}
	
	fmt.Fprintf(cachedFile, "  </cold-context>\n")
	fmt.Fprintf(cachedFile, "</context>\n")
	
	fmt.Printf("Generated %s with %d cold files\n", CachedContextFile, len(coldFiles))
	return nil
}

// writeFileToXML writes a file's content to the XML output with proper indentation
func (m *Manager) writeFileToXML(w io.Writer, file string, indent string) error {
	fmt.Fprintf(w, "%s<file path=\"%s\">\n", indent, file)
	
	// Read file content
	filePath := file
	if !filepath.IsAbs(file) {
		filePath = filepath.Join(m.workDir, file)
	}
	
	content, err := os.ReadFile(filePath)
	if err != nil {
		fmt.Fprintf(w, "%s  <error>%v</error>\n", indent, err)
		fmt.Fprintf(w, "%s</file>\n", indent)
		return err
	}
	
	// Write content directly without extra indentation (content already has its own)
	w.Write(content)
	
	// Ensure there's a newline before the closing tag
	if len(content) > 0 && content[len(content)-1] != '\n' {
		fmt.Fprintf(w, "\n")
	}
	
	fmt.Fprintf(w, "%s</file>\n", indent)
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

// parseRulesFile reads a rules file and separates patterns into main and cold contexts.
func (m *Manager) parseRulesFile(rulesPath string) (mainPatterns, coldPatterns []string, freezeCache, disableExpiration, disableCache bool, expireTime time.Duration, err error) {
	file, err := os.Open(rulesPath)
	if err != nil {
		return nil, nil, false, false, false, 0, err
	}
	defer file.Close()

	inColdSection := false
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "@freeze-cache" {
			freezeCache = true
			continue
		}
		if line == "@no-expire" {
			disableExpiration = true
			continue
		}
		if line == "@disable-cache" {
			disableCache = true
			continue
		}
		if strings.HasPrefix(line, "@expire-time ") {
			// Parse the duration argument
			durationStr := strings.TrimSpace(strings.TrimPrefix(line, "@expire-time"))
			if durationStr != "" {
				parsedDuration, parseErr := time.ParseDuration(durationStr)
				if parseErr != nil {
					return nil, nil, false, false, false, 0, fmt.Errorf("invalid duration format for @expire-time: %w", parseErr)
				}
				expireTime = parsedDuration
			}
			continue
		}
		if line == "---" {
			inColdSection = true
			continue
		}
		if line != "" && !strings.HasPrefix(line, "#") {
			if inColdSection {
				coldPatterns = append(coldPatterns, line)
			} else {
				mainPatterns = append(mainPatterns, line)
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, nil, false, false, false, 0, err
	}
	return mainPatterns, coldPatterns, freezeCache, disableExpiration, disableCache, expireTime, nil
}

// ShouldFreezeCache checks if the @freeze-cache directive is present in the rules file.
func (m *Manager) ShouldFreezeCache() (bool, error) {
	activeRulesPath := m.findActiveRulesFile()
	if activeRulesPath == "" {
		return false, nil
	}

	_, _, freezeCache, _, _, _, err := m.parseRulesFile(activeRulesPath)
	if err != nil {
		return false, fmt.Errorf("error parsing rules file for cache directive: %w", err)
	}

	return freezeCache, nil
}

// ShouldDisableExpiration checks if the @no-expire directive is present in the rules file.
func (m *Manager) ShouldDisableExpiration() (bool, error) {
	activeRulesPath := m.findActiveRulesFile()
	if activeRulesPath == "" {
		return false, nil
	}

	_, _, _, disableExpiration, _, _, err := m.parseRulesFile(activeRulesPath)
	if err != nil {
		return false, fmt.Errorf("error parsing rules file for cache directive: %w", err)
	}

	return disableExpiration, nil
}

// GetExpireTime returns the custom expiration duration if @expire-time directive is present.
// Returns 0 if no custom expiration time is set.
func (m *Manager) GetExpireTime() (time.Duration, error) {
	activeRulesPath := m.findActiveRulesFile()
	if activeRulesPath == "" {
		return 0, nil
	}

	_, _, _, _, _, expireTime, err := m.parseRulesFile(activeRulesPath)
	if err != nil {
		return 0, fmt.Errorf("error parsing rules file for expire time: %w", err)
	}

	return expireTime, nil
}

// ShouldDisableCache checks if the @disable-cache directive is present in the rules file.
func (m *Manager) ShouldDisableCache() (bool, error) {
	activeRulesPath := m.findActiveRulesFile()
	if activeRulesPath == "" {
		return false, nil
	}

	_, _, _, _, disableCache, _, err := m.parseRulesFile(activeRulesPath)
	if err != nil {
		return false, fmt.Errorf("error parsing rules file for cache directive: %w", err)
	}

	return disableCache, nil
}

// ResolveFilesFromRules dynamically resolves the list of files from the active rules file
func (m *Manager) ResolveFilesFromRules() ([]string, error) {
	// Use the centralized engine
	fileStatuses, err := m.ResolveAndClassifyAllFiles()
	if err != nil {
		return nil, err
	}
	
	// Filter for hot context files only
	var hotFiles []string
	for path, status := range fileStatuses {
		if status == StatusIncludedHot {
			// Convert absolute paths back to relative if within workDir
			relPath, err := filepath.Rel(m.workDir, path)
			if err == nil && !strings.HasPrefix(relPath, "..") {
				hotFiles = append(hotFiles, relPath)
			} else {
				hotFiles = append(hotFiles, path)
			}
		}
	}
	
	// Sort for consistent output
	sort.Strings(hotFiles)
	return hotFiles, nil
}

// ResolveColdContextFiles resolves the list of files from the "cold" section of a rules file.
func (m *Manager) ResolveColdContextFiles() ([]string, error) {
	// Use the centralized engine
	fileStatuses, err := m.ResolveAndClassifyAllFiles()
	if err != nil {
		return nil, err
	}
	
	// Filter for cold context files only
	var coldFiles []string
	for path, status := range fileStatuses {
		if status == StatusIncludedCold {
			// Convert absolute paths back to relative if within workDir
			relPath, err := filepath.Rel(m.workDir, path)
			if err == nil && !strings.HasPrefix(relPath, "..") {
				coldFiles = append(coldFiles, relPath)
			} else {
				coldFiles = append(coldFiles, path)
			}
		}
	}
	
	// Sort for consistent output
	sort.Strings(coldFiles)
	return coldFiles, nil
}

// resolveFilesFromPatterns resolves files from a given set of patterns
func (m *Manager) resolveFilesFromPatterns(patterns []string) ([]string, error) {
	if len(patterns) == 0 {
		return []string{}, nil
	}
	
	// Pre-process patterns to transform directory patterns into recursive globs
	processedPatterns := make([]string, 0, len(patterns))
	for _, pattern := range patterns {
		isExclude := strings.HasPrefix(pattern, "!")
		cleanPattern := pattern
		if isExclude {
			cleanPattern = strings.TrimPrefix(pattern, "!")
		}
		
		// Check if pattern contains glob characters
		hasGlob := strings.Contains(cleanPattern, "*") || strings.Contains(cleanPattern, "?")
		
		// Only transform plain directory patterns for INCLUSION patterns
		// Exclusion patterns like !tests should remain as-is for gitignore compatibility
		if !hasGlob && !isExclude {
			// Resolve the path to check if it exists and is a directory
			checkPath := cleanPattern
			if !filepath.IsAbs(cleanPattern) {
				checkPath = filepath.Join(m.workDir, cleanPattern)
			}
			checkPath = filepath.Clean(checkPath)
			
			if info, err := os.Stat(checkPath); err == nil && info.IsDir() {
				// Transform directory pattern to recursive glob
				processedPatterns = append(processedPatterns, cleanPattern+"/**")
				continue
			}
		}
		
		// Keep pattern as-is
		processedPatterns = append(processedPatterns, pattern)
	}
	
	// Use processed patterns for the rest of the logic
	patterns = processedPatterns
	
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
		
		// Check if this is an absolute path or a relative path that goes outside current directory
		if filepath.IsAbs(cleanPattern) || strings.HasPrefix(cleanPattern, "../") {
			// For absolute paths and relative paths going up, we'll walk them separately
			// Store the patterns that apply to this path
			basePath := cleanPattern
			
			// For relative paths, resolve them relative to workDir
			if !filepath.IsAbs(cleanPattern) {
				basePath = filepath.Join(m.workDir, cleanPattern)
				basePath = filepath.Clean(basePath)
			}
			
			// For inclusion patterns, determine the base path
			if strings.Contains(basePath, "*") || strings.Contains(basePath, "?") {
				// Pattern contains wildcards - use the directory part as base
				basePath = filepath.Dir(basePath)
				// Keep going up until we find a path without wildcards
				for strings.Contains(basePath, "*") || strings.Contains(basePath, "?") {
					basePath = filepath.Dir(basePath)
				}
			} else if strings.HasSuffix(basePath, "/") {
				// Directory pattern - remove trailing slash
				basePath = strings.TrimSuffix(basePath, "/")
			} else {
				// Could be a file or directory - check what it is
				if info, err := os.Stat(basePath); err == nil && info.IsDir() {
					// It's a directory, use as is
				} else {
					// File pattern or non-existent path - use directory part
					basePath = filepath.Dir(basePath)
				}
			}
			
			if _, exists := absolutePaths[basePath]; !exists {
				absolutePaths[basePath] = []string{}
			}
			// Store the original pattern (not the resolved basePath)
			absolutePaths[basePath] = append(absolutePaths[basePath], pattern)
		} else {
			// Relative pattern for current working directory
			relativePatterns = append(relativePatterns, pattern)
		}
	}
	
	// Second pass: add exclusion patterns to all base paths
	// Collect all exclusion patterns (both from relativePatterns and deferredExclusions)
	allExclusions := []string{}
	for _, pattern := range relativePatterns {
		if strings.HasPrefix(pattern, "!") {
			allExclusions = append(allExclusions, pattern)
		}
	}
	allExclusions = append(allExclusions, deferredExclusions...)
	
	// Add exclusion patterns to all absolute paths since they should apply globally
	for basePath := range absolutePaths {
		for _, exclusion := range allExclusions {
			absolutePaths[basePath] = append(absolutePaths[basePath], exclusion)
		}
	}

	// Process relative patterns using the CWD's gitignore rules.
	if len(relativePatterns) > 0 {
		err = m.walkAndMatchPatterns(m.workDir, relativePatterns, gitIgnoredForCWD, uniqueFiles, true)
		if err != nil {
			return nil, fmt.Errorf("error walking working directory: %w", err)
		}
	}

	// Process each absolute path with its own specific gitignore rules.
	for absPath, pathPatterns := range absolutePaths {
		// Check if the path exists
		if _, err := os.Stat(absPath); os.IsNotExist(err) {
			// Path doesn't exist, skip it
			continue
		}
		
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

	// Return the resolved file list
	return filesToInclude, nil
}

// resolveFileListFromRules dynamically resolves the list of files from a rules file
func (m *Manager) resolveFileListFromRules(rulesPath string) ([]string, error) {
	// Parse the rules file to get main and cold patterns
	mainPatterns, coldPatterns, _, _, _, _, err := m.parseRulesFile(rulesPath)
	if err != nil {
		return nil, fmt.Errorf("error parsing rules file: %w", err)
	}

	// If no main patterns, return empty list
	if len(mainPatterns) == 0 && len(coldPatterns) == 0 {
		return nil, fmt.Errorf("rules file is empty")
	}

	// Resolve files from main patterns
	mainFiles, err := m.resolveFilesFromPatterns(mainPatterns)
	if err != nil {
		return nil, fmt.Errorf("error resolving main context files: %w", err)
	}

	// Resolve files from cold patterns
	coldFiles, err := m.resolveFilesFromPatterns(coldPatterns)
	if err != nil {
		return nil, fmt.Errorf("error resolving cold context files: %w", err)
	}

	// Create a map of cold files for efficient exclusion
	coldFilesMap := make(map[string]bool)
	for _, file := range coldFiles {
		coldFilesMap[file] = true
	}

	// Filter main files to exclude any that are in cold files
	var finalMainFiles []string
	for _, file := range mainFiles {
		if !coldFilesMap[file] {
			finalMainFiles = append(finalMainFiles, file)
		}
	}

	return finalMainFiles, nil
}

// ResolveAndClassifyAllFiles is the centralized engine that resolves and classifies all files
// based on context rules. It returns a map of file paths to their NodeStatus.
func (m *Manager) ResolveAndClassifyAllFiles() (map[string]NodeStatus, error) {
	result := make(map[string]NodeStatus)
	
	// Parse rules file to get patterns
	rulesPath := filepath.Join(m.workDir, ActiveRulesFile)
	if _, err := os.Stat(rulesPath); os.IsNotExist(err) {
		// Try legacy rules file
		rulesPath = filepath.Join(m.workDir, RulesFile)
		if _, err := os.Stat(rulesPath); os.IsNotExist(err) {
			// No rules file exists
			return result, nil
		}
	}
	
	mainPatterns, coldPatterns, _, _, _, _, err := m.parseRulesFile(rulesPath)
	if err != nil {
		return nil, err
	}
	
	// Combine all patterns for classification
	allPatterns := append([]string{}, mainPatterns...)
	allPatterns = append(allPatterns, coldPatterns...)
	
	// Resolve hot context files
	hotFiles := make(map[string]bool)
	if err := m.resolveFilesIntoMap(mainPatterns, hotFiles); err != nil {
		return nil, err
	}
	
	// Resolve cold context files
	coldFiles := make(map[string]bool)
	if err := m.resolveFilesIntoMap(coldPatterns, coldFiles); err != nil {
		return nil, err
	}
	
	// Remove cold files from hot files (cold takes precedence)
	for f := range coldFiles {
		delete(hotFiles, f)
	}
	
	// Get all unique root paths to walk
	rootPaths := m.extractRootPaths(allPatterns)
	
	// Ensure the working directory itself is in the result
	result[m.workDir] = StatusDirectory
	
	// Walk each root and classify files
	for _, rootPath := range rootPaths {
		gitIgnoredFiles, err := m.getGitIgnoredFiles(rootPath)
		if err != nil {
			// Non-fatal, continue without gitignore
			gitIgnoredFiles = make(map[string]bool)
		}
		
		err = m.walkAndClassifyFiles(rootPath, allPatterns, gitIgnoredFiles, hotFiles, coldFiles, result)
		if err != nil {
			return nil, err
		}
	}
	
	// Post-process: remove empty directories (directories with no non-ignored children)
	result = m.removeEmptyDirectories(result)
	
	return result, nil
}

// removeEmptyDirectories removes directories that contain no included files
func (m *Manager) removeEmptyDirectories(fileStatuses map[string]NodeStatus) map[string]NodeStatus {
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
		if status == StatusIncludedHot || status == StatusIncludedCold || 
		   status == StatusExcludedByRule || status == StatusOmittedNoMatch {
			// This is a file with some status - mark its parent directory
			parent := filepath.Dir(path)
			markDirWithContent(parent)
		}
	}
	
	// Third pass: create the filtered result
	filtered := make(map[string]NodeStatus)
	for path, status := range fileStatuses {
		if status == StatusDirectory {
			// Only include directories that have content
			if dirsWithContent[path] {
				filtered[path] = status
			}
		} else if status != StatusIgnoredByGit {
			// Include all non-ignored files
			filtered[path] = status
		}
	}
	
	return filtered
}

// resolveFilesIntoMap is a helper that resolves patterns and adds files to the provided map
func (m *Manager) resolveFilesIntoMap(patterns []string, filesMap map[string]bool) error {
	files, err := m.resolveFilesFromPatterns(patterns)
	if err != nil {
		return err
	}
	for _, file := range files {
		filesMap[file] = true
	}
	return nil
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
			if basePath != "" && basePath != pattern {
				rootsMap[basePath] = true
			}
		} else if strings.HasPrefix(pattern, "../") {
			// For relative external paths like ../grove-flow/**/*.go
			// We need to find the first non-glob part
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

// walkAndClassifyFiles walks a directory and classifies each file based on context rules
func (m *Manager) walkAndClassifyFiles(rootPath string, patterns []string, gitIgnoredFiles, hotFiles, coldFiles map[string]bool, result map[string]NodeStatus) error {
	// Extract include patterns for classification
	var includePatterns []string
	for _, p := range patterns {
		if !strings.HasPrefix(p, "!") && p != "binary:include" && p != "!binary:exclude" {
			includePatterns = append(includePatterns, p)
		}
	}
	
	// First, ensure the root path itself is in the result as a directory
	if rootPath != m.workDir {
		// For external roots, add all parent directories up to but not including workDir
		current := rootPath
		for current != m.workDir && current != "/" && current != "." {
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
	
	return filepath.WalkDir(rootPath, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		
		// Skip git-ignored files and directories entirely
		if gitIgnoredFiles[path] {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		
		// Always skip .git and .grove directories
		if d.IsDir() && (d.Name() == ".git" || d.Name() == ".grove") {
			return filepath.SkipDir
		}
		
		// Skip the root directory itself
		if path == rootPath {
			return nil
		}
		
		// Get path relative to workDir for classification
		relPath, err := filepath.Rel(m.workDir, path)
		if err != nil {
			return err
		}
		
		// Create file key for map lookups
		fileKey := relPath
		if strings.HasPrefix(relPath, "..") {
			// File is outside workDir, use absolute path
			fileKey = path
		}
		
		// Add directories and files to the result
		if d.IsDir() {
			// Directories will be filtered later if they contain no included files
			result[path] = StatusDirectory
		} else {
			// Classify files
			if coldFiles[fileKey] {
				result[path] = StatusIncludedCold
			} else if hotFiles[fileKey] {
				result[path] = StatusIncludedHot  
			} else if m.fileMatchesAnyPattern(path, includePatterns) {
				// File matches an include pattern but isn't in the final context,
				// so it must have been excluded by a rule
				result[path] = StatusExcludedByRule
			} else {
				result[path] = StatusOmittedNoMatch
			}
		}
		
		return nil
	})
}

// fileMatchesAnyPattern checks if a file matches any of the given patterns
func (m *Manager) fileMatchesAnyPattern(filePath string, patterns []string) bool {
	for _, pattern := range patterns {
		// Get appropriate path for matching
		relPath, _ := filepath.Rel(m.workDir, filePath)
		relPath = filepath.ToSlash(relPath)
		
		if m.matchesPattern(relPath, pattern) {
			return true
		}
		
		// Also try matching against basename for patterns without slashes
		if !strings.Contains(pattern, "/") {
			if matched, _ := filepath.Match(pattern, filepath.Base(filePath)); matched {
				return true
			}
		}
	}
	return false
}

// matchesPattern checks if a path matches a single pattern
func (m *Manager) matchesPattern(path, pattern string) bool {
	// Handle ** patterns
	if strings.Contains(pattern, "**") {
		return matchDoubleStarPattern(pattern, path)
	}
	
	// Simple pattern matching
	matched, _ := filepath.Match(pattern, path)
	return matched
}

// walkAndMatchPatterns walks a directory and matches files against patterns
func (m *Manager) walkAndMatchPatterns(rootPath string, patterns []string, gitIgnoredFiles map[string]bool, uniqueFiles map[string]bool, useRelativePaths bool) error {
	// Pre-process patterns to identify directory exclusions and special flags
	dirExclusions := make(map[string]bool)
	includeBinary := false
	
	for _, pattern := range patterns {
		// Check for special pattern to include binary files
		if pattern == "!binary:exclude" || pattern == "binary:include" {
			includeBinary = true
			continue
		}
		
		if strings.HasPrefix(pattern, "!") {
			cleanPattern := strings.TrimPrefix(pattern, "!")
			cleanPattern = filepath.ToSlash(cleanPattern)
			
			// Check if this is a directory exclusion pattern without trailing slash
			// Patterns like !**/bin or !bin should exclude the directory and its contents
			if !strings.HasSuffix(cleanPattern, "/") {
				if strings.Contains(cleanPattern, "**") {
					// Extract the directory name from patterns like !**/bin
					parts := strings.Split(cleanPattern, "/")
					if len(parts) > 0 {
						dirName := parts[len(parts)-1]
						if dirName != "" && !strings.Contains(dirName, "*") && !strings.Contains(dirName, "?") {
							dirExclusions[dirName] = true
						}
					}
				} else if !strings.Contains(cleanPattern, "*") && !strings.Contains(cleanPattern, "?") {
					// Simple directory name like !bin
					dirExclusions[cleanPattern] = true
				}
			}
		}
	}

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
			
			// Check if this directory should be excluded based on pre-processed exclusions
			if dirExclusions[d.Name()] {
				// This directory matches an exclusion pattern, check if it should be skipped
				relPath, _ := filepath.Rel(rootPath, path)
				relPath = filepath.ToSlash(relPath)
				
				// Check all patterns to see if this directory is excluded
				isExcluded := false
				for _, pattern := range patterns {
					if strings.HasPrefix(pattern, "!") {
						cleanPattern := strings.TrimPrefix(pattern, "!")
						cleanPattern = filepath.ToSlash(cleanPattern)
						
						// Check various exclusion pattern formats
						if cleanPattern == d.Name() || // !bin matches bin directory
						   cleanPattern == relPath || // !path/to/bin matches specific path
						   (strings.Contains(cleanPattern, "**") && matchDoubleStarPattern(cleanPattern, relPath)) { // !**/bin matches any bin directory
							isExcluded = true
							break
						}
					}
				}
				
				if isExcluded {
					return filepath.SkipDir
				}
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

		// Skip binary files unless explicitly included
		if !includeBinary && isBinaryFile(path) {
			return nil
		}

		// --- Gitignore-style matching logic ---
		// Default to not included. A file must match an include pattern.
		isIncluded := false
		for _, pattern := range patterns {
			if pattern == "!binary:exclude" || pattern == "binary:include" {
				continue
			}
			isExclude := strings.HasPrefix(pattern, "!")
			cleanPattern := pattern
			if isExclude {
				cleanPattern = strings.TrimPrefix(pattern, "!")
			}

			match := false
			matchPath := relPath // Default path to match against (relative to walk root)

			// If pattern is absolute or starts with ../, we need to use a different path for matching.
			if filepath.IsAbs(cleanPattern) {
				matchPath = filepath.ToSlash(path) // Use the full absolute path of the file
			} else if strings.HasPrefix(cleanPattern, "../") {
				// Reconstruct path relative to workDir to give context to "../"
				relFromWorkDir, err := filepath.Rel(m.workDir, path)
				if err == nil {
					matchPath = filepath.ToSlash(relFromWorkDir)
				}
			}

			// Now perform the match using the correctly contextualized path
			if strings.Contains(cleanPattern, "**") {
				match = matchDoubleStarPattern(cleanPattern, matchPath)
			} else if matched, _ := filepath.Match(cleanPattern, matchPath); matched {
				match = true
			} else if !strings.Contains(cleanPattern, "/") { // Basename match (e.g., "*.go" or "tests")
				// Gitignore behavior: patterns without slashes match against the basename at any level
				if matched, _ := filepath.Match(cleanPattern, filepath.Base(matchPath)); matched {
					match = true
				}
				// Also check if this matches any directory component in the path
				if !match {
					parts := strings.Split(matchPath, "/")
					for _, part := range parts {
						if matched, _ := filepath.Match(cleanPattern, part); matched {
							match = true
							break
						}
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

// AppendRule adds a new rule to the active rules file.
// contextType can be "hot", "cold", or "exclude".
func (m *Manager) AppendRule(rulePath, contextType string) error {
	// Find or create the rules file
	rulesFilePath := m.findActiveRulesFile()
	if rulesFilePath == "" {
		// Create .grove/rules file
		groveDir := filepath.Join(m.workDir, GroveDir)
		if err := os.MkdirAll(groveDir, 0755); err != nil {
			return fmt.Errorf("error creating %s directory: %w", groveDir, err)
		}
		rulesFilePath = filepath.Join(m.workDir, ActiveRulesFile)
	}
	
	// Read existing content
	var lines []string
	if content, err := os.ReadFile(rulesFilePath); err == nil {
		scanner := bufio.NewScanner(strings.NewReader(string(content)))
		for scanner.Scan() {
			lines = append(lines, scanner.Text())
		}
	}
	
	// Prepare the new rule
	var newRule string
	switch contextType {
	case "exclude":
		newRule = "!" + rulePath
	default:
		newRule = rulePath
	}
	
	// Check if rule already exists
	for _, line := range lines {
		if strings.TrimSpace(line) == newRule {
			// Rule already exists, no need to add
			return nil
		}
	}
	
	// Find separator line index
	separatorIndex := -1
	for i, line := range lines {
		if strings.TrimSpace(line) == "---" {
			separatorIndex = i
			break
		}
	}
	
	// Insert the new rule based on context type
	switch contextType {
	case "hot", "exclude":
		if separatorIndex >= 0 {
			// Insert before separator
			lines = insertAt(lines, separatorIndex, newRule)
		} else {
			// No separator, append to end
			lines = append(lines, newRule)
		}
	case "cold":
		if separatorIndex >= 0 {
			// Append after separator
			lines = append(lines, newRule)
		} else {
			// No separator, add one first then the rule
			lines = append(lines, "---", newRule)
		}
	}
	
	// Write back to file
	content := strings.Join(lines, "\n")
	if len(lines) > 0 && !strings.HasSuffix(content, "\n") {
		content += "\n"
	}
	return os.WriteFile(rulesFilePath, []byte(content), 0644)
}

// insertAt inserts a string at the specified index in a slice
func insertAt(slice []string, index int, value string) []string {
	if index < 0 || index > len(slice) {
		return slice
	}
	
	result := make([]string, len(slice)+1)
	copy(result, slice[:index])
	result[index] = value
	copy(result[index+1:], slice[index:])
	return result
}

// matchDoubleStarPattern handles patterns with ** for recursive matching
func matchDoubleStarPattern(pattern, path string) bool {
	// Special case: pattern like "**/something/**" means "something" appears anywhere in path
	if strings.HasPrefix(pattern, "**/") && strings.HasSuffix(pattern, "/**") {
		middle := pattern[3:len(pattern)-3]
		// Check if middle appears as a complete path component
		pathParts := strings.Split(path, "/")
		for _, part := range pathParts {
			if matched, _ := filepath.Match(middle, part); matched {
				return true
			}
		}
		return false
	}
	
	// Split pattern at **
	parts := strings.Split(pattern, "**")
	
	if len(parts) == 2 {
		prefix := strings.TrimSuffix(parts[0], "/")
		suffix := strings.TrimPrefix(parts[1], "/")
		
		// Check prefix match
		if prefix != "" && !strings.HasPrefix(path, prefix) {
			return false
		}
		
		// Remove the prefix from the path for suffix matching
		pathAfterPrefix := path
		if prefix != "" {
			pathAfterPrefix = strings.TrimPrefix(path, prefix)
			pathAfterPrefix = strings.TrimPrefix(pathAfterPrefix, "/")
		}
		
		// Check suffix match
		if suffix != "" {
			// For patterns like "**/*.go", we need to check if the suffix matches
			// any part of the remaining path, not just the filename
			if !strings.Contains(suffix, "/") {
				// Simple suffix like "*.go" - check if the filename matches
				matched, _ := filepath.Match(suffix, filepath.Base(pathAfterPrefix))
				return matched
			} else {
				// Complex suffix with directory components
				// For example, "foo/*.go" should match "bar/baz/foo/test.go"
				// The ** means we need to try matching the suffix at all possible positions
				
				suffixParts := strings.Split(suffix, "/")
				pathParts := strings.Split(pathAfterPrefix, "/")
				
				// Try to match suffix against all possible positions in the path
				for i := 0; i <= len(pathParts)-len(suffixParts); i++ {
					match := true
					for j := 0; j < len(suffixParts); j++ {
						if matched, _ := filepath.Match(suffixParts[j], pathParts[i+j]); !matched {
							match = false
							break
						}
					}
					if match {
						return true
					}
				}
				return false
			}
		}
		
		// If only prefix is specified (or no suffix), it matches
		return true
	}
	
	// Handle multiple ** in pattern or patterns without **
	matched, _ := filepath.Match(pattern, path)
	return matched
}

// Common binary file extensions - defined at package level for performance
var binaryExtensions = map[string]bool{
	".exe": true, ".dll": true, ".so": true, ".dylib": true, ".a": true,
	".o": true, ".obj": true, ".lib": true, ".bin": true, ".dat": true,
	".db": true, ".sqlite": true, ".sqlite3": true,
	".jpg": true, ".jpeg": true, ".png": true, ".gif": true, ".bmp": true,
	".ico": true, ".tiff": true, ".webp": true,
	".mp3": true, ".mp4": true, ".avi": true, ".mov": true, ".wmv": true,
	".flv": true, ".webm": true, ".m4a": true, ".flac": true, ".wav": true,
	".zip": true, ".tar": true, ".gz": true, ".bz2": true, ".xz": true,
	".7z": true, ".rar": true, ".deb": true, ".rpm": true,
	".dmg": true, ".pkg": true, ".msi": true,
	".pdf": true, ".doc": true, ".docx": true, ".xls": true, ".xlsx": true,
	".ppt": true, ".pptx": true, ".odt": true, ".ods": true, ".odp": true,
	".pyc": true, ".pyo": true, ".class": true, ".jar": true, ".war": true,
	".woff": true, ".woff2": true, ".ttf": true, ".otf": true, ".eot": true,
	".wasm": true, ".node": true,
}

// isBinaryFile detects if a file is likely binary by checking the first 512 bytes
func isBinaryFile(path string) bool {
	// Check common binary file extensions first for performance
	ext := strings.ToLower(filepath.Ext(path))
	
	// If it's a known binary extension, return true immediately
	if binaryExtensions[ext] {
		return true
	}
	
	// If file has an extension, assume it's not binary (unless in binaryExtensions)
	// This avoids checking content for most source code files
	if ext != "" {
		return false
	}
	
	// Check for common text files without extensions
	basename := filepath.Base(path)
	commonTextFiles := map[string]bool{
		"Makefile": true, "makefile": true, "GNUmakefile": true,
		"Dockerfile": true, "dockerfile": true,
		"README": true, "LICENSE": true, "CHANGELOG": true,
		"AUTHORS": true, "CONTRIBUTORS": true, "PATENTS": true,
		"VERSION": true, "TODO": true, "NOTICE": true,
		"Jenkinsfile": true, "Rakefile": true, "Gemfile": true,
		"Vagrantfile": true, "Brewfile": true, "Podfile": true,
		"gradlew": true, "mvnw": true,
	}
	
	if commonTextFiles[basename] {
		return false
	}
	
	// Only check content for files without extensions (like Go binaries)
	file, err := os.Open(path)
	if err != nil {
		return false
	}
	defer file.Close()
	
	// Read first 512 bytes to check for binary content
	buffer := make([]byte, 512)
	n, err := file.Read(buffer)
	if err != nil && err != io.EOF {
		return false
	}
	
	// Check for common binary file signatures
	if n >= 4 {
		// ELF header (Linux/Unix executables including Go binaries)
		if buffer[0] == 0x7f && buffer[1] == 'E' && buffer[2] == 'L' && buffer[3] == 'F' {
			return true
		}
		// Mach-O header (macOS executables including Go binaries)
		if (buffer[0] == 0xfe && buffer[1] == 0xed && buffer[2] == 0xfa && (buffer[3] == 0xce || buffer[3] == 0xcf)) ||
		   (buffer[0] == 0xce && buffer[1] == 0xfa && buffer[2] == 0xed && buffer[3] == 0xfe) ||
		   (buffer[0] == 0xcf && buffer[1] == 0xfa && buffer[2] == 0xed && buffer[3] == 0xfe) {
			return true
		}
		// PE header (Windows executables)
		if buffer[0] == 'M' && buffer[1] == 'Z' {
			return true
		}
	}
	
	// Quick check for null bytes (strong indicator of binary)
	for i := 0; i < n; i++ {
		if buffer[i] == 0 {
			return true
		}
	}
	
	// Count non-text characters
	nonTextCount := 0
	for i := 0; i < n; i++ {
		b := buffer[i]
		// Count non-printable characters (excluding common whitespace)
		if b < 32 && b != '\t' && b != '\n' && b != '\r' {
			nonTextCount++
		}
	}
	
	// If more than 30% of characters are non-text, consider it binary
	if n > 0 && float64(nonTextCount)/float64(n) > 0.3 {
		return true
	}
	
	return false
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