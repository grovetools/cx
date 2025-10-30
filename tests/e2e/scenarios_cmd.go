// File: grove-context/tests/e2e/scenarios_cmd.go
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

// FromCmdScenario tests the `cx from-cmd` command that populates context from shell command output.
func FromCmdScenario() *harness.Scenario {
	return &harness.Scenario{
		Name:        "cx-from-cmd",
		Description: "Tests generating context rules from shell command output",
		Tags:        []string{"cx", "from-cmd"},
		Steps: []harness.Step{
			harness.NewStep("Setup test project structure", func(ctx *harness.Context) error {
				// Create various Go source files and test files
				if err := fs.WriteString(filepath.Join(ctx.RootDir, "main.go"), "package main\n// Main file"); err != nil {
					return err
				}
				if err := fs.WriteString(filepath.Join(ctx.RootDir, "utils.go"), "package main\n// Utils"); err != nil {
					return err
				}
				if err := fs.WriteString(filepath.Join(ctx.RootDir, "config.go"), "package main\n// Config"); err != nil {
					return err
				}
				// Create test files that should be excluded
				if err := fs.WriteString(filepath.Join(ctx.RootDir, "main_test.go"), "package main_test"); err != nil {
					return err
				}
				if err := fs.WriteString(filepath.Join(ctx.RootDir, "utils_test.go"), "package main_test"); err != nil {
					return err
				}
				// Create a subdirectory with more files
				// Create subdirectory
				pkgDir := filepath.Join(ctx.RootDir, "pkg")
				if err := os.MkdirAll(pkgDir, 0755); err != nil {
					return err
				}
				if err := fs.WriteString(filepath.Join(ctx.RootDir, "pkg", "handler.go"), "package pkg\n// Handler"); err != nil {
					return err
				}
				if err := fs.WriteString(filepath.Join(ctx.RootDir, "pkg", "handler_test.go"), "package pkg_test"); err != nil {
					return err
				}
				return nil
			}),
			harness.NewStep("Run 'cx from-cmd' with find command", func(ctx *harness.Context) error {
				cxBinary, err := FindProjectBinary()
				if err != nil {
					return err
				}

				// Use find command to get all non-test Go files
				shellCmd := "find . -name '*.go' | grep -v '_test.go' | sort"
				cmd := command.New(cxBinary, "from-cmd", shellCmd).Dir(ctx.RootDir)
				result := cmd.Run()

				// Show command output in the test log for debugging
				ctx.ShowCommandOutput(cmd.String(), result.Stdout, result.Stderr)

				if result.Error != nil {
					return fmt.Errorf("cx from-cmd failed: %w", result.Error)
				}

				// Verify the command reported success - check for the success message pattern
				if !strings.Contains(result.Stdout, ".grove/rules with") || !strings.Contains(result.Stdout, "files from command output") {
					return fmt.Errorf("expected success message in output, got: %s", result.Stdout)
				}

				return nil
			}),
			harness.NewStep("Verify .grove/rules was created", func(ctx *harness.Context) error {
				rulesPath := filepath.Join(ctx.RootDir, ".grove", "rules")
				content, err := fs.ReadString(rulesPath)
				if err != nil {
					return fmt.Errorf("could not read generated rules file: %w", err)
				}

				// Split content into lines for checking
				lines := strings.Split(strings.TrimSpace(content), "\n")

				// Should have exactly 4 non-test Go files
				expectedFiles := map[string]bool{
					"config.go":  false,
					"main.go":    false,
					"utils.go":   false,
					"handler.go": false, // May appear as pkg/handler.go or ./pkg/handler.go
				}

				for _, line := range lines {
					line = strings.TrimSpace(line)
					if line == "" {
						continue
					}
					// Check if this file was expected
					found := false
					for expected := range expectedFiles {
						// Handle different path formats (./file, file, path/file)
						if strings.HasSuffix(line, expected) || strings.Contains(line, "/"+expected) {
							expectedFiles[expected] = true
							found = true
							break
						}
					}
					// Ensure no test files are included
					if strings.Contains(line, "_test.go") {
						return fmt.Errorf("rules file should not include test files, but found: %s", line)
					}
					if !found && line != "" {
						// It's okay if the line contains ./ prefix or other variations
						// Just log it for debugging
						// Just continue, it's okay if the line contains ./ prefix or other variations
					}
				}

				// Count how many files were actually found
				foundCount := 0
				for _, found := range expectedFiles {
					if found {
						foundCount++
					}
				}

				// We expect all 4 files to be found
				if foundCount != 4 {
					return fmt.Errorf("expected 4 files in rules, but found %d. Missing files: %v", foundCount, expectedFiles)
				}

				return nil
			}),
			harness.NewStep("Run 'cx list' to verify context", func(ctx *harness.Context) error {
				cxBinary, err := FindProjectBinary()
				if err != nil {
					return err
				}

				cmd := ctx.Command(cxBinary, "list").Dir(ctx.RootDir)
				result := cmd.Run()

				ctx.ShowCommandOutput(cmd.String(), result.Stdout, result.Stderr)

				if result.Error != nil {
					return fmt.Errorf("cx list failed: %w", result.Error)
				}

				// Verify that the correct files are listed
				output := result.Stdout

				// Should contain non-test Go files
				if !strings.Contains(output, "main.go") {
					return fmt.Errorf("main.go should be in context")
				}
				if !strings.Contains(output, "utils.go") {
					return fmt.Errorf("utils.go should be in context")
				}
				if !strings.Contains(output, "config.go") {
					return fmt.Errorf("config.go should be in context")
				}
				if !strings.Contains(output, "handler.go") {
					return fmt.Errorf("pkg/handler.go should be in context")
				}

				// Should not contain test files
				if strings.Contains(output, "main_test.go") {
					return fmt.Errorf("main_test.go should not be in context")
				}
				if strings.Contains(output, "utils_test.go") {
					return fmt.Errorf("utils_test.go should not be in context")
				}
				if strings.Contains(output, "handler_test.go") {
					return fmt.Errorf("handler_test.go should not be in context")
				}

				return nil
			}),
		},
	}
}

// FromCmdPipelineScenario tests complex pipeline commands with cx from-cmd.
func FromCmdPipelineScenario() *harness.Scenario {
	return &harness.Scenario{
		Name:        "cx-from-cmd-pipeline",
		Description: "Tests cx from-cmd with complex shell pipelines",
		Tags:        []string{"cx", "from-cmd"},
		Steps: []harness.Step{
			harness.NewStep("Setup mixed file types", func(ctx *harness.Context) error {
				// Create various file types
				files := map[string]string{
					"app.js":              "console.log('app');",
					"server.js":           "console.log('server');",
					"test.js":             "console.log('test');",
					"index.html":          "<html></html>",
					"style.css":           "body { margin: 0; }",
					"README.md":           "# README",
					"package.json":        "{}",
					"node_modules/dep.js": "// dependency",
				}

				for path, content := range files {
					fullPath := filepath.Join(ctx.RootDir, path)
					if err := fs.WriteString(fullPath, content); err != nil {
						return err
					}
				}

				return nil
			}),
			harness.NewStep("Run 'cx from-cmd' with pipeline to select JS files", func(ctx *harness.Context) error {
				cxBinary, err := FindProjectBinary()
				if err != nil {
					return err
				}

				// Pipeline: find JS files, exclude node_modules, exclude test files
				shellCmd := "find . -name '*.js' | grep -v node_modules | grep -v test | sort"
				cmd := command.New(cxBinary, "from-cmd", shellCmd).Dir(ctx.RootDir)
				result := cmd.Run()

				ctx.ShowCommandOutput(cmd.String(), result.Stdout, result.Stderr)

				if result.Error != nil {
					return fmt.Errorf("cx from-cmd failed: %w", result.Error)
				}

				return nil
			}),
			harness.NewStep("Verify filtered files in rules", func(ctx *harness.Context) error {
				rulesPath := filepath.Join(ctx.RootDir, ".grove", "rules")
				content, err := fs.ReadString(rulesPath)
				if err != nil {
					return fmt.Errorf("could not read rules file: %w", err)
				}

				// Should contain only app.js and server.js
				if !strings.Contains(content, "app.js") {
					return fmt.Errorf("app.js should be in rules")
				}
				if !strings.Contains(content, "server.js") {
					return fmt.Errorf("server.js should be in rules")
				}

				// Should NOT contain these files
				if strings.Contains(content, "test.js") {
					return fmt.Errorf("test.js should not be in rules")
				}
				if strings.Contains(content, "node_modules") {
					return fmt.Errorf("node_modules files should not be in rules")
				}
				if strings.Contains(content, ".html") || strings.Contains(content, ".css") {
					return fmt.Errorf("non-JS files should not be in rules")
				}

				return nil
			}),
		},
	}
}

// CommandExpressionAbsolutePathsScenario tests @cmd: with commands that output absolute paths.
func CommandExpressionAbsolutePathsScenario() *harness.Scenario {
	return &harness.Scenario{
		Name:        "cx-cmd-expression-absolute-paths",
		Description: "Tests @cmd: expressions that output absolute file paths",
		Tags:        []string{"cx", "cmd-expression", "absolute-paths"},
		Steps: []harness.Step{
			harness.NewStep("Setup test project", func(ctx *harness.Context) error {
				// Create some Go files
				if err := fs.WriteString(filepath.Join(ctx.RootDir, "main.go"), "package main\n// Main"); err != nil {
					return err
				}
				if err := fs.WriteString(filepath.Join(ctx.RootDir, "utils.go"), "package main\n// Utils"); err != nil {
					return err
				}
				if err := fs.WriteString(filepath.Join(ctx.RootDir, "config.go"), "package main\n// Config"); err != nil {
					return err
				}

				// Create local grove.yml with allowed_paths configuration
				groveConfig := fmt.Sprintf(`context:
  allowed_paths:
    - %s
`, ctx.RootDir)
				groveYmlPath := filepath.Join(ctx.RootDir, "grove.yml")
				if err := fs.WriteString(groveYmlPath, groveConfig); err != nil {
					return err
				}

				return nil
			}),
			harness.NewStep("Create rules file with @cmd: using realpath", func(ctx *harness.Context) error {
				// Use realpath to get absolute paths
				rulesContent := `@cmd: find . -name '*.go' -type f | xargs realpath | sort`

				rulesPath := filepath.Join(ctx.RootDir, ".grove", "rules")
				return fs.WriteString(rulesPath, rulesContent)
			}),
			harness.NewStep("Run 'cx list' to verify absolute paths work", func(ctx *harness.Context) error {
				cxBinary, err := FindProjectBinary()
				if err != nil {
					return err
				}

				cmd := ctx.Command(cxBinary, "list").Dir(ctx.RootDir)
				result := cmd.Run()

				ctx.ShowCommandOutput(cmd.String(), result.Stdout, result.Stderr)

				if result.Error != nil {
					return fmt.Errorf("cx list failed: %w", result.Error)
				}

				output := result.Stdout

				// Verify all Go files are included
				if !strings.Contains(output, "main.go") {
					return fmt.Errorf("main.go should be in context")
				}
				if !strings.Contains(output, "utils.go") {
					return fmt.Errorf("utils.go should be in context")
				}
				if !strings.Contains(output, "config.go") {
					return fmt.Errorf("config.go should be in context")
				}

				return nil
			}),
			harness.NewStep("Verify context generation works", func(ctx *harness.Context) error {
				cxBinary, err := FindProjectBinary()
				if err != nil {
					return err
				}

				cmd := ctx.Command(cxBinary, "generate").Dir(ctx.RootDir)
				result := cmd.Run()

				ctx.ShowCommandOutput(cmd.String(), result.Stdout, result.Stderr)

				if result.Error != nil {
					return fmt.Errorf("cx generate failed: %w", result.Error)
				}

				// Verify context file was created and contains the files
				contextPath := filepath.Join(ctx.RootDir, ".grove", "context")
				content, err := fs.ReadString(contextPath)
				if err != nil {
					return fmt.Errorf("could not read context file: %w", err)
				}

				if !strings.Contains(content, "main.go") || !strings.Contains(content, "utils.go") {
					return fmt.Errorf("generated context should contain Go files")
				}

				return nil
			}),
		},
	}
}

// CommandExpressionInRulesScenario tests @cmd: expressions directly in the rules file.
func CommandExpressionInRulesScenario() *harness.Scenario {
	return &harness.Scenario{
		Name:        "cx-cmd-expression-in-rules",
		Description: "Tests using @cmd: expressions in rules file to dynamically include files",
		Tags:        []string{"cx", "rules", "cmd-expression"},
		Steps: []harness.Step{
			harness.NewStep("Setup test project with multiple file types", func(ctx *harness.Context) error {
				// Create Go source files
				if err := fs.WriteString(filepath.Join(ctx.RootDir, "main.go"), "package main\n// Main"); err != nil {
					return err
				}
				if err := fs.WriteString(filepath.Join(ctx.RootDir, "utils.go"), "package main\n// Utils"); err != nil {
					return err
				}
				if err := fs.WriteString(filepath.Join(ctx.RootDir, "config.go"), "package main\n// Config"); err != nil {
					return err
				}
				// Create test files
				if err := fs.WriteString(filepath.Join(ctx.RootDir, "main_test.go"), "package main\n// Test"); err != nil {
					return err
				}
				if err := fs.WriteString(filepath.Join(ctx.RootDir, "utils_test.go"), "package main\n// Test"); err != nil {
					return err
				}
				// Create other files
				if err := fs.WriteString(filepath.Join(ctx.RootDir, "README.md"), "# Project"); err != nil {
					return err
				}
				if err := fs.WriteString(filepath.Join(ctx.RootDir, "go.mod"), "module test"); err != nil {
					return err
				}
				return nil
			}),
			harness.NewStep("Create rules file with @cmd: expression", func(ctx *harness.Context) error {
				// Create rules with command expression to find non-test Go files
				rulesContent := `# Include README
README.md

# Include Go files except tests using command
@cmd: find . -name '*.go' | grep -v _test | sort

# Also include go.mod
go.mod`

				rulesPath := filepath.Join(ctx.RootDir, ".grove", "rules")
				return fs.WriteString(rulesPath, rulesContent)
			}),
			harness.NewStep("Run 'cx list' to trigger command execution", func(ctx *harness.Context) error {
				cxBinary, err := FindProjectBinary()
				if err != nil {
					return err
				}

				cmd := ctx.Command(cxBinary, "list").Dir(ctx.RootDir)
				result := cmd.Run()

				ctx.ShowCommandOutput(cmd.String(), result.Stdout, result.Stderr)

				if result.Error != nil {
					return fmt.Errorf("cx list failed: %w", result.Error)
				}

				output := result.Stdout

				// Verify that non-test Go files are included
				if !strings.Contains(output, "main.go") {
					return fmt.Errorf("main.go should be in context")
				}
				if !strings.Contains(output, "utils.go") {
					return fmt.Errorf("utils.go should be in context")
				}
				if !strings.Contains(output, "config.go") {
					return fmt.Errorf("config.go should be in context")
				}

				// Verify test files are excluded
				if strings.Contains(output, "_test.go") {
					return fmt.Errorf("test files should not be in context")
				}

				// Verify other files from static rules are included
				if !strings.Contains(output, "README.md") {
					return fmt.Errorf("README.md should be in context")
				}
				if !strings.Contains(output, "go.mod") {
					return fmt.Errorf("go.mod should be in context")
				}

				return nil
			}),
			harness.NewStep("Run 'cx generate' to verify context generation", func(ctx *harness.Context) error {
				cxBinary, err := FindProjectBinary()
				if err != nil {
					return err
				}

				cmd := ctx.Command(cxBinary, "generate").Dir(ctx.RootDir)
				result := cmd.Run()

				ctx.ShowCommandOutput(cmd.String(), result.Stdout, result.Stderr)

				if result.Error != nil {
					return fmt.Errorf("cx generate failed: %w", result.Error)
				}

				// Read generated context
				contextPath := filepath.Join(ctx.RootDir, ".grove", "context")
				content, err := fs.ReadString(contextPath)
				if err != nil {
					return fmt.Errorf("could not read context file: %w", err)
				}

				// Verify files from command are in context
				if !strings.Contains(content, "main.go") || !strings.Contains(content, "utils.go") || !strings.Contains(content, "config.go") {
					return fmt.Errorf("command-generated files missing from context")
				}

				// Verify test files are not in context
				if strings.Contains(content, "_test.go") {
					return fmt.Errorf("test files should not be in context")
				}

				return nil
			}),
		},
	}
}
