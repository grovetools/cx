package context

import (
	"io"
	"io/fs"
	"os"
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

func (c *mockResolutionContext) ExecCommand(cmd string) ([]string, error) {
	return nil, nil
}

func (c *mockResolutionContext) ResolveAliasLine(line string) (string, error) {
	return "", nil
}

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

	attrs, _, _, _ := ResolveAST([]RuleNode{node}, ctx)
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
	attrs, _, _, _ := ResolveAST([]RuleNode{node}, ctx)
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

func TestResolveAST_ImportedRulesNotSuppressedByUnrelatedExclusion(t *testing.T) {
	ctx := newMockCtx(map[string]string{
		"main.go":      "package main",
		"main_test.go": "package main",
		"go.mod":       "module example",
		"go.sum":       "h1:abc123",
	})

	localGo := &GlobNode{Pattern: "*.go", LineNum: 1, RawText: "*.go"}
	importedMod := &LiteralNode{ExpectedPath: "go.mod", LineNum: 5, RawText: "go.mod"}
	importedSum := &LiteralNode{ExpectedPath: "go.sum", LineNum: 5, RawText: "go.sum"}
	excludeTest := &GlobNode{Pattern: "*_test.go", LineNum: 10, RawText: "!*_test.go", Excluded: true}

	attrs, excl, _, _ := ResolveAST([]RuleNode{localGo, importedMod, importedSum, excludeTest}, ctx)

	goFiles := attrs[1]
	sort.Strings(goFiles)
	if len(goFiles) != 1 || !strings.HasSuffix(goFiles[0], "main.go") {
		t.Fatalf("expected main.go on line 1, got: %v", goFiles)
	}

	importedFiles := attrs[5]
	sort.Strings(importedFiles)
	if len(importedFiles) != 2 {
		t.Fatalf("expected go.mod and go.sum attributed to line 5, got: %v", importedFiles)
	}
	if !strings.HasSuffix(importedFiles[0], "go.mod") || !strings.HasSuffix(importedFiles[1], "go.sum") {
		t.Fatalf("expected go.mod and go.sum, got: %v", importedFiles)
	}

	if len(excl[10]) != 1 || !strings.HasSuffix(excl[10][0], "main_test.go") {
		t.Fatalf("expected main_test.go excluded at line 10, got: %v", excl[10])
	}
}

func TestResolveAST_ImportedAtLineZeroBug(t *testing.T) {
	ctx := newMockCtx(map[string]string{
		"main.go":      "package main",
		"main_test.go": "package main",
		"go.mod":       "module example",
	})

	importedMod := &LiteralNode{ExpectedPath: "go.mod", LineNum: 0, RawText: "go.mod"}
	localGo := &GlobNode{Pattern: "*.go", LineNum: 1, RawText: "*.go"}
	excludeTest := &GlobNode{Pattern: "*_test.go", LineNum: 10, RawText: "!*_test.go", Excluded: true}

	attrs, excl, _, _ := ResolveAST([]RuleNode{importedMod, localGo, excludeTest}, ctx)

	if len(attrs[0]) != 1 || !strings.HasSuffix(attrs[0][0], "go.mod") {
		t.Fatalf("go.mod at LineNum=0 should still be included, got attrs: %v", attrs)
	}
	if len(attrs[1]) != 1 || !strings.HasSuffix(attrs[1][0], "main.go") {
		t.Fatalf("main.go should be included at line 1, got: %v", attrs[1])
	}
	if len(excl[10]) != 1 || !strings.HasSuffix(excl[10][0], "main_test.go") {
		t.Fatalf("main_test.go should be excluded at line 10, got: %v", excl[10])
	}
}

// --- reRootRules tests ---

func TestReRootRules_BareRelativePaths(t *testing.T) {
	rules := []RuleInfo{
		{Pattern: "pkg/target.go", LineNum: 1},
		{Pattern: "cmd/main.go", LineNum: 2},
		{Pattern: "*.go", LineNum: 3},                 // floating, no /
		{Pattern: "/abs/path/file.go", LineNum: 4},    // absolute, skip
		{Pattern: "@a:core/pkg/alias/", LineNum: 5},   // alias, skip
		{Pattern: "http://example.com", LineNum: 6},   // URL, skip
		{Pattern: "pkg/sub/deep/file.go", LineNum: 7}, // path-like
	}

	reRootRules(rules, "/code/eco/flow")

	expected := map[int]string{
		1: "/code/eco/flow/pkg/target.go",
		2: "/code/eco/flow/cmd/main.go",
		3: "/code/eco/flow/**/*.go", // floating → recursive
		4: "/abs/path/file.go",      // unchanged
		5: "@a:core/pkg/alias/",     // unchanged
		6: "http://example.com",     // unchanged
		7: "/code/eco/flow/pkg/sub/deep/file.go",
	}

	for _, r := range rules {
		want, ok := expected[r.LineNum]
		if !ok {
			continue
		}
		if r.Pattern != want {
			t.Errorf("line %d: got %q, want %q", r.LineNum, r.Pattern, want)
		}
	}
}

func TestReRootRules_ExclusionPatternsAlsoReRooted(t *testing.T) {
	rules := []RuleInfo{
		{Pattern: "pkg/foo_test.go", IsExclude: true, LineNum: 1},
		{Pattern: "*_test.go", IsExclude: true, LineNum: 2},
	}

	reRootRules(rules, "/code/eco/flow")

	if rules[0].Pattern != "/code/eco/flow/pkg/foo_test.go" {
		t.Errorf("path exclusion not re-rooted: %s", rules[0].Pattern)
	}
	if rules[1].Pattern != "/code/eco/flow/**/*_test.go" {
		t.Errorf("floating exclusion not re-rooted: %s", rules[1].Pattern)
	}
}

// --- warnZeroMatchRules tests ---

func TestWarnZeroMatchRules_WarnsOnLiteralMiss(t *testing.T) {
	// Build rules with a literal path that matched 0 files.
	rules := []RuleInfo{
		{Pattern: "/code/flow/pkg/target.go", LineNum: 3, EffectiveLineNum: 3},
	}
	attr := AttributionResult{} // empty — line 3 matched nothing

	// Capture stderr output.
	old := captureStderr(func() {
		warnZeroMatchRules(rules, attr, nil, nil)
	})

	if !strings.Contains(old, "matched 0 files") {
		t.Fatalf("expected zero-match warning, got: %q", old)
	}
	if !strings.Contains(old, "line 3") {
		t.Fatalf("expected line number 3 in warning, got: %q", old)
	}
}

func TestWarnZeroMatchRules_NoWarnOnGlob(t *testing.T) {
	rules := []RuleInfo{
		{Pattern: "pkg/**/*.go", LineNum: 1, EffectiveLineNum: 1},
	}
	attr := AttributionResult{} // empty but it's a glob — no warning

	output := captureStderr(func() {
		warnZeroMatchRules(rules, attr, nil, nil)
	})

	if strings.Contains(output, "matched 0 files") {
		t.Fatalf("should not warn on glob patterns, got: %q", output)
	}
}

func TestWarnZeroMatchRules_NoWarnOnExclusion(t *testing.T) {
	rules := []RuleInfo{
		{Pattern: "pkg/missing.go", IsExclude: true, LineNum: 1, EffectiveLineNum: 1},
	}
	attr := AttributionResult{}

	output := captureStderr(func() {
		warnZeroMatchRules(rules, attr, nil, nil)
	})

	if strings.Contains(output, "matched 0 files") {
		t.Fatalf("should not warn on exclusion patterns, got: %q", output)
	}
}

func TestWarnZeroMatchRules_NoWarnWhenMatchExists(t *testing.T) {
	rules := []RuleInfo{
		{Pattern: "/code/flow/pkg/target.go", LineNum: 3, EffectiveLineNum: 3},
	}
	attr := AttributionResult{
		3: []string{"/code/flow/pkg/target.go"},
	}

	output := captureStderr(func() {
		warnZeroMatchRules(rules, attr, nil, nil)
	})

	if strings.Contains(output, "matched 0 files") {
		t.Fatalf("should not warn when files are matched, got: %q", output)
	}
}

func TestWarnZeroMatchRules_NoWarnOnDirectiveRules(t *testing.T) {
	rules := []RuleInfo{
		{
			Pattern:          "pkg/target.go",
			LineNum:          1,
			EffectiveLineNum: 1,
			Directives:       []SearchDirective{{Name: "grep", Query: "Foo"}},
		},
	}
	attr := AttributionResult{}

	output := captureStderr(func() {
		warnZeroMatchRules(rules, attr, nil, nil)
	})

	if strings.Contains(output, "matched 0 files") {
		t.Fatalf("should not warn on rules with directives, got: %q", output)
	}
}

func TestWarnZeroMatchRules_NoWarnWhenFilteredMatch(t *testing.T) {
	// Line 9's literal matched a file, but a later import line won
	// last-match-wins attribution, so line 9 lives in FilteredResult, not
	// AttributionResult. It matched — must not be reported as dead.
	rules := []RuleInfo{
		{Pattern: "/code/eco/grove-anthropic/pkg/logging/query_log.go", LineNum: 9, EffectiveLineNum: 9},
		{Pattern: "/code/eco/grove-anthropic/pkg/logging/query_log.go", LineNum: 12, EffectiveLineNum: 12},
	}
	attr := AttributionResult{
		12: []string{"/code/eco/grove-anthropic/pkg/logging/query_log.go"},
	}
	filt := FilteredResult{
		9: []FilteredFileInfo{{File: "/code/eco/grove-anthropic/pkg/logging/query_log.go", WinningLineNum: 12}},
	}

	output := captureStderr(func() {
		warnZeroMatchRules(rules, attr, filt, nil)
	})

	if strings.Contains(output, "matched 0 files") {
		t.Fatalf("should not warn when the line matched but lost attribution, got: %q", output)
	}
}

func TestWarnZeroMatchRules_NoWarnWhenExcludedByMatch(t *testing.T) {
	// A literal that matched a file later removed by an exclusion appears only
	// in ExcludedByResult. It matched — the file just got excluded — so no
	// zero-match warning.
	rules := []RuleInfo{
		{Pattern: "/code/eco/flow/pkg/secret.go", LineNum: 4, EffectiveLineNum: 4},
	}
	attr := AttributionResult{}
	eby := ExcludedByResult{
		4: []ExcludedByInfo{{File: "/code/eco/flow/pkg/secret.go", ExcludingLineNum: 5}},
	}

	output := captureStderr(func() {
		warnZeroMatchRules(rules, attr, nil, eby)
	})

	if strings.Contains(output, "matched 0 files") {
		t.Fatalf("should not warn when the line matched but was excluded, got: %q", output)
	}
}

// --- ResolveAST integration: re-rooted literal resolves under home repo ---

func TestResolveAST_ReRootedLiteralResolvesUnderHomeRepo(t *testing.T) {
	// Simulate a preset whose bare path "pkg/target.go" was re-rooted to
	// "/code/eco/flow/pkg/target.go" by reRootRules. The file lives under
	// the flow project at that absolute path.
	ctx := newMockCtx(map[string]string{
		"code/eco/flow/pkg/target.go": "package target",
		"code/eco/flow/pkg/other.go":  "package other",
		"code/eco/flow/cmd/main.go":   "package main",
	})

	// After re-rooting, the rules look like absolute paths:
	node1 := &LiteralNode{ExpectedPath: "/code/eco/flow/pkg/target.go", LineNum: 1, RawText: "pkg/target.go"}
	node2 := &LiteralNode{ExpectedPath: "/code/eco/flow/cmd/main.go", LineNum: 2, RawText: "cmd/main.go"}

	attrs, _, _, _ := ResolveAST([]RuleNode{node1, node2}, ctx)

	if len(attrs[1]) != 1 || !strings.HasSuffix(attrs[1][0], "target.go") {
		t.Fatalf("expected target.go at line 1, got: %v", attrs[1])
	}
	if len(attrs[2]) != 1 || !strings.HasSuffix(attrs[2][0], "main.go") {
		t.Fatalf("expected main.go at line 2, got: %v", attrs[2])
	}
}

func TestResolveAST_ReRootedWithExclusionStillWorks(t *testing.T) {
	// After re-rooting, exclusions should still apply correctly
	ctx := newMockCtx(map[string]string{
		"code/eco/flow/pkg/target.go":      "package target",
		"code/eco/flow/pkg/target_test.go": "package target",
	})

	include := &GlobNode{Pattern: "/code/eco/flow/pkg/**/*.go", LineNum: 1, RawText: "pkg/**/*.go"}
	exclude := &GlobNode{Pattern: "/code/eco/flow/pkg/**/*_test.go", LineNum: 2, RawText: "!*_test.go", Excluded: true}

	attrs, excl, _, _ := ResolveAST([]RuleNode{include, exclude}, ctx)

	if len(attrs[1]) != 1 || !strings.HasSuffix(attrs[1][0], "target.go") {
		t.Fatalf("expected only target.go at line 1, got: %v", attrs[1])
	}
	if len(excl[2]) != 1 || !strings.HasSuffix(excl[2][0], "target_test.go") {
		t.Fatalf("expected target_test.go excluded at line 2, got: %v", excl[2])
	}
}

func TestResolveAST_LastWriteWinsExclusion(t *testing.T) {
	ctx := newMockCtx(map[string]string{
		"a.go":      "package a",
		"a_test.go": "package a",
	})

	include := &GlobNode{Pattern: "*.go", LineNum: 1, RawText: "*.go"}
	exclude := &GlobNode{Pattern: "*_test.go", LineNum: 2, RawText: "!*_test.go", Excluded: true}

	attrs, excl, _, _ := ResolveAST([]RuleNode{include, exclude}, ctx)
	if len(attrs[1]) != 1 || !strings.HasSuffix(attrs[1][0], "a.go") {
		t.Fatalf("expected only a.go on line 1, got: %v", attrs[1])
	}
	if len(excl[2]) != 1 || !strings.HasSuffix(excl[2][0], "a_test.go") {
		t.Fatalf("expected a_test.go on exclusion line 2, got: %v", excl[2])
	}
}

// TestWarnZeroMatchRules_LiteralAndImportSameFile is the job-31 repro in
// miniature: a bare literal line and a later import/concept line both resolve
// the SAME file. Last-match-wins gives the import the attribution; the literal
// lands in FilteredResult. Feeding ResolveAST's real FilteredResult into
// warnZeroMatchRules must suppress the false "matched 0 files" warning.
func TestWarnZeroMatchRules_LiteralAndImportSameFile(t *testing.T) {
	ctx := newMockCtx(map[string]string{
		"code/eco/grove-anthropic/pkg/logging/query_log.go": "package logging",
	})

	// Line 9: the bare @a: literal. Line 12: the concept import that also
	// pulls in query_log.go (both re-rooted to the same absolute path).
	literal := &LiteralNode{ExpectedPath: "/code/eco/grove-anthropic/pkg/logging/query_log.go", LineNum: 9, RawText: "@a:grove-anthropic/pkg/logging/query_log.go"}
	imported := &LiteralNode{ExpectedPath: "/code/eco/grove-anthropic/pkg/logging/query_log.go", LineNum: 12, RawText: "@a:grove-anthropic::concept-query-ledger"}

	attr, _, filt, eby := ResolveAST([]RuleNode{literal, imported}, ctx)

	// Line 12 wins attribution; line 9 is filtered, not dead.
	if len(attr[12]) != 1 {
		t.Fatalf("expected line 12 to win attribution, got: %v", attr)
	}
	if len(filt[9]) != 1 {
		t.Fatalf("expected line 9 in FilteredResult, got: %v", filt)
	}

	rules := []RuleInfo{
		{Pattern: "/code/eco/grove-anthropic/pkg/logging/query_log.go", LineNum: 9, EffectiveLineNum: 9},
		{Pattern: "/code/eco/grove-anthropic/pkg/logging/query_log.go", LineNum: 12, EffectiveLineNum: 12},
	}
	output := captureStderr(func() {
		warnZeroMatchRules(rules, attr, filt, eby)
	})
	if strings.Contains(output, "matched 0 files") {
		t.Fatalf("no zero-match warning expected for a literal that lost attribution, got: %q", output)
	}
}

// captureStderr redirects os.Stderr for the duration of fn and returns
// whatever was written. Used to verify warning output.
func captureStderr(fn func()) string {
	r, w, _ := os.Pipe()
	old := os.Stderr
	os.Stderr = w
	fn()
	w.Close()
	os.Stderr = old
	out, _ := io.ReadAll(r)
	return string(out)
}
