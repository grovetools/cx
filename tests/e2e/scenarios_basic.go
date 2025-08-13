// File: grove-context/tests/e2e/scenarios_basic.go
package main

import (
    "fmt"
    "os"
    "path/filepath"
    "strings"

    "github.com/mattsolo1/grove-tend/pkg/harness"
    "github.com/mattsolo1/grove-tend/pkg/command"
    "github.com/mattsolo1/grove-tend/pkg/fs"
)

// BasicContextGenerationScenario tests the fundamental `cx generate` workflow.
func BasicContextGenerationScenario() *harness.Scenario {
    return &harness.Scenario{
        Name:        "cx-basic-generation",
        Description: "Tests creating a rules file and generating a context from it.",
        Tags:        []string{"cx", "smoke"},
        Steps: []harness.Step{
            harness.NewStep("Setup test project structure", func(ctx *harness.Context) error {
                // Create test files inside the harness's temporary directory.
                if err := fs.WriteString(filepath.Join(ctx.RootDir, "main.go"), "package main"); err != nil {
                    return err
                }
                if err := fs.WriteString(filepath.Join(ctx.RootDir, "README.md"), "# Test"); err != nil {
                    return err
                }
                // This file should be ignored.
                if err := fs.WriteString(filepath.Join(ctx.RootDir, "main_test.go"), "package main_test"); err != nil {
                    return err
                }
                return nil
            }),
            harness.NewStep("Create .grove/rules file", func(ctx *harness.Context) error {
                rulesContent := "**/*.go\n!**/*_test.go"
                rulesPath := filepath.Join(ctx.RootDir, ".grove", "rules")
                return fs.WriteString(rulesPath, rulesContent)
            }),
            harness.NewStep("Run 'cx generate'", func(ctx *harness.Context) error {
                cxBinary, err := FindProjectBinary()
                if err != nil {
                    return err
                }

                // Run the command within the temp directory.
                cmd := command.New(cxBinary, "generate").Dir(ctx.RootDir)
                result := cmd.Run()

                // Show command output in the test log for debugging.
                ctx.ShowCommandOutput(cmd.String(), result.Stdout, result.Stderr)
                return result.Error
            }),
            harness.NewStep("Verify generated .grove/context", func(ctx *harness.Context) error {
                contextPath := filepath.Join(ctx.RootDir, ".grove", "context")
                content, err := fs.ReadString(contextPath)
                if err != nil {
                    return fmt.Errorf("could not read generated context file: %w", err)
                }

                // Verify that main.go is included.
                if !strings.Contains(content, "<file path=\"main.go\">") {
                    return fmt.Errorf("context file is missing main.go")
                }

                // Verify that the test file is excluded.
                if strings.Contains(content, "main_test.go") {
                    return fmt.Errorf("context file should not include main_test.go")
                }
                return nil
            }),
        },
    }
}

// MissingRulesScenario tests that cx generate handles missing rules file gracefully.
func MissingRulesScenario() *harness.Scenario {
    return &harness.Scenario{
        Name:        "cx-missing-rules",
        Description: "Tests that cx generate creates empty context file with warning when no rules file exists.",
        Tags:        []string{"cx"},
        Steps: []harness.Step{
            harness.NewStep("Setup test project without rules", func(ctx *harness.Context) error {
                // Create some test files but no rules file
                if err := fs.WriteString(filepath.Join(ctx.RootDir, "main.go"), "package main"); err != nil {
                    return err
                }
                if err := fs.WriteString(filepath.Join(ctx.RootDir, "README.md"), "# Test Project"); err != nil {
                    return err
                }
                return nil
            }),
            harness.NewStep("Run 'cx generate' without rules file", func(ctx *harness.Context) error {
                cxBinary, err := FindProjectBinary()
                if err != nil {
                    return err
                }

                // Run the command within the temp directory.
                cmd := command.New(cxBinary, "generate").Dir(ctx.RootDir)
                result := cmd.Run()

                // Show command output in the test log for debugging.
                ctx.ShowCommandOutput(cmd.String(), result.Stdout, result.Stderr)
                
                // Command should succeed (not return an error)
                if result.Error != nil {
                    return fmt.Errorf("cx generate should succeed even without rules file, but got error: %w", result.Error)
                }
                
                // Verify warning message is shown
                if !strings.Contains(result.Stderr, "WARNING: No rules file found!") {
                    return fmt.Errorf("expected warning message about missing rules file in stderr, got: %s", result.Stderr)
                }
                
                if !strings.Contains(result.Stderr, "Create .grove/rules with patterns to include files") {
                    return fmt.Errorf("expected instruction to create .grove/rules in stderr")
                }
                
                return nil
            }),
            harness.NewStep("Verify empty context file was created", func(ctx *harness.Context) error {
                contextPath := filepath.Join(ctx.RootDir, ".grove", "context")
                
                // Verify the context file exists
                if _, err := os.Stat(contextPath); err != nil {
                    return fmt.Errorf("context file should exist even without rules: %w", err)
                }
                
                // Read the content
                content, err := fs.ReadString(contextPath)
                if err != nil {
                    return fmt.Errorf("could not read generated context file: %w", err)
                }

                // Verify the content has the expected comment
                if !strings.Contains(content, "No rules file found") {
                    return fmt.Errorf("context file should contain comment about missing rules, got: %s", content)
                }
                
                // Verify no actual files were included
                if strings.Contains(content, "main.go") || strings.Contains(content, "README.md") {
                    return fmt.Errorf("context file should not include any files when no rules exist")
                }
                
                return nil
            }),
        },
    }
}