package tree

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/mattsolo1/grove-context/pkg/context"
)

// NodeStatus is an alias for context.NodeStatus
type NodeStatus = context.NodeStatus

// Re-export NodeStatus constants for convenience
const (
	StatusIncludedHot    = context.StatusIncludedHot
	StatusIncludedCold   = context.StatusIncludedCold
	StatusExcludedByRule = context.StatusExcludedByRule
	StatusOmittedNoMatch = context.StatusOmittedNoMatch
	StatusIgnoredByGit   = context.StatusIgnoredByGit
	StatusDirectory      = context.StatusDirectory
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
func AnalyzeProjectTree(m *context.Manager, prune bool, showGitIgnored bool) (*FileNode, error) {
	// Use the centralized engine to get all file classifications
	fileStatuses, err := m.ResolveAndClassifyAllFiles(prune)
	if err != nil {
		return nil, err
	}

	// If showing gitignored files, we need to walk the entire tree to find them
	if showGitIgnored {
		gitIgnoredFiles, err := m.GetGitIgnoredFiles(m.GetWorkDir())
		if err == nil {
			// Walk the entire working directory to find all gitignored files
			err = filepath.WalkDir(m.GetWorkDir(), func(path string, d os.DirEntry, err error) error {
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
		relPath, err := filepath.Rel(m.GetWorkDir(), path)
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
		relPath, err := filepath.Rel(m.GetWorkDir(), path)
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
			relPath, err := filepath.Rel(m.GetWorkDir(), node.Path)
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
		root = buildTreeWithSyntheticRoot(m.GetWorkDir(), nodes)
	} else {
		// No external files, use the working directory as root
		root = buildTreeWithExternals(m.GetWorkDir(), nodes)
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
