package cmd

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/grovetools/cx/pkg/context"
	"github.com/spf13/cobra"
)

func machineTestRoot(child *cobra.Command, args ...string) (*bytes.Buffer, error) {
	root := &cobra.Command{Use: "cx"}
	root.PersistentFlags().Bool("json", false, "")
	root.AddCommand(child)
	out := &bytes.Buffer{}
	root.SetOut(out)
	root.SetErr(out)
	root.SetArgs(args)
	return out, root.Execute()
}

func writeMachineFixture(t *testing.T) (string, string) {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "hot.go"), []byte("package fixture\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "cold.md"), []byte("# Cold\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	rules := filepath.Join(dir, "concept-fixture.rules")
	if err := os.WriteFile(rules, []byte("hot.go\n---\ncold.md\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir, rules
}

func withMachineWorkDir(t *testing.T, dir string) {
	t.Helper()
	old := GlobalWorkDir
	GlobalWorkDir = dir
	t.Cleanup(func() { GlobalWorkDir = old })
}

func TestBuildMachineStatsReportsResolvedReadableAndUnreadable(t *testing.T) {
	dir := t.TempDir()
	readable := filepath.Join(dir, "readable.go")
	missing := filepath.Join(dir, "missing.go")
	if err := os.WriteFile(readable, []byte("package fixture\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	mgr := context.NewManager(dir)
	envelope, err := buildMachineStats(mgr, "fixture", "fixture.rules", []string{readable, missing}, nil, 1, 1)
	if err != nil {
		t.Fatal(err)
	}
	if envelope.SchemaVersion != 1 || len(envelope.Contexts) != 2 {
		t.Fatalf("unexpected envelope: %+v", envelope)
	}
	hot := envelope.Contexts[0]
	if hot.ResolvedFiles != 2 || hot.ReadableFiles != 1 || envelope.Totals.Unreadable != 1 {
		t.Fatalf("dishonest totals: hot=%+v totals=%+v", hot, envelope.Totals)
	}
	if len(hot.Files) != 1 || hot.FilesOmitted != 1 {
		t.Fatalf("manifest was not bounded: %+v", hot)
	}
	if len(hot.UnreadableFiles) != 1 || hot.UnreadableFiles[0] != missing {
		t.Fatalf("unreadable file not reported: %+v", hot.UnreadableFiles)
	}
	if envelope.Contexts[1].Files == nil || envelope.Contexts[1].LargestFiles == nil {
		t.Fatal("empty cold arrays must encode as [] rather than null")
	}
}

func TestListJSONSeparatesHotAndCold(t *testing.T) {
	dir, rules := writeMachineFixture(t)
	withMachineWorkDir(t, dir)
	out, err := machineTestRoot(NewListCmd(), "list", "--json", "--rules-file", rules)
	if err != nil {
		t.Fatal(err)
	}
	var got machineListEnvelope
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("invalid list JSON: %v\n%s", err, out.String())
	}
	if got.SchemaVersion != 1 || len(got.HotFiles) != 1 || len(got.ColdFiles) != 1 {
		t.Fatalf("unexpected list envelope: %+v", got)
	}
	if filepath.Base(got.HotFiles[0]) != "hot.go" || filepath.Base(got.ColdFiles[0]) != "cold.md" {
		t.Fatalf("hot/cold identity lost: %+v", got)
	}
}

func TestListLegacyOutputRemainsNewlineHotOnly(t *testing.T) {
	dir, rules := writeMachineFixture(t)
	withMachineWorkDir(t, dir)
	out, err := machineTestRoot(NewListCmd(), "list", "--rules-file", rules)
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(dir, "hot.go") + "\n"
	if out.String() != want {
		t.Fatalf("legacy list bytes changed:\ngot  %q\nwant %q", out.String(), want)
	}
}

func TestListJSONErrorsOnEmptyResolution(t *testing.T) {
	dir := t.TempDir()
	rules := filepath.Join(dir, "empty.rules")
	if err := os.WriteFile(rules, []byte("# no files\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	withMachineWorkDir(t, dir)
	_, err := machineTestRoot(NewListCmd(), "list", "--json", "--rules-file", rules)
	if err == nil || !strings.Contains(err.Error(), "0 files resolved") {
		t.Fatalf("expected tool-safe empty error, got %v", err)
	}
}

func TestStatsCompactEnvelopeAndLegacyJSONCompatibility(t *testing.T) {
	dir, rules := writeMachineFixture(t)
	withMachineWorkDir(t, dir)

	compactOut, err := machineTestRoot(NewStatsCmd(), "stats", "--format", "compact", "--rules-file", rules)
	if err != nil {
		t.Fatal(err)
	}
	var compact machineStatsEnvelope
	if err := json.Unmarshal(compactOut.Bytes(), &compact); err != nil {
		t.Fatalf("invalid compact JSON: %v\n%s", err, compactOut.String())
	}
	if compact.SchemaVersion != 1 || len(compact.Contexts) != 2 || compact.Totals.ResolvedFiles != 2 {
		t.Fatalf("unexpected compact stats: %+v", compact)
	}

	legacyOut, err := machineTestRoot(NewStatsCmd(), "stats", "--json", "--rules-file", rules)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(legacyOut.String(), "[\n") || strings.Contains(legacyOut.String(), "schema_version") {
		t.Fatalf("legacy --json shape changed: %s", legacyOut.String())
	}
	var legacy []context.ContextStats
	if err := json.Unmarshal(legacyOut.Bytes(), &legacy); err != nil || len(legacy) != 2 {
		t.Fatalf("legacy JSON no longer decodes as ContextStats array: %v %+v", err, legacy)
	}
}

func TestStatsCompactErrorsOnEmptyResolution(t *testing.T) {
	dir := t.TempDir()
	rules := filepath.Join(dir, "empty.rules")
	if err := os.WriteFile(rules, []byte("# no files\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	withMachineWorkDir(t, dir)
	_, err := machineTestRoot(NewStatsCmd(), "stats", "--format", "compact", "--rules-file", rules)
	if err == nil || !strings.Contains(err.Error(), "0 files resolved") {
		t.Fatalf("expected tool-safe empty error, got %v", err)
	}
}

func TestCanonicalConceptPresetPath(t *testing.T) {
	dir := filepath.Join("notebook", "workspaces", "cx", "context", "presets")
	got, err := canonicalConceptPresetPath(dir, "hot-cold-artifacts-stats")
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(dir, "concept-hot-cold-artifacts-stats.rules")
	if got != want {
		t.Fatalf("canonical preset = %q, want %q", got, want)
	}
	if strings.Contains(got, filepath.Join("concepts", "hot-cold-artifacts-stats", "context.rules")) {
		t.Fatalf("used legacy concept-local context.rules path: %s", got)
	}
	if _, err := canonicalConceptPresetPath(dir, "../escape"); err == nil {
		t.Fatal("expected unsafe concept id rejection")
	}
}
