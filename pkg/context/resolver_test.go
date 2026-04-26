package context

import (
	"io/fs"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"testing/fstest"
)

type mockResolutionContext struct {
	fsys     fstest.MapFS
	baseDir  string
	ignored  map[string]bool
	contents map[string]string
}

func newMockCtx(files map[string]string) *mockResolutionContext {
	mfs := fstest.MapFS{}
	for p, c := range files {
		mfs[p] = &fstest.MapFile{Data: []byte(c)}
	}
	return &mockResolutionContext{
		fsys:     mfs,
		baseDir:  "/",
		ignored:  map[string]bool{},
		contents: files,
	}
}

func (c *mockResolutionContext) Stat(path string) (fs.FileInfo, error) {
	rel := strings.TrimPrefix(path, "/")
	return c.fsys.Stat(rel)
}

func (c *mockResolutionContext) WalkDir(root string, fn fs.WalkDirFunc) error {
	rel := strings.TrimPrefix(root, "/")
	if rel == "" {
		rel = "."
	}
	return fs.WalkDir(c.fsys, rel, func(path string, d fs.DirEntry, err error) error {
		if path == "." {
			return fn("/", d, err)
		}
		return fn("/"+path, d, err)
	})
}

func (c *mockResolutionContext) MatchDirective(file, directive, query string) bool {
	rel := strings.TrimPrefix(file, "/")
	switch directive {
	case "grep":
		return strings.Contains(c.contents[rel], query)
	case "find":
		return strings.Contains(file, query)
	}
	return false
}

func (c *mockResolutionContext) MatchPattern(pattern, path string) bool {
	pat := strings.ToLower(pattern)
	p := strings.ToLower(path)
	if strings.Contains(pat, "**") {
		return matchDoubleStarPattern(pat, p)
	}
	if matched, _ := filepath.Match(pat, p); matched {
		return true
	}
	if !strings.Contains(pat, "/") {
		if matched, _ := filepath.Match(pat, filepath.Base(p)); matched {
			return true
		}
	}
	return false
}

func (c *mockResolutionContext) IsGitIgnored(path string) bool { return c.ignored[path] }

func (c *mockResolutionContext) BaseDir() string { return c.baseDir }

func TestResolveAST_FilterNodeWrappingGlob(t *testing.T) {
	ctx := newMockCtx(map[string]string{
		"pkg/a/foo.go": "package a\nfunc Foo() {}\n",
		"pkg/b/bar.go": "package b\nfunc Bar() {}\n",
	})

	inner := &GlobNode{Pattern: "pkg/**/*.go", LineNum: 1, RawText: "pkg/**/*.go"}
	node := &FilterNode{
		Child:      inner,
		Directives: []SearchDirective{{Name: "grep", Query: "Foo"}},
		LineNum:    1,
		RawText:    "pkg/**/*.go @grep: \"Foo\"",
	}

	attrs, _, _ := ResolveAST([]RuleNode{node}, ctx)
	files := attrs[1]
	sort.Strings(files)
	if len(files) != 1 || !strings.HasSuffix(files[0], "foo.go") {
		t.Fatalf("expected only foo.go via @grep filter, got: %v", files)
	}
}

func TestResolveAST_TrailingSlashDirAutoStarStar(t *testing.T) {
	ctx := newMockCtx(map[string]string{
		"src/a.go":     "package src",
		"src/sub/b.go": "package sub",
		"other.go":     "package other",
	})

	node := &GlobNode{Pattern: "src/", LineNum: 1, RawText: "src/"}
	attrs, _, _ := ResolveAST([]RuleNode{node}, ctx)
	files := attrs[1]
	sort.Strings(files)
	if len(files) != 2 {
		t.Fatalf("expected 2 files under src/, got: %v", files)
	}
	for _, f := range files {
		if !strings.HasPrefix(f, "/src/") {
			t.Errorf("unexpected file outside src/: %s", f)
		}
	}
}

func TestResolveAST_LastWriteWinsExclusion(t *testing.T) {
	ctx := newMockCtx(map[string]string{
		"a.go":      "package a",
		"a_test.go": "package a",
	})

	include := &GlobNode{Pattern: "*.go", LineNum: 1, RawText: "*.go"}
	exclude := &GlobNode{Pattern: "*_test.go", LineNum: 2, RawText: "!*_test.go", Excluded: true}

	attrs, excl, _ := ResolveAST([]RuleNode{include, exclude}, ctx)
	if len(attrs[1]) != 1 || !strings.HasSuffix(attrs[1][0], "a.go") {
		t.Fatalf("expected only a.go on line 1, got: %v", attrs[1])
	}
	if len(excl[2]) != 1 || !strings.HasSuffix(excl[2][0], "a_test.go") {
		t.Fatalf("expected a_test.go on exclusion line 2, got: %v", excl[2])
	}
}
