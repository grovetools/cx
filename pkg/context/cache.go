package context

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
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
	repoRoots   map[string]repo.RepoInfo // map local path -> RepoInfo
	repoCaches  map[string]*RepoCache    // map local path -> RepoCache
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
			repoRoots:   make(map[string]repo.RepoInfo),
			repoCaches:  make(map[string]*RepoCache),
		}

		if rm != nil {
			manifest, err := rm.LoadManifest()
			if err == nil {
				for _, repoInfo := range manifest.Repositories {
					provider.repoRoots[repoInfo.LocalPath] = repoInfo
				}
			}
		}
		globalStatsProvider = provider
	})
	return globalStatsProvider
}

// GetFileStats returns statistics for a file. It uses the cache if the file
// is in a managed repository, otherwise falls back to os.Stat.
func (sp *StatsProvider) GetFileStats(filePath string) (FileInfo, error) {
	absFilePath, err := filepath.Abs(filePath)
	if err != nil {
		return FileInfo{}, err
	}

	repoInfo, inRepo := sp.findRepoForPath(absFilePath)
	if !inRepo {
		// Not in a managed repo, fall back to os.Stat
		stat, err := os.Stat(absFilePath)
		if err != nil {
			return FileInfo{}, err
		}
		return FileInfo{
			Path:   filePath, // Return original path
			Size:   stat.Size(),
			Tokens: int(stat.Size() / 4),
		}, nil
	}

	// It's in a repo, use the cache
	cache, err := sp.getRepoCache(repoInfo)
	if err != nil {
		return FileInfo{}, fmt.Errorf("could not get repo cache for %s: %w", repoInfo.URL, err)
	}

	relPath, err := filepath.Rel(repoInfo.LocalPath, absFilePath)
	if err != nil {
		return FileInfo{}, err
	}
	relPath = filepath.ToSlash(relPath)

	// Find file in cache (could optimize with a map if needed)
	for _, f := range cache.Files {
		if f.Path == relPath {
			return FileInfo{
				Path:   filePath, // Return original path
				Size:   f.Size,
				Tokens: f.Tokens,
			}, nil
		}
	}

	return FileInfo{}, fmt.Errorf("file %s not found in cache for repo %s", relPath, repoInfo.URL)
}

// findRepoForPath checks if a file path is within a managed repository.
func (sp *StatsProvider) findRepoForPath(absFilePath string) (*repo.RepoInfo, bool) {
	sp.mu.RLock()
	defer sp.mu.RUnlock()
	for root, info := range sp.repoRoots {
		// Ensure root has a trailing separator for accurate matching
		rootWithSep := root
		if !strings.HasSuffix(rootWithSep, string(filepath.Separator)) {
			rootWithSep += string(filepath.Separator)
		}

		// Check if file is within this repo directory
		if strings.HasPrefix(absFilePath, rootWithSep) || absFilePath == root {
			repoInfo := info // Make a copy
			return &repoInfo, true
		}
	}
	return nil, false
}

// getRepoCache retrieves the cache for a repo, from memory, disk, or by generating it.
func (sp *StatsProvider) getRepoCache(repoInfo *repo.RepoInfo) (*RepoCache, error) {
	sp.mu.RLock()
	cache, found := sp.repoCaches[repoInfo.LocalPath]
	sp.mu.RUnlock()

	if found && cache.CommitHash == repoInfo.ResolvedCommit {
		return cache, nil
	}

	// Not in memory or stale, lock for writing
	sp.mu.Lock()
	defer sp.mu.Unlock()

	// Double-check in case another goroutine just populated it
	cache, found = sp.repoCaches[repoInfo.LocalPath]
	if found && cache.CommitHash == repoInfo.ResolvedCommit {
		return cache, nil
	}

	// Try loading from disk
	cachePath, err := GetCacheFilePathForRepo(repoInfo.URL)
	if err != nil {
		return nil, fmt.Errorf("failed to get cache path: %w", err)
	}

	if data, err := os.ReadFile(cachePath); err == nil {
		var diskCache RepoCache
		if json.Unmarshal(data, &diskCache) == nil {
			if diskCache.CommitHash == repoInfo.ResolvedCommit {
				sp.repoCaches[repoInfo.LocalPath] = &diskCache
				return &diskCache, nil
			}
		}
	}

	// Not on disk or stale, generate new cache
	newCache, err := sp.generateCacheForRepo(repoInfo)
	if err != nil {
		return nil, err
	}

	sp.repoCaches[repoInfo.LocalPath] = newCache
	return newCache, nil
}

// generateCacheForRepo walks a repository, gets stats for all files, and saves the cache.
func (sp *StatsProvider) generateCacheForRepo(repoInfo *repo.RepoInfo) (*RepoCache, error) {
	// Use `git ls-files` which is fast and respects .gitignore
	cmd := exec.Command("git", "ls-files", "-z")
	cmd.Dir = repoInfo.LocalPath
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to list files for repo %s: %w", repoInfo.URL, err)
	}

	newCache := &RepoCache{
		CommitHash: repoInfo.ResolvedCommit,
		CreatedAt:  time.Now().UTC(),
		RepoURL:    repoInfo.URL,
		LocalPath:  repoInfo.LocalPath,
	}

	files := strings.Split(string(bytes.TrimRight(output, "\x00")), "\x00")
	for _, relPath := range files {
		if relPath == "" {
			continue
		}
		absPath := filepath.Join(repoInfo.LocalPath, relPath)

		if IsBinaryFile(absPath) {
			continue
		}

		stat, err := os.Stat(absPath)
		if err != nil {
			continue // Skip files that we can't stat
		}

		newCache.Files = append(newCache.Files, CachedFileStats{
			Path:   relPath,
			Size:   stat.Size(),
			Tokens: int(stat.Size() / 4),
		})
	}

	// Save cache to disk
	cachePath, err := GetCacheFilePathForRepo(repoInfo.URL)
	if err != nil {
		return nil, fmt.Errorf("failed to get cache path: %w", err)
	}

	data, err := json.MarshalIndent(newCache, "", "  ")
	if err != nil {
		return nil, err
	}
	if err := os.WriteFile(cachePath, data, 0644); err != nil {
		return nil, err
	}

	return newCache, nil
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
