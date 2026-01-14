package context

import (
	"github.com/mattsolo1/grove-core/pkg/workspace"
)

// IsZombieWorktree checks if the given path is inside a deleted git worktree.
// This is a re-export of workspace.IsZombieWorktree from grove-core for convenience.
//
// A worktree is considered "zombie" if it's inside a .grove-worktrees directory
// but the .git file (which links to the main repo) is missing.
//
// This is used to prevent recreating .grove/rules files in deleted worktrees,
// which would cause "zombie" directories to reappear after cleanup.
var IsZombieWorktree = workspace.IsZombieWorktree

// IsZombieWorktreeCwd checks if the current working directory is inside a zombie worktree.
// This is a re-export of workspace.IsZombieWorktreeCwd from grove-core for convenience.
var IsZombieWorktreeCwd = workspace.IsZombieWorktreeCwd
