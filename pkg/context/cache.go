package context

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/mattsolo1/grove-core/pkg/repo"
)

const StatsCacheDirName = "stats-cache"

// CachedFileStats holds statistics for a single file in the cache.
type CachedFileStats struct {
	Path   string `json:"path"`
	Size   int64  `json:"size"`
	Tokens int    `json:"tokens"`
}

// RepoCache is the structure of the cache file stored in the stats cache directory.
type RepoCache struct {
	CommitHash string            `json:"commitHash"`
	CreatedAt  time.Time         `json:"createdAt"`
	RepoURL    string            `json:"repoUrl"`
	LocalPath  string            `json:"localPath"`
	Files      []CachedFileStats `json:"files"`
}

// getStatsCacheDir returns the path to the centralized stats cache directory.
func getStatsCacheDir() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	cacheDir := filepath.Join(homeDir, ".grove", "cx", "repos", StatsCacheDirName)

	// Ensure the directory exists
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return "", err
	}

	return cacheDir, nil
}

// GetCacheFilePathForRepo returns the path to the cache file for a given repository.
// Uses a hash of the repo URL for the filename to avoid filesystem issues.
// This is exported for use by cache invalidation logic.
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
	repoCaches  map[string]*RepoCache // map workspace path -> RepoCache
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
			// This will cause panics if not handled, but NewManager rarely fails.
			// For CLI tools, this is acceptable; errors will surface during use.
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

// GetFileStats returns statistics for a file.
// TODO: Re-implement caching using workspace paths instead of RepoInfo
func (sp *StatsProvider) GetFileStats(filePath string) (FileInfo, error) {
	// TODO: Implement workspace-based caching
	// For now, fall back to os.Stat for all files
	stat, err := os.Stat(filePath)
	if err != nil {
		return FileInfo{}, err
	}
	return FileInfo{
		Path:   filePath,
		Size:   stat.Size(),
		Tokens: int(stat.Size() / 4),
	}, nil
}

// TODO: Re-implement these functions using workspace-based caching
// For now, these are disabled and GetFileStats falls back to os.Stat

// getRepoCache would retrieve the cache for a workspace
func (sp *StatsProvider) getRepoCache(workspacePath, commitHash string) (*RepoCache, error) {
	sp.mu.RLock()
	cache, found := sp.repoCaches[workspacePath]
	sp.mu.RUnlock()

	if found && cache.CommitHash == commitHash {
		return cache, nil
	}

	// Not in memory or stale, lock for writing
	sp.mu.Lock()
	defer sp.mu.Unlock()

	// Double-check in case another goroutree just populated it
	cache, found = sp.repoCaches[workspacePath]
	if found && cache.CommitHash == commitHash {
		return cache, nil
	}

	// TODO: Implement workspace-based caching
	return nil, fmt.Errorf("workspace caching not yet implemented")
}

// IsBinaryFile is copied from resolve.go to be used here.
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
