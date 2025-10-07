package context

import (
	"os"
	"path/filepath"
	"sort"
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

// normalizePathKey returns a normalized version of the path for use as a map key
// to handle case-insensitive filesystems. It doesn't change the actual path,
// just provides a consistent key for deduplication.
func normalizePathKey(path string) string {
	// Clean the path and convert to lowercase for consistent map keys
	// This prevents duplicate entries on case-insensitive filesystems
	return strings.ToLower(filepath.Clean(path))
}

type FileNode struct {
	Path       string
	Name       string
	Status     NodeStatus
	IsDir      bool
	TokenCount int
	Children   []*FileNode
}

// AnalyzeProjectTree walks the entire project and creates a tree structure showing
// which files are included, excluded, or ignored based on context rules
func (m *Manager) AnalyzeProjectTree(prune bool, showGitIgnored bool) (*FileNode, error) {
	// Use the centralized engine to get all file classifications
	fileStatuses, err := m.ResolveAndClassifyAllFiles(prune)
	if err != nil {
		return nil, err
	}

	// If showing gitignored files, we need to walk the entire tree to find them
	if showGitIgnored {
		gitIgnoredFiles, err := m.getGitIgnoredFiles(m.workDir)
		if err == nil {
			// Walk the entire working directory to find all gitignored files
			err = filepath.WalkDir(m.workDir, func(path string, d os.DirEntry, err error) error {
				if err != nil {
					return nil // Continue walking even if we can't access a directory
				}

				// Skip .git directory
				if d.IsDir() && d.Name() == ".git" {
					return filepath.SkipDir
				}

				// If this file is gitignored and not already in our results, add it
				if gitIgnoredFiles[path] {
					if _, exists := fileStatuses[path]; !exists {
						fileStatuses[path] = StatusIgnoredByGit
					}
				}

				return nil
			})
		}
	}

	// Build FileNode map from the classifications
	// Use normalized paths as keys to handle case-insensitive filesystems
	nodes := make(map[string]*FileNode)
	pathMapping := make(map[string]string) // normalized -> original path
	hasExternalFiles := false
	hasExternalViewPaths := false

	// Check if any paths are from @view directives and are external
	for path := range fileStatuses {
		relPath, err := filepath.Rel(m.workDir, path)
		if err != nil || strings.HasPrefix(relPath, "..") {
			// This is an external path, check if it's a directory (likely from @view)
			if info, err := os.Stat(path); err == nil && info.IsDir() {
				hasExternalViewPaths = true
				break
			}
		}
	}

	for path, status := range fileStatuses {
		// Skip StatusIgnoredByGit unless explicitly requested
		if status == StatusIgnoredByGit && !showGitIgnored {
			continue
		}

		// Get a normalized key for deduplication
		normalizedKey := normalizePathKey(path)

		// Skip if we've already seen this path (different case)
		if existing, seen := pathMapping[normalizedKey]; seen {
			// If we've seen this path with a different case, skip it
			if existing != path {
				continue
			}
		}
		pathMapping[normalizedKey] = path

		// Check if this is an external file that is actually included (hot or cold context)
		relPath, err := filepath.Rel(m.workDir, path)
		isExternal := err != nil || strings.HasPrefix(relPath, "..")
		if isExternal && (status == StatusIncludedHot || status == StatusIncludedCold) {
			hasExternalFiles = true
		}

		// For directories outside workDir, only include them if they have included descendants
		// We'll handle this filtering later, for now just create the node
		isDir := status == StatusDirectory
		// Check if excluded items are actually directories
		if status == StatusExcludedByRule {
			if info, err := os.Stat(path); err == nil && info.IsDir() {
				isDir = true
			}
		}
		tokenCount := 0
		// Calculate token count for included files
		if !isDir && (status == StatusIncludedHot || status == StatusIncludedCold) {
			if info, err := os.Stat(path); err == nil {
				// Estimate tokens as roughly 4 bytes per token
				tokenCount = int(info.Size() / 4)
			}
		}

		node := &FileNode{
			Path:       path,
			Name:       filepath.Base(path),
			Status:     status,
			IsDir:      isDir,
			TokenCount: tokenCount,
			Children:   []*FileNode{},
		}
		// Use normalized key to prevent duplicates on case-insensitive filesystems
		nodeKey := normalizePathKey(path)
		nodes[nodeKey] = node
	}

	// If no external files are included and no @view paths, remove external directory nodes
	if !hasExternalFiles && !hasExternalViewPaths {
		filteredNodes := make(map[string]*FileNode)
		for nodeKey, node := range nodes {
			relPath, err := filepath.Rel(m.workDir, node.Path)
			isExternal := err != nil || strings.HasPrefix(relPath, "..")
			if !isExternal {
				filteredNodes[nodeKey] = node
			}
		}
		nodes = filteredNodes
	}

	// Build the tree structure
	var root *FileNode
	if hasExternalFiles || hasExternalViewPaths {
		// Create a synthetic root to show CWD and external paths as siblings
		root = buildTreeWithSyntheticRoot(m.workDir, nodes)
	} else {
		// No external files, use the working directory as root
		root = buildTreeWithExternals(m.workDir, nodes)
	}

	// Post-process the tree to infer directory statuses from their children
	setDirectoryStatuses(root)

	// Calculate directory token counts
	calculateDirectoryTokenCounts(root)

	return root, nil
}

// buildTreeWithSyntheticRoot creates a synthetic root showing the filesystem hierarchy
func buildTreeWithSyntheticRoot(workDir string, nodes map[string]*FileNode) *FileNode {
	// Create a synthetic root node
	syntheticRoot := &FileNode{
		Path:     "/",
		Name:     "/",
		Status:   StatusDirectory,
		IsDir:    true,
		Children: []*FileNode{},
	}

	// First, ensure all directory nodes exist
	for _, node := range nodes {
		ensureParentNodes("", node.Path, nodes)
	}

	// Build parent-child relationships
	for _, node := range nodes {
		parentPath := filepath.Dir(node.Path)
		parentKey := normalizePathKey(parentPath)

		if parent, exists := nodes[parentKey]; exists {
			parent.Children = append(parent.Children, node)
		}
	}

	// Mark the CWD node with (CWD) suffix to make it clear
	workDirKey := normalizePathKey(workDir)
	if cwdNode, exists := nodes[workDirKey]; exists {
		cwdNode.Name = filepath.Base(workDir) + " (CWD)"
	}

	// Add /Users as the only child of synthetic root
	usersKey := normalizePathKey("/Users")
	if usersNode, exists := nodes[usersKey]; exists {
		syntheticRoot.Children = append(syntheticRoot.Children, usersNode)
	}

	// Sort children at each level
	sortChildren(syntheticRoot)

	return syntheticRoot
}

// buildTreeWithExternals constructs a hierarchical tree that includes external directories
func buildTreeWithExternals(rootPath string, nodes map[string]*FileNode) *FileNode {
	// Get the root node from nodes map, or create it
	rootKey := normalizePathKey(rootPath)
	root, exists := nodes[rootKey]
	if !exists {
		root = &FileNode{
			Path:     rootPath,
			Name:     filepath.Base(rootPath),
			Status:   StatusDirectory,
			IsDir:    true,
			Children: []*FileNode{},
		}
		nodes[rootKey] = root
	}

	// First, ensure all directory nodes exist
	for _, node := range nodes {
		ensureParentNodes("", node.Path, nodes)
	}

	// Build parent-child relationships
	for _, node := range nodes {
		if node.Path == rootPath {
			continue // Skip the root itself
		}

		parentPath := filepath.Dir(node.Path)
		parentKey := normalizePathKey(parentPath)

		if parent, exists := nodes[parentKey]; exists {
			// Node has a parent in our set, add it as a child
			parent.Children = append(parent.Children, node)
		}
	}

	// buildTreeWithExternals should only show the working directory tree
	// External directory handling is done in buildTreeWithSyntheticRoot

	// Sort children at each level
	sortChildren(root)

	return root
}

// ensureParentNodes creates directory nodes for all parents of a path
func ensureParentNodes(rootPath string, path string, nodes map[string]*FileNode) {
	if path == rootPath || path == "/" || path == "." {
		return
	}

	parentPath := filepath.Dir(path)
	parentKey := normalizePathKey(parentPath)
	if _, exists := nodes[parentKey]; !exists {
		// Don't create parent if we've reached the rootPath
		if rootPath != "" && parentPath == rootPath {
			return
		}

		// Create the parent node
		nodes[parentKey] = &FileNode{
			Path:     parentPath,
			Name:     filepath.Base(parentPath),
			Status:   StatusDirectory,
			IsDir:    true,
			Children: []*FileNode{},
		}
		// Recursively ensure its parents exist
		ensureParentNodes(rootPath, parentPath, nodes)
	}
}

// sortChildren recursively sorts all children nodes
func sortChildren(node *FileNode) {
	if len(node.Children) == 0 {
		return
	}

	// Sort: directories first, then by name
	sort.Slice(node.Children, func(i, j int) bool {
		if node.Children[i].IsDir != node.Children[j].IsDir {
			return node.Children[i].IsDir
		}
		return strings.ToLower(node.Children[i].Name) < strings.ToLower(node.Children[j].Name)
	})

	// Recursively sort children
	for _, child := range node.Children {
		if child.IsDir {
			sortChildren(child)
		}
	}
}

// calculateDirectoryTokenCounts recursively calculates token counts for directories
func calculateDirectoryTokenCounts(node *FileNode) int {
	if !node.IsDir {
		return node.TokenCount
	}

	var totalTokens int
	for _, child := range node.Children {
		totalTokens += calculateDirectoryTokenCounts(child)
	}
	node.TokenCount = totalTokens
	return totalTokens
}

// setDirectoryStatuses infers directory status from children
func setDirectoryStatuses(node *FileNode) {
	if !node.IsDir || node.Status == StatusExcludedByRule {
		return
	}

	// Recurse to process children first
	for _, child := range node.Children {
		setDirectoryStatuses(child)
	}

	if len(node.Children) == 0 {
		return
	}

	// Count the status types among children
	hotCount, coldCount, excludedCount, omittedCount := 0, 0, 0, 0
	for _, child := range node.Children {
		switch child.Status {
		case StatusIncludedHot:
			hotCount++
		case StatusIncludedCold:
			coldCount++
		case StatusExcludedByRule:
			excludedCount++
		case StatusOmittedNoMatch:
			omittedCount++
		}
	}

	// Set directory status based on predominant child status
	// Priority: Hot > Cold > Excluded > Omitted
	if hotCount > 0 {
		node.Status = StatusIncludedHot
	} else if coldCount > 0 {
		node.Status = StatusIncludedCold
	} else if excludedCount > 0 {
		node.Status = StatusExcludedByRule
	} else if omittedCount > 0 {
		node.Status = StatusOmittedNoMatch
	}
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
		return nil, err
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
