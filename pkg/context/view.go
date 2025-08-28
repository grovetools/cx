package context

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

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
	
	// Build FileNode map from the classifications
	nodes := make(map[string]*FileNode)
	hasExternalFiles := false
	
	for path, status := range fileStatuses {
		// Skip StatusIgnoredByGit unless explicitly requested
		if status == StatusIgnoredByGit && !showGitIgnored {
			continue
		}
		
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
		nodes[path] = node
	}
	
	// If no external files are included, remove external directory nodes
	if !hasExternalFiles {
		filteredNodes := make(map[string]*FileNode)
		for path, node := range nodes {
			relPath, err := filepath.Rel(m.workDir, path)
			isExternal := err != nil || strings.HasPrefix(relPath, "..")
			if !isExternal {
				filteredNodes[path] = node
			}
		}
		nodes = filteredNodes
	}
	
	// Build the tree structure
	var root *FileNode
	if hasExternalFiles {
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
	for path := range nodes {
		ensureParentNodes("", path, nodes)
	}
	
	// Build parent-child relationships
	for path, node := range nodes {
		parentPath := filepath.Dir(path)
		
		if parent, exists := nodes[parentPath]; exists {
			parent.Children = append(parent.Children, node)
		}
	}
	
	// Mark the CWD node with (CWD) suffix to make it clear
	if cwdNode, exists := nodes[workDir]; exists {
		cwdNode.Name = filepath.Base(workDir) + " (CWD)"
	}
	
	// Add /Users as the only child of synthetic root
	for path, node := range nodes {
		if path == "/Users" {
			syntheticRoot.Children = append(syntheticRoot.Children, node)
			break
		}
	}
	
	// Sort children at each level
	sortChildren(syntheticRoot)
	
	return syntheticRoot
}


// buildTreeWithExternals constructs a hierarchical tree that includes external directories
func buildTreeWithExternals(rootPath string, nodes map[string]*FileNode) *FileNode {
	// Get the root node from nodes map, or create it
	root, exists := nodes[rootPath]
	if !exists {
		root = &FileNode{
			Path:     rootPath,
			Name:     filepath.Base(rootPath),
			Status:   StatusDirectory,
			IsDir:    true,
			Children: []*FileNode{},
		}
		nodes[rootPath] = root
	}
	
	// First, ensure all directory nodes exist
	for path := range nodes {
		ensureParentNodes("", path, nodes)
	}

	// Build parent-child relationships
	for path, node := range nodes {
		if path == rootPath {
			continue // Skip the root itself
		}
		
		parentPath := filepath.Dir(path)
		
		if parent, exists := nodes[parentPath]; exists {
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
	if _, exists := nodes[parentPath]; !exists {
		// Don't create parent if we've reached the rootPath
		if rootPath != "" && parentPath == rootPath {
			return
		}
		
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