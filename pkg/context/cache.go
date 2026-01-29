package context

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/grovetools/core/pkg/paths"
	"github.com/grovetools/core/pkg/repo"
)

const StatsCacheDirName = "stats-cache"

// CachedFileStats holds statistics for a single file in the cache.
type CachedFileStats struct {
	Path   string `json:"path"`
	Size   int64  `json:"size"`
	Tokens int    `json:"tokens"`
}

// RepoCache is the structure of the cache file stored in the stats cache directory.
// It is now keyed by the worktree path, which implicitly includes the commit hash.
type RepoCache struct {
	WorktreePath string                     `json:"worktreePath"`
	CreatedAt    time.Time                  `json:"createdAt"`
	Files        map[string]CachedFileStats `json:"files"` // Use map for faster lookups
}

// getStatsCacheDir returns the path to the centralized stats cache directory.
// It is now a sibling of the 'repos' directory for better organization.
func getStatsCacheDir() (string, error) {
	// Place stats-cache next to repos, not inside it
	cacheDir := filepath.Join(paths.CacheDir(), "cx", StatsCacheDirName)

	// Ensure the directory exists
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return "", err
	}

	return cacheDir, nil
}

// GetCacheFilePathForWorktree returns the path to the cache file for a given worktree path.
// Uses a hash of the worktree path for the filename.
func getCacheFilePathForWorktree(worktreePath string) (string, error) {
	cacheDir, err := getStatsCacheDir()
	if err != nil {
		return "", err
	}

	// Create a hash of the worktree path for a unique, filesystem-safe filename
	hash := sha256.Sum256([]byte(worktreePath))
	hashStr := hex.EncodeToString(hash[:])

	filename := fmt.Sprintf("%s.json", hashStr)

	return filepath.Join(cacheDir, filename), nil
}

// GetCacheFilePathForRepo returns the path to the cache file for a given repository.
// Deprecated: Use getCacheFilePathForWorktree instead. Kept for backwards compatibility.
func GetCacheFilePathForRepo(repoURL string) (string, error) {
	cacheDir, err := getStatsCacheDir()
	if err != nil {
		return "", err
	}

	// Create a hash of the repo URL for the filename
	hash := sha256.Sum256([]byte(repoURL))
	hashStr := hex.EncodeToString(hash[:])[:16] // Use first 16 chars of hash

	// Also create a sanitized version of the URL for the filename (for readability)
	sanitized := strings.ReplaceAll(repoURL, "https://", "")
	sanitized = strings.ReplaceAll(sanitized, "http://", "")
	sanitized = strings.ReplaceAll(sanitized, "/", "_")
	sanitized = strings.ReplaceAll(sanitized, ":", "_")
	sanitized = strings.ReplaceAll(sanitized, ".git", "")

	// Combine sanitized name with hash for uniqueness and readability
	filename := fmt.Sprintf("%s-%s.json", sanitized, hashStr)

	return filepath.Join(cacheDir, filename), nil
}

// StatsProvider provides file statistics, using a cache for managed repositories.
type StatsProvider struct {
	repoManager *repo.Manager
	repoCaches  map[string]*RepoCache // map worktree path -> RepoCache
	mu          sync.RWMutex
}

var (
	globalStatsProvider *StatsProvider
	providerOnce        sync.Once
)

// GetStatsProvider returns a singleton instance of the StatsProvider.
func GetStatsProvider() *StatsProvider {
	providerOnce.Do(func() {
		rm, err := repo.NewManager()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to initialize repo manager for stats provider: %v\n", err)
		}

		provider := &StatsProvider{
			repoManager: rm,
			repoCaches:  make(map[string]*RepoCache),
		}
		globalStatsProvider = provider
	})
	return globalStatsProvider
}

// GetFileStats returns statistics for a file, utilizing the worktree cache if applicable.
func (sp *StatsProvider) GetFileStats(filePath string) (FileInfo, error) {
	absPath, err := filepath.Abs(filePath)
	if err != nil {
		return FileInfo{}, err
	}

	// Check if the file is inside a managed worktree
	worktreePath, isWorktree := sp.findWorktreeForPath(absPath)
	if isWorktree {
		// Get the cache for this worktree (will generate if it doesn't exist)
		cache, err := sp.getWorktreeCache(worktreePath)
		if err != nil {
			// If caching fails, fall back to os.Stat
			return sp.getFileStatsUncached(filePath)
		}

		// Look up the file in the cache
		if stats, ok := cache.Files[absPath]; ok {
			return FileInfo{
				Path:   stats.Path,
				Size:   stats.Size,
				Tokens: stats.Tokens,
			}, nil
		}
	}

	// Fallback for local files or files not found in cache
	return sp.getFileStatsUncached(filePath)
}

// getFileStatsUncached provides file stats using os.Stat as a fallback.
func (sp *StatsProvider) getFileStatsUncached(filePath string) (FileInfo, error) {
	stat, err := os.Stat(filePath)
	if err != nil {
		return FileInfo{}, err
	}
	return FileInfo{
		Path:   filePath,
		Size:   stat.Size(),
		Tokens: int(stat.Size() / 4), // Simple token estimation
	}, nil
}

// findWorktreeForPath determines if a file is inside a cx-managed worktree.
// Returns the worktree root path and true if found, empty string and false otherwise.
func (sp *StatsProvider) findWorktreeForPath(absPath string) (worktreePath string, found bool) {
	const worktreeDir = ".grove-worktrees"
	if !strings.Contains(absPath, worktreeDir) {
		return "", false
	}

	parts := strings.Split(absPath, worktreeDir)
	if len(parts) < 2 {
		return "", false
	}

	// The path after .grove-worktrees will be something like "/{commit-hash}/path/to/file.go"
	subPath := strings.TrimPrefix(parts[1], string(filepath.Separator))
	commitAndFile := strings.SplitN(subPath, string(filepath.Separator), 2)
	if len(commitAndFile) == 0 || commitAndFile[0] == "" {
		return "", false
	}

	// The worktree path is everything up to and including the commit hash directory
	worktreePath = filepath.Join(parts[0], worktreeDir, commitAndFile[0])
	return worktreePath, true
}

// ExtractCommitFromPath extracts the commit hash from a worktree path.
// Returns the commit hash if the path is inside a .grove-worktrees directory, empty string otherwise.
func ExtractCommitFromPath(filePath string) string {
	const worktreeDir = ".grove-worktrees"
	if !strings.Contains(filePath, worktreeDir) {
		return ""
	}

	parts := strings.Split(filePath, worktreeDir)
	if len(parts) < 2 {
		return ""
	}

	// The path after .grove-worktrees will be something like "/{commit-hash}/path/to/file.go"
	subPath := strings.TrimPrefix(parts[1], string(filepath.Separator))
	commitAndFile := strings.SplitN(subPath, string(filepath.Separator), 2)
	if len(commitAndFile) == 0 || commitAndFile[0] == "" {
		return ""
	}

	return commitAndFile[0]
}

// getWorktreeCache retrieves the cache for a worktree, loading from disk or generating it if needed.
func (sp *StatsProvider) getWorktreeCache(worktreePath string) (*RepoCache, error) {
	sp.mu.RLock()
	cache, found := sp.repoCaches[worktreePath]
	sp.mu.RUnlock()

	if found {
		return cache, nil
	}

	// Not in memory, lock for writing
	sp.mu.Lock()
	defer sp.mu.Unlock()

	// Double-check in case another goroutine just populated it
	cache, found = sp.repoCaches[worktreePath]
	if found {
		return cache, nil
	}

	// Try to load from disk
	cacheFilePath, err := getCacheFilePathForWorktree(worktreePath)
	if err != nil {
		return nil, err
	}

	if data, err := os.ReadFile(cacheFilePath); err == nil {
		var loadedCache RepoCache
		if err := json.Unmarshal(data, &loadedCache); err == nil {
			sp.repoCaches[worktreePath] = &loadedCache
			return &loadedCache, nil
		}
	}

	// If not on disk or loading failed, generate it
	newCache, err := sp.generateCacheForWorktree(worktreePath)
	if err != nil {
		return nil, err
	}
	sp.repoCaches[worktreePath] = newCache
	return newCache, nil
}

// generateCacheForWorktree walks a worktree directory, computes stats for all files, and saves the cache.
func (sp *StatsProvider) generateCacheForWorktree(worktreePath string) (*RepoCache, error) {
	cache := &RepoCache{
		WorktreePath: worktreePath,
		CreatedAt:    time.Now(),
		Files:        make(map[string]CachedFileStats),
	}

	err := filepath.WalkDir(worktreePath, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Skip .git directories
		if d.IsDir() && d.Name() == ".git" {
			return filepath.SkipDir
		}

		if d.IsDir() || IsBinaryFile(path) {
			return nil
		}

		stat, err := d.Info()
		if err != nil {
			return nil // Skip files we can't stat
		}

		absPath, err := filepath.Abs(path)
		if err != nil {
			return nil // Skip if we can't get absolute path
		}

		cache.Files[absPath] = CachedFileStats{
			Path:   absPath,
			Size:   stat.Size(),
			Tokens: int(stat.Size() / 4),
		}
		return nil
	})

	if err != nil {
		return nil, err
	}

	// Save to disk
	cacheFilePath, err := getCacheFilePathForWorktree(worktreePath)
	if err != nil {
		return nil, err
	}
	data, err := json.Marshal(cache)
	if err != nil {
		return nil, err
	}
	if err := os.WriteFile(cacheFilePath, data, 0644); err != nil {
		// Non-fatal, just log a warning
		fmt.Fprintf(os.Stderr, "Warning: failed to save stats cache for %s: %v\n", worktreePath, err)
	}

	return cache, nil
}

// IsBinaryFile checks if a file is binary based on extension or content.
func IsBinaryFile(path string) bool {
	// Check common binary file extensions first for performance
	ext := strings.ToLower(filepath.Ext(path))
	if BinaryExtensions[ext] {
		return true
	}
	if ext != "" {
		return false
	}
	basename := filepath.Base(path)
	commonTextFiles := map[string]bool{
		"Makefile": true, "Dockerfile": true, "README": true, "LICENSE": true,
		"Jenkinsfile": true, "Rakefile": true, "Gemfile": true,
	}
	if commonTextFiles[basename] {
		return false
	}
	file, err := os.Open(path)
	if err != nil {
		return false
	}
	defer file.Close()
	buffer := make([]byte, 512)
	n, err := file.Read(buffer)
	if err != nil && err != io.EOF {
		return false
	}
	if n >= 4 {
		if buffer[0] == 0x7f && buffer[1] == 'E' && buffer[2] == 'L' && buffer[3] == 'F' {
			return true
		}
		if (buffer[0] == 0xfe && buffer[1] == 0xed && buffer[2] == 0xfa) || (buffer[0] == 0xce && buffer[1] == 0xfa && buffer[2] == 0xed) {
			return true
		}
		if buffer[0] == 'M' && buffer[1] == 'Z' {
			return true
		}
	}
	for i := 0; i < n; i++ {
		if buffer[i] == 0 {
			return true
		}
	}
	return false
}
