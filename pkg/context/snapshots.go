package context

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// SnapshotInfo contains information about a context snapshot
type SnapshotInfo struct {
	Name        string
	ModTime     time.Time
	FileCount   int
	TotalTokens int
	TotalSize   int64
	Description string
}

// ListSnapshots returns information about all saved snapshots
func (m *Manager) ListSnapshots() ([]SnapshotInfo, error) {
	// Check if snapshots directory exists
	snapshotsDir := filepath.Join(m.workDir, SnapshotsDir)
	if _, err := os.Stat(snapshotsDir); os.IsNotExist(err) {
		return nil, nil // No snapshots
	}

	// Read all snapshots
	entries, err := os.ReadDir(snapshotsDir)
	if err != nil {
		return nil, fmt.Errorf("error reading snapshots directory: %w", err)
	}

	// Collect snapshot information
	var snapshots []SnapshotInfo
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		// Skip hidden files and description files
		name := entry.Name()
		if strings.HasPrefix(name, ".") || strings.HasSuffix(name, ".desc") {
			continue
		}

		// Only process .rules files or files without extension (backward compatibility)
		if !strings.HasSuffix(name, ".rules") && strings.Contains(name, ".") {
			continue
		}

		info, err := m.getSnapshotInfo(name)
		if err != nil {
			// Skip files that can't be read
			continue
		}

		snapshots = append(snapshots, info)
	}

	return snapshots, nil
}

// getSnapshotInfo returns information about a specific snapshot
func (m *Manager) getSnapshotInfo(name string) (SnapshotInfo, error) {
	// Clean the display name
	displayName := strings.TrimSuffix(name, ".rules")
	info := SnapshotInfo{Name: displayName}

	// Get file info
	snapshotPath := filepath.Join(m.workDir, SnapshotsDir, name)
	stat, err := os.Stat(snapshotPath)
	if err != nil {
		return info, err
	}
	info.ModTime = stat.ModTime()

	// For .rules files, dynamically resolve the file list
	if strings.HasSuffix(name, ".rules") {
		// Resolve files from the snapshot rules
		hotFiles, coldFiles, err := m.ResolveFilesFromCustomRulesFile(snapshotPath)
		if err != nil {
			// If we can't resolve, just return minimal info
			info.FileCount = 0
			info.TotalTokens = 0
			info.TotalSize = 0
		} else {
			allFiles := append(hotFiles, coldFiles...)
			info.FileCount = len(allFiles)

			// Calculate total tokens and size
			for _, file := range allFiles {
				filePath := filepath.Join(m.workDir, file)
				if fileStat, err := os.Stat(filePath); err == nil {
					info.TotalSize += fileStat.Size()
					// Rough estimate: 4 characters per token
					info.TotalTokens += int(fileStat.Size() / 4)
				}
			}
		}
	} else {
		// Backward compatibility: old snapshot format
		files, err := m.ReadFilesList(snapshotPath)
		if err != nil {
			return info, err
		}
		info.FileCount = len(files)

		// Calculate total tokens and size
		for _, file := range files {
			if fileStat, err := os.Stat(file); err == nil {
				info.TotalSize += fileStat.Size()
				// Rough estimate: 4 characters per token
				info.TotalTokens += int(fileStat.Size() / 4)
			}
		}
	}

	// Try to read description file if it exists
	descPath := filepath.Join(m.workDir, SnapshotsDir, name+".desc")
	if descContent, err := os.ReadFile(descPath); err == nil {
		info.Description = strings.TrimSpace(string(descContent))
	}

	return info, nil
}

// SortSnapshots sorts a slice of snapshots based on the given criteria
func SortSnapshots(snapshots []SnapshotInfo, sortBy string, descending bool) {
	switch sortBy {
	case "name":
		sort.Slice(snapshots, func(i, j int) bool {
			if descending {
				return snapshots[i].Name > snapshots[j].Name
			}
			return snapshots[i].Name < snapshots[j].Name
		})
	case "size":
		sort.Slice(snapshots, func(i, j int) bool {
			if descending {
				return snapshots[i].TotalSize > snapshots[j].TotalSize
			}
			return snapshots[i].TotalSize < snapshots[j].TotalSize
		})
	case "tokens":
		sort.Slice(snapshots, func(i, j int) bool {
			if descending {
				return snapshots[i].TotalTokens > snapshots[j].TotalTokens
			}
			return snapshots[i].TotalTokens < snapshots[j].TotalTokens
		})
	case "files":
		sort.Slice(snapshots, func(i, j int) bool {
			if descending {
				return snapshots[i].FileCount > snapshots[j].FileCount
			}
			return snapshots[i].FileCount < snapshots[j].FileCount
		})
	default: // date
		sort.Slice(snapshots, func(i, j int) bool {
			if descending {
				return snapshots[i].ModTime.After(snapshots[j].ModTime)
			}
			return snapshots[i].ModTime.Before(snapshots[j].ModTime)
		})
	}
}

// PrintSnapshots displays snapshots in a formatted table
func PrintSnapshots(snapshots []SnapshotInfo) {
	if len(snapshots) == 0 {
		fmt.Println("No snapshots found. Use 'grove cx save <name>' to create a snapshot.")
		return
	}

	fmt.Println("Available snapshots:")
	fmt.Println()
	fmt.Printf("%-20s %-12s %-6s %-8s %-9s %s\n",
		"NAME", "DATE", "FILES", "TOKENS", "SIZE", "DESCRIPTION")
	fmt.Println(strings.Repeat("-", 80))

	for _, snap := range snapshots {
		dateStr := snap.ModTime.Format("2006-01-02")
		fmt.Printf("%-20s %-12s %-6d %-8s %-9s %s\n",
			truncateName(snap.Name, 20),
			dateStr,
			snap.FileCount,
			FormatTokenCount(snap.TotalTokens),
			FormatBytes(int(snap.TotalSize)),
			snap.Description,
		)
	}
}

// truncateName shortens a name for display
func truncateName(name string, maxLen int) string {
	if len(name) <= maxLen {
		return name
	}
	return name[:maxLen-3] + "..."
}
