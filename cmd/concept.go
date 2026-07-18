package cmd

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/grovetools/core/pkg/workspace"
	"github.com/spf13/cobra"

	"github.com/grovetools/cx/pkg/context"
)

// stampRegex matches the `## Context` size stamp maintained in concept
// overview.md files, e.g. "(~27k tokens, 12 files)" or "(~800 tokens, 3 files)".
var stampRegex = regexp.MustCompile(`\(~(\d+(?:\.\d+)?)(k?) tokens, (\d+) files\)`)

// parseStamp extracts stamped token/file counts from a single line.
// "(~27k tokens, 12 files)" yields (27000, 12, true).
func parseStamp(line string) (tokens, files int, ok bool) {
	m := stampRegex.FindStringSubmatch(line)
	if m == nil {
		return 0, 0, false
	}
	val, err := strconv.ParseFloat(m[1], 64)
	if err != nil {
		return 0, 0, false
	}
	if m[2] == "k" {
		val *= 1000
	}
	files, err = strconv.Atoi(m[3])
	if err != nil {
		return 0, 0, false
	}
	return int(math.Round(val)), files, true
}

// findStamp locates the first stamp in an overview.md body.
func findStamp(content []byte) (tokens, files int, ok bool) {
	loc := stampRegex.FindIndex(content)
	if loc == nil {
		return 0, 0, false
	}
	return parseStamp(string(content[loc[0]:loc[1]]))
}

// formatStamp renders measured numbers in the stamp convention used by
// concept overview docs: tokens rounded to the nearest 1k when >= 1000.
func formatStamp(tokens, files int) string {
	if tokens >= 1000 {
		return fmt.Sprintf("(~%dk tokens, %d files)", int(math.Round(float64(tokens)/1000)), files)
	}
	return fmt.Sprintf("(~%d tokens, %d files)", tokens, files)
}

// rewriteStampFile surgically replaces ONLY the stamp substring in
// overviewPath with the measured numbers; every other byte is preserved.
func rewriteStampFile(overviewPath string, tokens, files int) error {
	content, err := os.ReadFile(overviewPath)
	if err != nil {
		return err
	}
	loc := stampRegex.FindIndex(content)
	if loc == nil {
		return fmt.Errorf("no context stamp found in %s", overviewPath)
	}
	info, err := os.Stat(overviewPath)
	if err != nil {
		return err
	}
	var out []byte
	out = append(out, content[:loc[0]]...)
	out = append(out, []byte(formatStamp(tokens, files))...)
	out = append(out, content[loc[1]:]...)
	return os.WriteFile(overviewPath, out, info.Mode().Perm())
}

// driftPct computes the token drift percentage of measured vs stamped.
func driftPct(stamped, measured int) float64 {
	if stamped == 0 {
		if measured == 0 {
			return 0
		}
		return 100
	}
	return math.Abs(float64(measured-stamped)) / float64(stamped) * 100
}

// listConceptIDs enumerates concept directories (those containing an
// overview.md) under the workspace concepts dir.
func listConceptIDs(conceptsDir string) ([]string, error) {
	entries, err := os.ReadDir(conceptsDir)
	if err != nil {
		return nil, fmt.Errorf("cannot read concepts directory %s: %w", conceptsDir, err)
	}
	var ids []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		if _, err := os.Stat(filepath.Join(conceptsDir, e.Name(), "overview.md")); err == nil {
			ids = append(ids, e.Name())
		}
	}
	sort.Strings(ids)
	return ids, nil
}

// conceptFinding is a single preset lint / dead-alias finding.
type conceptFinding struct {
	LineNum  int    `json:"line"`
	Rule     string `json:"rule,omitempty"`
	Severity string `json:"severity,omitempty"`
	Message  string `json:"message"`
}

// conceptReport is the per-concept verify result.
type conceptReport struct {
	Concept        string           `json:"concept"`
	Workspace      string           `json:"workspace,omitempty"`
	OverviewPath   string           `json:"overview_path"`
	PresetPath     string           `json:"preset_path,omitempty"`
	StampFound     bool             `json:"stamp_found"`
	StampedTokens  int              `json:"stamped_tokens"`
	StampedFiles   int              `json:"stamped_files"`
	MeasuredTokens int              `json:"measured_tokens"`
	MeasuredFiles  int              `json:"measured_files"`
	TokenDriftPct  float64          `json:"token_drift_pct"`
	DriftExceeded  bool             `json:"drift_exceeded"`
	FileMismatch   bool             `json:"file_mismatch"`
	Fixed          bool             `json:"fixed,omitempty"`
	DeadAliases    []conceptFinding `json:"dead_aliases,omitempty"`
	LintIssues     []conceptFinding `json:"lint_issues,omitempty"`
	Errors         []string         `json:"errors,omitempty"`
	OK             bool             `json:"ok"`
}

// zeroMatchMessage is the lint warning emitted by pkg/context/lint.go for
// literal/glob patterns that resolve to nothing.
const zeroMatchMessage = "Pattern matches 0 files in the workspace"

// conceptFails reports whether a concept should fail verification:
// unfixed drift (token drift beyond tolerance, any file-count mismatch, or a
// missing stamp), dead alias lines, zero-match patterns, lint errors, or
// processing errors.
func conceptFails(r conceptReport) bool {
	driftBad := (!r.StampFound || r.DriftExceeded || r.FileMismatch) && !r.Fixed
	if driftBad || len(r.DeadAliases) > 0 || len(r.Errors) > 0 {
		return true
	}
	for _, li := range r.LintIssues {
		if li.Severity == "Error" || li.Message == zeroMatchMessage {
			return true
		}
	}
	return false
}

// isAliasRule reports whether a trimmed rule line is a workspace alias
// (@a:/@alias:) include line. Git aliases are excluded: their attribution is
// not tracked (paths are rewritten to clone dirs), so they cannot be
// dead-checked reliably.
func isAliasRule(trimmed string) bool {
	if strings.HasPrefix(trimmed, "@a:git:") || strings.HasPrefix(trimmed, "@alias:git:") {
		return false
	}
	return strings.HasPrefix(trimmed, "@a:") || strings.HasPrefix(trimmed, "@alias:")
}

// diagnoseAliasLine explains why an alias line contributed no files, using
// the manager's own alias resolution.
func diagnoseAliasLine(pm *context.Manager, rule string) string {
	body := strings.TrimPrefix(strings.TrimPrefix(rule, "@alias:"), "@a:")
	if fields := strings.Fields(body); len(fields) > 0 {
		body = fields[0]
	}
	if i := strings.Index(body, "::"); i >= 0 {
		project, ruleset := body[:i], body[i+2:]
		path, err := pm.ResolveProjectAlias(project)
		if err != nil {
			return fmt.Sprintf("dead alias: workspace '%s' does not resolve: %v", project, err)
		}
		if _, err := pm.FindRulesetFile(path, ruleset); err != nil {
			return fmt.Sprintf("dead alias: ruleset '%s' not found for workspace '%s' (%s)", ruleset, project, path)
		}
		return fmt.Sprintf("dead alias: ruleset '%s' resolves (%s) but contributes 0 files", ruleset, path)
	}
	project := body
	if i := strings.Index(body, "/"); i >= 0 {
		project = body[:i]
	}
	path, err := pm.ResolveProjectAlias(project)
	if err != nil {
		return fmt.Sprintf("dead alias: workspace '%s' does not resolve: %v", project, err)
	}
	return fmt.Sprintf("dead alias: resolves to %s but matches 0 files", path)
}

// detectDeadAliases flags @a:/@alias: include lines in the preset that
// contribute zero files to the assembled context.
func detectDeadAliases(pm *context.Manager, presetContent []byte) []conceptFinding {
	attribution, _, _, filtered, _, err := pm.ResolveFilesWithAttribution(string(presetContent))
	if err != nil {
		return []conceptFinding{{Severity: "Error", Message: fmt.Sprintf("attribution failed: %v", err)}}
	}
	var findings []conceptFinding
	lineNum := 0
	for _, raw := range strings.Split(string(presetContent), "\n") {
		lineNum++
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") || trimmed == "---" {
			continue
		}
		if strings.HasPrefix(trimmed, "!") {
			continue // exclusions are allowed to match nothing extra
		}
		if !isAliasRule(trimmed) {
			continue
		}
		if len(attribution[lineNum]) > 0 || len(filtered[lineNum]) > 0 {
			continue
		}
		findings = append(findings, conceptFinding{
			LineNum:  lineNum,
			Rule:     trimmed,
			Severity: "Warning",
			Message:  diagnoseAliasLine(pm, trimmed),
		})
	}
	return findings
}

// verifyConcept measures one concept preset the same way `cx stats` does and
// compares it against the overview.md stamp.
func verifyConcept(workDir, presetsDir, conceptsDir, id, workspaceName string, tolerance float64, fix bool) conceptReport {
	r := conceptReport{
		Concept:      id,
		Workspace:    workspaceName,
		OverviewPath: filepath.Join(conceptsDir, id, "overview.md"),
		PresetPath:   filepath.Join(presetsDir, "concept-"+id+context.RulesExt),
	}

	overview, err := os.ReadFile(r.OverviewPath)
	if err != nil {
		r.Errors = append(r.Errors, fmt.Sprintf("cannot read overview: %v", err))
		return r
	}
	r.StampedTokens, r.StampedFiles, r.StampFound = findStamp(overview)

	presetContent, err := os.ReadFile(r.PresetPath)
	if err != nil {
		r.Errors = append(r.Errors, fmt.Sprintf("cannot read preset: %v", err))
		return r
	}

	// Assemble the preset exactly like a job rules file: manager with a
	// rules-file override, then resolve + stats (see cmd/stats.go).
	pm := context.NewManagerWithOverride(workDir, r.PresetPath)
	files, err := pm.ResolveFilesFromRules()
	if err != nil {
		r.Errors = append(r.Errors, fmt.Sprintf("failed to resolve preset: %v", err))
		return r
	}
	stats, err := pm.GetStats("hot", files, 0)
	if err != nil {
		r.Errors = append(r.Errors, fmt.Sprintf("failed to compute stats: %v", err))
		return r
	}
	r.MeasuredTokens = stats.TotalTokens
	r.MeasuredFiles = len(files)

	// Preset lint findings (reuses pkg/context/lint.go, incl. the
	// zero-match-pattern warning) + dead @a: alias lines.
	if issues, lintErr := pm.LintRulesFile(r.PresetPath); lintErr == nil {
		for _, issue := range issues {
			r.LintIssues = append(r.LintIssues, conceptFinding{
				LineNum:  issue.LineNum,
				Rule:     issue.Line,
				Severity: issue.Severity,
				Message:  issue.Message,
			})
		}
	} else {
		r.Errors = append(r.Errors, fmt.Sprintf("lint failed: %v", lintErr))
	}
	r.DeadAliases = detectDeadAliases(pm, presetContent)

	if r.StampFound {
		r.TokenDriftPct = math.Round(driftPct(r.StampedTokens, r.MeasuredTokens)*10) / 10
		r.DriftExceeded = r.TokenDriftPct > tolerance
		r.FileMismatch = r.StampedFiles != r.MeasuredFiles
	}

	if fix && r.StampFound && (r.DriftExceeded || r.FileMismatch) {
		if err := rewriteStampFile(r.OverviewPath, r.MeasuredTokens, r.MeasuredFiles); err != nil {
			r.Errors = append(r.Errors, fmt.Sprintf("failed to rewrite stamp: %v", err))
		} else {
			r.Fixed = true
		}
	}

	r.OK = !conceptFails(r)
	return r
}

func printConceptReport(r conceptReport) {
	fmt.Printf("concept: %s\n", r.Concept)
	fmt.Printf("  preset:   %s\n", r.PresetPath)
	if r.StampFound {
		fmt.Printf("  stamped:  %s tokens, %d files\n", humanTokens(r.StampedTokens), r.StampedFiles)
		fmt.Printf("  measured: %s tokens, %d files (token drift %.1f%%)\n", humanTokens(r.MeasuredTokens), r.MeasuredFiles, r.TokenDriftPct)
	} else {
		fmt.Printf("  stamped:  (no context stamp found in overview.md)\n")
		fmt.Printf("  measured: %s tokens, %d files\n", humanTokens(r.MeasuredTokens), r.MeasuredFiles)
	}
	for _, f := range r.DeadAliases {
		fmt.Printf("  [%s] Line %d: %s\n", f.Severity, f.LineNum, f.Message)
	}
	for _, f := range r.LintIssues {
		fmt.Printf("  [%s] Line %d: %s\n", f.Severity, f.LineNum, f.Message)
	}
	for _, e := range r.Errors {
		fmt.Printf("  [Error] %s\n", e)
	}
	status := "OK"
	switch {
	case r.Fixed && !conceptFails(r):
		status = "FIXED (stamp updated)"
	case conceptFails(r):
		status = "DRIFT"
		if len(r.DeadAliases) > 0 || len(r.Errors) > 0 {
			status = "FAIL"
		}
	}
	fmt.Printf("  status:   %s\n\n", status)
}

func humanTokens(n int) string {
	if n >= 1000 {
		return fmt.Sprintf("~%dk", int(math.Round(float64(n)/1000)))
	}
	return fmt.Sprintf("~%d", n)
}

func newConceptVerifyCmd() *cobra.Command {
	var (
		all       bool
		fix       bool
		jsonOut   bool
		tolerance float64
	)

	cmd := &cobra.Command{
		Use:   "verify [concept-id]",
		Short: "Verify concept context stamps against their presets",
		Long: `Measures each concept's cx context preset (the same way 'cx stats' would)
and compares the result against the '## Context' stamp in the concept's
overview.md — the '(~Nk tokens, M files)' line.

Reports token drift, file-count mismatches, dead @a: alias lines, and
zero-match patterns in the preset. Exits nonzero when any concept exceeds
the tolerance or has dead/zero-match lines (unless --fix repaired the drift).`,
		Example: `  # Verify one concept of the current workspace
  cx concept verify rules-dsl

  # Verify all concepts, machine-readable
  cx concept verify --all --json

  # Re-stamp drifted overview.md files with measured numbers
  cx concept verify --all --fix`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if !all && len(args) == 0 {
				return fmt.Errorf("provide a concept id or use --all")
			}
			if all && len(args) > 0 {
				return fmt.Errorf("--all cannot be combined with a concept id")
			}

			mgr := context.NewManager(GetWorkDir())
			workDir := mgr.GetWorkDir()

			node, err := workspace.GetProjectByPath(workDir)
			if err != nil {
				return fmt.Errorf("cannot determine current workspace for %s: %w", workDir, err)
			}
			presetsDir, err := mgr.Locator().GetContextPresetsDir(node)
			if err != nil {
				return fmt.Errorf("cannot locate context presets dir: %w", err)
			}
			conceptsDir, err := mgr.Locator().GetNotesDir(node, "concepts")
			if err != nil {
				return fmt.Errorf("cannot locate concepts dir: %w", err)
			}

			var ids []string
			if all {
				ids, err = listConceptIDs(conceptsDir)
				if err != nil {
					return err
				}
				if len(ids) == 0 {
					fmt.Printf("No concepts found in %s\n", conceptsDir)
					return nil
				}
			} else {
				ids = []string{args[0]}
			}

			workspaceName := node.Name
			reports := make([]conceptReport, 0, len(ids))
			for _, id := range ids {
				reports = append(reports, verifyConcept(workDir, presetsDir, conceptsDir, id, workspaceName, tolerance, fix))
			}

			failed := 0
			for _, r := range reports {
				if conceptFails(r) {
					failed++
				}
			}

			if jsonOut {
				data, err := json.MarshalIndent(reports, "", "  ")
				if err != nil {
					return fmt.Errorf("failed to marshal report: %w", err)
				}
				fmt.Println(string(data))
			} else {
				for _, r := range reports {
					printConceptReport(r)
				}
				fmt.Printf("%d concept(s) verified, %d failing\n", len(reports), failed)
			}

			if failed > 0 {
				os.Exit(1)
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&all, "all", false, "Verify all concepts of the current workspace")
	cmd.Flags().BoolVar(&fix, "fix", false, "Rewrite drifted overview.md stamps with measured numbers")
	cmd.Flags().BoolVar(&jsonOut, "json", false, "Output a machine-readable JSON report")
	cmd.Flags().Float64Var(&tolerance, "tolerance", 10, "Token drift tolerance in percent (file-count mismatches always count as drift)")

	return cmd
}

// NewConceptCmd returns the `cx concept` command family.
func NewConceptCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "concept",
		Short: "Work with concept context presets",
		Long:  `Tools for keeping concept docs and their cx context presets aligned.`,
	}
	cmd.AddCommand(newConceptVerifyCmd())
	return cmd
}
