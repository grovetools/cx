package main

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/mattsolo1/grove-tend/pkg/command"
	"github.com/mattsolo1/grove-tend/pkg/fs"
	"github.com/mattsolo1/grove-tend/pkg/harness"
)

// GitAliasBasicScenario tests basic Git alias syntax without version
func GitAliasBasicScenario() *harness.Scenario {
	return &harness.Scenario{
		Name:        "cx-git-alias-basic",
		Description: "Tests basic @a:git:owner/repo alias syntax without version",
		Tags:        []string{"cx", "git", "alias"},
		Steps: []harness.Step{
			harness.NewStep("Setup test project", func(ctx *harness.Context) error {
				mainGo := `package main

import "fmt"

func main() {
    fmt.Println("Test project")
}`
				if err := fs.WriteString(filepath.Join(ctx.RootDir, "main.go"), mainGo); err != nil {
					return fmt.Errorf("failed to create main.go: %w", err)
				}
				return nil
			}),

			harness.NewStep("Create rules file with Git alias", func(ctx *harness.Context) error {
				rulesContent := `# Local files
*.go

# Include lipgloss using Git alias
@a:git:charmbracelet/lipgloss

# Exclude tests
!**/*_test.go`

				rulesPath := filepath.Join(ctx.RootDir, ".grove", "rules")
				if err := fs.WriteString(rulesPath, rulesContent); err != nil {
					return fmt.Errorf("failed to create rules file: %w", err)
				}
				return nil
			}),

			harness.NewStep("Run 'cx generate' with Git alias", func(ctx *harness.Context) error {
				cxBinary, err := FindProjectBinary()
				if err != nil {
					return err
				}

				cmd := command.New(cxBinary, "generate").Dir(ctx.RootDir)
				result := cmd.Run()

				if result.ExitCode != 0 {
					return fmt.Errorf("cx generate failed with exit code %d: %s\nStderr: %s",
						result.ExitCode, result.Stdout, result.Stderr)
				}

				if !strings.Contains(result.Stdout, "Generated .grove/context with") {
					return fmt.Errorf("expected 'Generated .grove/context with' in output, got: %s", result.Stdout)
				}

				return nil
			}),

			harness.NewStep("Verify repository was cloned", func(ctx *harness.Context) error {
				cxBinary, err := FindProjectBinary()
				if err != nil {
					return err
				}

				cmd := command.New(cxBinary, "repo", "list").Dir(ctx.RootDir)
				result := cmd.Run()

				if result.ExitCode != 0 {
					return fmt.Errorf("cx repo list failed: %s\nStderr: %s", result.Stdout, result.Stderr)
				}

				if !strings.Contains(result.Stdout, "github.com/charmbracelet/lipgloss") {
					return fmt.Errorf("expected lipgloss in repo list, got: %s", result.Stdout)
				}

				return nil
			}),

			harness.NewStep("Verify lipgloss files are included", func(ctx *harness.Context) error {
				cxBinary, err := FindProjectBinary()
				if err != nil {
					return err
				}

				cmd := command.New(cxBinary, "list").Dir(ctx.RootDir)
				result := cmd.Run()

				if result.ExitCode != 0 {
					return fmt.Errorf("cx list failed: %s\nStderr: %s", result.Stdout, result.Stderr)
				}

				if !strings.Contains(result.Stdout, "lipgloss") {
					return fmt.Errorf("expected lipgloss files in context, got: %s", result.Stdout)
				}

				// Count lipgloss Go files
				lipglossGoFiles := 0
				for _, line := range strings.Split(result.Stdout, "\n") {
					if strings.Contains(line, "lipgloss") && strings.HasSuffix(line, ".go") && !strings.Contains(line, "_test.go") {
						lipglossGoFiles++
					}
				}

				if lipglossGoFiles < 10 {
					return fmt.Errorf("expected at least 10 lipgloss .go files, found %d", lipglossGoFiles)
				}

				return nil
			}),
		},
	}
}

// GitAliasWithVersionScenario tests Git alias syntax with version
func GitAliasWithVersionScenario() *harness.Scenario {
	return &harness.Scenario{
		Name:        "cx-git-alias-version",
		Description: "Tests @a:git:owner/repo@version alias syntax",
		Tags:        []string{"cx", "git", "alias", "version"},
		Steps: []harness.Step{
			harness.NewStep("Setup test project", func(ctx *harness.Context) error {
				mainGo := `package main

func main() {}`
				return fs.WriteString(filepath.Join(ctx.RootDir, "main.go"), mainGo)
			}),

			harness.NewStep("Create rules file with versioned Git alias", func(ctx *harness.Context) error {
				rulesContent := `*.go
@a:git:charmbracelet/lipgloss@v0.13.0
!**/*_test.go`

				rulesPath := filepath.Join(ctx.RootDir, ".grove", "rules")
				return fs.WriteString(rulesPath, rulesContent)
			}),

			harness.NewStep("Run 'cx generate'", func(ctx *harness.Context) error {
				cxBinary, err := FindProjectBinary()
				if err != nil {
					return err
				}

				result := command.New(cxBinary, "generate").Dir(ctx.RootDir).Run()
				if result.ExitCode != 0 {
					return fmt.Errorf("cx generate failed: %s\nStderr: %s", result.Stdout, result.Stderr)
				}
				return nil
			}),

			harness.NewStep("Verify version in repo list", func(ctx *harness.Context) error {
				cxBinary, err := FindProjectBinary()
				if err != nil {
					return err
				}

				result := command.New(cxBinary, "repo", "list").Dir(ctx.RootDir).Run()
				if result.ExitCode != 0 {
					return fmt.Errorf("cx repo list failed: %s", result.Stderr)
				}

				if !strings.Contains(result.Stdout, "v0.13.0") {
					return fmt.Errorf("expected version v0.13.0 in repo list, got: %s", result.Stdout)
				}

				return nil
			}),
		},
	}
}

// GitAliasWithGlobPatternsScenario tests Git alias with glob patterns
func GitAliasWithGlobPatternsScenario() *harness.Scenario {
	return &harness.Scenario{
		Name:        "cx-git-alias-glob-patterns",
		Description: "Tests @a:git:owner/repo with glob patterns like /**/*.yml",
		Tags:        []string{"cx", "git", "alias", "glob"},
		Steps: []harness.Step{
			harness.NewStep("Setup test project", func(ctx *harness.Context) error {
				return fs.WriteString(filepath.Join(ctx.RootDir, "main.go"), "package main")
			}),

			harness.NewStep("Create rules file with Git alias and glob patterns", func(ctx *harness.Context) error {
				// Test multiple variations of glob patterns with Git aliases
				rulesContent := `# Local files
*.go

# Git alias without version but with glob pattern
@a:git:charmbracelet/lipgloss/**/*.go

# Git alias with version and glob pattern
@a:git:charmbracelet/bubbletea@v1.3.9/**/*.go

# Exclude tests
!**/*_test.go`

				rulesPath := filepath.Join(ctx.RootDir, ".grove", "rules")
				return fs.WriteString(rulesPath, rulesContent)
			}),

			harness.NewStep("Run 'cx generate' with glob patterns", func(ctx *harness.Context) error {
				cxBinary, err := FindProjectBinary()
				if err != nil {
					return err
				}

				result := command.New(cxBinary, "generate").Dir(ctx.RootDir).Run()
				if result.ExitCode != 0 {
					return fmt.Errorf("cx generate failed: %s\nStderr: %s", result.Stdout, result.Stderr)
				}
				return nil
			}),

			harness.NewStep("Verify both repositories were cloned", func(ctx *harness.Context) error {
				cxBinary, err := FindProjectBinary()
				if err != nil {
					return err
				}

				result := command.New(cxBinary, "repo", "list").Dir(ctx.RootDir).Run()
				if result.ExitCode != 0 {
					return fmt.Errorf("cx repo list failed: %s", result.Stderr)
				}

				if !strings.Contains(result.Stdout, "github.com/charmbracelet/lipgloss") {
					return fmt.Errorf("expected lipgloss in repo list, got: %s", result.Stdout)
				}

				if !strings.Contains(result.Stdout, "github.com/charmbracelet/bubbletea") {
					return fmt.Errorf("expected bubbletea in repo list, got: %s", result.Stdout)
				}

				if !strings.Contains(result.Stdout, "v1.3.9") {
					return fmt.Errorf("expected bubbletea version v1.3.9 in repo list, got: %s", result.Stdout)
				}

				return nil
			}),

			harness.NewStep("Verify glob patterns were applied correctly", func(ctx *harness.Context) error {
				cxBinary, err := FindProjectBinary()
				if err != nil {
					return err
				}

				result := command.New(cxBinary, "list").Dir(ctx.RootDir).Run()
				if result.ExitCode != 0 {
					return fmt.Errorf("cx list failed: %s", result.Stderr)
				}

				// Should have Go files from both repos
				hasLipgloss := false
				hasBubbletea := false

				for _, line := range strings.Split(result.Stdout, "\n") {
					if strings.Contains(line, "lipgloss") && strings.HasSuffix(line, ".go") && !strings.Contains(line, "_test.go") {
						hasLipgloss = true
					}
					if strings.Contains(line, "bubbletea") && strings.HasSuffix(line, ".go") && !strings.Contains(line, "_test.go") {
						hasBubbletea = true
					}
				}

				if !hasLipgloss {
					return fmt.Errorf("expected lipgloss .go files in context")
				}

				if !hasBubbletea {
					return fmt.Errorf("expected bubbletea .go files in context")
				}

				return nil
			}),
		},
	}
}

// GitAliasStatsPerLineScenario tests cx stats --per-line with Git aliases
func GitAliasStatsPerLineScenario() *harness.Scenario {
	return &harness.Scenario{
		Name:        "cx-git-alias-stats-per-line",
		Description: "Tests that cx stats --per-line correctly exposes Git info for alias rules",
		Tags:        []string{"cx", "git", "alias", "stats"},
		Steps: []harness.Step{
			harness.NewStep("Setup test project", func(ctx *harness.Context) error {
				return fs.WriteString(filepath.Join(ctx.RootDir, "main.go"), "package main")
			}),

			harness.NewStep("Create rules file with Git aliases", func(ctx *harness.Context) error {
				rulesContent := `*.go
@a:git:charmbracelet/lipgloss@v0.13.0
!**/*_test.go`

				rulesPath := filepath.Join(ctx.RootDir, ".grove", "rules")
				return fs.WriteString(rulesPath, rulesContent)
			}),

			harness.NewStep("Run 'cx generate' to clone repository", func(ctx *harness.Context) error {
				cxBinary, err := FindProjectBinary()
				if err != nil {
					return err
				}

				result := command.New(cxBinary, "generate").Dir(ctx.RootDir).Run()
				if result.ExitCode != 0 {
					return fmt.Errorf("cx generate failed: %s", result.Stderr)
				}
				return nil
			}),

			harness.NewStep("Run 'cx stats --per-line' and verify Git info", func(ctx *harness.Context) error {
				cxBinary, err := FindProjectBinary()
				if err != nil {
					return err
				}

				rulesPath := filepath.Join(ctx.RootDir, ".grove", "rules")
				result := command.New(cxBinary, "stats", rulesPath, "--per-line").Dir(ctx.RootDir).Run()
				if result.ExitCode != 0 {
					return fmt.Errorf("cx stats --per-line failed: %s", result.Stderr)
				}

				// The git alias line should have files from lipgloss and gitInfo
				// Check that at least one line in the output has gitInfo
				hasGitInfo := strings.Contains(result.Stdout, "\"gitInfo\"")
				if !hasGitInfo {
					return fmt.Errorf("expected at least one line with gitInfo in stats output, got: %s", result.Stdout)
				}

				// Check for the Git URL - it should be the transformed full URL
				hasRepoURL := strings.Contains(result.Stdout, "github.com/charmbracelet/lipgloss")
				if !hasRepoURL {
					return fmt.Errorf("expected repo URL in gitInfo, got: %s", result.Stdout)
				}

				// Check for version
				hasVersion := strings.Contains(result.Stdout, "v0.13.0")
				if !hasVersion {
					return fmt.Errorf("expected version v0.13.0 in gitInfo, got: %s", result.Stdout)
				}

				// Check that the entry with gitInfo has lipgloss files
				hasLipglossFiles := strings.Contains(result.Stdout, "lipgloss") && strings.Contains(result.Stdout, ".go")
				if !hasLipglossFiles {
					return fmt.Errorf("expected lipgloss files in the line with gitInfo, got: %s", result.Stdout)
				}

				return nil
			}),
		},
	}
}
