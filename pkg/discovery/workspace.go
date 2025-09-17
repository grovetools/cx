package discovery

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

// workspaceInfo matches the JSON output structure of `grove ws list --json`.
type workspaceInfo struct {
	Name      string         `json:"name"`
	Path      string         `json:"path"`
	Worktrees []worktreeInfo `json:"worktrees"`
}

// worktreeInfo represents a git worktree
type worktreeInfo struct {
	Path   string `json:"path"`
	Branch string `json:"branch"`
	IsMain bool   `json:"is_main"`
}

// discoverWorkspaceRepos finds all sub-repositories in the current Grove ecosystem.
func discoverWorkspaceRepos() ([]Repo, error) {
	// First try using grove ws list --json command
	cmd := exec.Command("grove", "ws", "list", "--json")
	output, err := cmd.Output()
	if err == nil {
		var workspaces []workspaceInfo
		if err := json.Unmarshal(output, &workspaces); err == nil {
			var repos []Repo
			
			// Sort workspaces for consistent ordering
			sort.Slice(workspaces, func(i, j int) bool {
				return workspaces[i].Name < workspaces[j].Name
			})
			
			for _, ws := range workspaces {
				shortName := getShortName(ws.Name)
				
				// Add the main repository first
				for _, wt := range ws.Worktrees {
					if wt.IsMain {
						repos = append(repos, Repo{
							Name:       shortName,
							Path:       wt.Path,
							ParentPath: "",
							IsWorktree: false,
							Branch:     wt.Branch,
						})
						break
					}
				}
				
				// Then add worktrees immediately after their parent
				for _, wt := range ws.Worktrees {
					if !wt.IsMain {
						repos = append(repos, Repo{
							Name:       wt.Branch,  // Just use branch name for worktrees
							Path:       wt.Path,
							ParentPath: ws.Path,
							IsWorktree: true,
							Branch:     wt.Branch,
						})
					}
				}
			}
			return repos, nil
		}
	}

	// Fallback: Try to find grove ecosystem directory by looking for grove.yml
	// Walk up the directory tree to find the grove ecosystem root
	cwd, err := os.Getwd()
	if err != nil {
		return []Repo{}, nil
	}

	ecosystemRoot := findGroveEcosystemRoot(cwd)
	if ecosystemRoot == "" {
		return []Repo{}, nil
	}

	// List grove-* directories in the ecosystem root
	entries, err := os.ReadDir(ecosystemRoot)
	if err != nil {
		return []Repo{}, nil
	}

	var repos []Repo
	for _, entry := range entries {
		if entry.IsDir() && strings.HasPrefix(entry.Name(), "grove-") {
			fullPath := filepath.Join(ecosystemRoot, entry.Name())
			shortName := getShortName(entry.Name())
			
			// Add the main repository
			repos = append(repos, Repo{
				Name:       shortName,
				Path:       fullPath,
				ParentPath: "",
				IsWorktree: false,
				Branch:     "main",
			})
			
			// Check for worktrees in .grove-worktrees directory
			worktreesPath := filepath.Join(fullPath, ".grove-worktrees")
			if worktreeEntries, err := os.ReadDir(worktreesPath); err == nil {
				for _, wtEntry := range worktreeEntries {
					if wtEntry.IsDir() {
						wtPath := filepath.Join(worktreesPath, wtEntry.Name())
						// Get branch name from git if possible, otherwise use directory name
						branchName := wtEntry.Name()
						
						repos = append(repos, Repo{
							Name:       branchName,
							Path:       wtPath,
							ParentPath: fullPath,
							IsWorktree: true,
							Branch:     branchName,
						})
					}
				}
			}
		}
	}

	return repos, nil
}

// findGroveEcosystemRoot walks up the directory tree to find the grove ecosystem root
func findGroveEcosystemRoot(startPath string) string {
	current := startPath
	for {
		groveYml := filepath.Join(current, "grove.yml")
		if _, err := os.Stat(groveYml); err == nil {
			// Also check if there are grove-* directories here
			entries, err := os.ReadDir(current)
			if err == nil {
				for _, entry := range entries {
					if entry.IsDir() && strings.HasPrefix(entry.Name(), "grove-") {
						return current
					}
				}
			}
		}

		parent := filepath.Dir(current)
		if parent == current {
			// Reached root directory
			break
		}
		current = parent
	}
	return ""
}

// getShortName converts grove repository names to their short names
func getShortName(repoName string) string {
	// Remove "grove-" prefix for display
	return strings.TrimPrefix(repoName, "grove-")
}