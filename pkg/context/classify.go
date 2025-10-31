package context

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
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

	hotRules, coldRules, viewPaths, err := m.expandAllRules(activeRulesFile, make(map[string]bool), 0)
	if err != nil {
		return nil, err
	}

	// Extract patterns from RuleInfo
	hotPatterns := make([]string, len(hotRules))
	for i, rule := range hotRules {
		pattern := rule.Pattern
		// Encode directive if present
		if rule.Directive != "" {
			pattern = pattern + "|||" + rule.Directive + "|||" + rule.DirectiveQuery
		}
		if rule.IsExclude {
			hotPatterns[i] = "!" + pattern
		} else {
			hotPatterns[i] = pattern
		}
	}

	coldPatterns := make([]string, len(coldRules))
	for i, rule := range coldRules {
		pattern := rule.Pattern
		// Encode directive if present
		if rule.Directive != "" {
			pattern = pattern + "|||" + rule.Directive + "|||" + rule.DirectiveQuery
		}
		if rule.IsExclude {
			coldPatterns[i] = "!" + pattern
		} else {
			coldPatterns[i] = pattern
		}
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

	// The old logic for handling viewPaths here was incorrect and has been removed.
	// New logic is added at the end of the function to correctly handle resolved file paths.

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
		gitIgnoredFiles, err := m.GetGitIgnoredFiles(rootPath)
		if err != nil {
			// Non-fatal, continue without gitignore
			gitIgnoredFiles = make(map[string]bool)
		}

		err = m.walkAndClassifyFiles(rootPath, allPatterns, gitIgnoredFiles, hotFiles, coldFiles, result)
		if err != nil {
			return nil, err
		}
	}

	// Add files from @view directives if they are not already in the result set
	if len(viewPaths) > 0 {
		viewOnlyFiles, err := m.resolveFilesFromPatterns(viewPaths)
		if err != nil {
			// Non-fatal error, just print a warning
			fmt.Fprintf(os.Stderr, "Warning: failed to resolve view-only patterns: %v\n", err)
		} else {
			for _, file := range viewOnlyFiles {
				if _, exists := result[file]; !exists {
					// Add the file with "omitted" status so it's visible in the TUI
					// but not part of the actual context.
					result[file] = StatusOmittedNoMatch
				}
			}
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
				// Check if the path exists and is a directory before adding it to the walk list.
				// This prevents WalkDir from crashing on non-existent paths from rules.
				if stat, err := os.Stat(basePath); err == nil && stat.IsDir() {
					rootsMap[basePath] = true
				}
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

		// Use the centralized matchPattern method from Manager
		if m.matchPattern(excludePattern, relPath) {
			return true
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

		// Use the centralized matchPattern method from Manager
		if m.matchPattern(pattern, relPath) {
			return true
		}
	}
	return false
}

// GetGitIgnoredFiles is the exported version of getGitIgnoredFiles
func (m *Manager) GetGitIgnoredFiles(rootPath string) (map[string]bool, error) {
	return m.getGitIgnoredFiles(rootPath)
}
