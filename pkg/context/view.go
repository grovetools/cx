package context

import (
	"path/filepath"
	"sort"
	"strings"
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
	// Use the centralized engine to get all file classifications
	fileStatuses, err := m.ResolveAndClassifyAllFiles()
	if err != nil {
		return nil, err
	}
	
	// Build FileNode map from the classifications
	nodes := make(map[string]*FileNode)
	hasExternalFiles := false
	
	for path, status := range fileStatuses {
		// Skip StatusIgnoredByGit - we don't want to show git-ignored files
		if status == StatusIgnoredByGit {
			continue
		}
		
		// Check if this is an external file
		relPath, err := filepath.Rel(m.workDir, path)
		if err != nil || strings.HasPrefix(relPath, "..") {
			hasExternalFiles = true
		}
		
		node := &FileNode{
			Path:     path,
			Name:     filepath.Base(path),
			Status:   status,
			IsDir:    status == StatusDirectory,
			Children: []*FileNode{},
		}
		nodes[path] = node
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
	
	// For external directories, we want to show them at the root level
	// but in a condensed way. Find the /Users node and add it as a direct child of root
	for path, node := range nodes {
		if path == "/Users" {
			// Check if it's already a child of root
			isChild := false
			for _, child := range root.Children {
				if child.Path == path {
					isChild = true
					break
				}
			}
			if !isChild {
				root.Children = append(root.Children, node)
			}
		}
	}

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