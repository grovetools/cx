package discovery

import (
	"path/filepath"
	
	"github.com/mattsolo1/grove-context/pkg/repo"
)

// discoverClonedRepos finds all repositories cloned and managed by `cx repo`.
func discoverClonedRepos() ([]Repo, error) {
	manager, err := repo.NewManager()
	if err != nil {
		return nil, err
	}

	cloned, err := manager.List()
	if err != nil {
		return nil, err
	}

	var repos []Repo
	for _, r := range cloned {
		version := r.PinnedVersion
		if version == "" {
			version = "default"
		}
		
		// Convert relative report path to absolute path
		var reportPath string
		if r.Audit.ReportPath != "" {
			reportPath = filepath.Join(r.LocalPath, r.Audit.ReportPath)
		}
		
		// Shorten commit hash for display
		commit := r.ResolvedCommit
		if len(commit) > 7 {
			commit = commit[:7]
		}
		
		repos = append(repos, Repo{
			Name:        r.URL, // Use URL as the name for cloned repos
			Path:        r.LocalPath,
			Version:     version,
			Commit:      commit,
			AuditStatus: r.Audit.Status,
			ReportPath:  reportPath,
		})
	}

	return repos, nil
}