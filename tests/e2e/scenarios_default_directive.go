// File: grove-context/tests/e2e/scenarios_default_directive.go
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

// DefaultDirectiveBasicScenario tests the basic @default directive functionality.
func DefaultDirectiveBasicScenario() *harness.Scenario {
	return &harness.Scenario{
		Name:        "cx-default-directive-basic",
		Description: "Tests importing rules from another project using @default directive.",
		Tags:        []string{"cx", "default"},
		Steps: []harness.Step{
			harness.NewStep("Setup multi-project structure", func(ctx *harness.Context) error {
				// Create main project (project-a)
				projectA := ctx.RootDir
				if err := fs.WriteString(filepath.Join(projectA, "a.go"), "package a"); err != nil {
					return err
				}
				if err := fs.WriteString(filepath.Join(projectA, "a.txt"), "a text"); err != nil {
					return err
				}
				
				// Create dependency project (project-b) as external directory
				projectB, err := os.MkdirTemp("", "grove-e2e-default-b-")
				if err != nil {
					return fmt.Errorf("failed to create project-b temp dir: %w", err)
				}
				// Store project B path for later cleanup
				projectBFile := filepath.Join(ctx.RootDir, ".project_b_path")
				if err := fs.WriteString(projectBFile, projectB); err != nil {
					os.RemoveAll(projectB)
					return err
				}
				if err := fs.WriteString(filepath.Join(projectB, "b.go"), "package b"); err != nil {
					return err
				}
				if err := fs.WriteString(filepath.Join(projectB, "b.txt"), "b text"); err != nil {
					return err
				}
				
				// Create grove.yml for project-b
				groveYmlContent := `version: 1.0
context:
  default_rules_path: .grove/default.rules`
				if err := fs.WriteString(filepath.Join(projectB, "grove.yml"), groveYmlContent); err != nil {
					return err
				}
				
				// Create default rules for project-b
				defaultRulesContent := `*.go
---
*.txt`
				if err := fs.WriteString(filepath.Join(projectB, ".grove/default.rules"), defaultRulesContent); err != nil {
					return err
				}
				
				return nil
			}),
			harness.NewStep("Create rules file with @default directive", func(ctx *harness.Context) error {
				// Read project B path
				projectBBytes, err := os.ReadFile(filepath.Join(ctx.RootDir, ".project_b_path"))
				if err != nil {
					return fmt.Errorf("failed to read project B path: %w", err)
				}
				projectB := string(projectBBytes)
				
				// Project A's rules file references project B using absolute path
				rulesContent := fmt.Sprintf(`*.go
@default: %s
---
*.txt`, projectB)
				rulesPath := filepath.Join(ctx.RootDir, ".grove", "rules")
				return fs.WriteString(rulesPath, rulesContent)
			}),
			harness.NewStep("Run 'cx generate'", func(ctx *harness.Context) error {
				cxBinary, err := FindProjectBinary()
				if err != nil {
					return err
				}

				cmd := command.New(cxBinary, "generate").Dir(ctx.RootDir)
				result := cmd.Run()

				ctx.ShowCommandOutput(cmd.String(), result.Stdout, result.Stderr)
				return result.Error
			}),
			harness.NewStep("Verify hot context includes imported rules", func(ctx *harness.Context) error {
				
				contextPath := filepath.Join(ctx.RootDir, ".grove", "context")
				content, err := fs.ReadString(contextPath)
				if err != nil {
					return fmt.Errorf("could not read generated context file: %w", err)
				}

				// Verify local files are included
				if !strings.Contains(content, "<file path=\"a.go\">") {
					return fmt.Errorf("context file is missing a.go")
				}
				
				
				// Verify files from project-b's hot context are included
				// On macOS, /var is a symlink to /private/var, so we need to check both
				if !strings.Contains(content, "/b.go\">") {
					return fmt.Errorf("context file is missing b.go imported from @default")
				}
				if !strings.Contains(content, "package b") {
					return fmt.Errorf("context file is missing content from b.go")
				}
				
				// Verify files from project-b's cold context are also included in hot
				// (because @default in hot section pulls everything as hot)
				if !strings.Contains(content, "/b.txt\">") {
					return fmt.Errorf("context file is missing b.txt imported from @default")
				}
				if !strings.Contains(content, "b text") {
					return fmt.Errorf("context file is missing content from b.txt")
				}
				
				// Local a.txt should NOT be in hot context (it's in cold section)
				if strings.Contains(content, "<file path=\"a.txt\">") {
					return fmt.Errorf("context file should not include a.txt in hot context")
				}
				
				return nil
			}),
			harness.NewStep("Verify cold context is correct", func(ctx *harness.Context) error {
				// Check the cached-context file directly
				cachedContextPath := filepath.Join(ctx.RootDir, ".grove", "cached-context")
				cachedContent, err := fs.ReadString(cachedContextPath)
				if err != nil {
					// It's OK if cached-context doesn't exist yet
					return nil
				}
				
				// Cold context should have a.txt
				if !strings.Contains(cachedContent, "a.txt") && !strings.Contains(cachedContent, "a text") {
					return fmt.Errorf("cold context should include a.txt")
				}
				
				// Should NOT have b.txt or b.go (they were pulled into hot context)
				if strings.Contains(cachedContent, "b.txt") || strings.Contains(cachedContent, "b.go") {
					return fmt.Errorf("cold context should not include project-b files (they should be in hot)")
				}
				
				return nil
			}),
			harness.NewStep("Cleanup external project", func(ctx *harness.Context) error {
				// Clean up project B
				projectBBytes, _ := os.ReadFile(filepath.Join(ctx.RootDir, ".project_b_path"))
				if len(projectBBytes) > 0 {
					os.RemoveAll(string(projectBBytes))
				}
				return nil
			}),
		},
	}
}

// DefaultDirectiveColdContextScenario tests @default in cold context section.
func DefaultDirectiveColdContextScenario() *harness.Scenario {
	return &harness.Scenario{
		Name:        "cx-default-directive-cold",
		Description: "Tests @default directive in cold context section.",
		Tags:        []string{"cx", "default"},
		Steps: []harness.Step{
			harness.NewStep("Setup multi-project structure", func(ctx *harness.Context) error {
				// Create main project (project-a)
				projectA := ctx.RootDir
				if err := fs.WriteString(filepath.Join(projectA, "a.go"), "package a"); err != nil {
					return err
				}
				if err := fs.WriteString(filepath.Join(projectA, "a.txt"), "a text"); err != nil {
					return err
				}
				
				// Create dependency project (project-c) as sibling
				projectC := filepath.Join(filepath.Dir(projectA), "project-c")
				if err := fs.WriteString(filepath.Join(projectC, "c.go"), "package c"); err != nil {
					return err
				}
				if err := fs.WriteString(filepath.Join(projectC, "c.txt"), "c text"); err != nil {
					return err
				}
				
				// Create grove.yml for project-c
				groveYmlContent := `version: 1.0
context:
  default_rules_path: rules.ctx`
				if err := fs.WriteString(filepath.Join(projectC, "grove.yml"), groveYmlContent); err != nil {
					return err
				}
				
				// Create default rules for project-c
				defaultRulesContent := `*.go
---
*.txt`
				if err := fs.WriteString(filepath.Join(projectC, "rules.ctx"), defaultRulesContent); err != nil {
					return err
				}
				
				return nil
			}),
			harness.NewStep("Create rules file with @default in cold section", func(ctx *harness.Context) error {
				// Project A's rules file references project C in cold section
				rulesContent := `*.go
---
*.txt
@default: ../project-c`
				rulesPath := filepath.Join(ctx.RootDir, ".grove", "rules")
				return fs.WriteString(rulesPath, rulesContent)
			}),
			harness.NewStep("Run 'cx generate'", func(ctx *harness.Context) error {
				cxBinary, err := FindProjectBinary()
				if err != nil {
					return err
				}

				cmd := command.New(cxBinary, "generate").Dir(ctx.RootDir)
				result := cmd.Run()

				ctx.ShowCommandOutput(cmd.String(), result.Stdout, result.Stderr)
				return result.Error
			}),
			harness.NewStep("Verify hot context has only local .go files", func(ctx *harness.Context) error {
				contextPath := filepath.Join(ctx.RootDir, ".grove", "context")
				content, err := fs.ReadString(contextPath)
				if err != nil {
					return fmt.Errorf("could not read generated context file: %w", err)
				}

				// Verify local .go file is included
				if !strings.Contains(content, "<file path=\"a.go\">") {
					return fmt.Errorf("context file is missing a.go")
				}
				
				// Project C files should NOT be in hot context
				if strings.Contains(content, "c.go") || strings.Contains(content, "c.txt") {
					return fmt.Errorf("hot context should not include files from project-c")
				}
				
				return nil
			}),
			harness.NewStep("Verify cold context includes imported rules", func(ctx *harness.Context) error {
				// Use cx list-cache to check cold context
				cxBinary, err := FindProjectBinary()
				if err != nil {
					return err
				}

				cmd := command.New(cxBinary, "list-cache").Dir(ctx.RootDir)
				result := cmd.Run()
				
				if result.Error != nil {
					return result.Error
				}
				
				// Cold context should have local a.txt
				if !strings.Contains(result.Stdout, "a.txt") {
					return fmt.Errorf("cold context should include a.txt, got: %s", result.Stdout)
				}
				
				// Cold context should have all files from project-c
				if !strings.Contains(result.Stdout, "c.go") {
					return fmt.Errorf("cold context should include ../project-c/c.go from @default, got: %s", result.Stdout)
				}
				
				if !strings.Contains(result.Stdout, "c.txt") {
					return fmt.Errorf("cold context should include ../project-c/c.txt from @default, got: %s", result.Stdout)
				}
				
				return nil
			}),
		},
	}
}

// DefaultDirectiveCircularScenario tests circular dependency handling.
func DefaultDirectiveCircularScenario() *harness.Scenario {
	return &harness.Scenario{
		Name:        "cx-default-directive-circular",
		Description: "Tests that circular @default references are handled gracefully.",
		Tags:        []string{"cx", "default"},
		Steps: []harness.Step{
			harness.NewStep("Setup circular project structure", func(ctx *harness.Context) error {
				// Create main project (project-a)
				projectA := ctx.RootDir
				if err := fs.WriteString(filepath.Join(projectA, "a.go"), "package a"); err != nil {
					return err
				}
				
				// Create project-a's grove.yml for being referenced
				groveYmlA := `version: 1.0
context:
  default_rules_path: .grove/rules`
				if err := fs.WriteString(filepath.Join(projectA, "grove.yml"), groveYmlA); err != nil {
					return err
				}
				
				// Create dependency project (project-b) as sibling
				projectB := filepath.Join(filepath.Dir(projectA), "project-b")
				if err := fs.WriteString(filepath.Join(projectB, "b.go"), "package b"); err != nil {
					return err
				}
				
				// Create grove.yml for project-b
				groveYmlB := `version: 1.0
context:
  default_rules_path: rules.ctx`
				if err := fs.WriteString(filepath.Join(projectB, "grove.yml"), groveYmlB); err != nil {
					return err
				}
				
				// Create project-b's rules that reference back to project-a (circular)
				rulesB := `*.go
@default: ../project-a`
				if err := fs.WriteString(filepath.Join(projectB, "rules.ctx"), rulesB); err != nil {
					return err
				}
				
				return nil
			}),
			harness.NewStep("Create rules file with circular reference", func(ctx *harness.Context) error {
				// Project A's rules file references project B, which references back to A
				rulesContent := `*.go
@default: ../project-b`
				rulesPath := filepath.Join(ctx.RootDir, ".grove", "rules")
				return fs.WriteString(rulesPath, rulesContent)
			}),
			harness.NewStep("Run 'cx generate' with circular dependency", func(ctx *harness.Context) error {
				cxBinary, err := FindProjectBinary()
				if err != nil {
					return err
				}

				cmd := command.New(cxBinary, "generate").Dir(ctx.RootDir)
				result := cmd.Run()

				ctx.ShowCommandOutput(cmd.String(), result.Stdout, result.Stderr)
				
				// Should not fail - circular dependencies should be handled
				return result.Error
			}),
			harness.NewStep("Verify context was generated correctly", func(ctx *harness.Context) error {
				contextPath := filepath.Join(ctx.RootDir, ".grove", "context")
				content, err := fs.ReadString(contextPath)
				if err != nil {
					return fmt.Errorf("could not read generated context file: %w", err)
				}

				// Should have a.go
				if !strings.Contains(content, "<file path=\"a.go\">") {
					return fmt.Errorf("context file is missing a.go")
				}
				
				// Should have b.go from the first resolution
				if !strings.Contains(content, "<file path=\"../project-b/b.go\">") {
					return fmt.Errorf("context file is missing ../project-b/b.go")
				}
				
				// Should not have duplicates or infinite recursion issues
				aCount := strings.Count(content, "package a")
				if aCount != 1 {
					return fmt.Errorf("expected exactly 1 instance of 'package a', got %d", aCount)
				}
				
				return nil
			}),
		},
	}
}