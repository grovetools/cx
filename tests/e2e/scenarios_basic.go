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
                // Auto-discover the cx binary path
                cxBinary := os.Getenv("CX_BINARY")
                if cxBinary == "" {
                    // Try common locations
                    candidates := []string{
                        "./bin/cx",
                        "../bin/cx", 
                        "../../bin/cx",
                        "cx", // In PATH
                    }
                    
                    for _, candidate := range candidates {
                        if _, err := os.Stat(candidate); err == nil {
                            if filepath.IsAbs(candidate) {
                                cxBinary = candidate
                            } else {
                                if abs, err := filepath.Abs(candidate); err == nil {
                                    cxBinary = abs
                                }
                            }
                            break
                        }
                    }
                    
                    if cxBinary == "" {
                        return fmt.Errorf("cx binary not found. Build it first with 'make build' or set CX_BINARY env var")
                    }
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