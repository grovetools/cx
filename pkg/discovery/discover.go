package discovery

import "sync"

// DiscoverAllRepos finds repositories from all available sources (workspace, cloned repos).
func DiscoverAllRepos() (*DiscoveredRepos, error) {
	var wg sync.WaitGroup
	var workspaceRepos []Repo
	var clonedRepos []Repo
	var workspaceErr, clonedErr error

	wg.Add(2)

	go func() {
		defer wg.Done()
		workspaceRepos, workspaceErr = discoverWorkspaceRepos()
	}()

	go func() {
		defer wg.Done()
		clonedRepos, clonedErr = discoverClonedRepos()
	}()

	wg.Wait()

	if workspaceErr != nil {
		// Log or handle error, but don't fail the entire discovery
	}
	if clonedErr != nil {
		// Log or handle error
	}

	return &DiscoveredRepos{
		WorkspaceRepos: workspaceRepos,
		ClonedRepos:    clonedRepos,
	}, nil
}