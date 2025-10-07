package context

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
	
	"github.com/mattsolo1/grove-core/config"
	"github.com/mattsolo1/grove-core/pkg/repo"
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
	workDir         string
	gitIgnoredCache map[string]map[string]bool // Cache for gitignored files by repository root
	aliasResolver   *AliasResolver              // Lazily initialized alias resolver
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

// LoadDefaultRulesContent loads only the default rules from grove.yml, ignoring any local rules files.
// It returns the default rules content and the path where rules should be written.
func (m *Manager) LoadDefaultRulesContent() (content []byte, rulesPath string) {
	rulesPath = filepath.Join(m.workDir, ActiveRulesFile)
	
	// Load grove.yml to check for default rules
	cfg, err := config.LoadFrom(m.workDir)
	if err != nil || cfg == nil {
		// No config, so no default rules
		return nil, rulesPath
	}

	// Use custom extension approach since the Context field may not exist in grove-core yet
	var contextConfig struct {
		DefaultRulesPath string `yaml:"default_rules_path"`
	}
	
	if err := cfg.UnmarshalExtension("context", &contextConfig); err != nil {
		// Extension doesn't exist or failed to unmarshal, no default rules
		return nil, rulesPath
	}

	if contextConfig.DefaultRulesPath != "" {
		// Project root is where grove.yml is found
		configPath, _ := config.FindConfigFile(m.workDir)
		projectRoot := filepath.Dir(configPath)
		if projectRoot == "" {
			projectRoot = m.workDir
		}
		defaultRulesPath := filepath.Join(projectRoot, contextConfig.DefaultRulesPath)

		content, err := os.ReadFile(defaultRulesPath)
		if err != nil {
			// Don't error out, just warn and act as if no default exists
			fmt.Fprintf(os.Stderr, "Warning: could not read default_rules_path %s: %v\n", defaultRulesPath, err)
			return nil, rulesPath
		}
		return content, rulesPath
	}

	return nil, rulesPath
}

// LoadRulesContent finds and reads the active rules file, falling back to grove.yml defaults.
// It returns the content of the rules, the path of the file read (if any), and an error.
func (m *Manager) LoadRulesContent() (content []byte, path string, err error) {
	// 1. Look for local .grove/rules
	localRulesPath := filepath.Join(m.workDir, ActiveRulesFile)
	if _, err := os.Stat(localRulesPath); err == nil {
		content, err := os.ReadFile(localRulesPath)
		if err != nil {
			return nil, "", fmt.Errorf("reading local rules file %s: %w", localRulesPath, err)
		}
		return content, localRulesPath, nil
	}

	// 2. If not found, look for legacy .grovectx
	legacyRulesPath := filepath.Join(m.workDir, RulesFile)
	if _, err := os.Stat(legacyRulesPath); err == nil {
		content, err := os.ReadFile(legacyRulesPath)
		if err != nil {
			return nil, "", fmt.Errorf("reading legacy rules file %s: %w", legacyRulesPath, err)
		}
		return content, legacyRulesPath, nil
	}

	// 3. If not found, check grove.yml for a default
	cfg, err := config.LoadFrom(m.workDir)
	if err != nil || cfg == nil {
		// No config, so no default rules
		return nil, "", nil
	}

	// Note: For now, we'll use a custom extension approach since the Context field 
	// may not exist in grove-core yet
	var contextConfig struct {
		DefaultRulesPath string `yaml:"default_rules_path"`
	}
	
	if err := cfg.UnmarshalExtension("context", &contextConfig); err != nil {
		// Extension doesn't exist or failed to unmarshal, no default rules
		return nil, "", nil
	}

	if contextConfig.DefaultRulesPath != "" {
		// Project root is where grove.yml is found
		configPath, _ := config.FindConfigFile(m.workDir)
		projectRoot := filepath.Dir(configPath)
		if projectRoot == "" {
			projectRoot = m.workDir
		}
		defaultRulesPath := filepath.Join(projectRoot, contextConfig.DefaultRulesPath)

		content, err := os.ReadFile(defaultRulesPath)
		if err != nil {
			// Don't error out, just warn and act as if no default exists
			fmt.Fprintf(os.Stderr, "Warning: could not read default_rules_path %s: %v\n", defaultRulesPath, err)
			return nil, "", nil
		}
		// Return the content, but the path is the *local* path where it *should* be written.
		return content, localRulesPath, nil
	}

	// 4. No local or default rules found
	return nil, "", nil
}

// resolveFilesFromRulesContent resolves files based on rules content provided as a byte slice.
func (m *Manager) resolveFilesFromRulesContent(rulesContent []byte) ([]string, error) {
	// Parse the rules content directly without recursion for this case
	// This is used by commands that provide rules content directly (not from a file)
	mainPatterns, coldPatterns, _, _, _, _, _, _, _, err := m.parseRulesFile(rulesContent)
	if err != nil {
		return nil, fmt.Errorf("error parsing rules content: %w", err)
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

// SetActiveRules sets the active rules file (.grove/rules) from a source file.
func (m *Manager) SetActiveRules(sourcePath string) error {
	// Check if source file exists
	if _, err := os.Stat(sourcePath); os.IsNotExist(err) {
		return fmt.Errorf("source rules file not found: %s", sourcePath)
	}

	// Read content from source
	content, err := os.ReadFile(sourcePath)
	if err != nil {
		return fmt.Errorf("error reading source rules file %s: %w", sourcePath, err)
	}

	// Ensure .grove directory exists
	groveDir := filepath.Join(m.workDir, GroveDir)
	if err := os.MkdirAll(groveDir, 0755); err != nil {
		return fmt.Errorf("error creating %s directory: %w", groveDir, err)
	}

	// Write to active rules file, overwriting if it exists
	activeRulesPath := filepath.Join(m.workDir, ActiveRulesFile)
	if err := os.WriteFile(activeRulesPath, content, 0644); err != nil {
		return fmt.Errorf("error writing active rules file: %w", err)
	}

	fmt.Printf("Set active rules from %s\n", sourcePath)
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

// GenerateContextFromRulesFile generates context from an explicit rules file path.
func (m *Manager) GenerateContextFromRulesFile(rulesFilePath string, useXMLFormat bool) error {
	// Ensure .grove directory exists for output files
	groveDir := filepath.Join(m.workDir, GroveDir)
	if err := os.MkdirAll(groveDir, 0755); err != nil {
		return fmt.Errorf("error creating %s directory: %w", groveDir, err)
	}

	absRulesFilePath, err := filepath.Abs(rulesFilePath)
	if err != nil {
		return fmt.Errorf("failed to get absolute path for rules file: %w", err)
	}

	// Read and display the rules file content
	rulesContent, err := os.ReadFile(absRulesFilePath)
	if err != nil {
		return fmt.Errorf("failed to read rules file: %w", err)
	}

	// Print rules file info to stderr
	fmt.Fprintf(os.Stderr, "📋 Using rules file: %s\n", absRulesFilePath)
	content := strings.TrimSpace(string(rulesContent))
	if content != "" {
		lines := strings.Split(content, "\n")
		fmt.Fprintf(os.Stderr, "📋 Rules content (%d lines):\n", len(lines))

		// Display with indentation, limit to first 10 lines
		maxLines := 10
		displayLines := lines
		if len(lines) > maxLines {
			displayLines = lines[:maxLines]
		}

		for _, line := range displayLines {
			fmt.Fprintf(os.Stderr, "   %s\n", line)
		}

		if len(lines) > maxLines {
			fmt.Fprintf(os.Stderr, "   ... (%d more lines)\n", len(lines)-maxLines)
		}
		fmt.Fprintln(os.Stderr)
	}
	
	hotPatterns, coldPatterns, _, err := m.resolveAllPatterns(absRulesFilePath, make(map[string]bool))
	if err != nil {
		return fmt.Errorf("failed to resolve patterns from rules file %s: %w", rulesFilePath, err)
	}
	
	hotFiles, err := m.resolveFilesFromPatterns(hotPatterns)
	if err != nil {
		return fmt.Errorf("error resolving hot context files: %w", err)
	}
	
	coldFiles, err := m.resolveFilesFromPatterns(coldPatterns)
	if err != nil {
		return fmt.Errorf("error resolving cold context files: %w", err)
	}
	
	// Cold-over-hot precedence
	coldFilesMap := make(map[string]bool)
	for _, file := range coldFiles {
		coldFilesMap[file] = true
	}
	
	var finalHotFiles []string
	for _, file := range hotFiles {
		if !coldFilesMap[file] {
			finalHotFiles = append(finalHotFiles, file)
		}
	}

	// Generate context files
	if err := m.generateContextFromFiles(finalHotFiles, useXMLFormat); err != nil {
		return err
	}
	
	if err := m.generateCachedContextFromFiles(coldFiles); err != nil {
		return err
	}
	
	return nil
}

// GenerateContext creates the context file from the files list
func (m *Manager) GenerateContext(useXMLFormat bool) error {
	// Ensure .grove directory exists
	groveDir := filepath.Join(m.workDir, GroveDir)
	if err := os.MkdirAll(groveDir, 0755); err != nil {
		return fmt.Errorf("error creating %s directory: %w", groveDir, err)
	}
	
	// Use ResolveFilesFromRules which handles @default directives
	filesToInclude, err := m.ResolveFilesFromRules()
	if err != nil {
		return fmt.Errorf("error resolving files from rules: %w", err)
	}
	
	// Handle case where no rules file exists
	if len(filesToInclude) == 0 {
		rulesContent, _, _ := m.LoadRulesContent()
		if rulesContent == nil {
			// Print visible warning to stderr
			fmt.Fprintf(os.Stderr, "\n⚠️  WARNING: No rules file found!\n")
			fmt.Fprintf(os.Stderr, "⚠️  Create %s with patterns to include files in the context.\n", ActiveRulesFile)
			fmt.Fprintf(os.Stderr, "⚠️  Generating empty context file.\n\n")
		}
	}
	
	return m.generateContextFromFiles(filesToInclude, useXMLFormat)
}

// generateContextFromFiles is a private helper that writes a list of files to the hot context file.
func (m *Manager) generateContextFromFiles(files []string, useXMLFormat bool) error {
	// Create context file
	contextPath := filepath.Join(m.workDir, ContextFile)
	ctxFile, err := os.Create(contextPath)
	if err != nil {
		return fmt.Errorf("error creating %s: %w", contextPath, err)
	}
	defer ctxFile.Close()
	
	// Write XML header and opening tags if using XML format
	if useXMLFormat {
		fmt.Fprintf(ctxFile, "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n")
		fmt.Fprintf(ctxFile, "<context>\n")
		fmt.Fprintf(ctxFile, "  <hot-context files=\"%d\" description=\"Files to be used for reference/background context to carry out the user's question/task to be provided later\">\n", len(files))
	}
	
	// If no files to include, write a comment explaining why
	if len(files) == 0 {
		if useXMLFormat {
			fmt.Fprintf(ctxFile, "    <!-- No rules file found. Create %s with patterns to include files. -->\n", ActiveRulesFile)
		} else {
			fmt.Fprintf(ctxFile, "# No rules file found. Create %s with patterns to include files.\n", ActiveRulesFile)
		}
	}
	
	// Write concatenated content
	for _, file := range files {
		if useXMLFormat {
			// Use the existing writeFileToXML method for consistency
			if err := m.writeFileToXML(ctxFile, file, "    "); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: error writing file %s: %v\n", file, err)
			}
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
	
	// Close XML tags if using XML format
	if useXMLFormat {
		fmt.Fprintf(ctxFile, "  </hot-context>\n")
		fmt.Fprintf(ctxFile, "</context>\n")
	}
	
	fmt.Printf("Generated %s with %d files\n", ContextFile, len(files))
	
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
	
	return m.generateCachedContextFromFiles(coldFiles)
}

// generateCachedContextFromFiles is a private helper that writes a list of files to the cold context files.
func (m *Manager) generateCachedContextFromFiles(coldFiles []string) error {
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

// parseRulesFile reads rules content and separates patterns into main and cold contexts.
func (m *Manager) parseRulesFile(rulesContent []byte) (mainPatterns, coldPatterns, mainDefaultPaths, coldDefaultPaths, viewPaths []string, freezeCache, disableExpiration, disableCache bool, expireTime time.Duration, err error) {
	if len(rulesContent) == 0 {
		return nil, nil, nil, nil, nil, false, false, false, 0, nil
	}

	// Create repo manager for processing Git URLs
	repoManager, repoErr := repo.NewManager()
	if repoErr != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not create repository manager: %v\n", repoErr)
	}

	// Initialize alias resolver for @alias: directives
	resolver := m.getAliasResolver()

	inColdSection := false
	scanner := bufio.NewScanner(bytes.NewReader(rulesContent))
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
			durationStr := strings.TrimSpace(strings.TrimPrefix(line, "@expire-time "))
			if durationStr != "" {
				parsedDuration, parseErr := time.ParseDuration(durationStr)
				if parseErr != nil {
					return nil, nil, nil, nil, nil, false, false, false, 0, fmt.Errorf("invalid duration format for @expire-time: %w", parseErr)
				}
				expireTime = parsedDuration
			}
			continue
		}
		// Support both @view: and @v: (short form)
		if strings.HasPrefix(line, "@view:") || strings.HasPrefix(line, "@v:") {
			// Normalize to @view: for processing
			normalizedLine := line
			if strings.HasPrefix(line, "@v:") {
				normalizedLine = "@view:" + strings.TrimPrefix(line, "@v:")
			}

			path := strings.TrimSpace(strings.TrimPrefix(normalizedLine, "@view:"))
			if path != "" {
				// Resolve alias if present in @view: directive (supports @alias: and @a:)
				if resolver != nil && (strings.Contains(path, "@alias:") || strings.Contains(path, "@a:")) {
					resolvedLine, resolveErr := resolver.ResolveLine(normalizedLine)
					if resolveErr != nil {
						fmt.Fprintf(os.Stderr, "Warning: could not resolve alias in @view line '%s': %v\n", line, resolveErr)
						continue // Skip this line if alias resolution fails
					}
					// Extract the path from the resolved line (removing @view: prefix)
					path = strings.TrimSpace(strings.TrimPrefix(resolvedLine, "@view:"))
				}
				viewPaths = append(viewPaths, path)
			}
			continue
		}
		if strings.HasPrefix(line, "@default:") {
			path := strings.TrimSpace(strings.TrimPrefix(line, "@default:"))
			if path != "" {
				if inColdSection {
					coldDefaultPaths = append(coldDefaultPaths, path)
				} else {
					mainDefaultPaths = append(mainDefaultPaths, path)
				}
			}
			continue
		}
		if line == "---" {
			inColdSection = true
			continue
		}
		if line != "" && !strings.HasPrefix(line, "#") {
			// Resolve alias if present (supports both @alias: and @a:), before further processing
			processedLine := line
			if resolver != nil && (strings.Contains(line, "@alias:") || strings.Contains(line, "@a:")) {
				resolvedLine, resolveErr := resolver.ResolveLine(line)
				if resolveErr != nil {
					fmt.Fprintf(os.Stderr, "Warning: could not resolve alias in line '%s': %v\n", line, resolveErr)
					continue // Skip this line if alias resolution fails
				}
				processedLine = resolvedLine
			}

			// Process Git URLs
			if repoManager != nil {
				isExclude := strings.HasPrefix(processedLine, "!")
				cleanLine := processedLine
				if isExclude {
					cleanLine = strings.TrimPrefix(processedLine, "!")
				}
				
				if isGitURL, repoURL, version := m.parseGitRule(cleanLine); isGitURL {
					// Clone/update the repository
					localPath, _, cloneErr := repoManager.Ensure(repoURL, version)
					if cloneErr != nil {
						fmt.Fprintf(os.Stderr, "Warning: could not clone repository %s: %v\n", repoURL, cloneErr)
						continue
					}
					
					// Replace the Git URL with the local path pattern
					processedLine = localPath + "/**"
					if isExclude {
						processedLine = "!" + processedLine
					}
				}
			}
			
			if inColdSection {
				coldPatterns = append(coldPatterns, processedLine)
			} else {
				mainPatterns = append(mainPatterns, processedLine)
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, nil, nil, nil, nil, false, false, false, 0, err
	}
	return mainPatterns, coldPatterns, mainDefaultPaths, coldDefaultPaths, viewPaths, freezeCache, disableExpiration, disableCache, expireTime, nil
}

// ShouldFreezeCache checks if the @freeze-cache directive is present in the rules file.
func (m *Manager) ShouldFreezeCache() (bool, error) {
	rulesContent, _, err := m.LoadRulesContent()
	if err != nil {
		return false, err
	}
	if rulesContent == nil {
		return false, nil
	}
	_, _, _, _, _, freezeCache, _, _, _, err := m.parseRulesFile(rulesContent)
	if err != nil {
		return false, fmt.Errorf("error parsing rules file for cache directive: %w", err)
	}

	return freezeCache, nil
}

// ShouldDisableExpiration checks if the @no-expire directive is present in the rules file.
func (m *Manager) ShouldDisableExpiration() (bool, error) {
	rulesContent, _, err := m.LoadRulesContent()
	if err != nil {
		return false, err
	}
	if rulesContent == nil {
		return false, nil
	}
	_, _, _, _, _, _, disableExpiration, _, _, err := m.parseRulesFile(rulesContent)
	if err != nil {
		return false, fmt.Errorf("error parsing rules file for cache directive: %w", err)
	}

	return disableExpiration, nil
}

// GetExpireTime returns the custom expiration duration if @expire-time directive is present.
// Returns 0 if no custom expiration time is set.
func (m *Manager) GetExpireTime() (time.Duration, error) {
	rulesContent, _, err := m.LoadRulesContent()
	if err != nil {
		return 0, err
	}
	if rulesContent == nil {
		return 0, nil
	}
	_, _, _, _, _, _, _, _, expireTime, err := m.parseRulesFile(rulesContent)
	if err != nil {
		return 0, fmt.Errorf("error parsing rules file for expire time: %w", err)
	}

	return expireTime, nil
}

// ShouldDisableCache checks if the @disable-cache directive is present in the rules file.
func (m *Manager) ShouldDisableCache() (bool, error) {
	rulesContent, _, err := m.LoadRulesContent()
	if err != nil {
		return false, err
	}
	if rulesContent == nil {
		return false, nil
	}
	_, _, _, _, _, _, _, disableCache, _, err := m.parseRulesFile(rulesContent)
	if err != nil {
		return false, fmt.Errorf("error parsing rules file for cache directive: %w", err)
	}

	return disableCache, nil
}

// resolveAllPatterns recursively resolves rules, including those from @default directives.
func (m *Manager) resolveAllPatterns(rulesPath string, visited map[string]bool) (hotPatterns, coldPatterns, viewPaths []string, err error) {
	absRulesPath, err := filepath.Abs(rulesPath)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to get absolute path for rules: %w", err)
	}

	if visited[absRulesPath] {
		// Circular dependency detected, return to prevent infinite loop.
		return nil, nil, nil, nil
	}
	visited[absRulesPath] = true

	rulesContent, err := os.ReadFile(absRulesPath)
	if err != nil {
		if os.IsNotExist(err) {
			// If a default rules file doesn't exist, it's not an error, just return empty.
			return nil, nil, nil, nil
		}
		return nil, nil, nil, fmt.Errorf("reading rules file %s: %w", absRulesPath, err)
	}

	localHot, localCold, mainDefaults, coldDefaults, localView, _, _, _, _, err := m.parseRulesFile(rulesContent)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("parsing rules file %s: %w", absRulesPath, err)
	}

	hotPatterns = append(hotPatterns, localHot...)
	coldPatterns = append(coldPatterns, localCold...)
	viewPaths = append(viewPaths, localView...)
	
	rulesDir := filepath.Dir(absRulesPath)

	// Process hot defaults
	for _, defaultPath := range mainDefaults {
		resolvedPath := defaultPath
		if !filepath.IsAbs(resolvedPath) {
			resolvedPath = filepath.Join(rulesDir, resolvedPath)
		}
		
		// First resolve the real path
		realPath, err := filepath.EvalSymlinks(resolvedPath)
		if err != nil {
			realPath = resolvedPath
		}
		
		// Load the config directly from the grove.yml file in that directory
		configFile := filepath.Join(realPath, "grove.yml")
		if _, err := os.Stat(configFile); os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "Warning: no grove.yml found at %s for @default path %s\n", configFile, defaultPath)
			continue
		}
		
		cfg, err := config.Load(configFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: could not load config for @default path %s (file: %s): %v\n", defaultPath, configFile, err)
			continue
		}
		
		var contextConfig struct {
			DefaultRulesPath string `yaml:"default_rules_path"`
		}
		if err := cfg.UnmarshalExtension("context", &contextConfig); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to unmarshal context extension for @default path %s: %v\n", defaultPath, err)
			continue
		}
		if contextConfig.DefaultRulesPath == "" {
			fmt.Fprintf(os.Stderr, "Warning: no default_rules_path found for @default path %s\n", defaultPath)
			continue
		}
		
		defaultRulesFile := filepath.Join(realPath, contextConfig.DefaultRulesPath)

		// Recursively resolve patterns from the default rules file
		// ALL patterns from the default (hot and cold) are added to the current HOT context.
		nestedHot, nestedCold, nestedView, err := m.resolveAllPatterns(defaultRulesFile, visited)
		if err != nil {
			return nil, nil, nil, err
		}
		// The patterns from external project need to be prefixed with the project path
		// so they resolve files from that project, not the current one
		for i, pattern := range nestedHot {
			if !strings.HasPrefix(pattern, "!") && !filepath.IsAbs(pattern) {
				nestedHot[i] = filepath.Join(realPath, pattern)
			} else if strings.HasPrefix(pattern, "!") && !filepath.IsAbs(pattern[1:]) {
				nestedHot[i] = "!" + filepath.Join(realPath, pattern[1:])
			}
		}
		for i, pattern := range nestedCold {
			if !strings.HasPrefix(pattern, "!") && !filepath.IsAbs(pattern) {
				nestedCold[i] = filepath.Join(realPath, pattern)
			} else if strings.HasPrefix(pattern, "!") && !filepath.IsAbs(pattern[1:]) {
				nestedCold[i] = "!" + filepath.Join(realPath, pattern[1:])
			}
		}
		hotPatterns = append(hotPatterns, nestedHot...)
		hotPatterns = append(hotPatterns, nestedCold...)
		
		// Add view paths from nested rules, adjusting relative paths
		for i, path := range nestedView {
			if !filepath.IsAbs(path) {
				nestedView[i] = filepath.Join(realPath, path)
			}
		}
		viewPaths = append(viewPaths, nestedView...)
	}

	// Process cold defaults
	for _, defaultPath := range coldDefaults {
		resolvedPath := defaultPath
		if !filepath.IsAbs(resolvedPath) {
			resolvedPath = filepath.Join(rulesDir, resolvedPath)
		}

		// First resolve the real path
		realPath, err := filepath.EvalSymlinks(resolvedPath)
		if err != nil {
			realPath = resolvedPath
		}
		
		// Load the config directly from the grove.yml file in that directory
		configFile := filepath.Join(realPath, "grove.yml")
		if _, err := os.Stat(configFile); os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "Warning: no grove.yml found at %s for @default path %s\n", configFile, defaultPath)
			continue
		}
		
		cfg, err := config.Load(configFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: could not load config for @default path %s (file: %s): %v\n", defaultPath, configFile, err)
			continue
		}

		var contextConfig struct {
			DefaultRulesPath string `yaml:"default_rules_path"`
		}
		if err := cfg.UnmarshalExtension("context", &contextConfig); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to unmarshal context extension for @default path %s: %v\n", defaultPath, err)
			continue
		}
		if contextConfig.DefaultRulesPath == "" {
			fmt.Fprintf(os.Stderr, "Warning: no default_rules_path found for @default path %s\n", defaultPath)
			continue
		}

		defaultRulesFile := filepath.Join(realPath, contextConfig.DefaultRulesPath)

		// Recursively resolve patterns from the default rules file
		// ALL patterns from the default are added to the current COLD context.
		nestedHot, nestedCold, nestedView, err := m.resolveAllPatterns(defaultRulesFile, visited)
		if err != nil {
			return nil, nil, nil, err
		}
		// The patterns from external project need to be prefixed with the project path
		for i, pattern := range nestedHot {
			if !strings.HasPrefix(pattern, "!") && !filepath.IsAbs(pattern) {
				nestedHot[i] = filepath.Join(realPath, pattern)
			} else if strings.HasPrefix(pattern, "!") && !filepath.IsAbs(pattern[1:]) {
				nestedHot[i] = "!" + filepath.Join(realPath, pattern[1:])
			}
		}
		for i, pattern := range nestedCold {
			if !strings.HasPrefix(pattern, "!") && !filepath.IsAbs(pattern) {
				nestedCold[i] = filepath.Join(realPath, pattern)
			} else if strings.HasPrefix(pattern, "!") && !filepath.IsAbs(pattern[1:]) {
				nestedCold[i] = "!" + filepath.Join(realPath, pattern[1:])
			}
		}
		coldPatterns = append(coldPatterns, nestedHot...)
		coldPatterns = append(coldPatterns, nestedCold...)
		
		// Add view paths from nested rules, adjusting relative paths
		for i, path := range nestedView {
			if !filepath.IsAbs(path) {
				nestedView[i] = filepath.Join(realPath, path)
			}
		}
		viewPaths = append(viewPaths, nestedView...)
	}

	return hotPatterns, coldPatterns, viewPaths, nil
}

// ResolveFilesFromRules dynamically resolves the list of files from the active rules file
func (m *Manager) ResolveFilesFromRules() ([]string, error) {
	// Find the active rules file to start the recursive resolution
	activeRulesFile := m.findActiveRulesFile()
	if activeRulesFile == "" {
		// If no rules file, check for defaults configured in grove.yml
		defaultContent, _ := m.LoadDefaultRulesContent()
		if defaultContent != nil {
			// Use the non-recursive content-based resolver
			return m.resolveFilesFromRulesContent(defaultContent)
		}
		// No active or default rules found
		return []string{}, nil
	}

	// Resolve all patterns recursively from the active rules file
	hotPatterns, coldPatterns, _, err := m.resolveAllPatterns(activeRulesFile, make(map[string]bool))
	if err != nil {
		return nil, fmt.Errorf("failed to resolve patterns: %w", err)
	}

	// Resolve files from hot patterns
	hotFiles, err := m.resolveFilesFromPatterns(hotPatterns)
	if err != nil {
		return nil, fmt.Errorf("error resolving hot context files: %w", err)
	}

	// Only resolve and filter cold patterns if there are any
	if len(coldPatterns) > 0 {
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

		// Filter main files to exclude any that are also in cold files
		var finalHotFiles []string
		for _, file := range hotFiles {
			if !coldFilesMap[file] {
				finalHotFiles = append(finalHotFiles, file)
			}
		}

		return finalHotFiles, nil
	}

	// No cold patterns, return hot files as is
	return hotFiles, nil
}

// ResolveFilesFromCustomRulesFile resolves both hot and cold files from a custom rules file path.
func (m *Manager) ResolveFilesFromCustomRulesFile(rulesFilePath string) (hotFiles []string, coldFiles []string, err error) {
	// Get absolute path for the rules file
	absRulesFilePath, err := filepath.Abs(rulesFilePath)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get absolute path for rules file: %w", err)
	}

	// Check if the rules file exists
	if _, err := os.Stat(absRulesFilePath); os.IsNotExist(err) {
		return nil, nil, fmt.Errorf("rules file not found: %s", absRulesFilePath)
	}

	// Resolve all patterns recursively from the custom rules file
	hotPatterns, coldPatterns, _, err := m.resolveAllPatterns(absRulesFilePath, make(map[string]bool))
	if err != nil {
		return nil, nil, fmt.Errorf("failed to resolve patterns from rules file: %w", err)
	}

	// Resolve files from hot patterns
	hotFiles, err = m.resolveFilesFromPatterns(hotPatterns)
	if err != nil {
		return nil, nil, fmt.Errorf("error resolving hot context files: %w", err)
	}

	// Resolve files from cold patterns
	if len(coldPatterns) > 0 {
		coldFiles, err = m.resolveFilesFromPatterns(coldPatterns)
		if err != nil {
			return nil, nil, fmt.Errorf("error resolving cold context files: %w", err)
		}

		// Apply cold-over-hot precedence: remove hot files that are also in cold
		coldFilesMap := make(map[string]bool)
		for _, file := range coldFiles {
			coldFilesMap[file] = true
		}

		var finalHotFiles []string
		for _, file := range hotFiles {
			if !coldFilesMap[file] {
				finalHotFiles = append(finalHotFiles, file)
			}
		}
		hotFiles = finalHotFiles
	}

	return hotFiles, coldFiles, nil
}

// ResolveColdContextFiles resolves the list of files from the "cold" section of a rules file.
func (m *Manager) ResolveColdContextFiles() ([]string, error) {
	// Use the centralized engine
	fileStatuses, err := m.ResolveAndClassifyAllFiles(false)
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

// preProcessPatterns transforms plain directory patterns into recursive globs.
func (m *Manager) preProcessPatterns(patterns []string) []string {
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
	return processedPatterns
}

// resolveFilesFromPatterns resolves files from a given set of patterns
func (m *Manager) resolveFilesFromPatterns(patterns []string) ([]string, error) {
	if len(patterns) == 0 {
		return []string{}, nil
	}
	
	// Use processed patterns for the rest of the logic
	patterns = m.preProcessPatterns(patterns)
	
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
				if info, err := os.Stat(basePath); err == nil {
					if info.IsDir() {
						// It's a directory, use as is
					} else {
						// It's a file, use its directory for walking
						basePath = filepath.Dir(basePath)
					}
				} else {
					// Non-existent path - could be a file pattern that doesn't exist yet
					// Use directory part for walking
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

		// Adjust patterns to be relative to the absPath we're walking
		adjustedPatterns := make([]string, 0, len(pathPatterns))
		for _, pattern := range pathPatterns {
			isGlob := strings.ContainsAny(pattern, "*?")
			
			// For patterns that start with the absPath we're walking, make them relative
			if strings.HasPrefix(pattern, absPath) {
				// Remove the absPath prefix to make the pattern relative
				relPattern := strings.TrimPrefix(pattern, absPath)
				relPattern = strings.TrimPrefix(relPattern, "/")
				if relPattern == "" {
					relPattern = "**" // If the pattern was just the directory itself, match everything
				}
				adjustedPatterns = append(adjustedPatterns, relPattern)
			} else if !isGlob && filepath.IsAbs(pattern) {
				// For absolute file paths that don't start with absPath, keep them absolute
				adjustedPatterns = append(adjustedPatterns, pattern)
			} else {
				// Keep the pattern as-is if it doesn't start with absPath
				adjustedPatterns = append(adjustedPatterns, pattern)
			}
		}
		
		// Walk the path and apply its patterns and gitignore rules.
		err = m.walkAndMatchPatterns(absPath, adjustedPatterns, gitIgnoredForAbsPath, uniqueFiles, false)
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
	// Read the rules file
	rulesContent, err := os.ReadFile(rulesPath)
	if err != nil {
		return nil, fmt.Errorf("error reading rules file: %w", err)
	}
	
	// Parse the rules content to get main and cold patterns
	mainPatterns, coldPatterns, _, _, _, _, _, _, _, err := m.parseRulesFile(rulesContent)
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
func (m *Manager) ResolveAndClassifyAllFiles(prune bool) (map[string]NodeStatus, error) {
	result := make(map[string]NodeStatus)
	
	// Find the active rules file to start the recursive resolution
	activeRulesFile := m.findActiveRulesFile()
	if activeRulesFile == "" {
		// If no rules file, check for defaults configured in grove.yml
		_, defaultRulesFile := m.LoadDefaultRulesContent()
		if _, err := os.Stat(defaultRulesFile); !os.IsNotExist(err) {
			activeRulesFile = defaultRulesFile
		} else {
			// No active or default rules found
			return result, nil
		}
	}

	hotPatterns, coldPatterns, viewPaths, err := m.resolveAllPatterns(activeRulesFile, make(map[string]bool))
	if err != nil {
		return nil, fmt.Errorf("failed to resolve all patterns: %w", err)
	}
	mainPatterns := hotPatterns
	
	// Combine all patterns for classification
	allPatterns := append([]string{}, mainPatterns...)
	allPatterns = append(allPatterns, coldPatterns...)
	
	// Pre-process patterns to ensure plain directories are handled as recursive globs.
	// This is critical for `extractRootPaths` to identify absolute directories.
	allPatterns = m.preProcessPatterns(allPatterns)
	
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
	
	// Get all unique root paths to walk from patterns
	rootPaths := m.extractRootPaths(allPatterns)
	
	// Add paths from @view directives to the list of roots to walk
	for _, vp := range viewPaths {
		var absPath string
		if filepath.IsAbs(vp) {
			absPath = vp
		} else {
			absPath = filepath.Join(m.workDir, vp)
		}
		
		// Make sure path is absolute
		absPath, err := filepath.Abs(absPath)
		if err == nil {
			// Check if the path exists before adding it
			if _, statErr := os.Stat(absPath); statErr == nil {
				rootPaths = append(rootPaths, absPath)
			}
		}
	}
	
	// De-duplicate rootPaths
	seen := make(map[string]struct{})
	uniqueRoots := []string{}
	for _, root := range rootPaths {
		if _, ok := seen[root]; !ok {
			seen[root] = struct{}{}
			uniqueRoots = append(uniqueRoots, root)
		}
	}
	rootPaths = uniqueRoots
	
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
	result = m.filterTreeNodes(result, prune)
	
	return result, nil
}

// filterTreeNodes filters the file tree based on the specified mode
// If prune is true, only directories containing context files (hot, cold, or excluded) are shown
// If prune is false, all directories containing any non-git-ignored files are shown
func (m *Manager) filterTreeNodes(fileStatuses map[string]NodeStatus, prune bool) map[string]NodeStatus {
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
		// Determine what constitutes "content" based on prune mode
		hasContent := false
		if prune {
			// In prune mode, only context files (hot, cold, excluded) count as content
			hasContent = status == StatusIncludedHot || status == StatusIncludedCold || 
			            status == StatusExcludedByRule
		} else {
			// In normal mode, any non-directory and non-git-ignored file counts as content
			hasContent = status != StatusDirectory && status != StatusIgnoredByGit
		}
		
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
			
			// If pattern is a file path (no glob), use its directory
			if basePath == pattern && !strings.ContainsAny(pattern, "*?[") {
				if stat, err := os.Stat(pattern); err == nil && !stat.IsDir() {
					basePath = filepath.Dir(pattern)
				}
			}
			
			if basePath != "" {
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
	
	// Track excluded directories so we can mark their contents as excluded
	excludedDirs := make(map[string]bool)
	
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
		
		// Note: We don't skip git-ignored files anymore, we classify them
		// so they can optionally be shown in cx view
		
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
		
		// Check if this path is inside an excluded directory
		isInsideExcludedDir := false
		for excludedDir := range excludedDirs {
			if strings.HasPrefix(path, excludedDir+string(filepath.Separator)) {
				isInsideExcludedDir = true
				break
			}
		}
		
		// Add directories and files to the result
		if d.IsDir() {
			// Check if the directory is git ignored
			if gitIgnoredFiles[path] {
				result[path] = StatusIgnoredByGit
				// Continue walking to show contents as gitignored
			} else if m.fileExplicitlyExcluded(path, patterns) || isInsideExcludedDir {
				result[path] = StatusExcludedByRule
				excludedDirs[path] = true
				// Continue walking to show contents as excluded
			} else {
				// Directories will be filtered later if they contain no included files
				result[path] = StatusDirectory
			}
		} else {
			// Classify files
			if gitIgnoredFiles[path] {
				// File is ignored by git
				result[path] = StatusIgnoredByGit
			} else if isInsideExcludedDir {
				// Files inside excluded directories are also excluded
				result[path] = StatusExcludedByRule
			} else if coldFiles[fileKey] {
				result[path] = StatusIncludedCold
			} else if hotFiles[fileKey] {
				result[path] = StatusIncludedHot  
			} else if m.fileMatchesAnyPattern(path, includePatterns) {
				// File matches an include pattern but isn't in the final context,
				// so it must have been excluded by a rule
				result[path] = StatusExcludedByRule
			} else if m.fileExplicitlyExcluded(path, patterns) {
				// File is explicitly excluded (has !filename rule)
				result[path] = StatusExcludedByRule
			} else {
				result[path] = StatusOmittedNoMatch
			}
		}
		
		return nil
	})
}

// fileExplicitlyExcluded checks if a file is explicitly excluded by a !pattern rule
func (m *Manager) fileExplicitlyExcluded(filePath string, patterns []string) bool {
	// Get path relative to workDir for matching
	relPath, _ := filepath.Rel(m.workDir, filePath)
	relPath = filepath.ToSlash(relPath)
	
	for _, pattern := range patterns {
		if !strings.HasPrefix(pattern, "!") {
			continue
		}
		
		// Remove the ! prefix to get the actual pattern
		excludePattern := strings.TrimPrefix(pattern, "!")
		
		// Check various matching approaches
		if m.matchesPattern(relPath, excludePattern) {
			return true
		}
		
		// Also try matching against basename for patterns without slashes
		if !strings.Contains(excludePattern, "/") {
			if matched, _ := filepath.Match(excludePattern, filepath.Base(filePath)); matched {
				return true
			}
		}
	}
	return false
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
	hasExplicitWorktreePattern := false
	
	for _, pattern := range patterns {
		// Check for special pattern to include binary files
		if pattern == "!binary:exclude" || pattern == "binary:include" {
			includeBinary = true
			continue
		}
		
		// Check if any pattern explicitly includes .grove-worktrees
		if !strings.HasPrefix(pattern, "!") && strings.Contains(pattern, ".grove-worktrees") {
			hasExplicitWorktreePattern = true
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
		// Only prune .grove-worktrees if no pattern explicitly includes it AND we're not already inside one
		if d.IsDir() {
			if d.Name() == ".git" || d.Name() == ".grove" {
				return filepath.SkipDir
			}
			// Skip .grove-worktrees directories UNLESS:
			// 1. We have an explicit pattern that includes .grove-worktrees, OR
			// 2. We're already walking inside a .grove-worktrees directory (rootPath contains it)
			if d.Name() == ".grove-worktrees" && 
			   !hasExplicitWorktreePattern && 
			   !strings.Contains(rootPath, string(filepath.Separator)+".grove-worktrees"+string(filepath.Separator)) {
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
			// Special handling for .grove-worktrees: by default, we exclude files inside these directories
			// because they often contain temporary or project-specific artifacts.
			// This exclusion is bypassed if any inclusion rule explicitly contains ".grove-worktrees",
			// indicating the user intentionally wants to include content from them.
			if strings.Contains(path, string(filepath.Separator)+".grove-worktrees"+string(filepath.Separator)) {
				isExplicitlyIncludedByRule := false
				for _, pattern := range patterns {
					if !strings.HasPrefix(pattern, "!") && strings.Contains(pattern, ".grove-worktrees") {
						isExplicitlyIncludedByRule = true
						break
					}
				}
				// Also check if we're walking from a root that contains .grove-worktrees
				if !isExplicitlyIncludedByRule && strings.Contains(rootPath, string(filepath.Separator)+".grove-worktrees"+string(filepath.Separator)) {
					isExplicitlyIncludedByRule = true
				}
				if !isExplicitlyIncludedByRule {
					// Only exclude if .grove-worktrees is a descendant of the working directory
					relPath, err := filepath.Rel(m.workDir, path)
					if err == nil && strings.Contains(relPath, ".grove-worktrees") {
						// The .grove-worktrees is within our working directory, exclude it
						return nil
					}
				}
				// If explicitly included, don't exclude it
			}
			
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
	
	// Convert to absolute paths and deduplicate based on canonical paths
	// Use a map to track seen paths (normalized for case-insensitive filesystems)
	seenPaths := make(map[string]string) // normalized -> original
	var absPaths []string
	
	for _, file := range files {
		absPath, err := filepath.Abs(file)
		if err != nil {
			absPaths = append(absPaths, file + " (error getting absolute path)")
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

// AppendRule adds a new rule to the active rules file.
// validateRuleSafety checks if a rule is safe to add
func (m *Manager) validateRuleSafety(rulePath string) error {
	// Skip validation for exclusion rules
	if strings.HasPrefix(rulePath, "!") {
		rulePath = strings.TrimPrefix(rulePath, "!")
	}
	
	// Count parent directory traversals
	traversalCount := strings.Count(rulePath, "../")
	if traversalCount > 2 {
		return fmt.Errorf("rule '%s' contains too many parent directory traversals (max 2 allowed)", rulePath)
	}
	
	// Check for patterns that could match everything
	if rulePath == "**" || rulePath == "/**" || strings.HasPrefix(rulePath, "../../../") {
		return fmt.Errorf("rule '%s' is too broad and could include system files", rulePath)
	}
	
	// Resolve the actual path to check boundaries
	absPath := filepath.Join(m.workDir, rulePath)
	absPath = filepath.Clean(absPath)
	
	// Get home directory for boundary checking
	homeDir, err := os.UserHomeDir()
	if err != nil {
		homeDir = ""
	}
	
	// Check if the rule would go above the home directory
	if homeDir != "" && len(absPath) < len(homeDir) {
		// Path is shorter than home dir, meaning it's above it
		homeParts := strings.Split(homeDir, string(filepath.Separator))
		absParts := strings.Split(absPath, string(filepath.Separator))
		if len(absParts) < len(homeParts)-1 { // Allow one level above home
			return fmt.Errorf("rule '%s' would include directories too far above home directory", rulePath)
		}
	}
	
	// Check against system directories (both Unix and Windows)
	dangerousPaths := []string{
		"/etc", "/usr", "/bin", "/sbin", "/System", "/Library",
		"/proc", "/sys", "/dev", "/root",
		"C:\\Windows", "C:\\Program Files", "C:\\ProgramData",
	}

	for _, dangerous := range dangerousPaths {
		if absPath == dangerous || strings.HasPrefix(absPath, dangerous+string(filepath.Separator)) {
			return fmt.Errorf("rule '%s' would include system directory '%s'", rulePath, dangerous)
		}
	}
	
	// Check if it's trying to include hidden system directories
	if strings.Contains(rulePath, "/.") && traversalCount > 0 {
		// Be extra careful with hidden directories when going up
		if strings.Contains(rulePath, "/.Trash") || strings.Contains(rulePath, "/.cache") || 
		   strings.Contains(rulePath, "/.config") {
			return fmt.Errorf("rule '%s' would include hidden system directories", rulePath)
		}
	}
	
	return nil
}

// contextType can be "hot", "cold", or "exclude".
func (m *Manager) AppendRule(rulePath, contextType string) error {
	// Validate the rule safety before adding
	if err := m.validateRuleSafety(rulePath); err != nil {
		return fmt.Errorf("safety validation failed: %w", err)
	}
	
	// First, remove any existing rules for this path to prevent duplicates
	// This makes the function idempotent and handles state changes
	if err := m.RemoveRuleForPath(rulePath); err != nil {
		// Non-fatal error, log and continue
		fmt.Fprintf(os.Stderr, "Warning: could not remove existing rules: %v\n", err)
	}
	
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
	case "exclude", "exclude-cold":
		newRule = "!" + rulePath
	default:
		newRule = rulePath
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
	case "cold", "exclude-cold":
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

// RuleStatus represents the current state of a rule
type RuleStatus int

const (
	RuleNotFound RuleStatus = iota // Rule doesn't exist
	RuleHot                        // Rule exists in hot context
	RuleCold                       // Rule exists in cold context  
	RuleExcluded                   // Rule exists as exclusion
)

// ToggleViewDirective adds or removes a `@view:` directive from the rules file.
func (m *Manager) ToggleViewDirective(path string) error {
	rulesFilePath := m.findActiveRulesFile()
	if rulesFilePath == "" {
		// Create .grove/rules file if it doesn't exist
		groveDir := filepath.Join(m.workDir, GroveDir)
		if err := os.MkdirAll(groveDir, 0755); err != nil {
			return fmt.Errorf("error creating %s directory: %w", groveDir, err)
		}
		rulesFilePath = filepath.Join(m.workDir, ActiveRulesFile)
	}

	content, err := os.ReadFile(rulesFilePath)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("error reading rules file: %w", err)
	}

	lines := strings.Split(string(content), "\n")
	var newLines []string
	found := false
	viewDirective := "@view: " + path

	for _, line := range lines {
		if strings.TrimSpace(line) == viewDirective {
			found = true
			continue // Remove the line
		}
		newLines = append(newLines, line)
	}

	if !found {
		// Add the directive to the top
		newLines = append([]string{viewDirective}, newLines...)
	}

	// Clean up empty lines at the end
	for len(newLines) > 0 && strings.TrimSpace(newLines[len(newLines)-1]) == "" {
		newLines = newLines[:len(newLines)-1]
	}

	newContent := strings.Join(newLines, "\n")
	if len(newLines) > 0 {
		newContent += "\n" // Ensure trailing newline
	}

	return os.WriteFile(rulesFilePath, []byte(newContent), 0644)
}

// GetRuleStatus checks the current status of a rule in the rules file
func (m *Manager) GetRuleStatus(rulePath string) RuleStatus {
	rulesFilePath := m.findActiveRulesFile()
	if rulesFilePath == "" {
		return RuleNotFound
	}
	
	content, err := os.ReadFile(rulesFilePath)
	if err != nil {
		return RuleNotFound
	}
	
	// Check for exclusion rule
	excludeRule := "!" + rulePath
	// Check for normal rule
	normalRule := rulePath
	
	lines := strings.Split(string(content), "\n")
	inColdSection := false
	
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "---" {
			inColdSection = true
			continue
		}
		
		if line == excludeRule {
			return RuleExcluded
		}
		
		if line == normalRule {
			if inColdSection {
				return RuleCold
			} else {
				return RuleHot
			}
		}
	}
	
	return RuleNotFound
}

// RemoveRule removes a specific rule from the rules file
func (m *Manager) RemoveRule(rulePath string) error {
	rulesFilePath := m.findActiveRulesFile()
	if rulesFilePath == "" {
		// No rules file exists, nothing to remove
		return nil
	}
	
	content, err := os.ReadFile(rulesFilePath)
	if err != nil {
		return fmt.Errorf("error reading rules file: %w", err)
	}
	
	lines := strings.Split(string(content), "\n")
	var newLines []string
	
	// Rules to potentially remove
	excludeRule := "!" + rulePath
	normalRule := rulePath
	
	for _, line := range lines {
		trimmedLine := strings.TrimSpace(line)
		// Skip the lines that match our rule (either normal or exclude form)
		if trimmedLine == excludeRule || trimmedLine == normalRule {
			continue
		}
		newLines = append(newLines, line)
	}
	
	// Clean up empty lines and unnecessary separators
	newLines = cleanupRulesLines(newLines)
	
	// Write back to file
	newContent := strings.Join(newLines, "\n")
	if len(newLines) > 0 && !strings.HasSuffix(newContent, "\n") {
		newContent += "\n"
	}
	
	return os.WriteFile(rulesFilePath, []byte(newContent), 0644)
}

// RemoveRuleForPath removes any rule that corresponds to the given repository path.
// Unlike RemoveRule which requires an exact match, this function will find and remove
// rules in various formats (path, !path, path/**, !path/**) that match the repository.
func (m *Manager) RemoveRuleForPath(path string) error {
	rulesFilePath := m.findActiveRulesFile()
	if rulesFilePath == "" {
		// No rules file exists, nothing to remove
		return nil
	}
	
	content, err := os.ReadFile(rulesFilePath)
	if err != nil {
		return fmt.Errorf("error reading rules file: %w", err)
	}
	
	lines := strings.Split(string(content), "\n")
	var newLines []string
	
	// Clean the input path
	path = strings.TrimSpace(path)
	path = strings.TrimSuffix(path, "/")
	
	// Generate all possible patterns to look for based on the path
	patternsToRemove := []string{
		path,                  // exact path
		"!" + path,           // excluded path
		path + "/**",         // recursive include
		"!" + path + "/**",   // recursive exclude
		path + "/*",          // single level include
		"!" + path + "/*",    // single level exclude
	}
	
	// Also check for relative paths starting with ./ or ../
	if !filepath.IsAbs(path) {
		patternsToRemove = append(patternsToRemove, 
			"./"+path,
			"!./"+path,
			"./"+path+"/**",
			"!./"+path+"/**",
		)
	}
	
	// Check each line and skip if it matches any of our patterns
	for _, line := range lines {
		trimmedLine := strings.TrimSpace(line)
		shouldRemove := false
		
		for _, pattern := range patternsToRemove {
			if trimmedLine == pattern {
				shouldRemove = true
				break
			}
		}
		
		if !shouldRemove {
			newLines = append(newLines, line)
		}
	}
	
	// Clean up empty lines and unnecessary separators
	newLines = cleanupRulesLines(newLines)
	
	// Write back to file
	newContent := strings.Join(newLines, "\n")
	if len(newLines) > 0 && !strings.HasSuffix(newContent, "\n") {
		newContent += "\n"
	}
	
	return os.WriteFile(rulesFilePath, []byte(newContent), 0644)
}

// cleanupRulesLines removes unnecessary separators and empty lines
func cleanupRulesLines(lines []string) []string {
	// Remove trailing empty lines
	for len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) == "" {
		lines = lines[:len(lines)-1]
	}
	
	// If only separator remains, remove it
	if len(lines) == 1 && strings.TrimSpace(lines[0]) == "---" {
		return []string{}
	}
	
	// Remove separator if there are no cold context rules after it
	hasColdRules := false
	separatorIndex := -1
	
	for i, line := range lines {
		if strings.TrimSpace(line) == "---" {
			separatorIndex = i
		} else if separatorIndex >= 0 && strings.TrimSpace(line) != "" {
			hasColdRules = true
			break
		}
	}
	
	// Remove separator if no cold rules follow
	if separatorIndex >= 0 && !hasColdRules {
		result := make([]string, 0, len(lines)-1)
		for i, line := range lines {
			if i != separatorIndex {
				result = append(result, line)
			}
		}
		lines = result
	}
	
	return lines
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