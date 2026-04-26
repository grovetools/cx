package context

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

type diffFixture struct {
	name  string
	files []string
	rules string
}

func diffCorpus() []diffFixture {
	return []diffFixture{
		{
			name:  "literal_only",
			files: []string{"main.go", "README.md", "src/a.go"},
			rules: "main.go\nREADME.md\n",
		},
		{
			name:  "star_glob_basename",
			files: []string{"main.go", "helper.go", "README.md"},
			rules: "*.go\n",
		},
		{
			name:  "doublestar",
			files: []string{"src/a.go", "src/sub/b.go", "src/sub/deep/c.go", "README.md"},
			rules: "**/*.go\n",
		},
		{
			name:  "exclusion_basename",
			files: []string{"main.go", "main_test.go", "helper.go", "util_test.go"},
			rules: "*.go\n!*_test.go\n",
		},
		{
			name:  "brace_expansion",
			files: []string{"src/a.go", "src/b.go", "src/c.go"},
			rules: "src/{a,b}.go\n",
		},
		{
			name:  "trailing_slash_dir",
			files: []string{"src/a.go", "src/sub/b.go", "other.go"},
			rules: "src/\n",
		},
		{
			name: "mixed_literal_and_glob_with_exclude",
			files: []string{
				"main.go", "README.md",
				"pkg/foo.go", "pkg/foo_test.go",
				"pkg/bar.go", "vendor/dep.go",
			},
			rules: "main.go\npkg/**/*.go\n!*_test.go\n!vendor/**\n",
		},
		{
			name: "comments_and_blanks",
			files: []string{
				"src/a.go", "src/b.go", "docs/x.md",
			},
			rules: "# header\n\nsrc/**/*.go\n\n# another\ndocs/**/*.md\n",
		},
		{
			name:  "deep_doublestar_with_suffix",
			files: []string{"a/b/c/foo.txt", "a/foo.txt", "x/foo.go"},
			rules: "**/foo.txt\n",
		},
		{
			name:  "exclude_then_reinclude_basename",
			files: []string{"main.go", "main_test.go", "helper.go"},
			rules: "*.go\n!main_test.go\n",
		},
	}
}

func setupDiffFixture(t *testing.T, fx diffFixture) (*Manager, string) {
	t.Helper()
	t.Setenv("GROVE_HOME", t.TempDir())
	dir := t.TempDir()
	for _, rel := range fx.files {
		full := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", full, err)
		}
		if err := os.WriteFile(full, []byte("x"), 0o644); err != nil {
			t.Fatalf("write %s: %v", full, err)
		}
	}
	groveTOML := fmt.Sprintf("[context]\nallowed_paths = [%q]\n", dir)
	if err := os.WriteFile(filepath.Join(dir, "grove.toml"), []byte(groveTOML), 0o644); err != nil {
		t.Fatalf("write grove.toml: %v", err)
	}
	rulesPath := filepath.Join(dir, "test.rules")
	if err := os.WriteFile(rulesPath, []byte(fx.rules), 0o644); err != nil {
		t.Fatalf("write rules: %v", err)
	}
	return NewManagerWithOverride(dir, rulesPath), dir
}

func normRel(p, base string) string {
	if rel, err := filepath.Rel(base, p); err == nil {
		return filepath.ToSlash(rel)
	}
	return filepath.ToSlash(p)
}

func toSet(paths []string, base string) map[string]struct{} {
	out := make(map[string]struct{}, len(paths))
	for _, p := range paths {
		out[normRel(p, base)] = struct{}{}
	}
	return out
}

func flattenAttrSet(attr AttributionResult, base string) map[string]struct{} {
	out := map[string]struct{}{}
	for _, files := range attr {
		for _, f := range files {
			out[normRel(f, base)] = struct{}{}
		}
	}
	return out
}

func setSymDiff(a, b map[string]struct{}) (onlyA, onlyB []string) {
	for k := range a {
		if _, ok := b[k]; !ok {
			onlyA = append(onlyA, k)
		}
	}
	for k := range b {
		if _, ok := a[k]; !ok {
			onlyB = append(onlyB, k)
		}
	}
	sort.Strings(onlyA)
	sort.Strings(onlyB)
	return
}

func TestDifferential_LegacyVsAST(t *testing.T) {
	for _, fx := range diffCorpus() {
		t.Run(fx.name, func(t *testing.T) {
			m, dir := setupDiffFixture(t, fx)

			legacy, err := m.ResolveFilesFromRules()
			if err != nil {
				t.Fatalf("legacy ResolveFilesFromRules: %v", err)
			}
			legacySet := toSet(legacy, dir)

			nodes, perrs := ParseToAST([]byte(fx.rules))
			if len(perrs) != 0 {
				t.Fatalf("ParseToAST produced parse errors: %+v", perrs)
			}
			astAttr, _, _ := ResolveAST(nodes, newProdResolutionContext(m))
			astSet := flattenAttrSet(astAttr, dir)

			onlyLegacy, onlyAST := setSymDiff(legacySet, astSet)
			if len(onlyLegacy) != 0 || len(onlyAST) != 0 {
				t.Fatalf(
					"DIVERGENCE in fixture %q (workDir=%s)\nrules:\n%s\nlegacy-only (%d):\n  %s\nast-only (%d):\n  %s\nlegacy-set (%d): %v\nast-set (%d): %v",
					fx.name, dir, fx.rules,
					len(onlyLegacy), strings.Join(onlyLegacy, "\n  "),
					len(onlyAST), strings.Join(onlyAST, "\n  "),
					len(legacySet), sortedKeys(legacySet),
					len(astSet), sortedKeys(astSet),
				)
			}
		})
	}
}

func sortedKeys(m map[string]struct{}) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
