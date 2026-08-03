package context

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// sliceFixture lays out a small tree plus a sibling directory to hold rules
// files. The rules file deliberately lives OUTSIDE the tree it describes, so a
// pattern like "*.go" cannot accidentally match the rules file itself and make
// the parity comparison trivially true.
func sliceFixture(t *testing.T) (root, rulesDir string) {
	t.Helper()
	base := t.TempDir()
	root = filepath.Join(base, "repo")
	rulesDir = filepath.Join(base, "rules")
	for _, dir := range []string{
		filepath.Join(root, "pkg"),
		filepath.Join(root, "cmd"),
		rulesDir,
	} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	files := map[string]string{
		"pkg/alpha.go":      "package pkg\n// alpha comment\nfunc Alpha() {}\n",
		"pkg/beta.go":       "package pkg\nfunc Beta() {}\n",
		"pkg/alpha_test.go": "package pkg\nfunc TestAlpha() {}\n",
		"cmd/main.go":       "package main\nfunc main() {}\n",
		"README.md":         "# readme\n",
	}
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(name)), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root, rulesDir
}

// TestResolveSliceMatchesRulesFileResolution is the load-bearing guarantee of
// `cx slice`: a caller who types lines on the command line gets exactly the
// file set they would get from a rules file holding those same lines.
func TestResolveSliceMatchesRulesFileResolution(t *testing.T) {
	root, rulesDir := sliceFixture(t)

	cases := []struct {
		name  string
		lines []string
	}{
		{"literal path", []string{"pkg/alpha.go"}},
		{"glob", []string{"pkg/*.go"}},
		{"recursive glob", []string{"**/*.go"}},
		{"glob with exclusion", []string{"pkg/*.go", "!pkg/*_test.go"}},
		{"brace expansion", []string{"pkg/{alpha,beta}.go"}},
		{"directory expands recursively", []string{"pkg"}},
		{"multiple lines", []string{"cmd/main.go", "pkg/beta.go", "README.md"}},
		{"grep directive", []string{`pkg/*.go @grep: "func Beta"`}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rulesPath := filepath.Join(rulesDir, strings.ReplaceAll(tc.name, " ", "-")+".rules")
			if err := os.WriteFile(rulesPath, []byte(strings.Join(tc.lines, "\n")+"\n"), 0o644); err != nil {
				t.Fatal(err)
			}

			mgr := NewManager(root)
			want, cold, err := mgr.ResolveFilesFromCustomRulesFile(rulesPath)
			if err != nil {
				t.Fatalf("rules-file resolution failed: %v", err)
			}
			if len(cold) != 0 {
				t.Fatalf("fixture unexpectedly produced cold files: %v", cold)
			}

			got, trees, err := mgr.ResolveSlice(tc.lines)
			if err != nil {
				t.Fatalf("slice resolution failed: %v", err)
			}
			if len(trees) != 0 {
				t.Fatalf("unexpected tree paths: %v", trees)
			}
			if len(want) == 0 {
				t.Fatalf("fixture case resolved nothing; the comparison would be vacuous")
			}
			if strings.Join(got, "\n") != strings.Join(want, "\n") {
				t.Fatalf("slice diverged from rules file\nslice: %v\nrules: %v", got, want)
			}
		})
	}
}

func TestResolveSliceRejectsHotColdSeparator(t *testing.T) {
	root, _ := sliceFixture(t)
	_, _, err := NewManager(root).ResolveSlice([]string{"pkg/alpha.go", "---", "pkg/beta.go"})
	if err == nil || !strings.Contains(err.Error(), "hot/cold separator") {
		t.Fatalf("expected the separator to be refused rather than silently dropping lines, got %v", err)
	}
}

func TestResolveSliceReportsTreePaths(t *testing.T) {
	root, _ := sliceFixture(t)
	files, trees, err := NewManager(root).ResolveSlice([]string{"pkg/alpha.go", "@tree:pkg"})
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 {
		t.Fatalf("tree line should not add files: %v", files)
	}
	if len(trees) != 1 {
		t.Fatalf("tree path was swallowed instead of reported: %v", trees)
	}
}

func TestResolveSliceWritesNoState(t *testing.T) {
	root, _ := sliceFixture(t)
	if _, _, err := NewManager(root).ResolveSlice([]string{"**/*.go"}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, GroveDir)); !os.IsNotExist(err) {
		t.Fatalf("slice created %s; it must write no state (stat err: %v)", GroveDir, err)
	}
}

// TestWriteSliceRendersGenerateBytes pins the wire format: the same
// `<file path=…>` blocks generate and flow's layer renderer emit, at the layer
// renderer's two-space indent, inside a single well-formed envelope.
func TestWriteSliceRendersGenerateBytes(t *testing.T) {
	root, _ := sliceFixture(t)
	mgr := NewManager(root)
	mgr.SetStripComments(false)

	var buf bytes.Buffer
	if err := mgr.WriteSlice(&buf, []string{"pkg/alpha.go", "pkg/beta.go"}); err != nil {
		t.Fatal(err)
	}

	want := "<slice files=\"2\">\n" +
		"  <file path=\"pkg/alpha.go\">\n" +
		"package pkg\n// alpha comment\nfunc Alpha() {}\n" +
		"  </file>\n" +
		"  <file path=\"pkg/beta.go\">\n" +
		"package pkg\nfunc Beta() {}\n" +
		"  </file>\n" +
		"</slice>\n"
	if buf.String() != want {
		t.Fatalf("slice bytes changed:\ngot  %q\nwant %q", buf.String(), want)
	}
}

func TestWriteSliceAppliesStripComments(t *testing.T) {
	root, _ := sliceFixture(t)
	mgr := NewManager(root)
	mgr.SetStripComments(true)
	t.Cleanup(func() { mgr.SetStripComments(false) })

	var buf bytes.Buffer
	if err := mgr.WriteSlice(&buf, []string{"pkg/alpha.go"}); err != nil {
		t.Fatal(err)
	}

	// The exact stripped body must match the shared strip machinery, not a
	// slice-local approximation of it.
	stripped := string(StripComments("pkg/alpha.go", []byte("package pkg\n// alpha comment\nfunc Alpha() {}\n")))
	want := "<slice files=\"1\">\n  <file path=\"pkg/alpha.go\">\n" + stripped + "  </file>\n</slice>\n"
	if buf.String() != want {
		t.Fatalf("stripped slice bytes changed:\ngot  %q\nwant %q", buf.String(), want)
	}
	if strings.Contains(buf.String(), "alpha comment") {
		t.Fatal("comment survived --strip-comments")
	}
}

// An unreadable file must be loud in both channels: a visible placeholder in
// the stream and a non-nil error, so a thinned bundle can never pass for whole.
func TestWriteSliceErrorsOnUnreadableFile(t *testing.T) {
	root, _ := sliceFixture(t)
	mgr := NewManager(root)
	mgr.SetStripComments(false)

	var buf bytes.Buffer
	err := mgr.WriteSlice(&buf, []string{"pkg/alpha.go", "pkg/missing.go"})
	if err == nil || !strings.Contains(err.Error(), "unreadable") {
		t.Fatalf("expected an unreadable-file error, got %v", err)
	}
	if !strings.Contains(buf.String(), "<error>") {
		t.Fatalf("hole not marked in the stream: %q", buf.String())
	}
}
