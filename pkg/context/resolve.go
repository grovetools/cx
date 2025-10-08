package context

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/mattsolo1/grove-core/config"
)

// resolveFilesFromRulesContent resolves files based on rules content provided as a byte slice.
func (m *Manager) resolveFilesFromRulesContent(rulesContent []byte) ([]string, error) {
	// Parse the rules content directly without recursion for this case
	// This is used by commands that provide rules content directly (not from a file)
	mainPatterns, coldPatterns, _, _, _, _, _, _, _, _, _, err := m.parseRulesFile(rulesContent)
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

	localHot, localCold, mainDefaults, coldDefaults, mainImports, coldImports, localView, _, _, _, _, err := m.parseRulesFile(rulesContent)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("parsing rules file %s: %w", absRulesPath, err)
	}

	hotPatterns = append(hotPatterns, localHot...)
	coldPatterns = append(coldPatterns, localCold...)
	viewPaths = append(viewPaths, localView...)

	rulesDir := filepath.Dir(absRulesPath)

	// Process hot rule set imports
	for _, importIdentifier := range mainImports {
		parts := strings.SplitN(importIdentifier, ":", 2)
		if len(parts) != 2 {
			fmt.Fprintf(os.Stderr, "Warning: invalid ruleset import format '%s'\n", importIdentifier)
			continue
		}
		projectAlias, rulesetName := parts[0], parts[1]

		projectPath, resolveErr := m.getAliasResolver().Resolve(projectAlias)
		if resolveErr != nil {
			fmt.Fprintf(os.Stderr, "Warning: could not resolve project alias '%s' for rule import: %v\n", projectAlias, resolveErr)
			continue
		}

		rulesFilePath := filepath.Join(projectPath, ".cx", rulesetName+".rules")

		nestedHot, nestedCold, nestedView, err := m.resolveAllPatterns(rulesFilePath, visited)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: could not resolve ruleset '%s' from project '%s': %v\n", rulesetName, projectAlias, err)
			continue
		}
		hotPatterns = append(hotPatterns, nestedHot...)
		coldPatterns = append(coldPatterns, nestedCold...)
		viewPaths = append(viewPaths, nestedView...)
	}

	// Process cold rule set imports
	for _, importIdentifier := range coldImports {
		parts := strings.SplitN(importIdentifier, ":", 2)
		if len(parts) != 2 {
			fmt.Fprintf(os.Stderr, "Warning: invalid ruleset import format '%s'\n", importIdentifier)
			continue
		}
		projectAlias, rulesetName := parts[0], parts[1]

		projectPath, resolveErr := m.getAliasResolver().Resolve(projectAlias)
		if resolveErr != nil {
			fmt.Fprintf(os.Stderr, "Warning: could not resolve project alias '%s' for rule import: %v\n", projectAlias, resolveErr)
			continue
		}

		rulesFilePath := filepath.Join(projectPath, ".cx", rulesetName+".rules")

		nestedHot, nestedCold, nestedView, err := m.resolveAllPatterns(rulesFilePath, visited)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: could not resolve ruleset '%s' from project '%s': %v\n", rulesetName, projectAlias, err)
			continue
		}
		// For cold imports, add everything to cold patterns
		coldPatterns = append(coldPatterns, nestedHot...)
		coldPatterns = append(coldPatterns, nestedCold...)
		viewPaths = append(viewPaths, nestedView...)
	}

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
	activeRulesFile := m.findActiveRulesFile()
	if activeRulesFile == "" {
		// If no rules file, check for defaults configured in grove.yml
		defaultContent, _ := m.LoadDefaultRulesContent()
		if defaultContent != nil {
			// Parse the default content to get cold patterns
			_, coldPatterns, _, _, _, _, _, _, _, _, _, err := m.parseRulesFile(defaultContent)
			if err != nil {
				return nil, err
			}
			coldFiles, err := m.resolveFilesFromPatterns(coldPatterns)
			if err != nil {
				return nil, err
			}
			sort.Strings(coldFiles)
			return coldFiles, nil
		}
		// No active or default rules found
		return []string{}, nil
	}

	// Resolve all patterns recursively from the active rules file
	_, coldPatterns, _, err := m.resolveAllPatterns(activeRulesFile, make(map[string]bool))
	if err != nil {
		return nil, fmt.Errorf("failed to resolve patterns for cold context: %w", err)
	}

	// Resolve files from only the cold patterns
	coldFiles, err := m.resolveFilesFromPatterns(coldPatterns)
	if err != nil {
		return nil, fmt.Errorf("error resolving cold context files: %w", err)
	}

	// Sort for consistent output
	sort.Strings(coldFiles)
	return coldFiles, nil
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

// decodeDirective extracts directive information from an encoded pattern
// Returns: cleanPattern, directive, query, hasDirective
func decodeDirective(pattern string) (string, string, string, bool) {
	parts := strings.Split(pattern, "|||")
	if len(parts) == 3 {
		return parts[0], parts[1], parts[2], true
	}
	return pattern, "", "", false
}

// applyDirectiveFilter filters a list of files based on a directive (@find or @grep)
func (m *Manager) applyDirectiveFilter(files []string, directive, query string) ([]string, error) {
	var filtered []string

	if directive == "find" {
		// @find: filter by filename/path
		for _, file := range files {
			if strings.Contains(file, query) {
				filtered = append(filtered, file)
			}
		}
	} else if directive == "grep" {
		// @grep: filter by file content (parallelized for performance)
		type result struct {
			file string
			err  error
		}

		resultChan := make(chan result, len(files))
		semaphore := make(chan struct{}, 10) // Limit to 10 concurrent goroutines

		for _, file := range files {
			file := file // Capture loop variable

			go func() {
				semaphore <- struct{}{} // Acquire semaphore
				defer func() { <-semaphore }() // Release semaphore

				// Construct absolute path for reading
				filePath := file
				if !filepath.IsAbs(file) {
					filePath = filepath.Join(m.workDir, file)
				}

				// Read file content
				content, err := os.ReadFile(filePath)
				if err != nil {
					// Skip files that can't be read
					resultChan <- result{file: "", err: nil}
					return
				}

				// Check if content contains the query
				if strings.Contains(string(content), query) {
					resultChan <- result{file: file, err: nil}
				} else {
					resultChan <- result{file: "", err: nil}
				}
			}()
		}

		// Collect results
		for i := 0; i < len(files); i++ {
			res := <-resultChan
			if res.file != "" {
				filtered = append(filtered, res.file)
			}
		}
		close(resultChan)
	}

	return filtered, nil
}

// resolveFilesFromPatterns resolves files from a given set of patterns
func (m *Manager) resolveFilesFromPatterns(patterns []string) ([]string, error) {
	if len(patterns) == 0 {
		return []string{}, nil
	}

	// First, decode any directive information and separate patterns
	type patternInfo struct {
		pattern   string
		directive string
		query     string
		isExclude bool
	}

	var patternInfos []patternInfo

	for _, pattern := range patterns {
		isExclude := strings.HasPrefix(pattern, "!")
		cleanPattern := pattern
		if isExclude {
			cleanPattern = strings.TrimPrefix(pattern, "!")
		}

		// Try to parse plain text directive first (e.g., "pattern @find: \"query\"")
		basePattern, directive, query, hasDirective := parseSearchDirective(cleanPattern)

		// If no plain text directive, try encoded format (e.g., "pattern|||find|||query")
		if !hasDirective {
			basePattern, directive, query, _ = decodeDirective(cleanPattern)
		}

		patternInfos = append(patternInfos, patternInfo{
			pattern:   basePattern,
			directive: directive,
			query:     query,
			isExclude: isExclude,
		})

		// For the traditional flow, we need to reconstruct the pattern
		// without directive encoding for preProcessPatterns
		if isExclude {
			patterns[len(patternInfos)-1] = "!" + basePattern
		} else {
			patterns[len(patternInfos)-1] = basePattern
		}
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
	var deferredExclusions []string            // Store exclusion patterns to process after inclusions

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

		// Check if this is an exact file path (no globs) that exists
		// This handles files returned by @cmd: directives
		if !strings.ContainsAny(cleanPattern, "*?[]") {
			// It's an exact path, check if it exists
			filePath := cleanPattern
			if !filepath.IsAbs(filePath) {
				filePath = filepath.Join(m.workDir, filePath)
			}
			filePath = filepath.Clean(filePath)

			if info, err := os.Stat(filePath); err == nil && !info.IsDir() {
				// It's an existing file, add it directly
				if filepath.IsAbs(cleanPattern) || strings.HasPrefix(cleanPattern, "../") {
					uniqueFiles[filePath] = true
				} else {
					// Use relative path from workDir
					relPath, err := filepath.Rel(m.workDir, filePath)
					if err == nil {
						uniqueFiles[relPath] = true
					} else {
						uniqueFiles[filePath] = true
					}
				}
				continue
			}
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

	// Apply directive filters if any patterns had directives
	// We need to match each file to its pattern and apply the appropriate filter
	// For simplicity, we'll apply directives pattern-by-pattern
	hasAnyDirective := false
	for _, info := range patternInfos {
		if info.directive != "" && !info.isExclude {
			hasAnyDirective = true
			break
		}
	}

	if hasAnyDirective {
		// Build a map of pattern index to its directive info
		// We need to re-match files to patterns to know which directive to apply
		filteredFiles := make(map[string]bool)

		for _, file := range filesToInclude {
			// Get path relative to workDir for matching
			relPath := file
			if !filepath.IsAbs(file) {
				relPath = file
			} else {
				if rp, err := filepath.Rel(m.workDir, file); err == nil {
					relPath = rp
				}
			}
			relPath = filepath.ToSlash(relPath)

			// Collect matching patterns into two lists: with and without directives
			var matchesWithDirective []patternInfo
			var matchesWithoutDirective []patternInfo

			for i, info := range patternInfos {
				if info.isExclude {
					continue
				}

				// Use the preprocessed pattern for matching
				patternToMatch := patterns[i]
				if strings.HasPrefix(patternToMatch, "!") {
					continue
				}

				// Convert pattern to same format (relative/absolute) as file path for matching
				// If the pattern is absolute but we're matching against a relative path, convert it
				patternForMatching := patternToMatch
				if filepath.IsAbs(patternToMatch) && !filepath.IsAbs(file) {
					// Pattern is absolute, file is relative - convert pattern to relative
					if rp, err := filepath.Rel(m.workDir, patternToMatch); err == nil {
						patternForMatching = filepath.ToSlash(rp)
					}
				} else if filepath.IsAbs(patternToMatch) && filepath.IsAbs(file) {
					// Both absolute - use absolute file path for matching
					patternForMatching = filepath.ToSlash(patternToMatch)
					relPath = filepath.ToSlash(file)
				}

				// Check if this pattern matches the file
				if m.matchPattern(patternForMatching, relPath) {
					if info.directive != "" {
						matchesWithDirective = append(matchesWithDirective, info)
					} else {
						matchesWithoutDirective = append(matchesWithoutDirective, info)
					}
				}
			}

			// Apply filtering logic:
			// - If any patterns with directives match, the file MUST pass at least one directive filter
			// - If only patterns without directives match, include the file
			// - If both match, directive patterns take precedence (file must pass a directive)
			shouldInclude := false

			if len(matchesWithDirective) > 0 {
				// File matched patterns with directives - it MUST pass at least one directive filter
				for _, info := range matchesWithDirective {
					filtered, err := m.applyDirectiveFilter([]string{file}, info.directive, info.query)
					if err != nil {
						return nil, fmt.Errorf("error applying directive filter: %w", err)
					}
					if len(filtered) > 0 {
						shouldInclude = true
						break
					}
				}
				// Note: if the file fails ALL directive filters, shouldInclude stays false
				// even if matchesWithoutDirective is not empty
			} else if len(matchesWithoutDirective) > 0 {
				// No directive patterns match, but non-directive patterns do - include the file
				shouldInclude = true
			}

			if shouldInclude {
				filteredFiles[file] = true
			}
		}

		// Replace filesToInclude with filtered results
		filesToInclude = make([]string, 0, len(filteredFiles))
		for file := range filteredFiles {
			filesToInclude = append(filesToInclude, file)
		}
		sort.Strings(filesToInclude)
	}

	// Return the resolved file list
	return filesToInclude, nil
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

// matchDoubleStarPattern handles patterns with ** for recursive matching
func matchDoubleStarPattern(pattern, path string) bool {
	// Special case: pattern like "**/something/**" means "something" appears anywhere in path
	if strings.HasPrefix(pattern, "**/") && strings.HasSuffix(pattern, "/**") {
		middle := pattern[3 : len(pattern)-3]
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
