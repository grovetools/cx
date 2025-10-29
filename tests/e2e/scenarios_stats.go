// File: grove-context/tests/e2e/scenarios_stats.go
package main

import (
	"encoding/json"
	"fmt"
	"path/filepath"

	"github.com/mattsolo1/grove-tend/pkg/command"
	"github.com/mattsolo1/grove-tend/pkg/fs"
	"github.com/mattsolo1/grove-tend/pkg/harness"
)

// StatsSupersededRuleScenario tests that `cx stats --per-line` provides a helpful message
// for rules that are superseded by later rules.
func StatsSupersededRuleScenario() *harness.Scenario {
	return &harness.Scenario{
		Name:        "cx-stats-superseded-rule",
		Description: "Tests that superseded rules show 'included by line X' in stats",
		Tags:        []string{"cx", "stats", "rules"},
		Steps: []harness.Step{
			harness.NewStep("Setup multi-project environment", func(ctx *harness.Context) error {
				// Create groves dir and config
				grovesDir := filepath.Join(ctx.RootDir, "mock-groves")
				groveConfigDir := filepath.Join(ctx.ConfigDir(), "grove")
				groveConfig := fmt.Sprintf(`groves:
  test:
    path: %s
    enabled: true
`, grovesDir)
				if err := fs.WriteString(filepath.Join(groveConfigDir, "grove.yml"), groveConfig); err != nil {
					return err
				}

				// Create my-lib project
				myLibDir := filepath.Join(grovesDir, "my-lib")
				if err := fs.WriteString(filepath.Join(myLibDir, "grove.yml"), `name: my-lib`); err != nil {
					return err
				}
				if err := fs.WriteString(filepath.Join(myLibDir, "specific-file.go"), "package mylib"); err != nil {
					return err
				}
				if err := fs.WriteString(filepath.Join(myLibDir, "another-file.go"), "package mylib"); err != nil {
					return err
				}
				if result := command.New("git", "init").Dir(myLibDir).Run(); result.Error != nil {
					return fmt.Errorf("failed to init git in my-lib: %w", result.Error)
				}

				// Create main project
				if err := fs.WriteString(filepath.Join(ctx.RootDir, "grove.yml"), `name: main-project`); err != nil {
					return err
				}

				return nil
			}),
			harness.NewStep("Create rules file with overlapping patterns", func(ctx *harness.Context) error {
				rules := `# Line 1
@a:my-lib/specific-file.go

# Line 4
@a:my-lib/**/*.go`
				return fs.WriteString(filepath.Join(ctx.RootDir, ".grove", "rules"), rules)
			}),
			harness.NewStep("Run 'cx stats --per-line' and verify superseded message", func(ctx *harness.Context) error {
				cx, err := FindProjectBinary()
				if err != nil {
					return err
				}
				cmd := ctx.Command(cx, "stats", "--per-line", ".grove/rules").Dir(ctx.RootDir)

				result := cmd.Run()
				ctx.ShowCommandOutput(cmd.String(), result.Stdout, result.Stderr)
				if result.Error != nil {
					return result.Error
				}

				// Parse JSON output
				var stats []struct {
					LineNumber     int `json:"lineNumber"`
					FileCount      int `json:"fileCount"`
					FilteredByLine []struct {
						LineNumber int `json:"lineNumber"`
						Count      int `json:"count"`
					} `json:"filteredByLine"`
				}
				if err := json.Unmarshal([]byte(result.Stdout), &stats); err != nil {
					return fmt.Errorf("failed to parse JSON: %w", err)
				}

				// Find stats for lines 2 and 5
				var line2Stats, line5Stats *struct {
					LineNumber     int `json:"lineNumber"`
					FileCount      int `json:"fileCount"`
					FilteredByLine []struct {
						LineNumber int `json:"lineNumber"`
						Count      int `json:"count"`
					} `json:"filteredByLine"`
				}

				for i := range stats {
					if stats[i].LineNumber == 2 {
						line2Stats = &stats[i]
					}
					if stats[i].LineNumber == 5 {
						line5Stats = &stats[i]
					}
				}

				if line2Stats == nil {
					return fmt.Errorf("stats for line 2 not found")
				}
				if line5Stats == nil {
					return fmt.Errorf("stats for line 5 not found")
				}

				// Assertions
				if line2Stats.FileCount != 0 {
					return fmt.Errorf("expected line 2 to have fileCount 0, got %d", line2Stats.FileCount)
				}
				if len(line2Stats.FilteredByLine) != 1 {
					return fmt.Errorf("expected line 2 to have one filteredByLine entry, got %d", len(line2Stats.FilteredByLine))
				}
				if line2Stats.FilteredByLine[0].LineNumber != 5 {
					return fmt.Errorf("expected line 2 to be filtered by line 5, but was %d", line2Stats.FilteredByLine[0].LineNumber)
				}
				if line2Stats.FilteredByLine[0].Count != 1 {
					return fmt.Errorf("expected filtered count to be 1, got %d", line2Stats.FilteredByLine[0].Count)
				}

				if line5Stats.FileCount != 2 {
					return fmt.Errorf("expected line 5 to have fileCount 2, got %d", line5Stats.FileCount)
				}

				return nil
			}),
		},
	}
}
