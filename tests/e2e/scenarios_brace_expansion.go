// File: grove-context/tests/e2e/scenarios_brace_expansion.go
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

// BraceExpansionBasicScenario tests basic brace expansion with a single brace set
func BraceExpansionBasicScenario() *harness.Scenario {
	return &harness.Scenario{
		Name:        "cx-brace-expansion-basic",
		Description: "Tests basic brace expansion with patterns like {a,b}",
		Tags:        []string{"cx", "brace-expansion"},
		Steps: []harness.Step{
			harness.NewStep("Setup test project with pkg and cmd directories", func(ctx *harness.Context) error {
				// Create pkg directory with Go files
				pkgDir := filepath.Join(ctx.RootDir, "pkg")
				if err := os.MkdirAll(pkgDir, 0755); err != nil {
					return err
				}
				if err := fs.WriteString(filepath.Join(pkgDir, "service.go"), "package pkg"); err != nil {
					return err
				}
				if err := fs.WriteString(filepath.Join(pkgDir, "util.go"), "package pkg"); err != nil {
					return err
				}

				// Create cmd directory with Go files
				cmdDir := filepath.Join(ctx.RootDir, "cmd")
				if err := os.MkdirAll(cmdDir, 0755); err != nil {
					return err
				}
				if err := fs.WriteString(filepath.Join(cmdDir, "main.go"), "package main"); err != nil {
					return err
				}
				if err := fs.WriteString(filepath.Join(cmdDir, "cli.go"), "package main"); err != nil {
					return err
				}

				// Create other directory that should not be included
				otherDir := filepath.Join(ctx.RootDir, "internal")
				if err := os.MkdirAll(otherDir, 0755); err != nil {
					return err
				}
				if err := fs.WriteString(filepath.Join(otherDir, "helper.go"), "package internal"); err != nil {
					return err
				}

				return nil
			}),
			harness.NewStep("Create .grove/rules with brace expansion pattern", func(ctx *harness.Context) error {
				// Pattern: {pkg,cmd}/**/*.go should expand to pkg/**/*.go and cmd/**/*.go
				rulesContent := "{pkg,cmd}/**/*.go"
				rulesPath := filepath.Join(ctx.RootDir, ".grove", "rules")
				return fs.WriteString(rulesPath, rulesContent)
			}),
			harness.NewStep("Run 'cx list' to see matched files", func(ctx *harness.Context) error {
				cxBinary, err := FindProjectBinary()
				if err != nil {
					return err
				}

				cmd := command.New(cxBinary, "list").Dir(ctx.RootDir)
				result := cmd.Run()

				ctx.ShowCommandOutput(cmd.String(), result.Stdout, result.Stderr)
				return result.Error
			}),
			harness.NewStep("Verify that pkg and cmd files are included", func(ctx *harness.Context) error {
				cxBinary, err := FindProjectBinary()
				if err != nil {
					return err
				}

				cmd := command.New(cxBinary, "list").Dir(ctx.RootDir)
				result := cmd.Run()

				if result.Error != nil {
					return result.Error
				}

				output := result.Stdout

				// Verify pkg files are included
				if !strings.Contains(output, "pkg/service.go") {
					return fmt.Errorf("output should contain pkg/service.go")
				}
				if !strings.Contains(output, "pkg/util.go") {
					return fmt.Errorf("output should contain pkg/util.go")
				}

				// Verify cmd files are included
				if !strings.Contains(output, "cmd/main.go") {
					return fmt.Errorf("output should contain cmd/main.go")
				}
				if !strings.Contains(output, "cmd/cli.go") {
					return fmt.Errorf("output should contain cmd/cli.go")
				}

				// Verify internal files are NOT included
				if strings.Contains(output, "internal/helper.go") {
					return fmt.Errorf("output should not contain internal/helper.go")
				}

				return nil
			}),
		},
	}
}

// BraceExpansionMultipleScenario tests multiple brace expansions in one pattern
func BraceExpansionMultipleScenario() *harness.Scenario {
	return &harness.Scenario{
		Name:        "cx-brace-expansion-multiple",
		Description: "Tests multiple brace sets like {a,b}/{c,d}",
		Tags:        []string{"cx", "brace-expansion"},
		Steps: []harness.Step{
			harness.NewStep("Setup test project with nested directories", func(ctx *harness.Context) error {
				// Create src/lib and src/app directories
				dirs := []string{
					filepath.Join(ctx.RootDir, "src", "lib"),
					filepath.Join(ctx.RootDir, "src", "app"),
					filepath.Join(ctx.RootDir, "tests", "lib"),
					filepath.Join(ctx.RootDir, "tests", "app"),
				}

				for _, dir := range dirs {
					if err := os.MkdirAll(dir, 0755); err != nil {
						return err
					}
				}

				// Add files to each directory
				files := map[string]string{
					filepath.Join(ctx.RootDir, "src", "lib", "core.go"):     "package lib",
					filepath.Join(ctx.RootDir, "src", "app", "main.go"):     "package app",
					filepath.Join(ctx.RootDir, "tests", "lib", "test.go"):   "package lib_test",
					filepath.Join(ctx.RootDir, "tests", "app", "e2e.go"):    "package app_test",
					filepath.Join(ctx.RootDir, "other.go"):                  "package main",
				}

				for path, content := range files {
					if err := fs.WriteString(path, content); err != nil {
						return err
					}
				}

				return nil
			}),
			harness.NewStep("Create .grove/rules with multiple brace expansions", func(ctx *harness.Context) error {
				// Pattern: {src,tests}/{lib,app}/**/*.go
				// Should expand to: src/lib/**/*.go, src/app/**/*.go, tests/lib/**/*.go, tests/app/**/*.go
				rulesContent := "{src,tests}/{lib,app}/**/*.go"
				rulesPath := filepath.Join(ctx.RootDir, ".grove", "rules")
				return fs.WriteString(rulesPath, rulesContent)
			}),
			harness.NewStep("Verify all combinations are matched", func(ctx *harness.Context) error {
				cxBinary, err := FindProjectBinary()
				if err != nil {
					return err
				}

				cmd := command.New(cxBinary, "list").Dir(ctx.RootDir)
				result := cmd.Run()

				ctx.ShowCommandOutput(cmd.String(), result.Stdout, result.Stderr)

				if result.Error != nil {
					return result.Error
				}

				output := result.Stdout

				// Verify all 4 combinations are included
				expectedFiles := []string{
					"src/lib/core.go",
					"src/app/main.go",
					"tests/lib/test.go",
					"tests/app/e2e.go",
				}

				for _, file := range expectedFiles {
					if !strings.Contains(output, file) {
						return fmt.Errorf("output should contain %s", file)
					}
				}

				// Verify other.go is NOT included
				if strings.Contains(output, "other.go") {
					return fmt.Errorf("output should not contain other.go")
				}

				return nil
			}),
		},
	}
}

// BraceExpansionWithExclusionScenario tests brace expansion with exclusion patterns
func BraceExpansionWithExclusionScenario() *harness.Scenario {
	return &harness.Scenario{
		Name:        "cx-brace-expansion-exclusion",
		Description: "Tests brace expansion combined with exclusion patterns",
		Tags:        []string{"cx", "brace-expansion"},
		Steps: []harness.Step{
			harness.NewStep("Setup test project", func(ctx *harness.Context) error {
				dirs := []string{
					filepath.Join(ctx.RootDir, "pkg"),
					filepath.Join(ctx.RootDir, "cmd"),
				}

				for _, dir := range dirs {
					if err := os.MkdirAll(dir, 0755); err != nil {
						return err
					}
				}

				files := map[string]string{
					filepath.Join(ctx.RootDir, "pkg", "service.go"):      "package pkg",
					filepath.Join(ctx.RootDir, "pkg", "service_test.go"): "package pkg",
					filepath.Join(ctx.RootDir, "cmd", "main.go"):         "package main",
					filepath.Join(ctx.RootDir, "cmd", "main_test.go"):    "package main",
				}

				for path, content := range files {
					if err := fs.WriteString(path, content); err != nil {
						return err
					}
				}

				return nil
			}),
			harness.NewStep("Create .grove/rules with brace expansion and exclusion", func(ctx *harness.Context) error {
				// Include pkg and cmd, but exclude test files
				rulesContent := "{pkg,cmd}/**/*.go\n!**/*_test.go"
				rulesPath := filepath.Join(ctx.RootDir, ".grove", "rules")
				return fs.WriteString(rulesPath, rulesContent)
			}),
			harness.NewStep("Verify non-test files are included, test files excluded", func(ctx *harness.Context) error {
				cxBinary, err := FindProjectBinary()
				if err != nil {
					return err
				}

				cmd := command.New(cxBinary, "list").Dir(ctx.RootDir)
				result := cmd.Run()

				ctx.ShowCommandOutput(cmd.String(), result.Stdout, result.Stderr)

				if result.Error != nil {
					return result.Error
				}

				output := result.Stdout

				// Verify non-test files are included
				if !strings.Contains(output, "pkg/service.go") {
					return fmt.Errorf("output should contain pkg/service.go")
				}
				if !strings.Contains(output, "cmd/main.go") {
					return fmt.Errorf("output should contain cmd/main.go")
				}

				// Verify test files are excluded
				if strings.Contains(output, "service_test.go") {
					return fmt.Errorf("output should not contain service_test.go")
				}
				if strings.Contains(output, "main_test.go") {
					return fmt.Errorf("output should not contain main_test.go")
				}

				return nil
			}),
		},
	}
}
