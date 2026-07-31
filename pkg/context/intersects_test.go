package context

import (
	"os"
	"path/filepath"
	"testing"
)

// Intersects backs `grove internal test-smart`: a repo's [[test_scopes]] point
// at a rules file, and a dirty file that lands inside that rules file's set is
// what selects the scope's tend scenarios.
//
// The regression: resolveFilesViaAST returns paths relative to the rules base
// dir, and Intersects compared them against absolute paths, so it returned
// false for every input — including a file the rules obviously match. Nothing
// failed loudly; test-smart just fell through to running the whole suite.
func TestIntersectsMatchesAFileInsideTheRules(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "pkg", "wire", "proto.go"), "package wire\n")
	mustWrite(t, filepath.Join(dir, "internal", "app", "hud.go"), "package app\n")
	mustWrite(t, filepath.Join(dir, ".cx", "scope.rules"), "pkg/wire/**/*.go\n")

	mgr := NewManager(dir)

	hit, err := mgr.Intersects(".cx/scope.rules", []string{"pkg/wire/proto.go"})
	if err != nil {
		t.Fatalf("Intersects: %v", err)
	}
	if !hit {
		t.Error("a dirty file inside the rules did not intersect")
	}

	hit, err = mgr.Intersects(".cx/scope.rules", []string{"internal/app/hud.go"})
	if err != nil {
		t.Fatalf("Intersects: %v", err)
	}
	if hit {
		t.Error("a dirty file outside the rules intersected")
	}
}

// Callers may hand over either form; both must resolve to the same file.
func TestIntersectsAcceptsAbsoluteChangedPaths(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "pkg", "wire", "proto.go"), "package wire\n")
	mustWrite(t, filepath.Join(dir, ".cx", "scope.rules"), "pkg/wire/proto.go\n")

	mgr := NewManager(dir)

	hit, err := mgr.Intersects(".cx/scope.rules", []string{filepath.Join(dir, "pkg", "wire", "proto.go")})
	if err != nil {
		t.Fatalf("Intersects: %v", err)
	}
	if !hit {
		t.Error("an absolute dirty path inside the rules did not intersect")
	}
}

func TestIntersectsMissingRulesFileIsAnError(t *testing.T) {
	mgr := NewManager(t.TempDir())
	if _, err := mgr.Intersects(".cx/nope.rules", []string{"main.go"}); err == nil {
		t.Error("expected an error for a rules file that does not exist")
	}
}

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
