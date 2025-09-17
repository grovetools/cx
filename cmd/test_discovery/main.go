package main

import (
	"fmt"
	"github.com/mattsolo1/grove-context/pkg/discovery"
	"path/filepath"
)

func main() {
	repos, err := discovery.DiscoverAllRepos()
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}
	
	fmt.Println("Compact Tabular Display (gmux-style):")
	fmt.Println("=====================================")
	fmt.Println()
	
	// Simulate the compact display
	maxNameWidth := 30
	count := 0
	lastParent := ""
	
	// Display workspace repos
	for _, repo := range repos.WorkspaceRepos {
		count++
		
		// Add spacing between different parent repos
		if !repo.IsWorktree && lastParent != "" {
			fmt.Println()
		}
		if !repo.IsWorktree {
			lastParent = repo.Path
		}
		
		// Format name with worktree indicator
		name := repo.Name
		if repo.IsWorktree {
			name = "└─ " + name
		}
		
		// Format path
		path := repo.Path
		if repo.IsWorktree && repo.ParentPath != "" {
			if rel, err := filepath.Rel(repo.ParentPath, repo.Path); err == nil {
				path = "./" + rel
			}
		}
		
		// Truncate path if too long
		if len(path) > 50 {
			path = "..." + path[len(path)-47:]
		}
		
		fmt.Printf("  %-*s  %s\n", maxNameWidth, name, path)
		
		if count >= 15 {
			fmt.Printf("\n  ... and %d more workspace repos\n", len(repos.WorkspaceRepos)-15)
			break
		}
	}
	
	if len(repos.ClonedRepos) > 0 {
		fmt.Println("\n─────────────────────────────────────────────────")
		fmt.Println()
		for _, repo := range repos.ClonedRepos {
			name := repo.Name
			// Shorten URL names
			if len(name) > 40 {
				name = name[:37] + "..."
			}
			fmt.Printf("  %-*s  %s\n", maxNameWidth, name, repo.Path)
		}
	}
	
	fmt.Println("\n[Navigation: ↑/↓ • Filter: / • Select: enter]")
}