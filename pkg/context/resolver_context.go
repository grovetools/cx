package context

import (
	"io/fs"
	"os"
	"path/filepath"

	"github.com/grovetools/core/util/pathutil"
)

// FileAttribution is the unit emission of a node's Resolve method. Each
// resolved file carries the originating rule line so the single-pass reducer
// can do last-write-wins attribution without a second m.matchPattern loop.
type FileAttribution struct {
	Path             string
	EffectiveLineNum int
	IsExclude        bool
}

// ResolutionContext abstracts the I/O surface used by node Resolve methods.
// Production wraps Manager; tests use a MapFS-backed mock.
type ResolutionContext interface {
	Stat(path string) (os.FileInfo, error)
	WalkDir(root string, fn fs.WalkDirFunc) error
	MatchDirective(file, directive, query string) bool
	MatchPattern(pattern, path string) bool
	IsGitIgnored(path string) bool
	BaseDir() string
	ExecCommand(cmd string) ([]string, error)
	ResolveAliasLine(line string) (string, error)
}

// prodResolutionContext is the production-mode ResolutionContext. It can
// optionally be primed with a pre-discovered file set so the AST attribution
// pass mirrors the legacy file-discovery pipeline (gitignore semantics, dir
// exclusions, binary skip) without re-walking the filesystem.
type prodResolutionContext struct {
	m            *Manager
	fileSet      map[string]bool
	gitIgnoredFn func(path string) bool
}

func newProdResolutionContext(m *Manager) *prodResolutionContext {
	return &prodResolutionContext{m: m}
}

// withFileSet primes the context with a pre-discovered file list. WalkDir
// then yields only those files (filtered by root prefix). Used by the
// attribution pass that runs after legacy file discovery.
func (c *prodResolutionContext) withFileSet(files []string) *prodResolutionContext {
	set := make(map[string]bool, len(files))
	for _, f := range files {
		set[f] = true
	}
	c.fileSet = set
	return c
}

func (c *prodResolutionContext) Stat(path string) (os.FileInfo, error) {
	return os.Stat(path)
}

func (c *prodResolutionContext) WalkDir(root string, fn fs.WalkDirFunc) error {
	if c.fileSet != nil {
		for f := range c.fileSet {
			if errCb := fn(f, fakeFileEntry(filepath.Base(f)), nil); errCb != nil {
				return errCb
			}
		}
		return nil
	}

	gitIgnored, _ := c.m.getGitIgnoredFiles(root)
	if gitIgnored == nil {
		gitIgnored = make(map[string]bool)
	}

	return filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			if d != nil && d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		if d.IsDir() {
			switch d.Name() {
			case ".git", ".grove", ".grove-worktrees":
				return filepath.SkipDir
			}
		}

		normalized, normErr := pathutil.NormalizeForLookup(path)
		if normErr != nil {
			normalized = path
		}
		if gitIgnored[normalized] {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		if !d.IsDir() && isBinaryFile(path) {
			return nil
		}

		return fn(path, d, err)
	})
}

type fakeFileEntry string

func (f fakeFileEntry) Name() string               { return string(f) }
func (f fakeFileEntry) IsDir() bool                { return false }
func (f fakeFileEntry) Type() fs.FileMode          { return 0 }
func (f fakeFileEntry) Info() (fs.FileInfo, error) { return nil, nil }

func (c *prodResolutionContext) MatchDirective(file, directive, query string) bool {
	return c.m.matchDirective(file, directive, query)
}

func (c *prodResolutionContext) MatchPattern(pattern, path string) bool {
	return c.m.matchPattern(pattern, path)
}

func (c *prodResolutionContext) IsGitIgnored(path string) bool {
	if c.fileSet != nil {
		return false
	}
	ignored, err := c.m.getGitIgnoredFiles(c.m.rulesBaseDir)
	if err != nil {
		return false
	}
	return ignored[path]
}

func (c *prodResolutionContext) BaseDir() string {
	return c.m.rulesBaseDir
}

func (c *prodResolutionContext) ExecCommand(cmd string) ([]string, error) {
	return c.m.executeCommandExpression(cmd)
}

func (c *prodResolutionContext) ResolveAliasLine(line string) (string, error) {
	resolver := c.m.getAliasResolver()
	if resolver == nil {
		return "", os.ErrNotExist
	}
	return resolver.ResolveLine(line)
}
