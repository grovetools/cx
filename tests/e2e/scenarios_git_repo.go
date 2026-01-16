package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	
	"github.com/grovetools/tend/pkg/harness"
	"github.com/grovetools/tend/pkg/command"
	"github.com/grovetools/tend/pkg/fs"
)

// GitRepositoryCloneScenario tests cloning a real Git repository and including it in context
func GitRepositoryCloneScenario() *harness.Scenario {
	return &harness.Scenario{
		Name:        "cx-git-repository-clone",
		Description: "Tests cloning a real Git repository (charmbracelet/lipgloss) and including it in context",
		Tags:        []string{"cx", "git", "repo"},
		Steps: []harness.Step{
			harness.NewStep("Setup test project with local files", func(ctx *harness.Context) error {
				// Create some local Go files
				mainGo := `package main

import "fmt"

func main() {
    fmt.Println("Hello from local project")
}`
				if err := fs.WriteString(filepath.Join(ctx.RootDir, "main.go"), mainGo); err != nil {
					return fmt.Errorf("failed to create main.go: %w", err)
				}
				
				readmeMd := `# Test Project

This is a test project that includes the lipgloss library.`
				if err := fs.WriteString(filepath.Join(ctx.RootDir, "README.md"), readmeMd); err != nil {
					return fmt.Errorf("failed to create README.md: %w", err)
				}
				
				// Create a test file that should be excluded
				testGo := `package main

import "testing"

func TestMain(t *testing.T) {
    t.Log("test file")
}`
				if err := fs.WriteString(filepath.Join(ctx.RootDir, "main_test.go"), testGo); err != nil {
					return fmt.Errorf("failed to create main_test.go: %w", err)
				}
				
				return nil
			}),
			
			harness.NewStep("Create rules file with Git repository URL", func(ctx *harness.Context) error {
				rulesContent := `# Include local Go and Markdown files
*.go
*.md
!*_test.go

# Include lipgloss library from GitHub with specific version
https://github.com/charmbracelet/lipgloss@v0.13.0

# Exclude examples and test data from the repository
!**/examples/**
!**/testdata/**
!**/*_test.go`
				
				rulesPath := filepath.Join(ctx.RootDir, ".grove", "rules")
				if err := fs.WriteString(rulesPath, rulesContent); err != nil {
					return fmt.Errorf("failed to create rules file: %w", err)
				}
				
				return nil
			}),
			
			harness.NewStep("Run 'cx generate' to clone repository and build context", func(ctx *harness.Context) error {
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
				
				// Check that it reports files were generated (success message goes to stderr via unified logger)
				if !strings.Contains(result.Stderr, "Generated context file") {
					return fmt.Errorf("expected 'Generated context file' in stderr, got: %s", result.Stderr)
				}
				
				return nil
			}),
			
			harness.NewStep("Verify repository was cloned and tracked", func(ctx *harness.Context) error {
				cxBinary, err := FindProjectBinary()
				if err != nil {
					return err
				}
				
				// Check repo list
				cmd := command.New(cxBinary, "repo", "list").Dir(ctx.RootDir)
				result := cmd.Run()
				
				if result.ExitCode != 0 {
					return fmt.Errorf("cx repo list failed: %s\nStderr: %s", result.Stdout, result.Stderr)
				}
				
				// Verify lipgloss appears in the list
				if !strings.Contains(result.Stdout, "github.com/charmbracelet/lipgloss") {
					return fmt.Errorf("expected lipgloss in repo list, got: %s", result.Stdout)
				}
				
				// Verify version v0.13.0 is shown
				if !strings.Contains(result.Stdout, "v0.13.0") {
					return fmt.Errorf("expected version v0.13.0 in repo list, got: %s", result.Stdout)
				}
				
				return nil
			}),
			
			harness.NewStep("Verify lipgloss files are included in context", func(ctx *harness.Context) error {
				cxBinary, err := FindProjectBinary()
				if err != nil {
					return err
				}
				
				// List files in context
				cmd := command.New(cxBinary, "list").Dir(ctx.RootDir)
				result := cmd.Run()
				
				if result.ExitCode != 0 {
					return fmt.Errorf("cx list failed: %s\nStderr: %s", result.Stdout, result.Stderr)
				}
				
				// Check for lipgloss files
				if !strings.Contains(result.Stdout, "lipgloss") {
					return fmt.Errorf("expected lipgloss files in context, got: %s", result.Stdout)
				}
				
				// Count lipgloss Go files (should have several)
				lipglossGoFiles := 0
				for _, line := range strings.Split(result.Stdout, "\n") {
					if strings.Contains(line, "lipgloss") && strings.HasSuffix(line, ".go") && !strings.Contains(line, "_test.go") {
						lipglossGoFiles++
					}
				}
				
				if lipglossGoFiles < 10 {
					return fmt.Errorf("expected at least 10 lipgloss .go files, found %d", lipglossGoFiles)
				}
				
				// Verify test files are excluded
				for _, line := range strings.Split(result.Stdout, "\n") {
					if strings.Contains(line, "lipgloss") && strings.Contains(line, "_test.go") {
						return fmt.Errorf("test files should be excluded but found: %s", line)
					}
					if strings.Contains(line, "examples/") {
						return fmt.Errorf("examples should be excluded but found: %s", line)
					}
				}
				
				// Verify local files are also included
				if !strings.Contains(result.Stdout, "main.go") {
					return fmt.Errorf("expected local main.go in context")
				}
				if !strings.Contains(result.Stdout, "README.md") {
					return fmt.Errorf("expected local README.md in context")
				}
				if strings.Contains(result.Stdout, "main_test.go") {
					return fmt.Errorf("local test file should be excluded")
				}
				
				return nil
			}),
			
			harness.NewStep("Verify manifest file was created", func(ctx *harness.Context) error {
				homeDir, err := os.UserHomeDir()
				if err != nil {
					return fmt.Errorf("failed to get home directory: %w", err)
				}
				
				manifestPath := filepath.Join(homeDir, ".grove", "cx", "manifest.json")
				if _, err := os.Stat(manifestPath); os.IsNotExist(err) {
					return fmt.Errorf("manifest file does not exist at %s", manifestPath)
				}
				
				// Read manifest to verify content
				manifestData, err := os.ReadFile(manifestPath)
				if err != nil {
					return fmt.Errorf("failed to read manifest: %w", err)
				}
				
				if !strings.Contains(string(manifestData), "github.com/charmbracelet/lipgloss") {
					return fmt.Errorf("manifest should contain lipgloss URL")
				}
				if !strings.Contains(string(manifestData), "v0.13.0") {
					return fmt.Errorf("manifest should contain pinned version")
				}
				
				return nil
			}),
			
			harness.NewStep("Test updating audit status", func(ctx *harness.Context) error {
				cxBinary, err := FindProjectBinary()
				if err != nil {
					return err
				}
				
				cmd := command.New(cxBinary, "repo", "audit", "https://github.com/charmbracelet/lipgloss", "--status=audited").Dir(ctx.RootDir)
				result := cmd.Run()
				
				if result.ExitCode != 0 {
					return fmt.Errorf("cx repo audit failed: %s\nStderr: %s", result.Stdout, result.Stderr)
				}
				
				if !strings.Contains(result.Stdout, "Updated audit status") {
					return fmt.Errorf("expected audit status update confirmation, got: %s", result.Stdout)
				}
				
				// Verify audit status in list
				listCmd := command.New(cxBinary, "repo", "list").Dir(ctx.RootDir)
				listResult := listCmd.Run()
				
				if listResult.ExitCode != 0 {
					return fmt.Errorf("cx repo list failed: %s\nStderr: %s", listResult.Stdout, listResult.Stderr)
				}
				
				if !strings.Contains(listResult.Stdout, "audited") {
					return fmt.Errorf("expected 'audited' status in repo list, got: %s", listResult.Stdout)
				}
				
				return nil
			}),
			
			harness.NewStep("Test repository sync", func(ctx *harness.Context) error {
				cxBinary, err := FindProjectBinary()
				if err != nil {
					return err
				}
				
				cmd := command.New(cxBinary, "repo", "sync").Dir(ctx.RootDir)
				result := cmd.Run()
				
				if result.ExitCode != 0 {
					return fmt.Errorf("cx repo sync failed: %s\nStderr: %s", result.Stdout, result.Stderr)
				}
				
				if !strings.Contains(result.Stdout, "All repositories synced successfully") {
					return fmt.Errorf("expected sync success message, got: %s", result.Stdout)
				}
				
				return nil
			}),
		},
	}
}