package context

import (
	"path/filepath"
	"testing"

	"github.com/mattsolo1/grove-core/pkg/workspace"
	"github.com/stretchr/testify/assert"
)

// mockProjects provides a consistent set of discovered projects for testing.
func mockProjects() []*workspace.ProjectInfo {
	return []*workspace.ProjectInfo{
		// An ecosystem
		{Name: "my-ecosystem", Path: "/path/to/my-ecosystem", IsEcosystem: true},
		// A repo within the ecosystem
		{Name: "my-repo", Path: "/path/to/my-ecosystem/my-repo", ParentEcosystemPath: "/path/to/my-ecosystem"},
		// A worktree within that repo
		{Name: "feature-branch", Path: "/path/to/my-ecosystem/my-repo/.grove-worktrees/feature-branch", ParentPath: "/path/to/my-ecosystem/my-repo", ParentEcosystemPath: "/path/to/my-ecosystem", IsWorktree: true},
		// A standalone project
		{Name: "standalone-project", Path: "/path/to/standalone-project"},
		// A standalone project with a worktree
		{Name: "standalone-worktree", Path: "/path/to/standalone-project/.grove-worktrees/standalone-worktree", ParentPath: "/path/to/standalone-project", IsWorktree: true},
	}
}

func TestAliasResolver_Resolve(t *testing.T) {
	resolver := &AliasResolver{projects: mockProjects()}

	tests := []struct {
		name      string
		alias     string
		expected  string
		expectErr bool
	}{
		{"1-level ecosystem", "my-ecosystem", "/path/to/my-ecosystem", false},
		{"1-level standalone", "standalone-project", "/path/to/standalone-project", false},
		{"2-level ecosystem:repo", "my-ecosystem:my-repo", "/path/to/my-ecosystem/my-repo", false},
		{"2-level repo:worktree", "standalone-project:standalone-worktree", "/path/to/standalone-project/.grove-worktrees/standalone-worktree", false},
		{"3-level ecosystem:repo:worktree", "my-ecosystem:my-repo:feature-branch", "/path/to/my-ecosystem/my-repo/.grove-worktrees/feature-branch", false},
		{"not found single", "non-existent", "", true},
		{"not found double", "my-ecosystem:non-existent", "", true},
		{"not found triple", "my-ecosystem:my-repo:non-existent", "", true},
		{"invalid format", "a:b:c:d", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// On Windows, paths should use backslashes.
			expected := filepath.FromSlash(tt.expected)

			result, err := resolver.Resolve(tt.alias)
			if tt.expectErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, expected, result)
			}
		})
	}
}

func TestAliasResolver_ResolveLine(t *testing.T) {
	resolver := &AliasResolver{projects: mockProjects()}

	tests := []struct {
		name      string
		line      string
		expected  string
		expectErr bool
	}{
		{"alias with pattern", "@alias:my-ecosystem:my-repo/src/**/*.go", "/path/to/my-ecosystem/my-repo/src/**/*.go", false},
		{"alias with exclusion", "!@alias:standalone-project/vendor/**", "!/path/to/standalone-project/vendor/**", false},
		{"alias only", "@alias:my-ecosystem", "/path/to/my-ecosystem", false},
		{"line without alias", "*.go", "*.go", false},
		{"commented alias", "# @alias:my-ecosystem", "# @alias:my-ecosystem", false},
		{"not found", "@alias:not-found/main.go", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// On Windows, paths should use backslashes.
			expected := filepath.FromSlash(tt.expected)

			result, err := resolver.ResolveLine(tt.line)
			if tt.expectErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, expected, result)
			}
		})
	}
}
