// File: grove-context/tests/e2e/scenarios_directive_imports.go
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/grovetools/tend/pkg/command"
	"github.com/grovetools/tend/pkg/fs"
	"github.com/grovetools/tend/pkg/harness"
)

// DirectiveWorkspaceGrepImportScenario tests @grep with a workspace ruleset import
func DirectiveWorkspaceGrepImportScenario() *harness.Scenario {
	return &harness.Scenario{
		Name:        "cx-directive-workspace-grep-import",
		Description: "Tests @grep directive with workspace ruleset imports",
		Tags:        []string{"cx", "search-directives", "ruleset", "import"},
		Steps: []harness.Step{
			harness.NewStep("Setup workspace ecosystem with two projects", func(ctx *harness.Context) error {
				// Create groves directory for isolated test environment
				grovesDir := filepath.Join(ctx.RootDir, "mock-groves")
				groveConfigDir := filepath.Join(ctx.ConfigDir(), "grove")

				// Create global grove.yml to discover projects
				groveConfig := fmt.Sprintf(`groves:
  test:
    path: %s
    enabled: true
`, grovesDir)
				if err := fs.WriteString(filepath.Join(groveConfigDir, "grove.yml"), groveConfig); err != nil {
					return err
				}

				// Create project-a and project-b directories
				projectADir := filepath.Join(grovesDir, "project-a")
				projectBDir := filepath.Join(grovesDir, "project-b")

				if err := os.MkdirAll(projectADir, 0755); err != nil {
					return err
				}
				if err := os.MkdirAll(projectBDir, 0755); err != nil {
					return err
				}

				// Create grove.yml for each project
				if err := fs.WriteString(filepath.Join(projectADir, "grove.yml"), `name: project-a`); err != nil {
					return err
				}
				if err := fs.WriteString(filepath.Join(projectBDir, "grove.yml"), `name: project-b`); err != nil {
					return err
				}

				// Create project-a/a.txt
				if err := fs.WriteString(filepath.Join(projectADir, "a.txt"), "This is file A.\n"); err != nil {
					return err
				}

				// Create project-b files - b1.txt contains "magic-word", b2.txt doesn't
				if err := fs.WriteString(filepath.Join(projectBDir, "b1.txt"), "This file contains the magic-word.\n"); err != nil {
					return err
				}
				if err := fs.WriteString(filepath.Join(projectBDir, "b2.txt"), "This file does not contain the special keyword.\n"); err != nil {
					return err
				}

				// Create project-b .cx/imported.rules
				projectBCxDir := filepath.Join(projectBDir, ".cx")
				if err := os.MkdirAll(projectBCxDir, 0755); err != nil {
					return err
				}
				importedRules := "# Include all text files\n*.txt\n"
				if err := fs.WriteString(filepath.Join(projectBCxDir, "imported.rules"), importedRules); err != nil {
					return err
				}

				// Initialize git repos
				if result := command.New("git", "init").Dir(projectADir).Run(); result.Error != nil {
					return fmt.Errorf("failed to init git in project-a: %w", result.Error)
				}
				if result := command.New("git", "init").Dir(projectBDir).Run(); result.Error != nil {
					return fmt.Errorf("failed to init git in project-b: %w", result.Error)
				}

				return nil
			}),
			harness.NewStep("Create project-a rules with @grep directive on import", func(ctx *harness.Context) error {
				grovesDir := filepath.Join(ctx.RootDir, "mock-groves")
				projectADir := filepath.Join(grovesDir, "project-a")

				// This should include a.txt from project-a, and only b1.txt from project-b (filtered by grep)
				mainRules := `# This should include a.txt from project-a
*.txt

# This should import rules from project-b and only include files with "magic-word"
@a:project-b::imported @grep: "magic-word"
`
				return fs.WriteString(filepath.Join(projectADir, ".grove", "rules"), mainRules)
			}),
			harness.NewStep("Verify only files matching grep are included", func(ctx *harness.Context) error {
				cxBinary, err := FindProjectBinary()
				if err != nil {
					return err
				}

				grovesDir := filepath.Join(ctx.RootDir, "mock-groves")
				projectADir := filepath.Join(grovesDir, "project-a")
				cmd := ctx.Command(cxBinary, "list").Dir(projectADir)
				result := cmd.Run()

				ctx.ShowCommandOutput(cmd.String(), result.Stdout, result.Stderr)

				if result.Error != nil {
					return result.Error
				}

				output := result.Stdout

				// Should include a.txt from project-a
				if !strings.Contains(output, "a.txt") {
					return fmt.Errorf("output should contain a.txt, got:\n%s", output)
				}

				// Should include b1.txt from project-b (has "magic-word")
				if !strings.Contains(output, "b1.txt") {
					return fmt.Errorf("output should contain b1.txt, got:\n%s", output)
				}

				// Should NOT include b2.txt from project-b (no "magic-word")
				if strings.Contains(output, "b2.txt") {
					return fmt.Errorf("output should not contain b2.txt, got:\n%s", output)
				}

				return nil
			}),
		},
	}
}

// DirectiveWorkspaceFindImportScenario tests @find with a workspace ruleset import
func DirectiveWorkspaceFindImportScenario() *harness.Scenario {
	return &harness.Scenario{
		Name:        "cx-directive-workspace-find-import",
		Description: "Tests @find directive with workspace ruleset imports",
		Tags:        []string{"cx", "search-directives", "ruleset", "import"},
		Steps: []harness.Step{
			harness.NewStep("Setup workspace ecosystem", func(ctx *harness.Context) error {
				grovesDir := filepath.Join(ctx.RootDir, "mock-groves")
				groveConfigDir := filepath.Join(ctx.ConfigDir(), "grove")

				// Create global grove.yml
				groveConfig := fmt.Sprintf(`groves:
  test:
    path: %s
    enabled: true
`, grovesDir)
				if err := fs.WriteString(filepath.Join(groveConfigDir, "grove.yml"), groveConfig); err != nil {
					return err
				}

				projectADir := filepath.Join(grovesDir, "project-a")
				projectBDir := filepath.Join(grovesDir, "project-b")

				if err := os.MkdirAll(projectADir, 0755); err != nil {
					return err
				}
				if err := os.MkdirAll(projectBDir, 0755); err != nil {
					return err
				}

				// Create grove.yml for each project
				if err := fs.WriteString(filepath.Join(projectADir, "grove.yml"), `name: project-a`); err != nil {
					return err
				}
				if err := fs.WriteString(filepath.Join(projectBDir, "grove.yml"), `name: project-b`); err != nil {
					return err
				}

				// Create project-b files
				if err := fs.WriteString(filepath.Join(projectBDir, "b1.txt"), "File b1\n"); err != nil {
					return err
				}
				if err := fs.WriteString(filepath.Join(projectBDir, "b2.txt"), "File b2\n"); err != nil {
					return err
				}

				// Create project-b .cx/imported.rules
				projectBCxDir := filepath.Join(projectBDir, ".cx")
				if err := os.MkdirAll(projectBCxDir, 0755); err != nil {
					return err
				}
				importedRules := "*.txt\n"
				if err := fs.WriteString(filepath.Join(projectBCxDir, "imported.rules"), importedRules); err != nil {
					return err
				}

				// Initialize git repos
				if result := command.New("git", "init").Dir(projectADir).Run(); result.Error != nil {
					return fmt.Errorf("failed to init git in project-a: %w", result.Error)
				}
				if result := command.New("git", "init").Dir(projectBDir).Run(); result.Error != nil {
					return fmt.Errorf("failed to init git in project-b: %w", result.Error)
				}

				return nil
			}),
			harness.NewStep("Create rules with @find directive on import", func(ctx *harness.Context) error {
				grovesDir := filepath.Join(ctx.RootDir, "mock-groves")
				projectADir := filepath.Join(grovesDir, "project-a")

				// Only include files matching "b1.txt" by filename
				findRules := `# This should import rules from project-b and only include b1.txt by filename
@a:project-b::imported @find: "b1.txt"
`
				return fs.WriteString(filepath.Join(projectADir, ".grove", "rules"), findRules)
			}),
			harness.NewStep("Verify only b1.txt is included", func(ctx *harness.Context) error {
				cxBinary, err := FindProjectBinary()
				if err != nil {
					return err
				}

				grovesDir := filepath.Join(ctx.RootDir, "mock-groves")
				projectADir := filepath.Join(grovesDir, "project-a")
				cmd := ctx.Command(cxBinary, "list").Dir(projectADir)
				result := cmd.Run()

				ctx.ShowCommandOutput(cmd.String(), result.Stdout, result.Stderr)

				if result.Error != nil {
					return result.Error
				}

				output := result.Stdout

				// Should include b1.txt
				if !strings.Contains(output, "b1.txt") {
					return fmt.Errorf("output should contain b1.txt, got:\n%s", output)
				}

				// Should NOT include b2.txt
				if strings.Contains(output, "b2.txt") {
					return fmt.Errorf("output should not contain b2.txt, got:\n%s", output)
				}

				return nil
			}),
		},
	}
}

// DirectiveGitGrepImportScenario tests @grep with a Git ruleset import
func DirectiveGitGrepImportScenario() *harness.Scenario {
	return &harness.Scenario{
		Name:        "cx-directive-git-grep-import",
		Description: "Tests @grep directive with Git repository ruleset imports",
		Tags:        []string{"cx", "search-directives", "ruleset", "import", "git"},
		Steps: []harness.Step{
			harness.NewStep("Setup projects with local git repo", func(ctx *harness.Context) error {
				projectADir := filepath.Join(ctx.RootDir, "project-a")
				remoteRepoDir := filepath.Join(ctx.RootDir, "remote-repo")

				if err := os.MkdirAll(projectADir, 0755); err != nil {
					return err
				}
				if err := os.MkdirAll(remoteRepoDir, 0755); err != nil {
					return err
				}

				// Create remote-repo files
				if err := fs.WriteString(filepath.Join(remoteRepoDir, "r1.txt"), "This is a file from the remote repo. It has the REMOTE-WORD.\n"); err != nil {
					return err
				}
				if err := fs.WriteString(filepath.Join(remoteRepoDir, "r2.txt"), "This is another file from the remote repo.\n"); err != nil {
					return err
				}

				// Create remote-repo .cx/remote.rules
				remoteRepoCxDir := filepath.Join(remoteRepoDir, ".cx")
				if err := os.MkdirAll(remoteRepoCxDir, 0755); err != nil {
					return err
				}
				remoteRules := "# Include all text files from the remote repo\n*.txt\n"
				if err := fs.WriteString(filepath.Join(remoteRepoCxDir, "remote.rules"), remoteRules); err != nil {
					return err
				}

				// Initialize remote-repo as git repo and commit
				if result := command.New("git", "init").Dir(remoteRepoDir).Run(); result.Error != nil {
					return fmt.Errorf("failed to init git in remote-repo: %w", result.Error)
				}
				if result := command.New("git", "config", "user.email", "test@example.com").Dir(remoteRepoDir).Run(); result.Error != nil {
					return fmt.Errorf("failed to set git email: %w", result.Error)
				}
				if result := command.New("git", "config", "user.name", "Test User").Dir(remoteRepoDir).Run(); result.Error != nil {
					return fmt.Errorf("failed to set git user: %w", result.Error)
				}
				if result := command.New("git", "add", ".").Dir(remoteRepoDir).Run(); result.Error != nil {
					return fmt.Errorf("failed to git add: %w", result.Error)
				}
				if result := command.New("git", "commit", "-m", "initial").Dir(remoteRepoDir).Run(); result.Error != nil {
					return fmt.Errorf("failed to commit: %w", result.Error)
				}

				// Initialize project-a
				if result := command.New("git", "init").Dir(projectADir).Run(); result.Error != nil {
					return fmt.Errorf("failed to init git in project-a: %w", result.Error)
				}

				return nil
			}),
			harness.NewStep("Create rules with @grep directive on git import", func(ctx *harness.Context) error {
				projectADir := filepath.Join(ctx.RootDir, "project-a")

				// Import from local git repo with grep directive
				gitRules := `# This should import rules from the local git repo and only include files with "REMOTE-WORD"
../remote-repo::remote @grep: "REMOTE-WORD"
`
				return fs.WriteString(filepath.Join(projectADir, ".grove", "rules"), gitRules)
			}),
			harness.NewStep("Verify only files matching grep are included", func(ctx *harness.Context) error {
				cxBinary, err := FindProjectBinary()
				if err != nil {
					return err
				}

				projectADir := filepath.Join(ctx.RootDir, "project-a")
				cmd := ctx.Command(cxBinary, "list").Dir(projectADir)
				result := cmd.Run()

				ctx.ShowCommandOutput(cmd.String(), result.Stdout, result.Stderr)

				if result.Error != nil {
					return result.Error
				}

				output := result.Stdout

				// Should include r1.txt (has "REMOTE-WORD")
				if !strings.Contains(output, "r1.txt") {
					return fmt.Errorf("output should contain r1.txt, got:\n%s", output)
				}

				// Should NOT include r2.txt (no "REMOTE-WORD")
				if strings.Contains(output, "r2.txt") {
					return fmt.Errorf("output should not contain r2.txt, got:\n%s", output)
				}

				return nil
			}),
		},
	}
}
