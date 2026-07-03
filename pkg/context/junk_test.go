package context

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// TestJunkDir_ImplicitExpansionSkipped verifies that a parent-directory glob
// does not ingest files under well-known junk directories.
func TestJunkDir_ImplicitExpansionSkipped(t *testing.T) {
	ctx := newMockCtx(map[string]string{
		"terraform/main.tf":                      "resource {}",
		"terraform/vars.tf":                      "variable {}",
		"terraform/.terraform/providers/aws.bin": "BINARYPROVIDER",
		"terraform/.terraform/terraform.tfstate": "{}",
		"web/app.js":                             "console.log(1)",
		"web/node_modules/left-pad/index.js":     "module.exports = {}",
		"py/main.py":                             "print(1)",
		"py/__pycache__/main.cpython-311.pyc":    "BYTECODE",
		"rust/src/lib.rs":                        "fn main() {}",
		"rust/target/debug/app":                  "BINARY",
	})

	node := &GlobNode{Pattern: "**", LineNum: 1, RawText: "**"}
	attrs, _, _, _ := ResolveAST([]RuleNode{node}, ctx)
	files := attrs[1]
	sort.Strings(files)

	for _, f := range files {
		for _, junk := range []string{".terraform", "node_modules", "__pycache__", "target"} {
			if strings.Contains(f, "/"+junk+"/") {
				t.Errorf("junk dir file leaked into expansion: %s (via %s)", f, junk)
			}
		}
	}

	// Sanity: the real source files must still be present.
	wantPresent := []string{"main.tf", "app.js", "main.py", "lib.rs"}
	for _, w := range wantPresent {
		found := false
		for _, f := range files {
			if strings.HasSuffix(f, w) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected source file %q to survive junk filtering, got: %v", w, files)
		}
	}
}

// TestJunkDir_ExplicitReincludeAllowed verifies that a pattern naming a junk
// directory explicitly still resolves its contents — only implicit expansion
// through a parent glob is filtered.
func TestJunkDir_ExplicitReincludeAllowed(t *testing.T) {
	ctx := newMockCtx(map[string]string{
		"web/app.js":                         "console.log(1)",
		"web/node_modules/critical/index.js": "module.exports = {}",
		"web/node_modules/critical/util.js":  "module.exports = {}",
	})

	// Explicit path component "node_modules" in the pattern → re-include.
	node := &GlobNode{Pattern: "web/node_modules/**", LineNum: 1, RawText: "web/node_modules/**"}
	attrs, _, _, _ := ResolveAST([]RuleNode{node}, ctx)
	files := attrs[1]
	sort.Strings(files)

	if len(files) != 2 {
		t.Fatalf("expected explicit node_modules glob to yield 2 files, got: %v", files)
	}
	for _, f := range files {
		if !strings.Contains(f, "node_modules") {
			t.Errorf("unexpected file outside explicit node_modules glob: %s", f)
		}
	}
}

// TestJunkDir_ReincludeGlobAllowed verifies a "**/node_modules/**" re-include
// glob is honored because the pattern names node_modules as a component.
func TestJunkDir_ReincludeGlobAllowed(t *testing.T) {
	ctx := newMockCtx(map[string]string{
		"a/node_modules/pkg/index.js": "x",
		"b/node_modules/pkg/index.js": "y",
	})

	node := &GlobNode{Pattern: "**/node_modules/**", LineNum: 1, RawText: "**/node_modules/**"}
	attrs, _, _, _ := ResolveAST([]RuleNode{node}, ctx)
	files := attrs[1]
	if len(files) != 2 {
		t.Fatalf("expected re-include glob to yield 2 node_modules files, got: %v", files)
	}
}

func TestPatternReferencesDir(t *testing.T) {
	cases := []struct {
		pattern string
		dir     string
		want    bool
	}{
		{"web/node_modules/**", "node_modules", true},
		{"**/node_modules/**", "node_modules", true},
		{"/abs/repo/node_modules/x", "node_modules", true},
		{"terraform/**", "node_modules", false},
		{"terraform/**", ".terraform", false},
		{"src/target/main.rs", "target", true},
		{"src/**", "target", false},
	}
	for _, c := range cases {
		if got := patternReferencesDir(c.pattern, c.dir); got != c.want {
			t.Errorf("patternReferencesDir(%q, %q) = %v, want %v", c.pattern, c.dir, got, c.want)
		}
	}
}

// TestWarnOversizedRules_FileCount verifies a rule that expands past the file
// count threshold warns loudly to stderr, naming the line and count.
func TestWarnOversizedRules_FileCount(t *testing.T) {
	dir := t.TempDir()
	var paths []string
	for i := 0; i <= oversizeFileThreshold; i++ { // one over the threshold
		p := filepath.Join(dir, fmt.Sprintf("f%04d.txt", i))
		if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
		paths = append(paths, p)
	}

	rules := []RuleInfo{{Pattern: "terraform/**", LineNum: 7, EffectiveLineNum: 7}}
	attr := AttributionResult{7: paths}

	out := captureStderr(func() { warnOversizedRules(rules, attr) })
	if !strings.Contains(out, "terraform/**") || !strings.Contains(out, "line 7") {
		t.Errorf("warning did not name the rule/line: %q", out)
	}
	if !strings.Contains(out, fmt.Sprintf("%d files", oversizeFileThreshold+1)) {
		t.Errorf("warning did not report the file count: %q", out)
	}
}

// TestWarnOversizedRules_ByteSize verifies the byte threshold triggers even
// when the file count is small.
func TestWarnOversizedRules_ByteSize(t *testing.T) {
	dir := t.TempDir()
	big := filepath.Join(dir, "state.tfstate")
	if err := os.WriteFile(big, make([]byte, oversizeByteThreshold+1), 0o644); err != nil {
		t.Fatal(err)
	}

	rules := []RuleInfo{{Pattern: "terraform/.terraform/**", LineNum: 3, EffectiveLineNum: 3}}
	attr := AttributionResult{3: {big}}

	out := captureStderr(func() { warnOversizedRules(rules, attr) })
	if !strings.Contains(out, "line 3") || !strings.Contains(out, "exceeds") {
		t.Errorf("expected byte-threshold warning, got: %q", out)
	}
}

// TestWarnOversizedRules_UnderThresholdSilent verifies no warning fires for a
// normal-sized rule.
func TestWarnOversizedRules_UnderThresholdSilent(t *testing.T) {
	dir := t.TempDir()
	var paths []string
	for i := 0; i < 3; i++ {
		p := filepath.Join(dir, fmt.Sprintf("f%d.go", i))
		if err := os.WriteFile(p, []byte("package x"), 0o644); err != nil {
			t.Fatal(err)
		}
		paths = append(paths, p)
	}
	rules := []RuleInfo{{Pattern: "pkg/**", LineNum: 1, EffectiveLineNum: 1}}
	attr := AttributionResult{1: paths}

	out := captureStderr(func() { warnOversizedRules(rules, attr) })
	if strings.TrimSpace(out) != "" {
		t.Errorf("expected no warning under threshold, got: %q", out)
	}
}

func TestIsJunkDir(t *testing.T) {
	for _, d := range []string{".terraform", "node_modules", ".zig-cache", "zig-out", ".venv", "__pycache__", "target", "dist"} {
		if !isJunkDir(d) {
			t.Errorf("expected %q to be a junk dir", d)
		}
	}
	for _, d := range []string{"src", "pkg", "cmd", "terraform", "lib"} {
		if isJunkDir(d) {
			t.Errorf("did not expect %q to be a junk dir", d)
		}
	}
}
