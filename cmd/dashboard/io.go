package dashboard

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/fsnotify/fsnotify"
	"github.com/mattsolo1/grove-context/pkg/context"
)

// Message types
type statsUpdatedMsg struct {
	hotStats  *context.ContextStats
	coldStats *context.ContextStats
	err       error
}

type fileChangeMsg struct{}

type errorMsg struct {
	err error
}

type tickMsg time.Time

// watchDirectory recursively adds directories to the watcher
func watchDirectory(path string, watcher *fsnotify.Watcher) error {
	// Add the directory itself
	if err := watcher.Add(path); err != nil {
		return err
	}

	// Walk through subdirectories
	entries, err := os.ReadDir(path)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			// Skip common directories that shouldn't be watched
			name := entry.Name()
			if name == ".git" || name == "node_modules" || name == "vendor" ||
				name == "dist" || name == "build" || name == ".grove-context" || name == ".grove-worktrees" {
				continue
			}

			subPath := filepath.Join(path, name)
			if err := watchDirectory(subPath, watcher); err != nil {
				// Ignore errors for individual subdirectories
				continue
			}
		}
	}

	return nil
}

// fetchStatsCmd fetches context statistics
func fetchStatsCmd() tea.Cmd {
	return func() tea.Msg {
		mgr := context.NewManager(".")

		// Get hot context files
		hotFiles, err := mgr.ResolveFilesFromRules()
		if err != nil {
			return statsUpdatedMsg{err: err}
		}

		// Get cold context files
		coldFiles, err := mgr.ResolveColdContextFiles()
		if err != nil {
			return statsUpdatedMsg{err: err}
		}

		// Get stats for both
		hotStats, err := mgr.GetStats("hot", hotFiles, 10)
		if err != nil {
			return statsUpdatedMsg{err: err}
		}

		coldStats, err := mgr.GetStats("cold", coldFiles, 10)
		if err != nil {
			return statsUpdatedMsg{err: err}
		}

		return statsUpdatedMsg{
			hotStats:  hotStats,
			coldStats: coldStats,
		}
	}
}

// waitForActivityCmd waits for file system activity
func (m *dashboardModel) waitForActivityCmd() tea.Cmd {
	return func() tea.Msg {
		select {
		case event, ok := <-m.watcher.Events:
			if !ok {
				return errorMsg{err: fmt.Errorf("watcher closed")}
			}

			// Ignore certain events and files
			if event.Op&fsnotify.Chmod == fsnotify.Chmod {
				return m.waitForActivityCmd()() // Continue waiting
			}

			// Check if it's a relevant file
			if isRelevantFile(event.Name) {
				// Add a small delay to ensure file write is complete
				time.Sleep(50 * time.Millisecond)
				return fileChangeMsg{}
			}

			return m.waitForActivityCmd()() // Continue waiting

		case err, ok := <-m.watcher.Errors:
			if !ok {
				return errorMsg{err: fmt.Errorf("watcher error channel closed")}
			}
			return errorMsg{err: err}
		}
	}
}

// tickCmd sends periodic tick messages
func tickCmd() tea.Cmd {
	return tea.Tick(100*time.Millisecond, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

// isRelevantFile checks if a file change is relevant to context
func isRelevantFile(path string) bool {
	// Clean the path
	path = filepath.Clean(path)
	base := filepath.Base(path)

	// Always relevant files
	if base == ".grove-context.yaml" || base == "CLAUDE.md" {
		return true
	}

	// Check if it's in .grove-context directory
	if strings.Contains(path, ".grove-context/") {
		return false
	}

	// Ignore hidden files and directories
	if strings.HasPrefix(base, ".") {
		return false
	}

	// Ignore common non-relevant directories
	parts := strings.Split(path, string(filepath.Separator))
	for _, part := range parts {
		if part == "node_modules" || part == "vendor" || part == ".git" ||
			part == "dist" || part == "build" || part == "target" ||
			part == "__pycache__" || part == ".pytest_cache" || part == ".grove-worktrees" {
			return false
		}
	}

	// Ignore common binary and temporary files
	ext := strings.ToLower(filepath.Ext(path))
	ignoredExts := []string{".pyc", ".pyo", ".so", ".dylib", ".dll", ".exe", ".bin", ".o", ".a", ".log", ".tmp", ".swp", ".swo", ".DS_Store"}
	for _, ignored := range ignoredExts {
		if ext == ignored {
			return false
		}
	}

	// Consider other files relevant
	return true
}
