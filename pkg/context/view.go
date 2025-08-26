package context

import (
	"fmt"
	"os"
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

	// Parse rules to get all patterns (for determining if a file was excluded by rule)
	var allIncludePatterns []string
	rulesPath := filepath.Join(m.workDir, ActiveRulesFile)
	if _, err := os.Stat(rulesPath); err == nil {
		mainPatterns, coldPatterns, _, _, _, _, err := m.parseRulesFile(rulesPath)
		if err == nil {
			// Collect all include patterns (non-negated ones)
			for _, p := range append(mainPatterns, coldPatterns...) {
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
				for _, p := range append(mainPatterns, coldPatterns...) {
					if !strings.HasPrefix(p, "!") {
						allIncludePatterns = append(allIncludePatterns, p)
					}
				}
			}
		}
	}

	// Walk the directory tree and collect all nodes
	nodes := make(map[string]*FileNode)
	err = filepath.WalkDir(m.workDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil // Skip files we can't read
		}

		// Skip .git and .grove directories
		relPath, _ := filepath.Rel(m.workDir, path)
		
		// Skip the root directory entry itself
		if path == m.workDir {
			return nil
		}
		
		// Check directory names for exclusion
		if d.IsDir() && (d.Name() == ".git" || d.Name() == ".grove") {
			return filepath.SkipDir
		}

		// Create node
		node := &FileNode{
			Path:     path,
			Name:     d.Name(),
			IsDir:    d.IsDir(),
			Children: []*FileNode{},
		}

		// Determine status
		if d.IsDir() {
			node.Status = StatusDirectory
		} else if gitIgnoredFiles[path] {
			node.Status = StatusIgnoredByGit
		} else if coldFilesMap[path] {
			node.Status = StatusIncludedCold
		} else if hotFilesMap[path] {
			node.Status = StatusIncludedHot
		} else if matchesAnyPattern(relPath, allIncludePatterns) {
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
		return nil, fmt.Errorf("failed to walk directory: %w", err)
	}

	// Build the tree structure
	root := buildTree(m.workDir, nodes)
	
	// If root has no children but we have nodes, something went wrong
	// Let's add direct children of workDir that might have been missed
	if len(root.Children) == 0 && len(nodes) > 0 {
		for path, node := range nodes {
			if filepath.Dir(path) == m.workDir {
				root.Children = append(root.Children, node)
			}
		}
		sortChildren(root)
	}
	
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
