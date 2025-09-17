package discovery

// Repo represents a single discovered repository.
type Repo struct {
	Name        string // Display name (e.g., "context" or "cx-view-convenience")
	Path        string // Full path to the repository
	ParentPath  string // For worktrees, path to parent repo (empty for main repos)
	IsWorktree  bool   // True if this is a worktree
	Branch      string // Branch name (for worktrees)
	Version     string // Pinned version (e.g., "v1.3.9", "default")
	Commit      string // Resolved commit hash (shortened)
	AuditStatus string // Audit status (e.g., "passed", "failed", "not_audited")
	ReportPath  string // Path to audit report file (if any)
}

// DiscoveredRepos holds categorized lists of discovered repositories.
type DiscoveredRepos struct {
	WorkspaceRepos []Repo
	ClonedRepos    []Repo
}