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

// StatsDirectiveSupersededScenario tests that a rule with a directive is not incorrectly marked
// as being "superseded" by another rule if its directive would have filtered out the file anyway.
func StatsDirectiveSupersededScenario() *harness.Scenario {
	return &harness.Scenario{
		Name:        "cx-stats-directive-superseded",
		Description: "Tests that directives are respected when calculating superseded rules in stats",
		Tags:        []string{"cx", "stats", "rules", "directives", "regression"},
		Steps: []harness.Step{
			harness.NewStep("Setup project with specific files", func(ctx *harness.Context) error {
				if err := fs.WriteString(filepath.Join(ctx.RootDir, "api", "user_api.go"), "package api"); err != nil {
					return err
				}
				if err := fs.WriteString(filepath.Join(ctx.RootDir, "api", "product_manager.go"), "package api"); err != nil {
					return err
				}
				return nil
			}),
			harness.NewStep("Create rules file with overlapping patterns and a directive", func(ctx *harness.Context) error {
				rules := `# Line 1
api/*.go @find: "manager"

# Line 4
api/user_api.go`
				return fs.WriteString(filepath.Join(ctx.RootDir, ".grove", "rules"), rules)
			}),
			harness.NewStep("Run 'cx stats --per-line' and verify directive is respected", func(ctx *harness.Context) error {
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
					LineNumber     int           `json:"lineNumber"`
					FileCount      int           `json:"fileCount"`
					FilteredByLine []interface{} `json:"filteredByLine"`
				}
				if err := json.Unmarshal([]byte(result.Stdout), &stats); err != nil {
					return fmt.Errorf("failed to parse JSON: %w", err)
				}

				var line2Stats, line5Stats *struct {
					LineNumber     int           `json:"lineNumber"`
					FileCount      int           `json:"fileCount"`
					FilteredByLine []interface{} `json:"filteredByLine"`
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

				// Assertions for Line 2 (api/*.go @find: "manager")
				if line2Stats.FileCount != 1 {
					return fmt.Errorf("expected line 2 to have fileCount 1 (for product_manager.go), got %d", line2Stats.FileCount)
				}
				// THE CORE OF THE TEST: filteredByLine should be empty.
				// user_api.go should NOT be considered a match for this line at all.
				if line2Stats.FilteredByLine != nil && len(line2Stats.FilteredByLine) > 0 {
					return fmt.Errorf("expected line 2 to have an empty filteredByLine array, but it was not. Bug is still present.")
				}

				// Assertions for Line 5 (api/user_api.go)
				if line5Stats.FileCount != 1 {
					return fmt.Errorf("expected line 5 to have fileCount 1 (for user_api.go), got %d", line5Stats.FileCount)
				}

				return nil
			}),
		},
	}
}

// StatsPrefixAttributionScenario tests that prefix matching in ** patterns respects directory boundaries.
// A pattern for "my-repo/**" should not match files in "my-repo-hihi/".
func StatsPrefixAttributionScenario() *harness.Scenario {
	return &harness.Scenario{
		Name:        "cx-stats-prefix-attribution",
		Description: "Tests that ** pattern prefix matching respects directory boundaries",
		Tags:        []string{"cx", "stats", "patterns", "regression"},
		Steps: []harness.Step{
			harness.NewStep("Setup directories with overlapping names", func(ctx *harness.Context) error {
				if err := fs.WriteString(filepath.Join(ctx.RootDir, "my-repo", "file1.txt"), "test content 1"); err != nil {
					return err
				}
				if err := fs.WriteString(filepath.Join(ctx.RootDir, "my-repo-hihi", "file2.txt"), "test content 2"); err != nil {
					return err
				}
				return nil
			}),
			harness.NewStep("Create rules file with patterns for both directories", func(ctx *harness.Context) error {
				rules := `my-repo/**
my-repo-hihi/**`
				return fs.WriteString(filepath.Join(ctx.RootDir, ".grove", "rules"), rules)
			}),
			harness.NewStep("Run 'cx stats --per-line' and verify correct attribution", func(ctx *harness.Context) error {
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
					LineNumber     int           `json:"lineNumber"`
					FileCount      int           `json:"fileCount"`
					FilteredByLine []interface{} `json:"filteredByLine"`
				}
				if err := json.Unmarshal([]byte(result.Stdout), &stats); err != nil {
					return fmt.Errorf("failed to parse JSON: %w", err)
				}

				var line1Stats, line2Stats *struct {
					LineNumber     int           `json:"lineNumber"`
					FileCount      int           `json:"fileCount"`
					FilteredByLine []interface{} `json:"filteredByLine"`
				}

				for i := range stats {
					if stats[i].LineNumber == 1 {
						line1Stats = &stats[i]
					}
					if stats[i].LineNumber == 2 {
						line2Stats = &stats[i]
					}
				}

				if line1Stats == nil {
					return fmt.Errorf("stats for line 1 not found")
				}
				if line2Stats == nil {
					return fmt.Errorf("stats for line 2 not found")
				}

				// Assertions for Line 1 (my-repo/**)
				if line1Stats.FileCount != 1 {
					return fmt.Errorf("expected line 1 to have fileCount 1 (my-repo/file1.txt), got %d", line1Stats.FileCount)
				}
				// Should not have filtered files
				if line1Stats.FilteredByLine != nil && len(line1Stats.FilteredByLine) > 0 {
					return fmt.Errorf("expected line 1 to have no filtered files, but found %v", line1Stats.FilteredByLine)
				}

				// Assertions for Line 2 (my-repo-hihi/**)
				if line2Stats.FileCount != 1 {
					return fmt.Errorf("expected line 2 to have fileCount 1 (my-repo-hihi/file2.txt), got %d", line2Stats.FileCount)
				}

				return nil
			}),
		},
	}
}
