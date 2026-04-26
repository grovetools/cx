package main

import (
	"encoding/json"
	"fmt"
	"path/filepath"

	"github.com/grovetools/tend/pkg/fs"
	"github.com/grovetools/tend/pkg/harness"
)

func StatsPerLineLintOverlayScenario() *harness.Scenario {
	return &harness.Scenario{
		Name:        "cx-stats-per-line-lint-overlay",
		Description: "@default and invalid-regex @grep lines get severity from lint overlay",
		Tags:        []string{"cx", "stats", "severity", "lint", "regression"},
		Steps: []harness.Step{
			harness.NewStep("Create project with @default and invalid-regex @grep rules", func(ctx *harness.Context) error {
				if err := fs.WriteString(filepath.Join(ctx.RootDir, "main.go"), "package main"); err != nil {
					return err
				}
				rules := "@default: /does/not/exist\n**/*.go @grep: \"[unclosed\""
				return fs.WriteString(filepath.Join(ctx.RootDir, ".grove", "rules"), rules)
			}),
			harness.NewStep("Run cx stats --per-line and verify both lines have severity", func(ctx *harness.Context) error {
				cx, err := FindProjectBinary()
				if err != nil {
					return err
				}
				cmd := ctx.Command(cx, "stats", "--per-line", ".grove/rules").Dir(ctx.RootDir)
				result := cmd.Run()
				ctx.ShowCommandOutput(cmd.String(), result.Stdout, result.Stderr)

				var stats []struct {
					LineNumber int    `json:"lineNumber"`
					Rule       string `json:"rule"`
					Severity   string `json:"severity"`
				}
				if err := json.Unmarshal([]byte(result.Stdout), &stats); err != nil {
					return fmt.Errorf("failed to parse JSON: %w\nOutput:\n%s", err, result.Stdout)
				}

				severityByLine := make(map[int]string)
				ruleByLine := make(map[int]string)
				for _, s := range stats {
					severityByLine[s.LineNumber] = s.Severity
					ruleByLine[s.LineNumber] = s.Rule
				}

				if _, ok := ruleByLine[1]; !ok {
					return fmt.Errorf("line 1 (@default): no record emitted; expected a record with severity=Warning")
				}
				if severityByLine[1] != "Warning" {
					return fmt.Errorf("line 1 (@default): expected severity=Warning, got %q", severityByLine[1])
				}

				if _, ok := ruleByLine[2]; !ok {
					return fmt.Errorf("line 2 (invalid-regex @grep): no record emitted")
				}
				if severityByLine[2] != "Warning" {
					return fmt.Errorf("line 2 (invalid-regex @grep): expected severity=Warning, got %q", severityByLine[2])
				}

				return nil
			}),
		},
	}
}
