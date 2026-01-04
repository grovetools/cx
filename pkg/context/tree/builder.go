package tree

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/mattsolo1/grove-context/pkg/context"
	"github.com/mattsolo1/grove-core/pkg/profiling"
	"github.com/mattsolo1/grove-core/util/pathutil"
)

// normalizePathKey returns a normalized version of the path for use as a map key
// to handle case-insensitive filesystems. It doesn't change the actual path,
// just provides a consistent key for deduplication.
func normalizePathKey(path string) string {
	// Normalize the path for case-insensitive filesystems and symlink resolution
	// This prevents duplicate entries on case-insensitive filesystems
	normalizedPath, err := pathutil.NormalizeForLookup(path)
	if err != nil {
		// Fallback to cleaning the path if normalization fails
		return filepath.Clean(path)
	}
	return normalizedPath
}

type FileNode struct {
	Path       string
	Name       string
	Status     context.NodeStatus
	IsDir      bool
	TokenCount int
	Children   []*FileNode
}

// AnalyzeProjectTree walks the entire project and creates a tree structure showing
// which files are included, excluded, or ignored based on context rules
func AnalyzeProjectTree(m *context.Manager, showGitIgnored bool) (*FileNode, error) {
	defer profiling.Start("tree.AnalyzeProjectTree").Stop()

	// Use the new unified classification engine to get all file classifications
	classifyStopper := profiling.Start("tree.ClassifyAllProjectFiles")
	fileStatuses, err := m.ClassifyAllProjectFiles(showGitIgnored)
	classifyStopper.Stop()
	if err != nil {
		return nil, err
	}

	statsProvider := context.GetStatsProvider()

	// Get canonical workDir for consistent path comparisons
	// This is critical on macOS where /var is a symlink to /private/var
	workDir := m.GetWorkDir()
	workDirCanonical := workDir
	if wd, err := filepath.EvalSymlinks(workDir); err == nil {
		workDirCanonical = wd
	}

	// Build FileNode map from the classifications
	// Use normalized paths as keys to handle case-insensitive filesystems
	nodes := make(map[string]*FileNode)
	pathMapping := make(map[string]string) // normalized -> original path
	hasExternalFiles := false
	hasExternalViewPaths := false

	// Check if any included files (hot/cold) are actually external to workDir
	// This is only true if we have @view directives pointing to external paths
	for path, status := range fileStatuses {
		if status == context.StatusIncludedHot || status == context.StatusIncludedCold {
			relPath, err := filepath.Rel(workDirCanonical, path)
			if err != nil || strings.HasPrefix(relPath, "..") {
				// This is an included file outside workDir - must be from @view directive
				hasExternalViewPaths = true
				break
			}
		}
	}

	for path, status := range fileStatuses {
		// Skip context.StatusIgnoredByGit unless explicitly requested
		if status == context.StatusIgnoredByGit && !showGitIgnored {
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
		relPath, err := filepath.Rel(workDirCanonical, path)
		isExternal := err != nil || strings.HasPrefix(relPath, "..")
		if isExternal && (status == context.StatusIncludedHot || status == context.StatusIncludedCold) {
			hasExternalFiles = true
		}

		// For directories outside workDir, only include them if they have included descendants
		// We'll handle this filtering later, for now just create the node
		isDir := status == context.StatusDirectory
		// Check if excluded items are actually directories
		if status == context.StatusExcludedByRule {
			if info, err := os.Stat(path); err == nil && info.IsDir() {
				isDir = true
			}
		}
		tokenCount := 0
		// Calculate token count for included files
		if !isDir && (status == context.StatusIncludedHot || status == context.StatusIncludedCold) {
			if info, err := statsProvider.GetFileStats(path); err == nil {
				tokenCount = info.Tokens
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
			relPath, err := filepath.Rel(workDirCanonical, node.Path)
			isExternal := err != nil || strings.HasPrefix(relPath, "..")
			if !isExternal {
				filteredNodes[nodeKey] = node
			}
		}
		nodes = filteredNodes
	}

	buildStopper := profiling.Start("tree.BuildTreeStructure")
	// Build the tree structure
	var root *FileNode
	if hasExternalFiles || hasExternalViewPaths {
		// Create a synthetic root to show CWD and external paths as siblings
		root = buildTreeWithSyntheticRoot(workDirCanonical, nodes)
	} else {
		// No external files, use the working directory as root
		root = buildTreeWithExternals(workDirCanonical, nodes)
	}
	buildStopper.Stop()

	postProcessStopper := profiling.Start("tree.PostProcess")
	// Post-process the tree to infer directory statuses from their children
	setDirectoryStatuses(root)

	// Calculate directory token counts
	calculateDirectoryTokenCounts(root)
	postProcessStopper.Stop()

	return root, nil
}

// buildTreeWithSyntheticRoot creates a synthetic root showing the filesystem hierarchy
func buildTreeWithSyntheticRoot(workDir string, nodes map[string]*FileNode) *FileNode {
	// Create a synthetic root node
	syntheticRoot := &FileNode{
		Path:     "/",
		Name:     "/",
		Status:   context.StatusDirectory,
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

	// Add common top-level directories to synthetic root
	// Check for common Unix top-level paths (not hardcoded to /Users which fails in test environments)
	commonTopLevelDirs := []string{"/Users", "/var", "/private", "/tmp", "/home", "/opt", "/usr"}
	for _, topLevelPath := range commonTopLevelDirs {
		topLevelKey := normalizePathKey(topLevelPath)
		if topLevelNode, exists := nodes[topLevelKey]; exists {
			syntheticRoot.Children = append(syntheticRoot.Children, topLevelNode)
		}
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
			Status:   context.StatusDirectory,
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
			Status:   context.StatusDirectory,
			IsDir:    true,
			Children: []*FileNode{},
		}
		// Recursively ensure its parents exist
		ensureParentNodes(rootPath, parentPath, nodes)
	}
}

// sortChildren recursively sorts all children nodes
func sortChildren(node *FileNode) {
	if node == nil || len(node.Children) == 0 {
		return
	}

	// Sort: directories first, then by name
	sort.Slice(node.Children, func(i, j int) bool {
		// Nil check for safety
		if node.Children[i] == nil || node.Children[j] == nil {
			return false
		}
		if node.Children[i].IsDir != node.Children[j].IsDir {
			return node.Children[i].IsDir
		}
		return strings.ToLower(node.Children[i].Name) < strings.ToLower(node.Children[j].Name)
	})

	// Recursively sort children
	for _, child := range node.Children {
		if child != nil && child.IsDir {
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
	if !node.IsDir || node.Status == context.StatusExcludedByRule {
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
		case context.StatusIncludedHot:
			hotCount++
		case context.StatusIncludedCold:
			coldCount++
		case context.StatusExcludedByRule:
			excludedCount++
		case context.StatusOmittedNoMatch:
			omittedCount++
		}
	}

	// Set directory status based on predominant child status
	// Priority: Hot > Cold > Excluded > Omitted
	if hotCount > 0 {
		node.Status = context.StatusIncludedHot
	} else if coldCount > 0 {
		node.Status = context.StatusIncludedCold
	} else if excludedCount > 0 {
		node.Status = context.StatusExcludedByRule
	} else if omittedCount > 0 {
		node.Status = context.StatusOmittedNoMatch
	}
}
