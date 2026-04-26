package context

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

type regressionFixture struct {
	name  string
	files []string
	rules string
	// expected is the set of relative file paths the AST resolver should produce.
	expected []string
	// skip, when non-empty, marks the fixture as unsupported in this harness.
	skip string
	// extraFiles are written verbatim into the workspace before running.
	extraFiles map[string]string
}

func regressionCorpus() []regressionFixture {
	return []regressionFixture{
		{
			name:     "literal_only",
			files:    []string{"main.go", "README.md", "src/a.go"},
			rules:    "main.go\nREADME.md\n",
			expected: []string{"main.go", "README.md"},
		},
		{
			name:     "star_glob_basename",
			files:    []string{"main.go", "helper.go", "README.md"},
			rules:    "*.go\n",
			expected: []string{"main.go", "helper.go"},
		},
		{
			name:     "doublestar",
			files:    []string{"src/a.go", "src/sub/b.go", "src/sub/deep/c.go", "README.md"},
			rules:    "**/*.go\n",
			expected: []string{"src/a.go", "src/sub/b.go", "src/sub/deep/c.go"},
		},
		{
			name:     "exclusion_basename",
			files:    []string{"main.go", "main_test.go", "helper.go", "util_test.go"},
			rules:    "*.go\n!*_test.go\n",
			expected: []string{"main.go", "helper.go"},
		},
		{
			name:     "brace_expansion",
			files:    []string{"src/a.go", "src/b.go", "src/c.go"},
			rules:    "src/{a,b}.go\n",
			expected: []string{"src/a.go", "src/b.go"},
		},
		{
			name:     "trailing_slash_dir",
			files:    []string{"src/a.go", "src/sub/b.go", "other.go"},
			rules:    "src/\n",
			expected: []string{"src/a.go", "src/sub/b.go"},
		},
		{
			name: "mixed_literal_and_glob_with_exclude",
			files: []string{
				"main.go", "README.md",
				"pkg/foo.go", "pkg/foo_test.go",
				"pkg/bar.go", "vendor/dep.go",
			},
			rules:    "main.go\npkg/**/*.go\n!*_test.go\n!vendor/**\n",
			expected: []string{"main.go", "pkg/foo.go", "pkg/bar.go"},
		},
		{
			name: "comments_and_blanks",
			files: []string{
				"src/a.go", "src/b.go", "docs/x.md",
			},
			rules:    "# header\n\nsrc/**/*.go\n\n# another\ndocs/**/*.md\n",
			expected: []string{"src/a.go", "src/b.go", "docs/x.md"},
		},
		{
			name:     "deep_doublestar_with_suffix",
			files:    []string{"a/b/c/foo.txt", "a/foo.txt", "x/foo.go"},
			rules:    "**/foo.txt\n",
			expected: []string{"a/b/c/foo.txt", "a/foo.txt"},
		},
		{
			name:     "exclude_then_reinclude_basename",
			files:    []string{"main.go", "main_test.go", "helper.go"},
			rules:    "*.go\n!main_test.go\n",
			expected: []string{"main.go", "helper.go"},
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
			rules:    "**/*.go @grep: \"package\"\n",
			expected: []string{"keep_a.go", "keep_b.go", "skip.go"},
		},
		{
			name: "filter_find_on_glob",
			files: []string{
				"src/util_helper.go", "src/main.go", "src/util_other.go",
			},
			rules:    "**/*.go @find: \"util\"\n",
			expected: []string{"src/util_helper.go", "src/util_other.go"},
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
			rules:    "**/*.go @grep: \"package\"\n!vendor/**\n",
			expected: []string{"a.go", "b.go"},
		},
		{
			name: "hotcold_split_basic",
			files: []string{
				"main.go", "helper.go", "vendor/dep.go", "README.md",
			},
			rules: "**/*.go\nREADME.md\n---\nvendor/**\n",
			// AST sees the union (no hot/cold partition at this layer).
			expected: []string{"main.go", "helper.go", "README.md", "vendor/dep.go"},
		},
		{
			name:  "default_directive_local",
			files: []string{"main.go", "extra.go"},
			rules: "main.go\n",
			skip:  "@default requires grove.toml workspace + default_rules preset infra",
		},
		{
			name:  "cmd_directive",
			files: []string{"main.go", "helper.go"},
			rules: "@cmd: ls\n",
			// ls in the temp dir returns all files including grove.toml and test.rules.
			expected: []string{"grove.toml", "helper.go", "main.go", "test.rules"},
		},
		{
			name:  "alias_to_workspace_subpath",
			files: []string{"main.go"},
			rules: "@a:test:proj/main.go\n",
			skip:  "alias resolution requires ecosystem config not available in this harness",
		},
		{
			name:  "imports_of_imports",
			files: []string{"main.go", "helper.go"},
			extraFiles: map[string]string{
				"nested.rules": "helper.go\n",
			},
			rules: "main.go\nnested.rules\n",
			// nested.rules is treated as a literal file, not an import.
			expected: []string{"main.go", "nested.rules"},
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
			rules:    "**/*.go @grep: \"package\" @grep: \"keyword\"\n",
			expected: []string{"a.go"},
		},
		{
			name:  "exclude_with_alias_negation",
			files: []string{"main.go"},
			rules: "@a:eco:proj\n!@a:eco:proj/sub\n",
			skip:  "alias resolution requires ecosystem config not available in this harness",
		},
	}
}

func setupRegressionFixture(t *testing.T, fx regressionFixture) (*Manager, string) {
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

func flattenAttrSet(attr AttributionResult, base string) map[string]struct{} {
	out := map[string]struct{}{}
	for _, files := range attr {
		for _, f := range files {
			rel, err := filepath.Rel(base, f)
			if err == nil {
				out[filepath.ToSlash(rel)] = struct{}{}
			} else {
				out[filepath.ToSlash(f)] = struct{}{}
			}
		}
	}
	return out
}

func TestRegression_ASTResolver(t *testing.T) {
	for _, fx := range regressionCorpus() {
		t.Run(fx.name, func(t *testing.T) {
			if fx.skip != "" {
				t.Skip(fx.skip)
			}
			m, dir := setupRegressionFixture(t, fx)

			nodes, perrs := ParseToAST([]byte(fx.rules))
			if len(perrs) != 0 {
				t.Fatalf("ParseToAST produced parse errors: %+v", perrs)
			}
			astAttr, _, _ := ResolveAST(nodes, newProdResolutionContext(m))
			astSet := flattenAttrSet(astAttr, dir)

			expectedSet := make(map[string]struct{}, len(fx.expected))
			for _, f := range fx.expected {
				expectedSet[f] = struct{}{}
			}

			var onlyExpected, onlyAST []string
			for k := range expectedSet {
				if _, ok := astSet[k]; !ok {
					onlyExpected = append(onlyExpected, k)
				}
			}
			for k := range astSet {
				if _, ok := expectedSet[k]; !ok {
					onlyAST = append(onlyAST, k)
				}
			}
			sort.Strings(onlyExpected)
			sort.Strings(onlyAST)

			if len(onlyExpected) != 0 || len(onlyAST) != 0 {
				t.Fatalf(
					"MISMATCH in fixture %q (workDir=%s)\nrules:\n%s\nmissing (%d):\n  %s\nextra (%d):\n  %s",
					fx.name, dir, fx.rules,
					len(onlyExpected), strings.Join(onlyExpected, "\n  "),
					len(onlyAST), strings.Join(onlyAST, "\n  "),
				)
			}
		})
	}
}
