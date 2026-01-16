// File: grove-context/tests/e2e/scenarios_git_repo_rules.go
package main

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/grovetools/tend/pkg/command"
	"github.com/grovetools/tend/pkg/fs"
	"github.com/grovetools/tend/pkg/harness"
)

// GitRepoRulesScenario tests importing rulesets from external Git repositories.
func GitRepoRulesScenario() *harness.Scenario {
	return &harness.Scenario{
		Name:        "cx-git-repo-rules",
		Description: "Tests importing rules files from external Git repositories using @a:git:owner/repo::ruleset syntax",
		Tags:        []string{"cx", "git", "repo", "rules", "ruleset"},
		Steps: []harness.Step{
			harness.NewStep("Setup local Git repository with rulesets", func(ctx *harness.Context) error {
				// Create a source repository with rulesets, then create a bare clone
				// This simulates a "remote" repository that can be cloned via file://
				sourceRepoPath := filepath.Join(ctx.RootDir, "source-repo")

				// Create some files in the source repo
				if err := fs.WriteString(filepath.Join(sourceRepoPath, "README.md"), "# Test Repo\n"); err != nil {
					return err
				}
				if err := fs.WriteString(filepath.Join(sourceRepoPath, "src", "main.go"), "package main\n"); err != nil {
					return err
				}
				if err := fs.WriteString(filepath.Join(sourceRepoPath, "src", "utils.go"), "package main\n\nfunc Util() {}"); err != nil {
					return err
				}
				if err := fs.WriteString(filepath.Join(sourceRepoPath, "tests", "main_test.go"), "package main\n\nimport \"testing\""); err != nil {
					return err
				}
				if err := fs.WriteString(filepath.Join(sourceRepoPath, "docs", "guide.md"), "# Guide\n"); err != nil {
					return err
				}

				// Create .cx/default.rules file
				defaultRules := `# Default rules for test-repo
src/**/*.go
!tests/**
`
				if err := fs.WriteString(filepath.Join(sourceRepoPath, ".cx", "default.rules"), defaultRules); err != nil {
					return err
				}

				// Create .cx/docs.rules file
				docsRules := `# Documentation rules
docs/**/*.md
README.md
`
				if err := fs.WriteString(filepath.Join(sourceRepoPath, ".cx", "docs.rules"), docsRules); err != nil {
					return err
				}

				// Initialize git and commit
				if result := command.New("git", "init").Dir(sourceRepoPath).Run(); result.Error != nil {
					return fmt.Errorf("failed to init git in source repo: %w", result.Error)
				}
				if result := command.New("git", "add", ".").Dir(sourceRepoPath).Run(); result.Error != nil {
					return fmt.Errorf("failed to git add in source repo: %w", result.Error)
				}
				if result := command.New("git", "commit", "-m", "Initial commit").Dir(sourceRepoPath).Run(); result.Error != nil {
					return fmt.Errorf("failed to git commit in source repo: %w", result.Error)
				}

				// Use the source repository directly as the "remote" for file:// URL
				// No need to create a bare clone - the source repo itself can be cloned
				repoPath := sourceRepoPath

				// Store the repo path in context for use in later steps
				ctx.Set("bareRepoPath", repoPath)

				return nil
			}),

			harness.NewStep("Setup main project with git repo ruleset import", func(ctx *harness.Context) error {
				// Create main project
				projectDir := filepath.Join(ctx.RootDir, "my-project")

				// Create grove.yml for the project
				groveConfig := `name: my-project`
				if err := fs.WriteString(filepath.Join(projectDir, "grove.yml"), groveConfig); err != nil {
					return err
				}

				// Get the bare repo path from context
				bareRepoPath := ctx.Get("bareRepoPath").(string)

				// Create a .grove/rules file that imports from the local git repo using file:// URL
				rulesContent := fmt.Sprintf(`# Main project rules
*.go

# Import default ruleset from local test-repo
@a:git:file://%s@main::default
`, bareRepoPath)
				if err := fs.WriteString(filepath.Join(projectDir, ".grove", "rules"), rulesContent); err != nil {
					return err
				}

				// Create a local file to test that both local and imported rules work
				if err := fs.WriteString(filepath.Join(projectDir, "main.go"), "package main\n\nfunc main() {}"); err != nil {
					return err
				}

				// Initialize git
				if result := command.New("git", "init").Dir(projectDir).Run(); result.Error != nil {
					return fmt.Errorf("failed to init git in project: %w", result.Error)
				}

				return nil
			}),

			harness.NewStep("Run 'cx generate' to process rules", func(ctx *harness.Context) error {
				projectDir := filepath.Join(ctx.RootDir, "my-project")
				cxBinary, err := FindProjectBinary()
				if err != nil {
					return err
				}

				result := command.New(cxBinary, "generate").Dir(projectDir).Run()

				if result.ExitCode != 0 {
					return fmt.Errorf("cx generate failed: %s\nStderr: %s", result.Stdout, result.Stderr)
				}
				return nil
			}),

			harness.NewStep("Verify default ruleset import includes correct files", func(ctx *harness.Context) error {
				projectDir := filepath.Join(ctx.RootDir, "my-project")
				cxBinary, err := FindProjectBinary()
				if err != nil {
					return err
				}

				result := command.New(cxBinary, "list").Dir(projectDir).Run()

				if result.ExitCode != 0 {
					return fmt.Errorf("cx list failed: %s\nStderr: %s", result.Stdout, result.Stderr)
				}

				output := result.Stdout

				// Should include local main.go
				if !strings.Contains(output, "main.go") {
					return fmt.Errorf("expected main.go to be included but it wasn't")
				}

				// Should include src/main.go and src/utils.go from the imported repo
				if !strings.Contains(output, "src/main.go") {
					return fmt.Errorf("expected src/main.go from imported repo to be included but it wasn't. Output:\n%s", output)
				}
				if !strings.Contains(output, "src/utils.go") {
					return fmt.Errorf("expected src/utils.go from imported repo to be included but it wasn't")
				}

				// Should NOT include tests/main_test.go (excluded by the default.rules)
				if strings.Contains(output, "tests/main_test.go") {
					return fmt.Errorf("expected tests/main_test.go to be excluded but it was included")
				}

				// Should NOT include docs (not in default.rules)
				if strings.Contains(output, "docs/guide.md") {
					return fmt.Errorf("expected docs/guide.md to be excluded but it was included")
				}

				return nil
			}),

			harness.NewStep("Test different ruleset import (docs)", func(ctx *harness.Context) error {
				projectDir := filepath.Join(ctx.RootDir, "my-project")

				// Get the bare repo path from context
				bareRepoPath := ctx.Get("bareRepoPath").(string)

				// Update rules file to import docs ruleset instead
				rulesContent := fmt.Sprintf(`# Main project rules
*.go

# Import docs ruleset from local test-repo
@a:git:file://%s@main::docs
`, bareRepoPath)
				if err := fs.WriteString(filepath.Join(projectDir, ".grove", "rules"), rulesContent); err != nil {
					return err
				}

				cxBinary, err := FindProjectBinary()
				if err != nil {
					return err
				}

				// Run cx generate to process the updated rules
				genResult := command.New(cxBinary, "generate").Dir(projectDir).Run()
				if genResult.ExitCode != 0 {
					return fmt.Errorf("cx generate failed: %s\nStderr: %s", genResult.Stdout, genResult.Stderr)
				}

				result := command.New(cxBinary, "list").Dir(projectDir).Run()

				if result.ExitCode != 0 {
					return fmt.Errorf("cx list failed: %s\nStderr: %s", result.Stdout, result.Stderr)
				}

				output := result.Stdout

				// Should include local main.go
				if !strings.Contains(output, "main.go") {
					return fmt.Errorf("expected main.go to be included but it wasn't")
				}

				// Should include README.md and docs/guide.md from the imported repo
				if !strings.Contains(output, "README.md") {
					return fmt.Errorf("expected README.md from imported repo to be included but it wasn't")
				}
				if !strings.Contains(output, "docs/guide.md") {
					return fmt.Errorf("expected docs/guide.md from imported repo to be included but it wasn't")
				}

				// Should NOT include src files (not in docs.rules)
				if strings.Contains(output, "src/main.go") {
					return fmt.Errorf("expected src/main.go to be excluded but it was included")
				}

				return nil
			}),
		},
	}
}
