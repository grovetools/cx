package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseStamp(t *testing.T) {
	cases := []struct {
		name       string
		line       string
		wantTokens int
		wantFiles  int
		wantOK     bool
	}{
		{
			name:       "real overview stamp line",
			line:       "Preset: `concept-rules-dsl` — load with `cx rules load /nb/context/presets/concept-rules-dsl.rules`, or reference from any grovetools repo with `@a:cx::concept-rules-dsl`. (~27k tokens, 7 files)",
			wantTokens: 27000,
			wantFiles:  7,
			wantOK:     true,
		},
		{
			name:       "bare stamp",
			line:       "(~132k tokens, 29 files)",
			wantTokens: 132000,
			wantFiles:  29,
			wantOK:     true,
		},
		{
			name:       "fractional k",
			line:       "(~1.5k tokens, 3 files)",
			wantTokens: 1500,
			wantFiles:  3,
			wantOK:     true,
		},
		{
			name:       "sub-1k stamp without k suffix",
			line:       "(~800 tokens, 2 files)",
			wantTokens: 800,
			wantFiles:  2,
			wantOK:     true,
		},
		{
			name:   "no stamp",
			line:   "Preset: `concept-x` — load with cx rules load.",
			wantOK: false,
		},
		{
			name:   "tokens without files is not a stamp",
			line:   "(~12k tokens)",
			wantOK: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tokens, files, ok := parseStamp(tc.line)
			if ok != tc.wantOK {
				t.Fatalf("parseStamp(%q) ok = %v, want %v", tc.line, ok, tc.wantOK)
			}
			if !ok {
				return
			}
			if tokens != tc.wantTokens || files != tc.wantFiles {
				t.Errorf("parseStamp(%q) = (%d, %d), want (%d, %d)", tc.line, tokens, files, tc.wantTokens, tc.wantFiles)
			}
		})
	}
}

func TestFormatStampRoundTrip(t *testing.T) {
	cases := []struct {
		tokens, files int
		want          string
	}{
		{27412, 12, "(~27k tokens, 12 files)"},
		{27500, 12, "(~28k tokens, 12 files)"},
		{1000, 1, "(~1k tokens, 1 files)"},
		{999, 2, "(~999 tokens, 2 files)"},
		{0, 0, "(~0 tokens, 0 files)"},
	}
	for _, tc := range cases {
		got := formatStamp(tc.tokens, tc.files)
		if got != tc.want {
			t.Errorf("formatStamp(%d, %d) = %q, want %q", tc.tokens, tc.files, got, tc.want)
		}
		// Everything formatStamp writes must parse back.
		if _, _, ok := parseStamp(got); !ok {
			t.Errorf("formatStamp output %q does not round-trip through parseStamp", got)
		}
	}
}

func TestDriftPct(t *testing.T) {
	cases := []struct {
		stamped, measured int
		want              float64
	}{
		{27000, 27000, 0},
		{27000, 29700, 10},
		{10000, 5000, 50},
		{10000, 15000, 50},
		{0, 0, 0},
		{0, 500, 100},
	}
	for _, tc := range cases {
		got := driftPct(tc.stamped, tc.measured)
		if got != tc.want {
			t.Errorf("driftPct(%d, %d) = %v, want %v", tc.stamped, tc.measured, got, tc.want)
		}
	}
}

// fixtureOverview builds a realistic concept overview.md body around a stamp.
const fixtureOverview = `---
title: Rules DSL
tags: [concepts, cx]
---

# Rules DSL

Body text that must never change.

## Design

- bullet one
- bullet two

## Context

Preset: ` + "`concept-rules-dsl`" + ` — load with ` + "`cx rules load /nb/context/presets/concept-rules-dsl.rules`" + `, or reference from any grovetools repo with ` + "`@a:cx::concept-rules-dsl`" + `. (~27k tokens, 7 files)

## Related

- another-concept
`

// writeFixtureNotebook lays out a temp notebook workspace dir with a concept
// overview.md and returns the overview path.
func writeFixtureNotebook(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	conceptDir := filepath.Join(dir, "workspaces", "cx", "concepts", "rules-dsl")
	if err := os.MkdirAll(conceptDir, 0o755); err != nil {
		t.Fatal(err)
	}
	overview := filepath.Join(conceptDir, "overview.md")
	if err := os.WriteFile(overview, []byte(fixtureOverview), 0o644); err != nil {
		t.Fatal(err)
	}
	return overview
}

func TestFindStamp(t *testing.T) {
	tokens, files, ok := findStamp([]byte(fixtureOverview))
	if !ok {
		t.Fatal("findStamp did not locate the stamp in the fixture overview")
	}
	if tokens != 27000 || files != 7 {
		t.Errorf("findStamp = (%d, %d), want (27000, 7)", tokens, files)
	}

	if _, _, ok := findStamp([]byte("# No stamp here\n\nbody\n")); ok {
		t.Error("findStamp reported a stamp in stampless content")
	}
}

func TestRewriteStampFileIsSurgical(t *testing.T) {
	overview := writeFixtureNotebook(t)

	original, err := os.ReadFile(overview)
	if err != nil {
		t.Fatal(err)
	}

	if err := rewriteStampFile(overview, 31240, 9); err != nil {
		t.Fatalf("rewriteStampFile: %v", err)
	}

	rewritten, err := os.ReadFile(overview)
	if err != nil {
		t.Fatal(err)
	}

	// Only the stamp substring may differ; every other byte must be identical.
	want := strings.Replace(string(original), "(~27k tokens, 7 files)", "(~31k tokens, 9 files)", 1)
	if string(rewritten) != want {
		t.Errorf("rewriteStampFile changed more than the stamp:\n--- got ---\n%s\n--- want ---\n%s", rewritten, want)
	}

	tokens, files, ok := findStamp(rewritten)
	if !ok || tokens != 31000 || files != 9 {
		t.Errorf("rewritten stamp parses as (%d, %d, %v), want (31000, 9, true)", tokens, files, ok)
	}
}

func TestRewriteStampFileNoStamp(t *testing.T) {
	dir := t.TempDir()
	overview := filepath.Join(dir, "overview.md")
	if err := os.WriteFile(overview, []byte("# Concept\n\nno stamp\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := rewriteStampFile(overview, 1000, 2); err == nil {
		t.Error("expected error rewriting a stampless overview, got nil")
	}
}

func TestListConceptIDs(t *testing.T) {
	dir := t.TempDir()
	for _, id := range []string{"zeta", "alpha"} {
		conceptDir := filepath.Join(dir, id)
		if err := os.MkdirAll(conceptDir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(conceptDir, "overview.md"), []byte("# x\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	// A directory without overview.md and a stray file must be ignored.
	if err := os.MkdirAll(filepath.Join(dir, "not-a-concept"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	ids, err := listConceptIDs(dir)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"alpha", "zeta"}
	if len(ids) != len(want) || ids[0] != want[0] || ids[1] != want[1] {
		t.Errorf("listConceptIDs = %v, want %v", ids, want)
	}
}

func TestConceptFails(t *testing.T) {
	cases := []struct {
		name string
		r    conceptReport
		want bool
	}{
		{"clean", conceptReport{StampFound: true}, false},
		{"missing stamp", conceptReport{StampFound: false}, true},
		{"drift exceeded", conceptReport{StampFound: true, DriftExceeded: true}, true},
		{"file mismatch", conceptReport{StampFound: true, FileMismatch: true}, true},
		{"drift fixed", conceptReport{StampFound: true, DriftExceeded: true, FileMismatch: true, Fixed: true}, false},
		{"dead alias", conceptReport{StampFound: true, DeadAliases: []conceptFinding{{Message: "dead"}}}, true},
		{"zero-match lint", conceptReport{StampFound: true, LintIssues: []conceptFinding{{Severity: "Warning", Message: zeroMatchMessage}}}, true},
		{"benign lint warning", conceptReport{StampFound: true, LintIssues: []conceptFinding{{Severity: "Warning", Message: "Pattern is overly broad and may match too many files"}}}, false},
		{"lint error", conceptReport{StampFound: true, LintIssues: []conceptFinding{{Severity: "Error", Message: "boom"}}}, true},
		{"processing error", conceptReport{StampFound: true, Errors: []string{"cannot read preset"}}, true},
		{"fix does not absolve dead alias", conceptReport{StampFound: true, Fixed: true, DeadAliases: []conceptFinding{{Message: "dead"}}}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := conceptFails(tc.r); got != tc.want {
				t.Errorf("conceptFails(%+v) = %v, want %v", tc.r, got, tc.want)
			}
		})
	}
}
