package context

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

type NodeStatus int

const (
	StatusIncludedHot    NodeStatus = iota // In hot context
	StatusIncludedCold                     // In cold context
	StatusExcludedByRule                   // Matched an include rule, but then an exclude rule
	StatusOmittedNoMatch                   // Not matched by any include rule
	StatusIgnoredByGit                     // Ignored by .gitignore
	StatusDirectory                        // A directory containing other nodes
)

type FileNode struct {
	Path     string
	Name     string
	Status   NodeStatus
	IsDir    bool
	Children []*FileNode
}

// AnalyzeProjectTree walks the entire project and creates a tree structure showing
// which files are included, excluded, or ignored based on context rules
func (m *Manager) AnalyzeProjectTree() (*FileNode, error) {
	// Get git ignored files
	gitIgnoredFiles, err := m.getGitIgnoredFiles("")
	if err != nil {
		// Non-fatal error, continue without git ignore support
		gitIgnoredFiles = make(map[string]bool)
	}

	// Get hot context files
	hotFiles, err := m.ResolveFilesFromRules()
	if err != nil {
		return nil, fmt.Errorf("failed to resolve hot context files: %w", err)
	}
	hotFilesMap := make(map[string]bool)
	for _, f := range hotFiles {
		absPath := f
		if !filepath.IsAbs(f) {
			absPath = filepath.Join(m.workDir, f)
		}
		hotFilesMap[absPath] = true
	}

	// Get cold context files
	coldFiles, err := m.ResolveColdContextFiles()
	if err != nil {
		// Non-fatal error, continue without cold context
		coldFiles = []string{}
	}
	coldFilesMap := make(map[string]bool)
	for _, f := range coldFiles {
		absPath := f
		if !filepath.IsAbs(f) {
			absPath = filepath.Join(m.workDir, f)
		}
		coldFilesMap[absPath] = true
	}

	// Parse rules to get all patterns and identify walk roots
	var allIncludePatterns []string
	var allPatterns []string
	walkRoots := make(map[string]bool)
	walkRoots[m.workDir] = true // Always include the main working directory

	rulesPath := filepath.Join(m.workDir, ActiveRulesFile)
	if _, err := os.Stat(rulesPath); err == nil {
		mainPatterns, coldPatterns, _, _, _, _, err := m.parseRulesFile(rulesPath)
		if err == nil {
			allPatterns = append(mainPatterns, coldPatterns...)
			// Collect all include patterns (non-negated ones)
			for _, p := range allPatterns {
				if !strings.HasPrefix(p, "!") {
					allIncludePatterns = append(allIncludePatterns, p)
				}
			}
		}
	} else {
		// Try legacy rules file
		rulesPath = filepath.Join(m.workDir, RulesFile)
		if _, err := os.Stat(rulesPath); err == nil {
			mainPatterns, coldPatterns, _, _, _, _, err := m.parseRulesFile(rulesPath)
			if err == nil {
				allPatterns = append(mainPatterns, coldPatterns...)
				for _, p := range allPatterns {
					if !strings.HasPrefix(p, "!") {
						allIncludePatterns = append(allIncludePatterns, p)
					}
				}
			}
		}
	}

	// Identify external walk roots from patterns
	for _, pattern := range allIncludePatterns {
		if strings.HasPrefix(pattern, "!") {
			continue // Skip exclusion patterns
		}
		
		// Check if pattern points to external directory
		if filepath.IsAbs(pattern) || strings.HasPrefix(pattern, "../") {
			// Extract base directory from pattern
			basePath := extractBasePathFromPattern(pattern, m.workDir)
			if basePath != "" && basePath != m.workDir {
				walkRoots[basePath] = true
			}
		}
	}

	// Walk all directory trees and collect all nodes
	nodes := make(map[string]*FileNode)
	
	// Walk each root directory
	for walkRoot := range walkRoots {
		// Skip if directory doesn't exist
		if _, err := os.Stat(walkRoot); os.IsNotExist(err) {
			continue
		}
		
		err = filepath.WalkDir(walkRoot, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return nil // Skip files we can't read
			}

			// Skip the root directory entry itself if it's the same as walkRoot
			if path == walkRoot && walkRoot != m.workDir {
				return nil
			}
			
			// Check directory names for exclusion
			if d.IsDir() && (d.Name() == ".git" || d.Name() == ".grove") {
				return filepath.SkipDir
			}

			// Skip git-ignored files and directories entirely
			if gitIgnoredFiles[path] {
				if d.IsDir() {
					return filepath.SkipDir // Skip the entire directory
				}
				return nil // Skip the file
			}
			
			// For directories, use git check-ignore to see if the directory itself is ignored
			if d.IsDir() && isGitIgnored(path) {
				return filepath.SkipDir
			}

			// Create node
			node := &FileNode{
				Path:     path,
				Name:     d.Name(),
				IsDir:    d.IsDir(),
				Children: []*FileNode{},
			}

			// Determine relative path for pattern matching
			var matchPath string
			if walkRoot == m.workDir {
				matchPath, _ = filepath.Rel(m.workDir, path)
			} else {
				// For external directories, use path relative to workDir for pattern matching
				matchPath, _ = filepath.Rel(m.workDir, path)
			}

			// Determine status
			if d.IsDir() {
				node.Status = StatusDirectory
			} else if coldFilesMap[path] {
				node.Status = StatusIncludedCold
			} else if hotFilesMap[path] {
				node.Status = StatusIncludedHot
			} else if matchesAnyPatternWithContext(path, matchPath, allIncludePatterns, m.workDir) {
				// File matches an include pattern but isn't in the final context,
				// so it must have been excluded by a rule
				node.Status = StatusExcludedByRule
			} else {
				node.Status = StatusOmittedNoMatch
			}

			nodes[path] = node
			return nil
		})

		if err != nil {
			return nil, fmt.Errorf("failed to walk directory %s: %w", walkRoot, err)
		}
	}

	// Build the tree structure
	root := buildTreeWithExternalRoots(m.workDir, nodes, walkRoots)
	
	return root, nil
}

// matchesAnyPattern checks if a path matches any of the given patterns
func matchesAnyPattern(path string, patterns []string) bool {
	for _, pattern := range patterns {
		if matchesPattern(path, pattern) {
			return true
		}
	}
	return false
}

// matchesPattern checks if a path matches a gitignore-style pattern
func matchesPattern(path string, pattern string) bool {
	// Handle directory patterns ending with /
	if strings.HasSuffix(pattern, "/") {
		return false // We're only matching files, not directories
	}

	// Handle ** patterns
	if strings.Contains(pattern, "**") {
		parts := strings.Split(pattern, "**")
		if len(parts) == 2 {
			prefix := strings.TrimSuffix(parts[0], "/")
			suffix := strings.TrimPrefix(parts[1], "/")

			if prefix != "" && !strings.HasPrefix(path, prefix) {
				return false
			}
			if suffix != "" && !strings.HasSuffix(path, suffix) {
				return false
			}
			if prefix == "" && suffix != "" {
				// Pattern like "**/*.go"
				return strings.HasSuffix(path, suffix)
			}
			return true
		}
	}

	// Simple glob matching
	matched, _ := filepath.Match(pattern, path)
	if matched {
		return true
	}

	// Try matching against just the filename
	matched, _ = filepath.Match(pattern, filepath.Base(path))
	return matched
}

// buildTree constructs a hierarchical tree from a flat map of nodes
func buildTree(rootPath string, nodes map[string]*FileNode) *FileNode {
	root := &FileNode{
		Path:     rootPath,
		Name:     filepath.Base(rootPath),
		Status:   StatusDirectory,
		IsDir:    true,
		Children: []*FileNode{},
	}

	// First, ensure all directory nodes exist
	for path := range nodes {
		ensureParentNodes(rootPath, path, nodes)
	}

	// Now build parent-child relationships
	for path, node := range nodes {
		if path == rootPath {
			continue
		}

		parentPath := filepath.Dir(path)
		if parent, exists := nodes[parentPath]; exists {
			parent.Children = append(parent.Children, node)
		} else if parentPath == rootPath {
			root.Children = append(root.Children, node)
		}
	}

	// Sort children at each level
	sortChildren(root)

	return root
}

// ensureParentNodes creates directory nodes for all parents of a path
func ensureParentNodes(rootPath string, path string, nodes map[string]*FileNode) {
	if path == rootPath {
		return
	}

	parentPath := filepath.Dir(path)
	if _, exists := nodes[parentPath]; !exists && parentPath != rootPath {
		// Create the parent node
		nodes[parentPath] = &FileNode{
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
		return node.Children[i].Name < node.Children[j].Name
	})

	// Recursively sort children
	for _, child := range node.Children {
		if child.IsDir {
			sortChildren(child)
		}
	}
}

// extractBasePathFromPattern extracts the non-glob base directory from a pattern
func extractBasePathFromPattern(pattern string, workDir string) string {
	cleanPattern := filepath.Clean(pattern)
	
	// Handle absolute paths
	if filepath.IsAbs(cleanPattern) {
		// Find the base directory before any wildcards
		parts := strings.Split(cleanPattern, string(filepath.Separator))
		for i, part := range parts {
			if strings.ContainsAny(part, "*?[") {
				// Found wildcard, return path up to this point
				if i > 0 {
					result := filepath.Join(parts[:i]...)
					// On Unix systems, filepath.Join removes the leading slash, so we need to add it back
					if !strings.HasPrefix(result, string(filepath.Separator)) && strings.HasPrefix(cleanPattern, string(filepath.Separator)) {
						result = string(filepath.Separator) + result
					}
					return result
				}
				return string(filepath.Separator)
			}
		}
		// No wildcards, check if it's a directory
		if info, err := os.Stat(cleanPattern); err == nil && info.IsDir() {
			return cleanPattern
		}
		return filepath.Dir(cleanPattern)
	}
	
	// Handle relative paths starting with ../
	if strings.HasPrefix(cleanPattern, "../") {
		// Resolve relative to workDir
		absPath := filepath.Join(workDir, cleanPattern)
		absPath = filepath.Clean(absPath)
		
		// Find the base directory before any wildcards
		parts := strings.Split(absPath, string(filepath.Separator))
		for i, part := range parts {
			if strings.ContainsAny(part, "*?[") {
				// Found wildcard, return path up to this point
				if i > 0 {
					return string(filepath.Separator) + filepath.Join(parts[:i]...)
				}
				return string(filepath.Separator)
			}
		}
		// No wildcards, check if it's a directory
		if info, err := os.Stat(absPath); err == nil && info.IsDir() {
			return absPath
		}
		return filepath.Dir(absPath)
	}
	
	return ""
}

// matchesAnyPatternWithContext checks if a path matches any pattern, considering external paths
func matchesAnyPatternWithContext(absPath, relPath string, patterns []string, workDir string) bool {
	for _, pattern := range patterns {
		if matchesPatternWithContext(absPath, relPath, pattern, workDir) {
			return true
		}
	}
	return false
}

// matchesPatternWithContext checks if a path matches a pattern, handling external paths
func matchesPatternWithContext(absPath, relPath string, pattern string, workDir string) bool {
	cleanPattern := strings.TrimSpace(pattern)
	
	// Handle directory patterns ending with /
	if strings.HasSuffix(cleanPattern, "/") {
		return false // We're only matching files, not directories
	}
	
	// Determine which path to use for matching
	var matchPath string
	if filepath.IsAbs(cleanPattern) {
		// For absolute patterns, use the absolute path
		matchPath = absPath
	} else if strings.HasPrefix(cleanPattern, "../") {
		// For patterns that go up, use relative path from workDir
		matchPath = relPath
	} else {
		// For regular relative patterns
		matchPath = relPath
	}
	
	// Convert paths to forward slashes for consistent matching
	matchPath = filepath.ToSlash(matchPath)
	cleanPattern = filepath.ToSlash(cleanPattern)
	
	return matchesPattern(matchPath, cleanPattern)
}

// buildTreeWithExternalRoots builds a tree that includes external directories
func buildTreeWithExternalRoots(mainRoot string, nodes map[string]*FileNode, walkRoots map[string]bool) *FileNode {
	// Create a synthetic root that will contain both the current directory and external directories
	syntheticRoot := &FileNode{
		Path:     "",
		Name:     "Project Context",
		Status:   StatusDirectory,
		IsDir:    true,
		Children: []*FileNode{},
	}
	
	// Build trees for each walk root separately
	rootTrees := make(map[string]*FileNode)
	
	// First, ensure all directory nodes exist for proper parent-child relationships
	for walkRoot := range walkRoots {
		// Get all nodes belonging to this walk root
		walkRootNodes := make(map[string]*FileNode)
		for path, node := range nodes {
			if strings.HasPrefix(path, walkRoot) || path == walkRoot {
				walkRootNodes[path] = node
			}
		}
		
		// Ensure parent nodes exist within this walk root
		for path := range walkRootNodes {
			ensureParentNodesWithinRoot(walkRoot, path, walkRootNodes)
		}
		
		// Build parent-child relationships within this walk root
		for path, node := range walkRootNodes {
			if path == walkRoot {
				continue
			}
			
			parentPath := filepath.Dir(path)
			if parent, exists := walkRootNodes[parentPath]; exists {
				// Check if child already exists to avoid duplicates
				childExists := false
				for _, existingChild := range parent.Children {
					if existingChild.Path == path {
						childExists = true
						break
					}
				}
				if !childExists {
					parent.Children = append(parent.Children, node)
				}
			}
		}
		
		// Create or get the root node for this walk root
		var rootNode *FileNode
		if existingNode, exists := walkRootNodes[walkRoot]; exists {
			rootNode = existingNode
		} else {
			// Create a new root node if it doesn't exist
			rootNode = &FileNode{
				Path:     walkRoot,
				Name:     filepath.Base(walkRoot),
				Status:   StatusDirectory,
				IsDir:    true,
				Children: []*FileNode{},
			}
			
			// Add direct children
			for path, node := range walkRootNodes {
				if filepath.Dir(path) == walkRoot && path != walkRoot {
					rootNode.Children = append(rootNode.Children, node)
				}
			}
		}
		
		// Set appropriate name for the root node
		if walkRoot == mainRoot {
			rootNode.Name = "." + string(filepath.Separator) + " (current directory)"
		} else {
			// For external directories, show a clearer path
			relPath, err := filepath.Rel(mainRoot, walkRoot)
			if err == nil {
				rootNode.Name = relPath
			} else {
				rootNode.Name = walkRoot
			}
		}
		
		rootTrees[walkRoot] = rootNode
	}
	
	// Add all root trees to the synthetic root
	// First add the main working directory
	if mainRootNode, exists := rootTrees[mainRoot]; exists {
		syntheticRoot.Children = append(syntheticRoot.Children, mainRootNode)
	}
	
	// Then add external directories
	for walkRoot, rootNode := range rootTrees {
		if walkRoot != mainRoot {
			syntheticRoot.Children = append(syntheticRoot.Children, rootNode)
		}
	}
	
	// Sort children at each level
	sortChildren(syntheticRoot)
	
	return syntheticRoot
}

// gitIgnoreCache caches git check-ignore results to avoid repeated calls
var gitIgnoreCache = make(map[string]bool)

// isGitIgnored uses git check-ignore to determine if a path is ignored
func isGitIgnored(path string) bool {
	// Check cache first
	if result, exists := gitIgnoreCache[path]; exists {
		return result
	}
	
	cmd := exec.Command("git", "check-ignore", path)
	err := cmd.Run()
	// git check-ignore returns exit code 0 if the path is ignored
	result := err == nil
	
	// Cache the result
	gitIgnoreCache[path] = result
	return result
}

// ensureParentNodesWithinRoot creates directory nodes for all parents of a path within a specific root
func ensureParentNodesWithinRoot(rootPath string, path string, nodes map[string]*FileNode) {
	if path == rootPath {
		return
	}

	parentPath := filepath.Dir(path)
	if _, exists := nodes[parentPath]; !exists && parentPath != rootPath && strings.HasPrefix(parentPath, rootPath) {
		// Create the parent node
		nodes[parentPath] = &FileNode{
			Path:     parentPath,
			Name:     filepath.Base(parentPath),
			Status:   StatusDirectory,
			IsDir:    true,
			Children: []*FileNode{},
		}
		// Recursively ensure its parents exist
		ensureParentNodesWithinRoot(rootPath, parentPath, nodes)
	}
}
