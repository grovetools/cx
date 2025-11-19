// File: grove-context/tests/e2e/scenarios_git_repo_rules.go
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mattsolo1/grove-tend/pkg/command"
	"github.com/mattsolo1/grove-tend/pkg/fs"
	"github.com/mattsolo1/grove-tend/pkg/harness"
)

// GitRepoRulesScenario tests importing rulesets from external Git repositories.
func GitRepoRulesScenario() *harness.Scenario {
	return &harness.Scenario{
		Name:        "cx-git-repo-rules",
		Description: "Tests importing rules files from external Git repositories using @a:git:owner/repo::ruleset syntax",
		Tags:        []string{"cx", "git", "repo", "rules", "ruleset"},
		Steps: []harness.Step{
			harness.NewStep("Setup mock Git repository with ruleset", func(ctx *harness.Context) error {
				// Create a mock "remote" repository that will be cloned
				// We'll put it in a temp location and simulate it as if it were cloned
				mockRepoPath := filepath.Join(ctx.RootDir, "mock-remote-repos", "github.com", "test-org", "test-repo")

				// Create some files in the mock repo
				if err := fs.WriteString(filepath.Join(mockRepoPath, "README.md"), "# Test Repo\n"); err != nil {
					return err
				}
				if err := fs.WriteString(filepath.Join(mockRepoPath, "src", "main.go"), "package main\n"); err != nil {
					return err
				}
				if err := fs.WriteString(filepath.Join(mockRepoPath, "src", "utils.go"), "package main\n\nfunc Util() {}"); err != nil {
					return err
				}
				if err := fs.WriteString(filepath.Join(mockRepoPath, "tests", "main_test.go"), "package main\n\nimport \"testing\""); err != nil {
					return err
				}
				if err := fs.WriteString(filepath.Join(mockRepoPath, "docs", "guide.md"), "# Guide\n"); err != nil {
					return err
				}

				// Create a .cx/default.rules file in the mock repo
				defaultRules := `# Default rules for test-repo
src/**/*.go
!tests/**
`
				if err := fs.WriteString(filepath.Join(mockRepoPath, ".cx", "default.rules"), defaultRules); err != nil {
					return err
				}

				// Create a .cx/docs.rules file in the mock repo
				docsRules := `# Documentation rules
docs/**/*.md
README.md
`
				if err := fs.WriteString(filepath.Join(mockRepoPath, ".cx", "docs.rules"), docsRules); err != nil {
					return err
				}

				// Initialize git in the mock repo
				if result := command.New("git", "init").Dir(mockRepoPath).Run(); result.Error != nil {
					return fmt.Errorf("failed to init git in mock repo: %w", result.Error)
				}
				if result := command.New("git", "add", ".").Dir(mockRepoPath).Run(); result.Error != nil {
					return fmt.Errorf("failed to git add in mock repo: %w", result.Error)
				}
				if result := command.New("git", "commit", "-m", "Initial commit").Dir(mockRepoPath).Run(); result.Error != nil {
					return fmt.Errorf("failed to git commit in mock repo: %w", result.Error)
				}

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

				// Create a .grove/rules file that imports from the git repo
				rulesContent := `# Main project rules
*.go

# Import default ruleset from test-repo
@a:git:test-org/test-repo::default
`
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

			harness.NewStep("Mock repository clone by symlinking", func(ctx *harness.Context) error {
				// Create the .grove/cx/repos directory structure that grove would use
				// and symlink our mock repo there to simulate a clone
				mockRepoPath := filepath.Join(ctx.RootDir, "mock-remote-repos", "github.com", "test-org", "test-repo")

				homeDir, err := os.UserHomeDir()
				if err != nil {
					return fmt.Errorf("failed to get home directory: %w", err)
				}
				cxReposDir := filepath.Join(homeDir, ".grove", "cx", "repos")

				// Create the target directory
				targetPath := filepath.Join(cxReposDir, "github.com_test-org_test-repo_000000")

				// Symlink the mock repo to the cx repos directory
				if result := command.New("ln", "-s", mockRepoPath, targetPath).Run(); result.Error != nil {
					return fmt.Errorf("failed to symlink mock repo: %w", result.Error)
				}

				// Create a manifest file to register the repository
				manifestContent := fmt.Sprintf(`repositories:
  https://github.com/test-org/test-repo:
    pinned_version: ""
    resolved_commit: "000000"
    last_synced_at: "2024-01-01T00:00:00Z"
    audit:
      status: passed
      report_path: ""
`)
				manifestPath := filepath.Join(homeDir, ".grove", "cx", "manifest.yml")
				if err := fs.WriteString(manifestPath, manifestContent); err != nil {
					return err
				}

				return nil
			}),

			harness.NewStep("Verify default ruleset import includes correct files", func(ctx *harness.Context) error {
				projectDir := filepath.Join(ctx.RootDir, "my-project")
				result := command.New("grove", "cx", "list").Dir(projectDir).Run()

				if result.Error != nil {
					return fmt.Errorf("cx list failed: %w", result.Error)
				}

				output := result.Stdout

				// Should include local main.go
				if !strings.Contains(output, "main.go") {
					return fmt.Errorf("expected main.go to be included but it wasn't")
				}

				// Should include src/main.go and src/utils.go from the imported repo
				if !strings.Contains(output, "src/main.go") {
					return fmt.Errorf("expected src/main.go from imported repo to be included but it wasn't")
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

				// Update rules file to import docs ruleset instead
				rulesContent := `# Main project rules
*.go

# Import docs ruleset from test-repo
@a:git:test-org/test-repo::docs
`
				if err := fs.WriteString(filepath.Join(projectDir, ".grove", "rules"), rulesContent); err != nil {
					return err
				}

				result := command.New("grove", "cx", "list").Dir(projectDir).Run()

				if result.Error != nil {
					return fmt.Errorf("cx list failed: %w", result.Error)
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

			harness.NewStep("Test direct GitHub URL with ruleset", func(ctx *harness.Context) error {
				projectDir := filepath.Join(ctx.RootDir, "my-project")

				// Update rules file to use full GitHub URL syntax
				rulesContent := `# Main project rules
*.go

# Import using full URL
https://github.com/test-org/test-repo::default
`
				if err := fs.WriteString(filepath.Join(projectDir, ".grove", "rules"), rulesContent); err != nil {
					return err
				}

				result := command.New("grove", "cx", "list").Dir(projectDir).Run()

				if result.Error != nil {
					return fmt.Errorf("cx list failed: %w", result.Error)
				}

				output := result.Stdout

				// Should include src files from the imported repo
				if !strings.Contains(output, "src/main.go") {
					return fmt.Errorf("expected src/main.go from imported repo to be included but it wasn't")
				}

				return nil
			}),
		},
	}
}
