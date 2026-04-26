package context

import (
	"fmt"
	"math/rand"
	"reflect"
	"sort"
	"strings"
	"testing"
)

const fuzzIterations = 500

func randFakeFS(rng *rand.Rand) map[string]string {
	fileNames := []string{"a.go", "b.go", "c.txt", "main.go", "helper.go", "x_test.go", "doc.md", "README.md"}
	dirs := []string{"", "src/", "pkg/", "src/sub/", "pkg/inner/", "docs/"}
	n := 5 + rng.Intn(16)
	out := map[string]string{}
	for i := 0; i < n; i++ {
		dir := dirs[rng.Intn(len(dirs))]
		name := fileNames[rng.Intn(len(fileNames))]
		path := dir + name
		out[path] = fmt.Sprintf("package x // file %d %s", i, name)
	}
	return out
}

func randAST(rng *rand.Rand) []RuleNode {
	patterns := []string{"*.go", "**/*.go", "src/**/*.go", "*.md", "README.md", "main.go", "**/x_test.go", "pkg/**"}
	n := 3 + rng.Intn(8)
	nodes := make([]RuleNode, 0, n)
	for i := 0; i < n; i++ {
		p := patterns[rng.Intn(len(patterns))]
		excluded := rng.Intn(4) == 0
		line := i + 1
		var node RuleNode
		if hasGlobMeta(p) {
			node = &GlobNode{Pattern: p, LineNum: line, RawText: p, Excluded: excluded}
		} else {
			node = &LiteralNode{ExpectedPath: p, LineNum: line, RawText: p, Excluded: excluded}
		}
		if rng.Intn(5) == 0 {
			node = &FilterNode{
				Child:      node,
				Directives: []SearchDirective{{Name: "grep", Query: "package"}},
				LineNum:    line,
				RawText:    p,
				Excluded:   excluded,
			}
			if rng.Intn(20) == 0 {
				node = &FilterNode{
					Child:      node,
					Directives: []SearchDirective{{Name: "find", Query: ".go"}},
					LineNum:    line,
					RawText:    p,
					Excluded:   excluded,
				}
			}
		}
		switch rng.Intn(20) {
		case 0:
			targets := []string{"@a:eco:proj", "@a:foo", "@alias:eco:proj:wt/main.go"}
			rulesets := []string{"", "dev", "ruleset"}
			node = &ImportNode{
				Target:   targets[rng.Intn(len(targets))],
				Ruleset:  rulesets[rng.Intn(len(rulesets))],
				LineNum:  line,
				RawText:  p,
				Excluded: excluded,
			}
		case 1:
			cmds := []string{"ls", "find . -name '*.go'", "echo hello"}
			node = &CommandNode{
				Command:  cmds[rng.Intn(len(cmds))],
				LineNum:  line,
				RawText:  p,
				Excluded: excluded,
			}
		}
		nodes = append(nodes, node)
	}
	return nodes
}

func TestResolveAST_Fuzz_ExclusionInclusionExclusive(t *testing.T) {
	rng := rand.New(rand.NewSource(0x1337))
	for i := 0; i < fuzzIterations; i++ {
		ctx := newMockCtx(randFakeFS(rng))
		nodes := randAST(rng)
		attr, excl, _ := ResolveAST(nodes, ctx)
		included := map[string]bool{}
		for _, files := range attr {
			for _, f := range files {
				included[f] = true
			}
		}
		for line, files := range excl {
			for _, f := range files {
				if included[f] {
					t.Fatalf("iter %d: path %q appears as both included and excluded (line %d)", i, f, line)
				}
			}
		}
	}
}

func TestResolveAST_Fuzz_GlobEmissionsUnderPatternRoot(t *testing.T) {
	rng := rand.New(rand.NewSource(0xC0DE))
	for i := 0; i < fuzzIterations; i++ {
		ctx := newMockCtx(randFakeFS(rng))
		// Confine to GlobNode-only ASTs so we can assert path-prefix invariants.
		var nodes []RuleNode
		patterns := []string{"src/**/*.go", "pkg/**", "docs/**/*.md", "src/sub/**"}
		for j := 0; j < 3+rng.Intn(4); j++ {
			p := patterns[rng.Intn(len(patterns))]
			nodes = append(nodes, &GlobNode{Pattern: p, LineNum: j + 1, RawText: p})
		}
		for _, n := range nodes {
			gn := n.(*GlobNode)
			root := staticPrefixOf(gn.Pattern)
			attrs := gn.Resolve(ctx)
			for _, a := range attrs {
				rel := strings.TrimPrefix(a.Path, "/")
				if root != "" && !strings.HasPrefix(rel, root) {
					t.Fatalf("iter %d: glob %q emitted %q outside static root %q", i, gn.Pattern, a.Path, root)
				}
			}
		}
	}
}

func staticPrefixOf(pattern string) string {
	idx := strings.IndexAny(pattern, "*?[")
	if idx < 0 {
		return pattern
	}
	prefix := pattern[:idx]
	if slash := strings.LastIndex(prefix, "/"); slash >= 0 {
		return prefix[:slash+1]
	}
	return ""
}

func TestResolveAST_Fuzz_FilterSubsetOfChild(t *testing.T) {
	rng := rand.New(rand.NewSource(0xACE))
	for i := 0; i < fuzzIterations; i++ {
		ctx := newMockCtx(randFakeFS(rng))
		patterns := []string{"**/*.go", "src/**/*.go", "*.md"}
		p := patterns[rng.Intn(len(patterns))]
		child := &GlobNode{Pattern: p, LineNum: 1, RawText: p}
		filter := &FilterNode{
			Child:      child,
			Directives: []SearchDirective{{Name: "grep", Query: "package"}},
			LineNum:    1,
			RawText:    p,
		}
		childAttrs := child.Resolve(ctx)
		filterAttrs := filter.Resolve(ctx)
		childPaths := map[string]bool{}
		for _, a := range childAttrs {
			childPaths[a.Path] = true
		}
		for _, a := range filterAttrs {
			if !childPaths[a.Path] {
				t.Fatalf("iter %d: FilterNode emitted %q not in child set (pattern=%q)", i, a.Path, p)
			}
		}
	}
}

func TestResolveAST_Fuzz_Deterministic(t *testing.T) {
	rng := rand.New(rand.NewSource(0xDEADBEEF))
	for i := 0; i < fuzzIterations; i++ {
		fs := randFakeFS(rng)
		nodes := randAST(rng)
		ctx1 := newMockCtx(fs)
		ctx2 := newMockCtx(fs)
		a1, e1, f1 := ResolveAST(nodes, ctx1)
		a2, e2, f2 := ResolveAST(nodes, ctx2)
		if !sameAttribution(a1, a2) {
			t.Fatalf("iter %d: AttributionResult not deterministic\nrun1=%v\nrun2=%v", i, a1, a2)
		}
		if !sameAttribution(e1, e2) {
			t.Fatalf("iter %d: ExclusionResult not deterministic", i)
		}
		if !reflect.DeepEqual(normalizeFiltered(f1), normalizeFiltered(f2)) {
			t.Fatalf("iter %d: FilteredResult not deterministic", i)
		}
	}
}

func sameAttribution(a, b map[int][]string) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		w := append([]string(nil), b[k]...)
		x := append([]string(nil), v...)
		sort.Strings(w)
		sort.Strings(x)
		if !reflect.DeepEqual(w, x) {
			return false
		}
	}
	return true
}

func normalizeFiltered(in FilteredResult) map[int][]FilteredFileInfo {
	out := make(map[int][]FilteredFileInfo, len(in))
	for k, v := range in {
		c := append([]FilteredFileInfo(nil), v...)
		sort.Slice(c, func(i, j int) bool {
			if c[i].File != c[j].File {
				return c[i].File < c[j].File
			}
			return c[i].WinningLineNum < c[j].WinningLineNum
		})
		out[k] = c
	}
	return out
}
