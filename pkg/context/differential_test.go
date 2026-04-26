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
	// skip, when non-empty, marks the fixture as a known-divergent or
	// unsupported case; the driver calls t.Skip(skip) instead of running.
	skip string
	// extraFiles are written verbatim into the workspace before running:
	// map[relativePath]content. Used by fixtures that need sibling
	// .rules files (e.g. @default targets, imports-of-imports).
	extraFiles map[string]string
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
		{
			name: "filter_grep_on_glob",
			files: []string{
				"keep_a.go", "keep_b.go", "skip.go", "README.md",
			},
			extraFiles: map[string]string{
				"keep_a.go": "package x\nfunc A() {}\n",
				"keep_b.go": "package x\nfunc B() {}\n",
				"skip.go":   "// no package keyword here in body\n",
				"README.md": "doc\n",
			},
			rules: "**/*.go @grep: \"package\"\n",
		},
		{
			name: "filter_find_on_glob",
			files: []string{
				"src/util_helper.go", "src/main.go", "src/util_other.go",
			},
			rules: "**/*.go @find: \"util\"\n",
		},
		{
			name: "filter_grep_with_exclusion",
			files: []string{
				"a.go", "b.go", "vendor/c.go",
			},
			extraFiles: map[string]string{
				"a.go":        "package a\n",
				"b.go":        "package b\n",
				"vendor/c.go": "package c\n",
			},
			rules: "**/*.go @grep: \"package\"\n!vendor/**\n",
		},
		{
			name: "hotcold_split_basic",
			files: []string{
				"main.go", "helper.go", "vendor/dep.go", "README.md",
			},
			rules: "**/*.go\nREADME.md\n---\nvendor/**\n",
		},
		{
			name:  "default_directive_local",
			files: []string{"main.go", "extra.go"},
			// @default points at a workspace whose grove.toml exposes a
			// default_rules preset; setting that up requires the full
			// grove config + preset machinery. Skip until Phase 5 wires
			// a test-friendly handle.
			rules: "main.go\n",
			skip:  "Phase 4.5: @default requires grove.toml workspace + default_rules preset infra; differential harness has no hook for that yet.",
		},
		{
			name:  "cmd_directive",
			files: []string{"main.go", "helper.go"},
			rules: "@cmd: ls\n",
		},
		{
			name:  "alias_to_workspace_subpath",
			files: []string{"main.go"},
			rules: "@a:test:proj/main.go\n",
		},
		{
			// Phase 5A semantics: a plain literal path ending in `.rules`
			// is a literal file inclusion, NOT an import. To recurse into
			// another rules file, use `@a:<workspace>::<name>`. Both
			// legacy and AST paths agree on literal-file treatment.
			name:  "imports_of_imports",
			files: []string{"main.go", "helper.go"},
			extraFiles: map[string]string{
				"nested.rules": "helper.go\n",
			},
			rules: "main.go\nnested.rules\n",
		},
		{
			name: "deep_filter_chain",
			files: []string{
				"a.go", "b.go",
			},
			extraFiles: map[string]string{
				"a.go": "package x\n// keyword\n",
				"b.go": "package x\n",
			},
			rules: "**/*.go @grep: \"package\" @grep: \"keyword\"\n",
		},
		{
			name:  "exclude_with_alias_negation",
			files: []string{"main.go"},
			rules: "@a:eco:proj\n!@a:eco:proj/sub\n",
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
	for rel, content := range fx.extraFiles {
		full := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", full, err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatalf("write extra %s: %v", full, err)
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
			if fx.skip != "" {
				t.Skip(fx.skip)
			}
			m, dir := setupDiffFixture(t, fx)

			legacySet, err := legacyUnionSet(m, dir, fx.rules)
			if err != nil {
				t.Fatalf("legacy resolution: %v", err)
			}

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

// legacyUnionSet returns the legacy walker's hot ∪ cold file set. The
// hot/cold partition is a separate concern: the differential test only
// asserts that both paths see the same total set of files.
func legacyUnionSet(m *Manager, dir, rules string) (map[string]struct{}, error) {
	if !strings.Contains(rules, "\n---\n") && !strings.HasSuffix(strings.TrimSpace(rules), "---") {
		legacy, err := m.ResolveFilesFromRules()
		if err != nil {
			return nil, err
		}
		return toSet(legacy, dir), nil
	}
	// For hot/cold rules, ResolveFilesFromRules subtracts cold from hot.
	// Bypass that filter by resolving hot and cold patterns separately and
	// unioning, mirroring what the AST resolver (which has no partition
	// notion) does.
	hotRules, coldRules, _, _, err := m.expandAllRules(filepath.Join(dir, "test.rules"), make(map[string]bool), 0)
	if err != nil {
		return nil, err
	}
	all := append([]RuleInfo{}, hotRules...)
	all = append(all, coldRules...)
	patterns := make([]string, 0, len(all))
	for _, r := range all {
		p := encodeDirectives(r.Pattern, r.Directives)
		if r.IsExclude {
			p = "!" + p
		}
		patterns = append(patterns, p)
	}
	files, err := m.resolveFilesFromPatterns(patterns)
	if err != nil {
		return nil, err
	}
	return toSet(files, dir), nil
}

func sortedKeys(m map[string]struct{}) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
