package cmd

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/grovetools/core/logging"
	"github.com/spf13/cobra"
)

// sliceTestRun executes the slice command with three distinct sinks:
// the command's stdout, the command's stderr, and a stand-in for the PROCESS
// stdout that cx's loggers write to by default. Keeping the third separate is
// the point — a log line that reaches it is exactly the `cx show` defect this
// command must not reproduce.
func sliceTestRun(t *testing.T, args ...string) (stdout, stderr, processStdout *bytes.Buffer, err error) {
	t.Helper()

	processStdout = &bytes.Buffer{}
	previous := logging.SwapGlobalOutput(processStdout)
	t.Cleanup(func() { logging.SetGlobalOutput(previous) })

	stdout, stderr = &bytes.Buffer{}, &bytes.Buffer{}
	root := &cobra.Command{Use: "cx"}
	root.PersistentFlags().Bool("json", false, "")
	root.AddCommand(NewSliceCmd())
	root.SetOut(stdout)
	root.SetErr(stderr)
	root.SetArgs(append([]string{"slice"}, args...))
	return stdout, stderr, processStdout, root.Execute()
}

func sliceCmdFixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "pkg"), 0o755); err != nil {
		t.Fatal(err)
	}
	for name, body := range map[string]string{
		"pkg/alpha.go": "package pkg\n// alpha comment\nfunc Alpha() {}\n",
		"pkg/beta.go":  "package pkg\nfunc Beta() {}\n",
	} {
		if err := os.WriteFile(filepath.Join(dir, filepath.FromSlash(name)), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	withMachineWorkDir(t, dir)
	return dir
}

// TestSliceStdoutCarriesOnlyContent is the regression test for the defect the
// command was written to avoid: cx's loggers write to the process stdout by
// default, so `cx show` interleaves log lines with the bundle it prints. Slice
// must land every diagnostic on stderr and leave stdout byte-exact.
func TestSliceStdoutCarriesOnlyContent(t *testing.T) {
	sliceCmdFixture(t)

	stdout, stderr, processStdout, err := sliceTestRun(t, "pkg/alpha.go", "pkg/beta.go")
	if err != nil {
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
	if stdout.String() != want {
		t.Fatalf("stdout is not pure content:\ngot  %q\nwant %q", stdout.String(), want)
	}
	if processStdout.Len() != 0 {
		t.Fatalf("log output leaked to the process stdout: %q", processStdout.String())
	}
	// The redirection has to be real, not merely quiet: the command logs its
	// resolution context, and that line must be observable on stderr.
	if !strings.Contains(stderr.String(), "Slice resolution context") {
		t.Fatalf("resolution log did not reach stderr: %q", stderr.String())
	}
}

func TestSliceStripCommentsAppliesStripMachinery(t *testing.T) {
	sliceCmdFixture(t)

	stdout, _, _, err := sliceTestRun(t, "--strip-comments", "pkg/alpha.go")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(stdout.String(), "alpha comment") {
		t.Fatalf("comment survived --strip-comments: %q", stdout.String())
	}
	if !strings.Contains(stdout.String(), "func Alpha() {}") {
		t.Fatalf("code was lost along with the comment: %q", stdout.String())
	}
}

func TestSliceStatsEmitsCompactEnvelopeInsteadOfContent(t *testing.T) {
	sliceCmdFixture(t)

	stdout, _, _, err := sliceTestRun(t, "--stats", "pkg/*.go")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(stdout.String(), "<file path=") {
		t.Fatalf("--stats emitted content as well as the envelope: %q", stdout.String())
	}

	var envelope machineStatsEnvelope
	if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
		t.Fatalf("invalid compact JSON: %v\n%s", err, stdout.String())
	}
	if envelope.SchemaVersion != 1 {
		t.Fatalf("stats envelope must be schema_version 1: %+v", envelope)
	}
	if len(envelope.Contexts) != 2 || envelope.Contexts[0].ContextType != "hot" {
		t.Fatalf("envelope shape diverged from 'cx stats --format compact': %+v", envelope.Contexts)
	}
	if envelope.Totals.ResolvedFiles != 2 || envelope.Totals.TotalTokens == 0 {
		t.Fatalf("stats cannot preflight size: %+v", envelope.Totals)
	}
	if envelope.RulesPath != sliceStatsRulesPath {
		t.Fatalf("rules_path should name the slice, not a file that does not describe it: %q", envelope.RulesPath)
	}
}

func TestSliceEmptyResolutionExitsNonZero(t *testing.T) {
	sliceCmdFixture(t)

	stdout, _, _, err := sliceTestRun(t, "pkg/nothing-matches-*.zzz")
	if err == nil || !strings.Contains(err.Error(), "0 files resolved") {
		t.Fatalf("expected a non-zero exit on empty resolution, got %v", err)
	}
	if stdout.Len() != 0 {
		t.Fatalf("empty resolution still wrote to stdout: %q", stdout.String())
	}
}

func TestSliceWritesNoState(t *testing.T) {
	dir := sliceCmdFixture(t)

	if _, _, _, err := sliceTestRun(t, "pkg/*.go"); err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := sliceTestRun(t, "--stats", "pkg/*.go"); err != nil {
		t.Fatal(err)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	if len(names) != 1 || names[0] != "pkg" {
		t.Fatalf("slice left artifacts behind: %v", names)
	}
}
