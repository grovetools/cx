// File: grove-context/tests/e2e/scenarios_search_directives.go
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/grovetools/tend/pkg/command"
	"github.com/grovetools/tend/pkg/fs"
	"github.com/grovetools/tend/pkg/harness"
)

// FindDirectiveScenario tests the @find directive for filtering by filename
func FindDirectiveScenario() *harness.Scenario {
	return &harness.Scenario{
		Name:        "cx-find-directive",
		Description: "Tests @find directive for filtering files by filename/path",
		Tags:        []string{"cx", "search-directives"},
		Steps: []harness.Step{
			harness.NewStep("Setup test project with various files", func(ctx *harness.Context) error {
				// Create directories
				dirs := []string{
					filepath.Join(ctx.RootDir, "pkg", "managers"),
					filepath.Join(ctx.RootDir, "pkg", "api"),
					filepath.Join(ctx.RootDir, "pkg", "utils"),
				}

				for _, dir := range dirs {
					if err := os.MkdirAll(dir, 0o755); err != nil {
						return err
					}
				}

				// Create files - some with "manager" in the name
				files := map[string]string{
					filepath.Join(ctx.RootDir, "pkg", "managers", "user_manager.go"): "package managers\n\ntype UserManager struct {}",
					filepath.Join(ctx.RootDir, "pkg", "managers", "file_manager.go"): "package managers\n\ntype FileManager struct {}",
					filepath.Join(ctx.RootDir, "pkg", "api", "user_api.go"):          "package api\n\ntype UserAPI struct {}",
					filepath.Join(ctx.RootDir, "pkg", "api", "file_api.go"):          "package api\n\ntype FileAPI struct {}",
					filepath.Join(ctx.RootDir, "pkg", "utils", "helper.go"):          "package utils\n\nfunc Help() {}",
				}

				for path, content := range files {
					if err := fs.WriteString(path, content); err != nil {
						return err
					}
				}

				return nil
			}),
			harness.NewStep("Create .grove/rules with @find directive", func(ctx *harness.Context) error {
				// Only include Go files that have "manager" in their path
				rulesContent := "pkg/**/*.go @find: \"manager\""
				rulesPath := filepath.Join(ctx.RootDir, ".grove", "rules")
				return fs.WriteString(rulesPath, rulesContent)
			}),
			harness.NewStep("Verify only manager files are included", func(ctx *harness.Context) error {
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

				// Verify manager files are included
				if !strings.Contains(output, "user_manager.go") {
					return fmt.Errorf("output should contain user_manager.go")
				}
				if !strings.Contains(output, "file_manager.go") {
					return fmt.Errorf("output should contain file_manager.go")
				}

				// Verify non-manager files are NOT included
				if strings.Contains(output, "user_api.go") {
					return fmt.Errorf("output should not contain user_api.go")
				}
				if strings.Contains(output, "file_api.go") {
					return fmt.Errorf("output should not contain file_api.go")
				}
				if strings.Contains(output, "helper.go") {
					return fmt.Errorf("output should not contain helper.go")
				}

				return nil
			}),
		},
	}
}

// FindDirectiveGlobScenario tests the @find directive with glob patterns
func FindDirectiveGlobScenario() *harness.Scenario {
	return &harness.Scenario{
		Name:        "cx-find-directive-glob",
		Description: "Tests @find directive using glob patterns",
		Tags:        []string{"cx", "search-directives"},
		Steps: []harness.Step{
			harness.NewStep("Setup test project with various files", func(ctx *harness.Context) error {
				dirs := []string{
					filepath.Join(ctx.RootDir, "pkg", "managers"),
					filepath.Join(ctx.RootDir, "pkg", "api"),
				}
				for _, dir := range dirs {
					if err := os.MkdirAll(dir, 0o755); err != nil {
						return err
					}
				}

				files := map[string]string{
					filepath.Join(ctx.RootDir, "pkg", "managers", "user_manager_test.go"): "package managers",
					filepath.Join(ctx.RootDir, "pkg", "managers", "file_manager.go"):      "package managers",
					filepath.Join(ctx.RootDir, "pkg", "api", "api_test.go"):               "package api",
					filepath.Join(ctx.RootDir, "pkg", "api", "file_api.go"):               "package api",
				}

				for path, content := range files {
					if err := fs.WriteString(path, content); err != nil {
						return err
					}
				}
				return nil
			}),
			harness.NewStep("Create .grove/rules with @find glob directive", func(ctx *harness.Context) error {
				rulesContent := `pkg/**/*.go @find: "*_test.go"`
				rulesPath := filepath.Join(ctx.RootDir, ".grove", "rules")
				return fs.WriteString(rulesPath, rulesContent)
			}),
			harness.NewStep("Verify only test files are included", func(ctx *harness.Context) error {
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

				if !strings.Contains(output, "user_manager_test.go") {
					return fmt.Errorf("output should contain user_manager_test.go")
				}
				if !strings.Contains(output, "api_test.go") {
					return fmt.Errorf("output should contain api_test.go")
				}
				if strings.Contains(output, "file_manager.go") {
					return fmt.Errorf("output should not contain file_manager.go")
				}
				if strings.Contains(output, "file_api.go") {
					return fmt.Errorf("output should not contain file_api.go")
				}

				return nil
			}),
		},
	}
}

// FindDirectiveRegexScenario tests the @find directive with regex patterns
func FindDirectiveRegexScenario() *harness.Scenario {
	return &harness.Scenario{
		Name:        "cx-find-directive-regex",
		Description: "Tests @find directive using regex patterns",
		Tags:        []string{"cx", "search-directives"},
		Steps: []harness.Step{
			harness.NewStep("Setup test project with various files", func(ctx *harness.Context) error {
				dirs := []string{
					filepath.Join(ctx.RootDir, "pkg", "managers"),
				}
				for _, dir := range dirs {
					if err := os.MkdirAll(dir, 0o755); err != nil {
						return err
					}
				}

				files := map[string]string{
					filepath.Join(ctx.RootDir, "pkg", "managers", "a_manager.go"):  "package managers",
					filepath.Join(ctx.RootDir, "pkg", "managers", "1_manager.go"):  "package managers",
					filepath.Join(ctx.RootDir, "pkg", "managers", "12_manager.go"): "package managers",
				}

				for path, content := range files {
					if err := fs.WriteString(path, content); err != nil {
						return err
					}
				}
				return nil
			}),
			harness.NewStep("Create .grove/rules with @find regex directive", func(ctx *harness.Context) error {
				rulesContent := "pkg/**/*.go @find: \"[0-9]+_manager\\.go$\""
				rulesPath := filepath.Join(ctx.RootDir, ".grove", "rules")
				return fs.WriteString(rulesPath, rulesContent)
			}),
			harness.NewStep("Verify only regex matching files are included", func(ctx *harness.Context) error {
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

				if strings.Contains(output, "a_manager.go") {
					return fmt.Errorf("output should not contain a_manager.go")
				}
				if !strings.Contains(output, "1_manager.go") {
					return fmt.Errorf("output should contain 1_manager.go")
				}
				if !strings.Contains(output, "12_manager.go") {
					return fmt.Errorf("output should contain 12_manager.go")
				}

				return nil
			}),
		},
	}
}

// FindDirectiveDoubleStarGlobScenario tests the @find directive with double-star glob patterns
func FindDirectiveDoubleStarGlobScenario() *harness.Scenario {
	return &harness.Scenario{
		Name:        "cx-find-directive-doublestar",
		Description: "Tests @find directive using double-star glob patterns",
		Tags:        []string{"cx", "search-directives"},
		Steps: []harness.Step{
			harness.NewStep("Setup test project with nested directories", func(ctx *harness.Context) error {
				dirs := []string{
					filepath.Join(ctx.RootDir, "app", "services", "http"),
					filepath.Join(ctx.RootDir, "app", "services", "grpc"),
				}
				for _, dir := range dirs {
					if err := os.MkdirAll(dir, 0o755); err != nil {
						return err
					}
				}

				files := map[string]string{
					filepath.Join(ctx.RootDir, "app", "services", "http", "test_server.go"): "package http",
					filepath.Join(ctx.RootDir, "app", "services", "grpc", "test_client.go"): "package grpc",
					filepath.Join(ctx.RootDir, "app", "services", "http", "server.go"):      "package http",
				}

				for path, content := range files {
					if err := fs.WriteString(path, content); err != nil {
						return err
					}
				}
				return nil
			}),
			harness.NewStep("Create .grove/rules with @find double-star directive", func(ctx *harness.Context) error {
				// Pattern **/test_*.go uses matchDoubleStarPattern: empty prefix, suffix test_*.go
				rulesContent := `app/**/*.go @find: "**/test_*.go"`
				rulesPath := filepath.Join(ctx.RootDir, ".grove", "rules")
				return fs.WriteString(rulesPath, rulesContent)
			}),
			harness.NewStep("Verify only test_ prefixed files are included", func(ctx *harness.Context) error {
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

				if !strings.Contains(output, "test_server.go") {
					return fmt.Errorf("output should contain test_server.go")
				}
				if !strings.Contains(output, "test_client.go") {
					return fmt.Errorf("output should contain test_client.go")
				}
				if strings.Contains(output, "app/services/http/server.go") {
					return fmt.Errorf("output should not contain server.go (without test_ prefix)")
				}

				return nil
			}),
		},
	}
}

// FindDirectiveInvalidRegexFallbackScenario tests the @find directive with invalid regex strings
func FindDirectiveInvalidRegexFallbackScenario() *harness.Scenario {
	return &harness.Scenario{
		Name:        "cx-find-directive-invalid-regex",
		Description: "Tests @find directive falls back to substring on invalid regex",
		Tags:        []string{"cx", "search-directives"},
		Steps: []harness.Step{
			harness.NewStep("Setup test project", func(ctx *harness.Context) error {
				if err := os.MkdirAll(filepath.Join(ctx.RootDir, "pkg"), 0o755); err != nil {
					return err
				}

				files := map[string]string{
					filepath.Join(ctx.RootDir, "pkg", "draft_[unbalanced.go"): "package pkg",
					filepath.Join(ctx.RootDir, "pkg", "draft_normal.go"):      "package pkg",
				}

				for path, content := range files {
					if err := fs.WriteString(path, content); err != nil {
						return err
					}
				}
				return nil
			}),
			harness.NewStep("Create rules with invalid regex @find directive", func(ctx *harness.Context) error {
				// "[unbal" is an invalid regex and invalid glob, forcing fallback to substring matching
				rulesContent := `pkg/**/*.go @find: "[unbal"`
				rulesPath := filepath.Join(ctx.RootDir, ".grove", "rules")
				return fs.WriteString(rulesPath, rulesContent)
			}),
			harness.NewStep("Verify substring fallback works", func(ctx *harness.Context) error {
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

				if !strings.Contains(output, "draft_[unbalanced.go") {
					return fmt.Errorf("output should contain draft_[unbalanced.go")
				}
				if strings.Contains(output, "draft_normal.go") {
					return fmt.Errorf("output should not contain draft_normal.go")
				}

				return nil
			}),
		},
	}
}

// FindDirectiveFullPathRegexScenario tests the @find directive with full path regex matches
func FindDirectiveFullPathRegexScenario() *harness.Scenario {
	return &harness.Scenario{
		Name:        "cx-find-directive-fullpath-regex",
		Description: "Tests @find directive regex matching against full paths",
		Tags:        []string{"cx", "search-directives"},
		Steps: []harness.Step{
			harness.NewStep("Setup test project", func(ctx *harness.Context) error {
				dirs := []string{
					filepath.Join(ctx.RootDir, "pkg", "auth"),
					filepath.Join(ctx.RootDir, "pkg", "billing"),
				}
				for _, dir := range dirs {
					if err := os.MkdirAll(dir, 0o755); err != nil {
						return err
					}
				}

				files := map[string]string{
					filepath.Join(ctx.RootDir, "pkg", "auth", "api_handler.go"):    "package auth",
					filepath.Join(ctx.RootDir, "pkg", "billing", "api_handler.go"): "package billing",
				}

				for path, content := range files {
					if err := fs.WriteString(path, content); err != nil {
						return err
					}
				}
				return nil
			}),
			harness.NewStep("Create rules with full path regex directive", func(ctx *harness.Context) error {
				// Regex matches the auth path segment but not billing; no ^ anchor since paths are absolute
				rulesContent := "pkg/**/*.go @find: \"pkg/auth/.*_handler\\.go$\""
				rulesPath := filepath.Join(ctx.RootDir, ".grove", "rules")
				return fs.WriteString(rulesPath, rulesContent)
			}),
			harness.NewStep("Verify regex path matching", func(ctx *harness.Context) error {
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

				if !strings.Contains(output, "auth/api_handler.go") {
					return fmt.Errorf("output should contain auth/api_handler.go")
				}
				if strings.Contains(output, "billing/api_handler.go") {
					return fmt.Errorf("output should not contain billing/api_handler.go")
				}

				return nil
			}),
		},
	}
}

// GrepDirectiveScenario tests the @grep directive for filtering by file content
func GrepDirectiveScenario() *harness.Scenario {
	return &harness.Scenario{
		Name:        "cx-grep-directive",
		Description: "Tests @grep directive for filtering files by content",
		Tags:        []string{"cx", "search-directives"},
		Steps: []harness.Step{
			harness.NewStep("Setup test project with files containing different content", func(ctx *harness.Context) error {
				// Create directories
				dirs := []string{
					filepath.Join(ctx.RootDir, "pkg", "managers"),
					filepath.Join(ctx.RootDir, "pkg", "api"),
				}

				for _, dir := range dirs {
					if err := os.MkdirAll(dir, 0o755); err != nil {
						return err
					}
				}

				// Create files - some containing "UserManager" in the content
				files := map[string]string{
					filepath.Join(ctx.RootDir, "pkg", "managers", "user.go"): "package managers\n\ntype UserManager struct {\n\tID int\n}",
					filepath.Join(ctx.RootDir, "pkg", "managers", "file.go"): "package managers\n\ntype FileHandler struct {\n\tPath string\n}",
					filepath.Join(ctx.RootDir, "pkg", "api", "user_api.go"):  "package api\n\nimport \"myapp/pkg/managers\"\n\nfunc GetUser() *managers.UserManager {\n\treturn nil\n}",
					filepath.Join(ctx.RootDir, "pkg", "api", "file_api.go"):  "package api\n\nfunc GetFile() string {\n\treturn \"\"\n}",
				}

				for path, content := range files {
					if err := fs.WriteString(path, content); err != nil {
						return err
					}
				}

				return nil
			}),
			harness.NewStep("Create .grove/rules with @grep directive", func(ctx *harness.Context) error {
				// Only include Go files that contain "UserManager" in their content
				rulesContent := "pkg/**/*.go @grep: \"UserManager\""
				rulesPath := filepath.Join(ctx.RootDir, ".grove", "rules")
				return fs.WriteString(rulesPath, rulesContent)
			}),
			harness.NewStep("Verify only files containing UserManager are included", func(ctx *harness.Context) error {
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

				// Verify files with UserManager in content are included
				if !strings.Contains(output, "pkg/managers/user.go") {
					return fmt.Errorf("output should contain pkg/managers/user.go")
				}
				if !strings.Contains(output, "pkg/api/user_api.go") {
					return fmt.Errorf("output should contain pkg/api/user_api.go")
				}

				// Verify files without UserManager are NOT included
				if strings.Contains(output, "file.go") {
					return fmt.Errorf("output should not contain file.go")
				}
				if strings.Contains(output, "file_api.go") {
					return fmt.Errorf("output should not contain file_api.go")
				}

				return nil
			}),
		},
	}
}

// RecentDirectiveScenario tests the @recent directive for filtering by modification time
func RecentDirectiveScenario() *harness.Scenario {
	return &harness.Scenario{
		Name:        "cx-recent-directive",
		Description: "Tests @recent directive for filtering files by modification time",
		Tags:        []string{"cx", "search-directives"},
		Steps: []harness.Step{
			harness.NewStep("Setup test project with old and new files", func(ctx *harness.Context) error {
				// Create directories
				dirs := []string{
					filepath.Join(ctx.RootDir, "pkg", "new"),
					filepath.Join(ctx.RootDir, "pkg", "old"),
				}

				for _, dir := range dirs {
					if err := os.MkdirAll(dir, 0o755); err != nil {
						return err
					}
				}

				// Create files
				newFilePath := filepath.Join(ctx.RootDir, "pkg", "new", "new_file.go")
				oldFilePath := filepath.Join(ctx.RootDir, "pkg", "old", "old_file.go")

				if err := fs.WriteString(newFilePath, "package new\n"); err != nil {
					return err
				}
				if err := fs.WriteString(oldFilePath, "package old\n"); err != nil {
					return err
				}

				// Change modification time of old file to 10 days ago
				tenDaysAgo := time.Now().Add(-10 * 24 * time.Hour)
				if err := os.Chtimes(oldFilePath, tenDaysAgo, tenDaysAgo); err != nil {
					return err
				}

				return nil
			}),
			harness.NewStep("Create .grove/rules with @recent directive", func(ctx *harness.Context) error {
				// Only include Go files modified in the last 7 days
				rulesContent := "pkg/**/*.go @recent: 7d"
				rulesPath := filepath.Join(ctx.RootDir, ".grove", "rules")
				return fs.WriteString(rulesPath, rulesContent)
			}),
			harness.NewStep("Verify only recent files are included", func(ctx *harness.Context) error {
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

				// Verify new files are included
				if !strings.Contains(output, "new_file.go") {
					return fmt.Errorf("output should contain new_file.go but got:\n%s", output)
				}

				// Verify old files are NOT included
				if strings.Contains(output, "old_file.go") {
					return fmt.Errorf("output should not contain old_file.go but got:\n%s", output)
				}

				return nil
			}),
		},
	}
}

// GlobalRecentDirectiveScenario tests that a standalone @recent: directive applies to all patterns below it
func GlobalRecentDirectiveScenario() *harness.Scenario {
	return &harness.Scenario{
		Name:        "cx-global-recent-directive",
		Description: "Tests global @recent: directive applying to all patterns below it",
		Tags:        []string{"cx", "search-directives"},
		Steps: []harness.Step{
			harness.NewStep("Setup workspace with old and new files", func(ctx *harness.Context) error {
				dirs := []string{
					filepath.Join(ctx.RootDir, "src"),
					filepath.Join(ctx.RootDir, "docs"),
				}
				for _, dir := range dirs {
					if err := os.MkdirAll(dir, 0o755); err != nil {
						return err
					}
				}

				// Create files
				files := map[string]string{
					filepath.Join(ctx.RootDir, "src", "a.go"):  "package src\n",
					filepath.Join(ctx.RootDir, "src", "b.go"):  "package src\n",
					filepath.Join(ctx.RootDir, "docs", "c.md"): "# Doc\n",
				}
				for path, content := range files {
					if err := fs.WriteString(path, content); err != nil {
						return err
					}
				}

				// Backdate b.go and c.md to 10 days ago
				tenDaysAgo := time.Now().Add(-10 * 24 * time.Hour)
				for _, path := range []string{
					filepath.Join(ctx.RootDir, "src", "b.go"),
					filepath.Join(ctx.RootDir, "docs", "c.md"),
				} {
					if err := os.Chtimes(path, tenDaysAgo, tenDaysAgo); err != nil {
						return err
					}
				}

				return nil
			}),
			harness.NewStep("Create rules with global @recent directive", func(ctx *harness.Context) error {
				rulesContent := "@recent: 3d\nsrc/**/*.go\ndocs/**/*.md"
				rulesPath := filepath.Join(ctx.RootDir, ".grove", "rules")
				return fs.WriteString(rulesPath, rulesContent)
			}),
			harness.NewStep("Verify only recent files are included", func(ctx *harness.Context) error {
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

				if !strings.Contains(output, "a.go") {
					return fmt.Errorf("output should contain a.go but got:\n%s", output)
				}
				if strings.Contains(output, "b.go") {
					return fmt.Errorf("output should not contain b.go but got:\n%s", output)
				}
				if strings.Contains(output, "c.md") {
					return fmt.Errorf("output should not contain c.md but got:\n%s", output)
				}

				return nil
			}),
		},
	}
}

// RecentTimeUnitsScenario tests inline @recent: with different time units (w, h, quoted)
func RecentTimeUnitsScenario() *harness.Scenario {
	return &harness.Scenario{
		Name:        "cx-recent-time-units",
		Description: "Tests @recent: directive with weeks, hours, and quoted values",
		Tags:        []string{"cx", "search-directives"},
		Steps: []harness.Step{
			harness.NewStep("Setup files with various ages in subdirectories", func(ctx *harness.Context) error {
				// Each file goes in its own subdirectory so we can use glob patterns
				// (literal file paths bypass directive filtering)
				dirs := []string{"hours", "days", "olddays", "weeks"}
				for _, d := range dirs {
					if err := os.MkdirAll(filepath.Join(ctx.RootDir, d), 0o755); err != nil {
						return err
					}
				}

				files := map[string]string{
					filepath.Join(ctx.RootDir, "hours", "recent.txt"):   "recent\n",
					filepath.Join(ctx.RootDir, "days", "recent.txt"):    "somewhat old\n",
					filepath.Join(ctx.RootDir, "olddays", "recent.txt"): "old\n",
					filepath.Join(ctx.RootDir, "weeks", "recent.txt"):   "very old\n",
				}
				for path, content := range files {
					if err := fs.WriteString(path, content); err != nil {
						return err
					}
				}

				// Set modification times
				now := time.Now()
				ages := map[string]time.Duration{
					"hours/recent.txt":   1 * time.Hour,
					"days/recent.txt":    3 * 24 * time.Hour,
					"olddays/recent.txt": 10 * 24 * time.Hour,
					"weeks/recent.txt":   21 * 24 * time.Hour,
				}
				for name, age := range ages {
					path := filepath.Join(ctx.RootDir, name)
					t := now.Add(-age)
					if err := os.Chtimes(path, t, t); err != nil {
						return err
					}
				}

				return nil
			}),
			harness.NewStep("Create rules with different time units", func(ctx *harness.Context) error {
				rulesContent := `hours/**/*.txt @recent: 24h
days/**/*.txt @recent: 1w
olddays/**/*.txt @recent: "2w"
weeks/**/*.txt @recent: 2w`
				rulesPath := filepath.Join(ctx.RootDir, ".grove", "rules")
				return fs.WriteString(rulesPath, rulesContent)
			}),
			harness.NewStep("Verify correct filtering by time unit", func(ctx *harness.Context) error {
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

				// hours/recent.txt: 1h old, threshold 24h -> included
				if !strings.Contains(output, "hours") {
					return fmt.Errorf("output should contain hours/recent.txt but got:\n%s", output)
				}
				// days/recent.txt: 3d old, threshold 1w -> included
				if !strings.Contains(output, "days/recent.txt") || !strings.Contains(output, filepath.Join("days", "recent.txt")) {
					return fmt.Errorf("output should contain days/recent.txt but got:\n%s", output)
				}
				// olddays/recent.txt: 10d old, threshold "2w" (14d) -> included
				if !strings.Contains(output, "olddays") {
					return fmt.Errorf("output should contain olddays/recent.txt but got:\n%s", output)
				}
				// weeks/recent.txt: 21d old, threshold 2w (14d) -> excluded
				if strings.Contains(output, filepath.Join("weeks", "recent.txt")) {
					return fmt.Errorf("output should not contain weeks/recent.txt but got:\n%s", output)
				}

				return nil
			}),
		},
	}
}

// RecentCombinedDirectivesScenario tests @recent: alongside @grep: in the same rules file
func RecentCombinedDirectivesScenario() *harness.Scenario {
	return &harness.Scenario{
		Name:        "cx-combined-recent-directives",
		Description: "Tests combining @recent: with @grep: across different patterns",
		Tags:        []string{"cx", "search-directives"},
		Steps: []harness.Step{
			harness.NewStep("Setup workspace", func(ctx *harness.Context) error {
				dirs := []string{
					filepath.Join(ctx.RootDir, "src"),
					filepath.Join(ctx.RootDir, "config"),
				}
				for _, dir := range dirs {
					if err := os.MkdirAll(dir, 0o755); err != nil {
						return err
					}
				}

				files := map[string]string{
					filepath.Join(ctx.RootDir, "src", "manager.go"):          "package src\n\ntype Manager struct{}",
					filepath.Join(ctx.RootDir, "src", "old_manager.go"):      "package src\n\ntype OldManager struct{}",
					filepath.Join(ctx.RootDir, "config", "secrets.yaml"):     "password: secret123",
					filepath.Join(ctx.RootDir, "config", "old_secrets.yaml"): "password: oldsecret",
				}
				for path, content := range files {
					if err := fs.WriteString(path, content); err != nil {
						return err
					}
				}

				// Backdate old files
				tenDaysAgo := time.Now().Add(-10 * 24 * time.Hour)
				for _, name := range []string{"src/old_manager.go", "config/old_secrets.yaml"} {
					path := filepath.Join(ctx.RootDir, name)
					if err := os.Chtimes(path, tenDaysAgo, tenDaysAgo); err != nil {
						return err
					}
				}

				return nil
			}),
			harness.NewStep("Create rules with @recent and @grep", func(ctx *harness.Context) error {
				rulesContent := "src/**/*.go @recent: 7d\nconfig/**/*.yaml @grep: \"password\""
				rulesPath := filepath.Join(ctx.RootDir, ".grove", "rules")
				return fs.WriteString(rulesPath, rulesContent)
			}),
			harness.NewStep("Verify correct filtering", func(ctx *harness.Context) error {
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

				// manager.go: recent, included by @recent: 7d
				if !strings.Contains(output, "src/manager.go") {
					return fmt.Errorf("output should contain src/manager.go but got:\n%s", output)
				}
				// old_manager.go: 10d old, excluded by @recent: 7d
				if strings.Contains(output, "old_manager.go") {
					return fmt.Errorf("output should not contain old_manager.go but got:\n%s", output)
				}
				// Both yaml files contain "password", included by @grep
				if !strings.Contains(output, "secrets.yaml") {
					return fmt.Errorf("output should contain secrets.yaml but got:\n%s", output)
				}
				if !strings.Contains(output, "old_secrets.yaml") {
					return fmt.Errorf("output should contain old_secrets.yaml but got:\n%s", output)
				}

				return nil
			}),
		},
	}
}

// RecentInvalidDurationScenario tests that invalid @recent: duration results in no matches
func RecentInvalidDurationScenario() *harness.Scenario {
	return &harness.Scenario{
		Name:        "cx-recent-invalid-duration",
		Description: "Tests that invalid @recent: duration produces no matches",
		Tags:        []string{"cx", "search-directives"},
		Steps: []harness.Step{
			harness.NewStep("Setup workspace with a file", func(ctx *harness.Context) error {
				dir := filepath.Join(ctx.RootDir, "src")
				if err := os.MkdirAll(dir, 0o755); err != nil {
					return err
				}
				return fs.WriteString(filepath.Join(dir, "main.go"), "package main\n")
			}),
			harness.NewStep("Create rules with invalid duration", func(ctx *harness.Context) error {
				rulesContent := "src/**/*.go @recent: xyz"
				rulesPath := filepath.Join(ctx.RootDir, ".grove", "rules")
				return fs.WriteString(rulesPath, rulesContent)
			}),
			harness.NewStep("Verify no files matched with invalid duration", func(ctx *harness.Context) error {
				cxBinary, err := FindProjectBinary()
				if err != nil {
					return err
				}

				cmd := command.New(cxBinary, "list").Dir(ctx.RootDir)
				result := cmd.Run()
				ctx.ShowCommandOutput(cmd.String(), result.Stdout, result.Stderr)

				// Invalid duration silently fails the directive filter,
				// so the file should not appear in output
				output := strings.TrimSpace(result.Stdout)
				if strings.Contains(output, "main.go") {
					return fmt.Errorf("output should not contain main.go when duration is invalid, got:\n%s", output)
				}

				return nil
			}),
		},
	}
}

// AliasWithDirectiveScenario tests combining aliases with search directives
func AliasWithDirectiveScenario() *harness.Scenario {
	return &harness.Scenario{
		Name:        "cx-alias-with-directive",
		Description: "Tests combining @alias with @grep directive to filter aliased files",
		Tags:        []string{"cx", "search-directives", "alias"},
		Steps: []harness.Step{
			harness.NewStep("Setup ecosystem with library project", func(ctx *harness.Context) error {
				// Create groves directory inside test root for isolation
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

				// Create library project with multiple files
				libAlphaDir := filepath.Join(grovesDir, "lib-alpha")

				// File containing "UserManager"
				if err := fs.WriteString(filepath.Join(libAlphaDir, "user_manager.go"), "package alpha\n\ntype UserManager struct {\n\tID int\n}"); err != nil {
					return err
				}

				// File NOT containing "UserManager"
				if err := fs.WriteString(filepath.Join(libAlphaDir, "other_file.go"), "package alpha\n\ntype OtherStruct struct {\n\tName string\n}"); err != nil {
					return err
				}

				// Another file containing "UserManager"
				if err := fs.WriteString(filepath.Join(libAlphaDir, "user_api.go"), "package alpha\n\nimport \"lib/managers\"\n\nfunc GetUser() *managers.UserManager {\n\treturn nil\n}"); err != nil {
					return err
				}

				if err := fs.WriteString(filepath.Join(libAlphaDir, "grove.yml"), `name: lib-alpha`); err != nil {
					return err
				}

				// Initialize as git repo
				if result := command.New("git", "init").Dir(libAlphaDir).Run(); result.Error != nil {
					return fmt.Errorf("failed to init git in lib-alpha: %w", result.Error)
				}

				// Create main project
				if err := fs.WriteString(filepath.Join(ctx.RootDir, "grove.yml"), `name: test-main`); err != nil {
					return err
				}

				return nil
			}),
			harness.NewStep("Create .grove/rules with alias and grep directive", func(ctx *harness.Context) error {
				// Combine alias with grep directive - need to include a glob pattern with the alias
				rulesContent := `@alias:lib-alpha/**/*.go @grep: "UserManager"`
				rulesPath := filepath.Join(ctx.RootDir, ".grove", "rules")
				return fs.WriteString(rulesPath, rulesContent)
			}),
			harness.NewStep("Verify only files containing UserManager are included", func(ctx *harness.Context) error {
				cxBinary, err := FindProjectBinary()
				if err != nil {
					return err
				}

				cmd := ctx.Command(cxBinary, "list").Dir(ctx.RootDir)
				result := cmd.Run()

				ctx.ShowCommandOutput(cmd.String(), result.Stdout, result.Stderr)

				if result.Error != nil {
					return result.Error
				}

				output := result.Stdout

				// Verify files with UserManager in content are included
				if !strings.Contains(output, "user_manager.go") {
					return fmt.Errorf("output should contain user_manager.go")
				}
				if !strings.Contains(output, "user_api.go") {
					return fmt.Errorf("output should contain user_api.go")
				}

				// Verify file without UserManager is NOT included
				if strings.Contains(output, "other_file.go") {
					return fmt.Errorf("output should not contain other_file.go")
				}

				return nil
			}),
		},
	}
}

// FindInvertedDirectiveScenario tests the @find!: directive for excluding by filename
func FindInvertedDirectiveScenario() *harness.Scenario {
	return &harness.Scenario{
		Name:        "cx-find-inverted-directive",
		Description: "Tests @find!: directive for excluding files by filename/path",
		Tags:        []string{"cx", "search-directives"},
		Steps: []harness.Step{
			harness.NewStep("Setup test project with various files", func(ctx *harness.Context) error {
				dirs := []string{
					filepath.Join(ctx.RootDir, "pkg", "managers"),
					filepath.Join(ctx.RootDir, "pkg", "api"),
				}
				for _, dir := range dirs {
					if err := os.MkdirAll(dir, 0o755); err != nil {
						return err
					}
				}

				files := map[string]string{
					filepath.Join(ctx.RootDir, "pkg", "managers", "user_manager.go"): "package managers",
					filepath.Join(ctx.RootDir, "pkg", "managers", "file_manager.go"): "package managers",
					filepath.Join(ctx.RootDir, "pkg", "api", "user_api.go"):          "package api",
					filepath.Join(ctx.RootDir, "pkg", "api", "file_api.go"):          "package api",
				}
				for path, content := range files {
					if err := fs.WriteString(path, content); err != nil {
						return err
					}
				}
				return nil
			}),
			harness.NewStep("Create .grove/rules with @find!: directive", func(ctx *harness.Context) error {
				rulesContent := "pkg/**/*.go @find!: \"manager\""
				rulesPath := filepath.Join(ctx.RootDir, ".grove", "rules")
				return fs.WriteString(rulesPath, rulesContent)
			}),
			harness.NewStep("Verify manager files are excluded", func(ctx *harness.Context) error {
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

				// Verify non-manager files ARE included
				if !strings.Contains(output, "user_api.go") || !strings.Contains(output, "file_api.go") {
					return fmt.Errorf("output should contain user_api.go and file_api.go, got: %s", output)
				}

				// Verify manager files are NOT included
				if strings.Contains(output, "user_manager.go") || strings.Contains(output, "file_manager.go") {
					return fmt.Errorf("output should not contain manager files, got: %s", output)
				}

				return nil
			}),
		},
	}
}

// GrepInvertedDirectiveScenario tests the @grep!: directive for excluding by file content
func GrepInvertedDirectiveScenario() *harness.Scenario {
	return &harness.Scenario{
		Name:        "cx-grep-inverted-directive",
		Description: "Tests @grep!: directive for excluding files by content",
		Tags:        []string{"cx", "search-directives"},
		Steps: []harness.Step{
			harness.NewStep("Setup test project with files containing different content", func(ctx *harness.Context) error {
				dirs := []string{filepath.Join(ctx.RootDir, "pkg", "api")}
				for _, dir := range dirs {
					if err := os.MkdirAll(dir, 0o755); err != nil {
						return err
					}
				}

				files := map[string]string{
					filepath.Join(ctx.RootDir, "pkg", "api", "generated.go"): "package api\n// Code generated by tool. DO NOT EDIT.\ntype Gen struct{}",
					filepath.Join(ctx.RootDir, "pkg", "api", "manual.go"):    "package api\n\ntype Manual struct{}",
				}
				for path, content := range files {
					if err := fs.WriteString(path, content); err != nil {
						return err
					}
				}
				return nil
			}),
			harness.NewStep("Create .grove/rules with @grep!: directive", func(ctx *harness.Context) error {
				rulesContent := "pkg/**/*.go @grep!: \"generated\""
				rulesPath := filepath.Join(ctx.RootDir, ".grove", "rules")
				return fs.WriteString(rulesPath, rulesContent)
			}),
			harness.NewStep("Verify files containing 'generated' are excluded", func(ctx *harness.Context) error {
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

				if !strings.Contains(output, "manual.go") {
					return fmt.Errorf("output should contain manual.go, got: %s", output)
				}
				if strings.Contains(output, "generated.go") {
					return fmt.Errorf("output should not contain generated.go, got: %s", output)
				}

				return nil
			}),
		},
	}
}

// GlobalFindInvertedDirectiveScenario tests standalone @find!: on its own line
func GlobalFindInvertedDirectiveScenario() *harness.Scenario {
	return &harness.Scenario{
		Name:        "cx-global-find-inverted-directive",
		Description: "Tests standalone @find!: directive applied to all patterns below it",
		Tags:        []string{"cx", "search-directives"},
		Steps: []harness.Step{
			harness.NewStep("Setup test project", func(ctx *harness.Context) error {
				files := map[string]string{
					filepath.Join(ctx.RootDir, "src", "user_test.go"):   "package src",
					filepath.Join(ctx.RootDir, "src", "user.go"):        "package src",
					filepath.Join(ctx.RootDir, "lib", "helper_test.go"): "package lib",
					filepath.Join(ctx.RootDir, "lib", "helper.go"):      "package lib",
				}
				for path, content := range files {
					if err := fs.WriteString(path, content); err != nil {
						return err
					}
				}
				return nil
			}),
			harness.NewStep("Create rules with global @find!: directive", func(ctx *harness.Context) error {
				// Global directive excludes test files from all patterns below
				rulesContent := `@find!: "_test"
src/**/*.go
lib/**/*.go`
				return fs.WriteString(filepath.Join(ctx.RootDir, ".grove", "rules"), rulesContent)
			}),
			harness.NewStep("Verify test files are excluded from all patterns", func(ctx *harness.Context) error {
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

				// Non-test files should be included
				if !strings.Contains(output, "user.go") {
					return fmt.Errorf("output should contain user.go, got: %s", output)
				}
				if !strings.Contains(output, "helper.go") {
					return fmt.Errorf("output should contain helper.go, got: %s", output)
				}
				// Test files should be excluded
				if strings.Contains(output, "user_test.go") {
					return fmt.Errorf("output should not contain user_test.go, got: %s", output)
				}
				if strings.Contains(output, "helper_test.go") {
					return fmt.Errorf("output should not contain helper_test.go, got: %s", output)
				}
				return nil
			}),
		},
	}
}

// GlobalGrepInvertedDirectiveScenario tests standalone @grep!: on its own line
func GlobalGrepInvertedDirectiveScenario() *harness.Scenario {
	return &harness.Scenario{
		Name:        "cx-global-grep-inverted-directive",
		Description: "Tests standalone @grep!: directive applied to all patterns below it",
		Tags:        []string{"cx", "search-directives"},
		Steps: []harness.Step{
			harness.NewStep("Setup test project", func(ctx *harness.Context) error {
				files := map[string]string{
					filepath.Join(ctx.RootDir, "src", "auto.go"):   "package src\n// Code generated by tool. DO NOT EDIT.",
					filepath.Join(ctx.RootDir, "src", "manual.go"): "package src\n\nfunc DoWork() {}",
					filepath.Join(ctx.RootDir, "lib", "gen.go"):    "package lib\n// Code generated by tool. DO NOT EDIT.",
					filepath.Join(ctx.RootDir, "lib", "hand.go"):   "package lib\n\nfunc Help() {}",
				}
				for path, content := range files {
					if err := fs.WriteString(path, content); err != nil {
						return err
					}
				}
				return nil
			}),
			harness.NewStep("Create rules with global @grep!: directive", func(ctx *harness.Context) error {
				rulesContent := `@grep!: "Code generated"
src/**/*.go
lib/**/*.go`
				return fs.WriteString(filepath.Join(ctx.RootDir, ".grove", "rules"), rulesContent)
			}),
			harness.NewStep("Verify generated files are excluded from all patterns", func(ctx *harness.Context) error {
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

				if !strings.Contains(output, "manual.go") {
					return fmt.Errorf("output should contain manual.go, got: %s", output)
				}
				if !strings.Contains(output, "hand.go") {
					return fmt.Errorf("output should contain hand.go, got: %s", output)
				}
				if strings.Contains(output, "auto.go") {
					return fmt.Errorf("output should not contain auto.go, got: %s", output)
				}
				if strings.Contains(output, "gen.go") {
					return fmt.Errorf("output should not contain gen.go, got: %s", output)
				}
				return nil
			}),
		},
	}
}

// CombinedInvertedNormalDirectivesScenario tests mixing inverted and normal directives
func CombinedInvertedNormalDirectivesScenario() *harness.Scenario {
	return &harness.Scenario{
		Name:        "cx-combined-inverted-normal-directives",
		Description: "Tests combining @find!: with @grep: in the same rules file",
		Tags:        []string{"cx", "search-directives"},
		Steps: []harness.Step{
			harness.NewStep("Setup test project", func(ctx *harness.Context) error {
				files := map[string]string{
					filepath.Join(ctx.RootDir, "pkg", "user_test.go"):    "package pkg\n\nfunc TestUser() {}",
					filepath.Join(ctx.RootDir, "pkg", "user.go"):         "package pkg\n\ntype User struct{ Name string }",
					filepath.Join(ctx.RootDir, "pkg", "admin.go"):        "package pkg\n\ntype Admin struct{ Role string }",
					filepath.Join(ctx.RootDir, "pkg", "admin_test.go"):   "package pkg\n\nfunc TestAdmin() {}",
					filepath.Join(ctx.RootDir, "config", "app.yaml"):     "database:\n  host: localhost",
					filepath.Join(ctx.RootDir, "config", "secrets.yaml"): "database:\n  password: secret123",
				}
				for path, content := range files {
					if err := fs.WriteString(path, content); err != nil {
						return err
					}
				}
				return nil
			}),
			harness.NewStep("Create rules with mixed directives", func(ctx *harness.Context) error {
				// Exclude test files by name, include only yaml files containing "password"
				rulesContent := `pkg/**/*.go @find!: "_test"
config/**/*.yaml @grep: "password"`
				return fs.WriteString(filepath.Join(ctx.RootDir, ".grove", "rules"), rulesContent)
			}),
			harness.NewStep("Verify correct filtering", func(ctx *harness.Context) error {
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

				// Non-test Go files should be included
				if !strings.Contains(output, "user.go") {
					return fmt.Errorf("output should contain user.go, got: %s", output)
				}
				if !strings.Contains(output, "admin.go") {
					return fmt.Errorf("output should contain admin.go, got: %s", output)
				}
				// Test files excluded by @find!:
				if strings.Contains(output, "user_test.go") {
					return fmt.Errorf("output should not contain user_test.go, got: %s", output)
				}
				if strings.Contains(output, "admin_test.go") {
					return fmt.Errorf("output should not contain admin_test.go, got: %s", output)
				}
				// Only secrets.yaml included by @grep:
				if !strings.Contains(output, "secrets.yaml") {
					return fmt.Errorf("output should contain secrets.yaml, got: %s", output)
				}
				if strings.Contains(output, "app.yaml") {
					return fmt.Errorf("output should not contain app.yaml, got: %s", output)
				}
				return nil
			}),
		},
	}
}

// AliasWithInvertedDirectiveScenario tests combining aliases with inverted directives
func AliasWithInvertedDirectiveScenario() *harness.Scenario {
	return &harness.Scenario{
		Name:        "cx-alias-with-inverted-directive",
		Description: "Tests combining @alias with @find!: directive to filter aliased files",
		Tags:        []string{"cx", "search-directives", "alias"},
		Steps: []harness.Step{
			harness.NewStep("Setup ecosystem with library project", func(ctx *harness.Context) error {
				grovesDir := filepath.Join(ctx.RootDir, "mock-groves")
				groveConfigDir := filepath.Join(ctx.ConfigDir(), "grove")

				groveConfig := fmt.Sprintf(`groves:
  test:
    path: %s
    enabled: true
`, grovesDir)
				if err := fs.WriteString(filepath.Join(groveConfigDir, "grove.yml"), groveConfig); err != nil {
					return err
				}

				libDir := filepath.Join(grovesDir, "lib-beta")

				files := map[string]string{
					filepath.Join(libDir, "user.go"):      "package beta\n\ntype User struct{}",
					filepath.Join(libDir, "user_test.go"): "package beta\n\nfunc TestUser() {}",
					filepath.Join(libDir, "api.go"):       "package beta\n\ntype API struct{}",
					filepath.Join(libDir, "api_test.go"):  "package beta\n\nfunc TestAPI() {}",
				}
				for path, content := range files {
					if err := fs.WriteString(path, content); err != nil {
						return err
					}
				}

				if err := fs.WriteString(filepath.Join(libDir, "grove.yml"), `name: lib-beta`); err != nil {
					return err
				}
				if result := command.New("git", "init").Dir(libDir).Run(); result.Error != nil {
					return fmt.Errorf("failed to init git in lib-beta: %w", result.Error)
				}

				if err := fs.WriteString(filepath.Join(ctx.RootDir, "grove.yml"), `name: test-alias-inv`); err != nil {
					return err
				}

				return nil
			}),
			harness.NewStep("Create rules with alias and @find!: directive", func(ctx *harness.Context) error {
				rulesContent := `@alias:lib-beta/**/*.go @find!: "_test"`
				return fs.WriteString(filepath.Join(ctx.RootDir, ".grove", "rules"), rulesContent)
			}),
			harness.NewStep("Verify test files are excluded from aliased project", func(ctx *harness.Context) error {
				cxBinary, err := FindProjectBinary()
				if err != nil {
					return err
				}

				cmd := ctx.Command(cxBinary, "list").Dir(ctx.RootDir)
				result := cmd.Run()
				ctx.ShowCommandOutput(cmd.String(), result.Stdout, result.Stderr)
				if result.Error != nil {
					return result.Error
				}

				output := result.Stdout

				if !strings.Contains(output, "user.go") {
					return fmt.Errorf("output should contain user.go, got: %s", output)
				}
				if !strings.Contains(output, "api.go") {
					return fmt.Errorf("output should contain api.go, got: %s", output)
				}
				if strings.Contains(output, "user_test.go") {
					return fmt.Errorf("output should not contain user_test.go, got: %s", output)
				}
				if strings.Contains(output, "api_test.go") {
					return fmt.Errorf("output should not contain api_test.go, got: %s", output)
				}
				return nil
			}),
		},
	}
}

// FindInvertedAllExcludedScenario tests that @find!: works when all files match the exclusion
func FindInvertedAllExcludedScenario() *harness.Scenario {
	return &harness.Scenario{
		Name:        "cx-find-inverted-all-excluded",
		Description: "Tests @find!: edge case where all files match the exclusion pattern",
		Tags:        []string{"cx", "search-directives", "edge-case"},
		Steps: []harness.Step{
			harness.NewStep("Setup project where all files match the exclusion", func(ctx *harness.Context) error {
				files := map[string]string{
					filepath.Join(ctx.RootDir, "src", "user_test.go"):  "package src",
					filepath.Join(ctx.RootDir, "src", "admin_test.go"): "package src",
				}
				for path, content := range files {
					if err := fs.WriteString(path, content); err != nil {
						return err
					}
				}
				return nil
			}),
			harness.NewStep("Create rules that exclude all matched files", func(ctx *harness.Context) error {
				rulesContent := `src/**/*.go @find!: "_test"`
				return fs.WriteString(filepath.Join(ctx.RootDir, ".grove", "rules"), rulesContent)
			}),
			harness.NewStep("Verify empty result", func(ctx *harness.Context) error {
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

				output := strings.TrimSpace(result.Stdout)

				if strings.Contains(output, "_test.go") {
					return fmt.Errorf("output should not contain any test files, got: %s", output)
				}
				return nil
			}),
		},
	}
}

// FindInvertedNoneExcludedScenario tests that @find!: works when no files match the exclusion
func FindInvertedNoneExcludedScenario() *harness.Scenario {
	return &harness.Scenario{
		Name:        "cx-find-inverted-none-excluded",
		Description: "Tests @find!: edge case where no files match the exclusion (all pass through)",
		Tags:        []string{"cx", "search-directives", "edge-case"},
		Steps: []harness.Step{
			harness.NewStep("Setup project where no files match exclusion", func(ctx *harness.Context) error {
				files := map[string]string{
					filepath.Join(ctx.RootDir, "src", "user.go"):  "package src",
					filepath.Join(ctx.RootDir, "src", "admin.go"): "package src",
				}
				for path, content := range files {
					if err := fs.WriteString(path, content); err != nil {
						return err
					}
				}
				return nil
			}),
			harness.NewStep("Create rules with exclusion that matches nothing", func(ctx *harness.Context) error {
				rulesContent := `src/**/*.go @find!: "nonexistent_pattern"`
				return fs.WriteString(filepath.Join(ctx.RootDir, ".grove", "rules"), rulesContent)
			}),
			harness.NewStep("Verify all files pass through", func(ctx *harness.Context) error {
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

				if !strings.Contains(output, "user.go") {
					return fmt.Errorf("output should contain user.go, got: %s", output)
				}
				if !strings.Contains(output, "admin.go") {
					return fmt.Errorf("output should contain admin.go, got: %s", output)
				}
				return nil
			}),
		},
	}
}

// CombinedFindAndGrepDirectiveScenario tests combining @find and @grep on the same rule line (AND logic)
func CombinedFindAndGrepDirectiveScenario() *harness.Scenario {
	return &harness.Scenario{
		Name:        "cx-combined-find-and-grep-directive",
		Description: "Tests combining @find and @grep directives on the same rule to AND-filter files",
		Tags:        []string{"cx", "search-directives"},
		Steps: []harness.Step{
			harness.NewStep("Setup test project", func(ctx *harness.Context) error {
				dirs := []string{
					filepath.Join(ctx.RootDir, "pkg", "api"),
					filepath.Join(ctx.RootDir, "pkg", "core"),
				}
				for _, dir := range dirs {
					if err := os.MkdirAll(dir, 0o755); err != nil {
						return err
					}
				}

				files := map[string]string{
					// Matches both find ("api") and grep ("User")
					filepath.Join(ctx.RootDir, "pkg", "api", "user_api.go"): "package api\n\ntype UserAPI struct {}",
					// Matches find ("api") but NOT grep ("User")
					filepath.Join(ctx.RootDir, "pkg", "api", "system_api.go"): "package api\n\ntype SystemAPI struct {}",
					// Matches grep ("User") but NOT find ("api")
					filepath.Join(ctx.RootDir, "pkg", "core", "user_core.go"): "package core\n\ntype UserCore struct {}",
					// Matches neither
					filepath.Join(ctx.RootDir, "pkg", "core", "system_core.go"): "package core\n\ntype SystemCore struct {}",
				}

				for path, content := range files {
					if err := fs.WriteString(path, content); err != nil {
						return err
					}
				}
				return nil
			}),
			harness.NewStep("Create rules with both @find and @grep on same line", func(ctx *harness.Context) error {
				rulesContent := `pkg/**/*.go @find: "api" @grep: "User"`
				return fs.WriteString(filepath.Join(ctx.RootDir, ".grove", "rules"), rulesContent)
			}),
			harness.NewStep("Verify AND logic: only files matching both directives", func(ctx *harness.Context) error {
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

				// Should ONLY include user_api.go (matches both "api" in path AND "User" in content)
				if !strings.Contains(output, "user_api.go") {
					return fmt.Errorf("output should contain user_api.go (matches both @find and @grep)")
				}
				if strings.Contains(output, "system_api.go") {
					return fmt.Errorf("output should not contain system_api.go (matches @find but not @grep)")
				}
				if strings.Contains(output, "user_core.go") {
					return fmt.Errorf("output should not contain user_core.go (matches @grep but not @find)")
				}
				if strings.Contains(output, "system_core.go") {
					return fmt.Errorf("output should not contain system_core.go (matches neither)")
				}
				return nil
			}),
		},
	}
}

// GlobalMultiDirectiveScenario tests multiple global directives acting as AND filters
func GlobalMultiDirectiveScenario() *harness.Scenario {
	return &harness.Scenario{
		Name:        "cx-global-multi-directive",
		Description: "Tests multiple global @find/@grep directives applied as AND to all patterns",
		Tags:        []string{"cx", "search-directives"},
		Steps: []harness.Step{
			harness.NewStep("Setup test project", func(ctx *harness.Context) error {
				dirs := []string{
					filepath.Join(ctx.RootDir, "pkg", "api"),
					filepath.Join(ctx.RootDir, "pkg", "core"),
				}
				for _, dir := range dirs {
					if err := os.MkdirAll(dir, 0o755); err != nil {
						return err
					}
				}

				files := map[string]string{
					filepath.Join(ctx.RootDir, "pkg", "api", "user_api.go"):     "package api\n\ntype UserAPI struct {}",
					filepath.Join(ctx.RootDir, "pkg", "api", "system_api.go"):   "package api\n\ntype SystemAPI struct {}",
					filepath.Join(ctx.RootDir, "pkg", "core", "user_core.go"):   "package core\n\ntype UserCore struct {}",
					filepath.Join(ctx.RootDir, "pkg", "core", "system_core.go"): "package core\n\ntype SystemCore struct {}",
				}

				for path, content := range files {
					if err := fs.WriteString(path, content); err != nil {
						return err
					}
				}
				return nil
			}),
			harness.NewStep("Create rules with multiple global directives", func(ctx *harness.Context) error {
				rulesContent := "@find: \"api\"\n@grep: \"User\"\npkg/**/*.go"
				return fs.WriteString(filepath.Join(ctx.RootDir, ".grove", "rules"), rulesContent)
			}),
			harness.NewStep("Verify global AND logic", func(ctx *harness.Context) error {
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

				// Only user_api.go matches both global @find: "api" AND @grep: "User"
				if !strings.Contains(output, "user_api.go") {
					return fmt.Errorf("output should contain user_api.go")
				}
				if strings.Contains(output, "system_api.go") {
					return fmt.Errorf("output should not contain system_api.go")
				}
				if strings.Contains(output, "user_core.go") {
					return fmt.Errorf("output should not contain user_core.go")
				}
				if strings.Contains(output, "system_core.go") {
					return fmt.Errorf("output should not contain system_core.go")
				}
				return nil
			}),
		},
	}
}

// MultipleFindDirectivesScenario tests multiple @find: directives AND-ed together
func MultipleFindDirectivesScenario() *harness.Scenario {
	return &harness.Scenario{
		Name:        "cx-multiple-find-directives",
		Description: "Tests multiple @find: directives of the same type AND together",
		Tags:        []string{"cx", "search-directives"},
		Steps: []harness.Step{
			harness.NewStep("Setup test project with nested directories", func(ctx *harness.Context) error {
				dirs := []string{
					filepath.Join(ctx.RootDir, "pkg", "api", "v1"),
					filepath.Join(ctx.RootDir, "pkg", "api", "v2"),
					filepath.Join(ctx.RootDir, "pkg", "core", "v1"),
				}
				for _, dir := range dirs {
					if err := os.MkdirAll(dir, 0o755); err != nil {
						return err
					}
				}

				files := map[string]string{
					filepath.Join(ctx.RootDir, "pkg", "api", "v1", "user.go"):   "package v1\n\ntype User struct {}",
					filepath.Join(ctx.RootDir, "pkg", "api", "v2", "user.go"):   "package v2\n\ntype User struct {}",
					filepath.Join(ctx.RootDir, "pkg", "core", "v1", "user.go"):  "package v1\n\ntype User struct {}",
					filepath.Join(ctx.RootDir, "pkg", "api", "v1", "system.go"): "package v1\n\ntype System struct {}",
				}

				for path, content := range files {
					if err := fs.WriteString(path, content); err != nil {
						return err
					}
				}
				return nil
			}),
			harness.NewStep("Create rules with multiple @find directives", func(ctx *harness.Context) error {
				rulesContent := `pkg/**/*.go @find: "api" @find: "v1" @find: "user"`
				return fs.WriteString(filepath.Join(ctx.RootDir, ".grove", "rules"), rulesContent)
			}),
			harness.NewStep("Verify only files matching all three @find directives", func(ctx *harness.Context) error {
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

				// Should include only pkg/api/v1/user.go (matches all three @find directives)
				if !strings.Contains(output, "pkg/api/v1/user.go") {
					return fmt.Errorf("output should contain pkg/api/v1/user.go (matches all three @find directives)")
				}
				// Should exclude pkg/api/v2/user.go (fails @find: "v1")
				if strings.Contains(output, "pkg/api/v2/user.go") {
					return fmt.Errorf("output should not contain pkg/api/v2/user.go (fails @find: \"v1\")")
				}
				// Should exclude pkg/core/v1/user.go (fails @find: "api")
				if strings.Contains(output, "pkg/core/v1/user.go") {
					return fmt.Errorf("output should not contain pkg/core/v1/user.go (fails @find: \"api\")")
				}
				// Should exclude pkg/api/v1/system.go (fails @find: "user")
				if strings.Contains(output, "system.go") {
					return fmt.Errorf("output should not contain system.go (fails @find: \"user\")")
				}
				return nil
			}),
		},
	}
}

// DirectiveAndWithExclusionsScenario tests that exclusion patterns work alongside AND directives
func DirectiveAndWithExclusionsScenario() *harness.Scenario {
	return &harness.Scenario{
		Name:        "cx-directive-and-with-exclusions",
		Description: "Tests that exclusion patterns still work alongside AND directives",
		Tags:        []string{"cx", "search-directives"},
		Steps: []harness.Step{
			harness.NewStep("Setup test project", func(ctx *harness.Context) error {
				dirs := []string{
					filepath.Join(ctx.RootDir, "pkg", "api"),
				}
				for _, dir := range dirs {
					if err := os.MkdirAll(dir, 0o755); err != nil {
						return err
					}
				}

				files := map[string]string{
					filepath.Join(ctx.RootDir, "pkg", "api", "user.go"):      "package api\n\ntype UserManager struct {}",
					filepath.Join(ctx.RootDir, "pkg", "api", "user_test.go"): "package api\n\ntype UserManager struct {}",
					filepath.Join(ctx.RootDir, "pkg", "api", "system.go"):    "package api\n\ntype SystemManager struct {}",
				}

				for path, content := range files {
					if err := fs.WriteString(path, content); err != nil {
						return err
					}
				}
				return nil
			}),
			harness.NewStep("Create rules with directives and exclusion", func(ctx *harness.Context) error {
				rulesContent := "pkg/**/*.go @find: \"api\" @grep: \"UserManager\"\n!*_test.go"
				return fs.WriteString(filepath.Join(ctx.RootDir, ".grove", "rules"), rulesContent)
			}),
			harness.NewStep("Verify exclusion works with AND directives", func(ctx *harness.Context) error {
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

				// Should include pkg/api/user.go (matches both directives, not excluded)
				if !strings.Contains(output, "pkg/api/user.go") {
					return fmt.Errorf("output should contain pkg/api/user.go")
				}
				// Should exclude user_test.go (caught by exclusion pattern)
				if strings.Contains(output, "user_test.go") {
					return fmt.Errorf("output should not contain user_test.go (excluded by !*_test.go)")
				}
				// Should exclude system.go (fails @grep: "UserManager")
				if strings.Contains(output, "system.go") {
					return fmt.Errorf("output should not contain system.go (fails @grep: \"UserManager\")")
				}
				return nil
			}),
		},
	}
}

// DirectiveAndInColdContextScenario tests multiple directives in the cold context section
func DirectiveAndInColdContextScenario() *harness.Scenario {
	return &harness.Scenario{
		Name:        "cx-directive-and-in-cold-context",
		Description: "Tests multiple directives in the --- cold context section",
		Tags:        []string{"cx", "search-directives"},
		Steps: []harness.Step{
			harness.NewStep("Setup test project with hot and cold files", func(ctx *harness.Context) error {
				dirs := []string{
					filepath.Join(ctx.RootDir, "hot", "api"),
					filepath.Join(ctx.RootDir, "cold", "api"),
				}
				for _, dir := range dirs {
					if err := os.MkdirAll(dir, 0o755); err != nil {
						return err
					}
				}

				files := map[string]string{
					filepath.Join(ctx.RootDir, "hot", "api", "user.go"):    "package api\n\ntype UserManager struct {}",
					filepath.Join(ctx.RootDir, "cold", "api", "user.go"):   "package api\n\ntype UserManager struct {}",
					filepath.Join(ctx.RootDir, "cold", "api", "system.go"): "package api\n\ntype SystemManager struct {}",
				}

				for path, content := range files {
					if err := fs.WriteString(path, content); err != nil {
						return err
					}
				}
				return nil
			}),
			harness.NewStep("Create rules with cold context directives", func(ctx *harness.Context) error {
				rulesContent := "hot/**/*.go\n---\ncold/**/*.go @find: \"api\" @grep: \"UserManager\""
				return fs.WriteString(filepath.Join(ctx.RootDir, ".grove", "rules"), rulesContent)
			}),
			harness.NewStep("Verify hot files via cx list", func(ctx *harness.Context) error {
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
				// Hot section should have hot/api/user.go
				if !strings.Contains(output, "hot/api/user.go") {
					return fmt.Errorf("output should contain hot/api/user.go in hot section")
				}
				return nil
			}),
			harness.NewStep("Generate and verify cold files in cached-context-files", func(ctx *harness.Context) error {
				cxBinary, err := FindProjectBinary()
				if err != nil {
					return err
				}

				// Use ctx.Command to ensure sandboxed environment variables are injected
				cmd := ctx.Command(cxBinary, "generate").Dir(ctx.RootDir)
				result := cmd.Run()
				ctx.ShowCommandOutput(cmd.String(), result.Stdout, result.Stderr)
				if result.Error != nil {
					return result.Error
				}

				// cached-context-files only contains cold files
				cachedPath := findCachedContextFilesListOrFallback(ctx.RootDir)
				data, err := os.ReadFile(cachedPath)
				if err != nil {
					return fmt.Errorf("failed to read cached-context-files: %w", err)
				}

				output := string(data)

				// Cold file cold/api/user.go should appear (matches both directives)
				if !strings.Contains(output, "cold/api/user.go") {
					return fmt.Errorf("output should contain cold/api/user.go (matches both directives)")
				}
				// Cold file cold/api/system.go should NOT appear (fails grep)
				if strings.Contains(output, "cold/api/system.go") {
					return fmt.Errorf("output should not contain cold/api/system.go (fails @grep: \"UserManager\")")
				}
				return nil
			}),
		},
	}
}

// DirectiveAndWithBraceExpansionScenario tests directives combined with brace expansion
func DirectiveAndWithBraceExpansionScenario() *harness.Scenario {
	return &harness.Scenario{
		Name:        "cx-directive-and-with-brace-expansion",
		Description: "Tests directives combined with brace expansion",
		Tags:        []string{"cx", "search-directives"},
		Steps: []harness.Step{
			harness.NewStep("Setup test project with multiple directories", func(ctx *harness.Context) error {
				dirs := []string{
					filepath.Join(ctx.RootDir, "pkg", "api"),
					filepath.Join(ctx.RootDir, "pkg", "core"),
					filepath.Join(ctx.RootDir, "pkg", "utils"),
				}
				for _, dir := range dirs {
					if err := os.MkdirAll(dir, 0o755); err != nil {
						return err
					}
				}

				files := map[string]string{
					filepath.Join(ctx.RootDir, "pkg", "api", "user.go"):   "package api\n\ntype InitManager struct {}",
					filepath.Join(ctx.RootDir, "pkg", "core", "user.go"):  "package core\n\ntype InitManager struct {}",
					filepath.Join(ctx.RootDir, "pkg", "utils", "user.go"): "package utils\n\ntype InitManager struct {}",
					filepath.Join(ctx.RootDir, "pkg", "api", "system.go"): "package api\n\ntype RunManager struct {}",
				}

				for path, content := range files {
					if err := fs.WriteString(path, content); err != nil {
						return err
					}
				}
				return nil
			}),
			harness.NewStep("Create rules with brace expansion and directives", func(ctx *harness.Context) error {
				rulesContent := `pkg/{api,core}/**/*.go @find: "user" @grep: "Init"`
				return fs.WriteString(filepath.Join(ctx.RootDir, ".grove", "rules"), rulesContent)
			}),
			harness.NewStep("Verify brace expansion with AND directives", func(ctx *harness.Context) error {
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

				// Should include pkg/api/user.go (in brace expansion, matches find+grep)
				if !strings.Contains(output, "pkg/api/user.go") {
					return fmt.Errorf("output should contain pkg/api/user.go")
				}
				// Should include pkg/core/user.go (in brace expansion, matches find+grep)
				if !strings.Contains(output, "pkg/core/user.go") {
					return fmt.Errorf("output should contain pkg/core/user.go")
				}
				// Should exclude pkg/utils/user.go (not in brace expansion)
				if strings.Contains(output, "pkg/utils/user.go") {
					return fmt.Errorf("output should not contain pkg/utils/user.go (not in brace expansion)")
				}
				// Should exclude pkg/api/system.go (fails @find: "user" and @grep: "Init")
				if strings.Contains(output, "system.go") {
					return fmt.Errorf("output should not contain system.go (fails find+grep)")
				}
				return nil
			}),
		},
	}
}

// UnquotedInlineSearchDirectivesScenario tests inline unquoted search directives
func UnquotedInlineSearchDirectivesScenario() *harness.Scenario {
	return &harness.Scenario{
		Name:        "cx-unquoted-inline-search-directives",
		Description: "Tests inline unquoted @grep and @find directives, including queries with spaces and mixed with quoted directives",
		Tags:        []string{"cx", "search-directives"},
		Steps: []harness.Step{
			harness.NewStep("Setup test project with various files", func(ctx *harness.Context) error {
				dirs := []string{
					filepath.Join(ctx.RootDir, "pkg"),
					filepath.Join(ctx.RootDir, "cmd"),
					filepath.Join(ctx.RootDir, "docs"),
				}

				for _, dir := range dirs {
					if err := os.MkdirAll(dir, 0o755); err != nil {
						return err
					}
				}

				files := map[string]string{
					filepath.Join(ctx.RootDir, "pkg", "user_manager.go"): "package pkg\n\ntype User Manager struct {}",
					filepath.Join(ctx.RootDir, "pkg", "auth_manager.go"): "package pkg\n\ntype AuthManager struct {}",
					filepath.Join(ctx.RootDir, "cmd", "main_server.go"):  "package main\n\nfunc main() {}",
					filepath.Join(ctx.RootDir, "cmd", "util.go"):         "package cmd\n\nfunc util() {}",
					filepath.Join(ctx.RootDir, "docs", "readme.md"):      "Here is some \"quoted text\" to test",
				}

				for path, content := range files {
					if err := fs.WriteString(path, content); err != nil {
						return err
					}
				}

				return nil
			}),
			harness.NewStep("Create .grove/rules with mixed quoted and unquoted directives", func(ctx *harness.Context) error {
				rulesContent := `pkg/**/*.go @grep: User Manager
cmd/**/*.go @find: main_server
docs/**/*.md @grep: "quoted text"`
				rulesPath := filepath.Join(ctx.RootDir, ".grove", "rules")
				return fs.WriteString(rulesPath, rulesContent)
			}),
			harness.NewStep("Verify correct files are matched", func(ctx *harness.Context) error {
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

				// Unquoted grep with space should match user_manager.go
				if !strings.Contains(output, "pkg/user_manager.go") {
					return fmt.Errorf("output should contain pkg/user_manager.go (unquoted grep with space)")
				}
				// Unquoted find should match main_server.go
				if !strings.Contains(output, "cmd/main_server.go") {
					return fmt.Errorf("output should contain cmd/main_server.go (unquoted find)")
				}
				// Quoted grep should match readme.md
				if !strings.Contains(output, "docs/readme.md") {
					return fmt.Errorf("output should contain docs/readme.md (quoted grep)")
				}

				// Should NOT include non-matching files
				if strings.Contains(output, "auth_manager.go") {
					return fmt.Errorf("output should not contain auth_manager.go")
				}
				if strings.Contains(output, "cmd/util.go") {
					return fmt.Errorf("output should not contain cmd/util.go")
				}

				return nil
			}),
		},
	}
}

// UnquotedGlobalSearchDirectivesScenario tests global unquoted search directives
func UnquotedGlobalSearchDirectivesScenario() *harness.Scenario {
	return &harness.Scenario{
		Name:        "cx-unquoted-global-search-directives",
		Description: "Tests global unquoted @grep and @find directives applied to subsequent patterns",
		Tags:        []string{"cx", "search-directives"},
		Steps: []harness.Step{
			harness.NewStep("Setup test project with source and test files", func(ctx *harness.Context) error {
				dirs := []string{
					filepath.Join(ctx.RootDir, "src"),
					filepath.Join(ctx.RootDir, "tests"),
				}

				for _, dir := range dirs {
					if err := os.MkdirAll(dir, 0o755); err != nil {
						return err
					}
				}

				files := map[string]string{
					filepath.Join(ctx.RootDir, "src", "handler.go"):        "package src\n\nfunc HandleRequest() {}",
					filepath.Join(ctx.RootDir, "src", "server.go"):         "package src\n\nfunc Start() {}",
					filepath.Join(ctx.RootDir, "tests", "handler_test.go"): "package tests\n\nfunc TestHandler() { _ = HandleRequest }",
					filepath.Join(ctx.RootDir, "tests", "server_test.go"):  "package tests\n\nfunc TestServer() {}",
				}

				for path, content := range files {
					if err := fs.WriteString(path, content); err != nil {
						return err
					}
				}

				return nil
			}),
			harness.NewStep("Create .grove/rules with global unquoted directives", func(ctx *harness.Context) error {
				rulesContent := `@grep: HandleRequest
src/**/*.go

@find: handler_test
tests/**/*.go`
				rulesPath := filepath.Join(ctx.RootDir, ".grove", "rules")
				return fs.WriteString(rulesPath, rulesContent)
			}),
			harness.NewStep("Verify correct files are matched", func(ctx *harness.Context) error {
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

				// Global unquoted grep should match handler.go
				if !strings.Contains(output, "src/handler.go") {
					return fmt.Errorf("output should contain src/handler.go (global unquoted grep)")
				}
				// Global unquoted find should match handler_test.go
				if !strings.Contains(output, "tests/handler_test.go") {
					return fmt.Errorf("output should contain tests/handler_test.go (global unquoted find)")
				}

				// Should NOT include non-matching files
				if strings.Contains(output, "src/server.go") {
					return fmt.Errorf("output should not contain src/server.go")
				}
				if strings.Contains(output, "server_test.go") {
					return fmt.Errorf("output should not contain server_test.go")
				}

				return nil
			}),
		},
	}
}

// MalformedSearchDirectivesScenario tests edge cases around quotes in search directives.
// Specifically tests that inline directives with interior quotes are treated as unquoted
// queries rather than failing to parse.
func MalformedSearchDirectivesScenario() *harness.Scenario {
	return &harness.Scenario{
		Name:        "cx-malformed-search-directives",
		Description: "Tests fallback behaviors for interior quotes in inline directives",
		Tags:        []string{"cx", "search-directives"},
		Steps: []harness.Step{
			harness.NewStep("Setup test project with various files", func(ctx *harness.Context) error {
				if err := os.MkdirAll(filepath.Join(ctx.RootDir, "src"), 0o755); err != nil {
					return err
				}
				files := map[string]string{
					// Content has an interior quote: `internal"helper`
					filepath.Join(ctx.RootDir, "src", "util.go"): "package src\n\n// internal\"helper func",
					// Content without the interior quote pattern
					filepath.Join(ctx.RootDir, "src", "server.go"): "package src\n\nfunc Start() {}",
				}
				for path, content := range files {
					if err := fs.WriteString(path, content); err != nil {
						return err
					}
				}
				return nil
			}),
			harness.NewStep("Create rules with interior-quote directive", func(ctx *harness.Context) error {
				// Inline grep with interior quote: query is `internal"helper`
				// This tests that a quote character in the middle of an unquoted query
				// doesn't cause parsing to fail.
				// Must use glob patterns (not exact file paths) because directive filters
				// are only applied during glob resolution.
				rulesContent := "src/**/*.go @grep: internal\"helper"
				return fs.WriteString(filepath.Join(ctx.RootDir, ".grove", "rules"), rulesContent)
			}),
			harness.NewStep("Verify correct files are matched", func(ctx *harness.Context) error {
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
				// util.go matched: content contains `internal"helper`
				if !strings.Contains(output, "src/util.go") {
					return fmt.Errorf("output should contain src/util.go (interior quote grep)")
				}
				// server.go NOT matched: content doesn't contain `internal"helper`
				if strings.Contains(output, "src/server.go") {
					return fmt.Errorf("output should not contain src/server.go")
				}
				return nil
			}),
		},
	}
}

// GrepIDirectiveScenario tests the @grep-i directive for case-insensitive content filtering
func GrepIDirectiveScenario() *harness.Scenario {
	return &harness.Scenario{
		Name:        "cx-grep-i-directive",
		Description: "Tests @grep-i directive for case-insensitive file content filtering",
		Tags:        []string{"cx", "search-directives"},
		Steps: []harness.Step{
			harness.NewStep("Setup test project with various cased content", func(ctx *harness.Context) error {
				dir := filepath.Join(ctx.RootDir, "pkg", "managers")
				if err := os.MkdirAll(dir, 0o755); err != nil {
					return err
				}

				files := map[string]string{
					filepath.Join(dir, "user.go"):       "package managers\n\ntype UserManager struct {}",
					filepath.Join(dir, "user_lower.go"): "package managers\n\nvar usermanager = true",
					filepath.Join(dir, "user_upper.go"): "package managers\n\nconst USERMANAGER = 1",
					filepath.Join(dir, "file.go"):       "package managers\n\ntype FileHandler struct {}",
				}

				for path, content := range files {
					if err := fs.WriteString(path, content); err != nil {
						return err
					}
				}
				return nil
			}),
			harness.NewStep("Create .grove/rules with @grep-i directive", func(ctx *harness.Context) error {
				rulesContent := `pkg/**/*.go @grep-i: "usermanager"`
				rulesPath := filepath.Join(ctx.RootDir, ".grove", "rules")
				return fs.WriteString(rulesPath, rulesContent)
			}),
			harness.NewStep("Verify case-insensitive grep matches all casings", func(ctx *harness.Context) error {
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

				// All three files with various casings of "usermanager" should be included
				for _, expected := range []string{"user.go", "user_lower.go", "user_upper.go"} {
					if !strings.Contains(output, expected) {
						return fmt.Errorf("output should contain %s", expected)
					}
				}

				// File without any casing of "usermanager" should NOT be included
				if strings.Contains(output, "file.go") {
					return fmt.Errorf("output should not contain file.go")
				}

				return nil
			}),
		},
	}
}

// GlobalGrepIDirectiveScenario tests standalone @grep-i: directive applied to all patterns
func GlobalGrepIDirectiveScenario() *harness.Scenario {
	return &harness.Scenario{
		Name:        "cx-global-grep-i-directive",
		Description: "Tests global @grep-i directive applies case-insensitive filtering to all patterns",
		Tags:        []string{"cx", "search-directives"},
		Steps: []harness.Step{
			harness.NewStep("Setup test project with logger content in various cases", func(ctx *harness.Context) error {
				dirs := []string{
					filepath.Join(ctx.RootDir, "src", "app"),
					filepath.Join(ctx.RootDir, "src", "pkg"),
				}
				for _, dir := range dirs {
					if err := os.MkdirAll(dir, 0o755); err != nil {
						return err
					}
				}

				files := map[string]string{
					filepath.Join(ctx.RootDir, "src", "app", "main.go"):   "package app\n\nvar LOGGER = true",
					filepath.Join(ctx.RootDir, "src", "pkg", "utils.go"):  "package pkg\n\nvar logger = newLogger()",
					filepath.Join(ctx.RootDir, "src", "pkg", "config.go"): "package pkg\n\ntype Settings struct {}",
				}

				for path, content := range files {
					if err := fs.WriteString(path, content); err != nil {
						return err
					}
				}
				return nil
			}),
			harness.NewStep("Create .grove/rules with global @grep-i directive", func(ctx *harness.Context) error {
				rulesContent := "@grep-i: \"logger\"\nsrc/**/*.go"
				rulesPath := filepath.Join(ctx.RootDir, ".grove", "rules")
				return fs.WriteString(rulesPath, rulesContent)
			}),
			harness.NewStep("Verify global grep-i filters all patterns case-insensitively", func(ctx *harness.Context) error {
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

				// Files with "LOGGER" and "logger" should be included
				if !strings.Contains(output, "src/app/main.go") {
					return fmt.Errorf("output should contain src/app/main.go")
				}
				if !strings.Contains(output, "src/pkg/utils.go") {
					return fmt.Errorf("output should contain src/pkg/utils.go")
				}

				// File without any casing of "logger" should NOT be included
				if strings.Contains(output, "config.go") {
					return fmt.Errorf("output should not contain config.go")
				}

				return nil
			}),
		},
	}
}

// GrepVsGrepIScenario contrasts case-sensitive @grep: with case-insensitive @grep-i:
func GrepVsGrepIScenario() *harness.Scenario {
	return &harness.Scenario{
		Name:        "cx-grep-vs-grep-i",
		Description: "Contrasts @grep (case-sensitive) with @grep-i (case-insensitive)",
		Tags:        []string{"cx", "search-directives"},
		Steps: []harness.Step{
			harness.NewStep("Setup test project with strict and loose directories", func(ctx *harness.Context) error {
				dirs := []string{
					filepath.Join(ctx.RootDir, "pkg", "auth", "strict"),
					filepath.Join(ctx.RootDir, "pkg", "auth", "loose"),
				}
				for _, dir := range dirs {
					if err := os.MkdirAll(dir, 0o755); err != nil {
						return err
					}
				}

				files := map[string]string{
					filepath.Join(ctx.RootDir, "pkg", "auth", "strict", "file1.go"): "package strict\n\nvar AuthToken = true",
					filepath.Join(ctx.RootDir, "pkg", "auth", "strict", "file2.go"): "package strict\n\nvar authtoken = true",
					filepath.Join(ctx.RootDir, "pkg", "auth", "loose", "file1.go"):  "package loose\n\nvar AuthToken = true",
					filepath.Join(ctx.RootDir, "pkg", "auth", "loose", "file2.go"):  "package loose\n\nvar authtoken = true",
				}

				for path, content := range files {
					if err := fs.WriteString(path, content); err != nil {
						return err
					}
				}
				return nil
			}),
			harness.NewStep("Create .grove/rules with @grep and @grep-i", func(ctx *harness.Context) error {
				rulesContent := "pkg/auth/strict/*.go @grep: \"authtoken\"\npkg/auth/loose/*.go @grep-i: \"authtoken\""
				rulesPath := filepath.Join(ctx.RootDir, ".grove", "rules")
				return fs.WriteString(rulesPath, rulesContent)
			}),
			harness.NewStep("Verify grep is case-sensitive and grep-i is case-insensitive", func(ctx *harness.Context) error {
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

				// @grep: "authtoken" (case-sensitive) should only match exact lowercase
				if !strings.Contains(output, "pkg/auth/strict/file2.go") {
					return fmt.Errorf("output should contain pkg/auth/strict/file2.go (exact case match)")
				}
				if strings.Contains(output, "pkg/auth/strict/file1.go") {
					return fmt.Errorf("output should not contain pkg/auth/strict/file1.go (AuthToken != authtoken)")
				}

				// @grep-i: "authtoken" (case-insensitive) should match both
				if !strings.Contains(output, "pkg/auth/loose/file1.go") {
					return fmt.Errorf("output should contain pkg/auth/loose/file1.go (case-insensitive match)")
				}
				if !strings.Contains(output, "pkg/auth/loose/file2.go") {
					return fmt.Errorf("output should contain pkg/auth/loose/file2.go (case-insensitive match)")
				}

				return nil
			}),
		},
	}
}

// CombinedSearchDirectivesScenario tests @grep-i alongside @find, @grep, and regular patterns
func CombinedSearchDirectivesScenario() *harness.Scenario {
	return &harness.Scenario{
		Name:        "cx-combined-search-directives",
		Description: "Tests @grep-i alongside @find, @grep, and regular patterns in one rules file",
		Tags:        []string{"cx", "search-directives"},
		Steps: []harness.Step{
			harness.NewStep("Setup test project with multiple directories", func(ctx *harness.Context) error {
				dirs := []string{
					filepath.Join(ctx.RootDir, "src", "models"),
					filepath.Join(ctx.RootDir, "src", "controllers"),
					filepath.Join(ctx.RootDir, "src", "utils"),
					filepath.Join(ctx.RootDir, "tests"),
				}
				for _, dir := range dirs {
					if err := os.MkdirAll(dir, 0o755); err != nil {
						return err
					}
				}

				files := map[string]string{
					filepath.Join(ctx.RootDir, "src", "models", "user.go"):      "package models\n\ntype AccountInfo struct {}",
					filepath.Join(ctx.RootDir, "src", "controllers", "auth.go"): "package controllers\n\nvar SecretToken = true",
					filepath.Join(ctx.RootDir, "src", "utils", "helper.go"):     "package utils\n\nconst HELPER_FUNC = 1",
					filepath.Join(ctx.RootDir, "tests", "main_test.go"):         "package tests\n\nfunc TestMain() {}",
				}

				for path, content := range files {
					if err := fs.WriteString(path, content); err != nil {
						return err
					}
				}
				return nil
			}),
			harness.NewStep("Create .grove/rules with mixed directives", func(ctx *harness.Context) error {
				rulesContent := "src/**/*.go @find: \"user\"\nsrc/controllers/*.go @grep: \"SecretToken\"\nsrc/utils/*.go @grep-i: \"helper_func\"\ntests/**/*_test.go"
				rulesPath := filepath.Join(ctx.RootDir, ".grove", "rules")
				return fs.WriteString(rulesPath, rulesContent)
			}),
			harness.NewStep("Verify all directives work together correctly", func(ctx *harness.Context) error {
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

				// All four files should be included via their respective directives
				expectedFiles := []string{
					"src/models/user.go",      // via @find: "user"
					"src/controllers/auth.go", // via @grep: "SecretToken"
					"src/utils/helper.go",     // via @grep-i: "helper_func"
					"tests/main_test.go",      // via plain pattern
				}

				for _, file := range expectedFiles {
					if !strings.Contains(output, file) {
						return fmt.Errorf("output should contain %s", file)
					}
				}

				return nil
			}),
		},
	}
}

// CombinedDirectivesScenario tests combining both directives with regular patterns
func CombinedDirectivesScenario() *harness.Scenario {
	return &harness.Scenario{
		Name:        "cx-combined-directives",
		Description: "Tests combining @find and @grep directives with regular patterns",
		Tags:        []string{"cx", "search-directives"},
		Steps: []harness.Step{
			harness.NewStep("Setup complex test project", func(ctx *harness.Context) error {
				dirs := []string{
					filepath.Join(ctx.RootDir, "src"),
					filepath.Join(ctx.RootDir, "tests"),
					filepath.Join(ctx.RootDir, "config"),
				}

				for _, dir := range dirs {
					if err := os.MkdirAll(dir, 0o755); err != nil {
						return err
					}
				}

				files := map[string]string{
					filepath.Join(ctx.RootDir, "src", "manager.go"):      "package src\n\ntype Manager struct {}",
					filepath.Join(ctx.RootDir, "src", "handler.go"):      "package src\n\ntype Handler struct {}",
					filepath.Join(ctx.RootDir, "tests", "utils_test.go"): "package tests\n\nfunc TestHelper() {}",
					filepath.Join(ctx.RootDir, "tests", "helper.go"):     "package tests\n\nfunc Helper() {}",
					filepath.Join(ctx.RootDir, "config", "config.yaml"):  "app:\n  name: test",
					filepath.Join(ctx.RootDir, "config", "secrets.yaml"): "secrets:\n  key: value",
				}

				for path, content := range files {
					if err := fs.WriteString(path, content); err != nil {
						return err
					}
				}

				return nil
			}),
			harness.NewStep("Create rules with multiple directives", func(ctx *harness.Context) error {
				// Use @find for Go files with "manager" in the name,
				// @grep for YAML files containing "secrets",
				// and regular pattern for test files
				rulesContent := `src/**/*.go @find: "manager"
config/**/*.yaml @grep: "secrets"
tests/**/*_test.go`
				rulesPath := filepath.Join(ctx.RootDir, ".grove", "rules")
				return fs.WriteString(rulesPath, rulesContent)
			}),
			harness.NewStep("Verify correct files are matched", func(ctx *harness.Context) error {
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

				// Should include: manager.go (via @find), secrets.yaml (via @grep), utils_test.go (via pattern)
				expectedFiles := []string{
					"src/manager.go",
					"config/secrets.yaml",
					"tests/utils_test.go",
				}

				for _, file := range expectedFiles {
					if !strings.Contains(output, file) {
						return fmt.Errorf("output should contain %s", file)
					}
				}

				// Should NOT include: handler.go, config.yaml, helper.go
				unexpectedFiles := []string{
					"handler.go",
					"config.yaml",
					"helper.go",
				}

				for _, file := range unexpectedFiles {
					if strings.Contains(output, file) {
						return fmt.Errorf("output should not contain %s", file)
					}
				}

				return nil
			}),
		},
	}
}

// InvalidGrepRegexScenario tests that an invalid regex in @grep fails fast with an error
func InvalidGrepRegexScenario() *harness.Scenario {
	return &harness.Scenario{
		Name:        "cx-invalid-grep-regex",
		Description: "Tests that an invalid regex in a @grep directive fails fast with an error message",
		Tags:        []string{"cx", "search-directives", "error"},
		Steps: []harness.Step{
			harness.NewStep("Setup test project", func(ctx *harness.Context) error {
				return fs.WriteString(filepath.Join(ctx.RootDir, "main.go"), "package main\n")
			}),
			harness.NewStep("Create rules with invalid regex", func(ctx *harness.Context) error {
				rulesContent := `*.go @grep: "[invalid"`
				rulesPath := filepath.Join(ctx.RootDir, ".grove", "rules")
				return fs.WriteString(rulesPath, rulesContent)
			}),
			harness.NewStep("Verify command fails with expected error message", func(ctx *harness.Context) error {
				cxBinary, err := FindProjectBinary()
				if err != nil {
					return err
				}

				cmd := command.New(cxBinary, "list").Dir(ctx.RootDir)
				result := cmd.Run()

				ctx.ShowCommandOutput(cmd.String(), result.Stdout, result.Stderr)

				if result.Error == nil {
					return fmt.Errorf("expected command to fail, but it succeeded")
				}

				output := result.Stdout + result.Stderr
				if !strings.Contains(output, "invalid regex") {
					return fmt.Errorf("expected error message to contain 'invalid regex', got: %s", output)
				}

				return nil
			}),
		},
	}
}

// ValidGrepRegexScenario tests that a valid regex pattern correctly filters files
func ValidGrepRegexScenario() *harness.Scenario {
	return &harness.Scenario{
		Name:        "cx-valid-grep-regex",
		Description: "Tests that a valid regex in @grep correctly matches file content",
		Tags:        []string{"cx", "search-directives"},
		Steps: []harness.Step{
			harness.NewStep("Setup test project with function and struct files", func(ctx *harness.Context) error {
				files := map[string]string{
					filepath.Join(ctx.RootDir, "func.go"):   "package main\n\nfunc MyFunction() {}\n",
					filepath.Join(ctx.RootDir, "struct.go"): "package main\n\ntype MyStruct struct {}\n",
				}
				for path, content := range files {
					if err := fs.WriteString(path, content); err != nil {
						return err
					}
				}
				return nil
			}),
			harness.NewStep("Create rules with valid regex", func(ctx *harness.Context) error {
				rulesContent := `*.go @grep: "func\s+\w+"`
				rulesPath := filepath.Join(ctx.RootDir, ".grove", "rules")
				return fs.WriteString(rulesPath, rulesContent)
			}),
			harness.NewStep("Verify only function file matches", func(ctx *harness.Context) error {
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

				if !strings.Contains(output, "func.go") {
					return fmt.Errorf("output should contain func.go")
				}
				if strings.Contains(output, "struct.go") {
					return fmt.Errorf("output should not contain struct.go")
				}

				return nil
			}),
		},
	}
}

// GrepRegexVsLiteralScenario tests that @grep uses regex matching, not literal
func GrepRegexVsLiteralScenario() *harness.Scenario {
	return &harness.Scenario{
		Name:        "cx-grep-regex-vs-literal",
		Description: "Tests that @grep evaluates query as regex, matching patterns like .*Manager",
		Tags:        []string{"cx", "search-directives"},
		Steps: []harness.Step{
			harness.NewStep("Setup test project with manager and non-manager files", func(ctx *harness.Context) error {
				files := map[string]string{
					filepath.Join(ctx.RootDir, "user.go"):  "package main\n\ntype UserManager struct {}\n",
					filepath.Join(ctx.RootDir, "file.go"):  "package main\n\ntype FileManager struct {}\n",
					filepath.Join(ctx.RootDir, "other.go"): "package main\n\ntype OtherSystem struct {}\n",
				}
				for path, content := range files {
					if err := fs.WriteString(path, content); err != nil {
						return err
					}
				}
				return nil
			}),
			harness.NewStep("Create rules with regex pattern", func(ctx *harness.Context) error {
				rulesContent := `*.go @grep: ".*Manager"`
				rulesPath := filepath.Join(ctx.RootDir, ".grove", "rules")
				return fs.WriteString(rulesPath, rulesContent)
			}),
			harness.NewStep("Verify regex matches manager files only", func(ctx *harness.Context) error {
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

				if !strings.Contains(output, "user.go") {
					return fmt.Errorf("output should contain user.go")
				}
				if !strings.Contains(output, "file.go") {
					return fmt.Errorf("output should contain file.go")
				}
				if strings.Contains(output, "other.go") {
					return fmt.Errorf("output should not contain other.go")
				}

				return nil
			}),
		},
	}
}

// EmptyGrepQueryScenario tests that an empty @grep query doesn't crash
func EmptyGrepQueryScenario() *harness.Scenario {
	return &harness.Scenario{
		Name:        "cx-empty-grep-query",
		Description: "Tests that an empty @grep query is handled gracefully without errors",
		Tags:        []string{"cx", "search-directives"},
		Steps: []harness.Step{
			harness.NewStep("Setup test project", func(ctx *harness.Context) error {
				return fs.WriteString(filepath.Join(ctx.RootDir, "main.go"), "package main\n\nfunc main() {}\n")
			}),
			harness.NewStep("Create rules with empty grep query", func(ctx *harness.Context) error {
				rulesContent := `*.go @grep: ""`
				rulesPath := filepath.Join(ctx.RootDir, ".grove", "rules")
				return fs.WriteString(rulesPath, rulesContent)
			}),
			harness.NewStep("Verify command succeeds without errors", func(ctx *harness.Context) error {
				cxBinary, err := FindProjectBinary()
				if err != nil {
					return err
				}

				cmd := command.New(cxBinary, "list").Dir(ctx.RootDir)
				result := cmd.Run()

				ctx.ShowCommandOutput(cmd.String(), result.Stdout, result.Stderr)

				if result.Error != nil {
					return fmt.Errorf("expected command to succeed, but got error: %v", result.Error)
				}

				return nil
			}),
		},
	}
}

func PlainGlobDirectiveScenario() *harness.Scenario {
	return &harness.Scenario{
		Name:        "cx-plain-glob-directive",
		Description: "@grep on plain glob filters file content (Phase 3A bug fix)",
		Tags:        []string{"cx", "search-directives", "phase3"},
		Steps: []harness.Step{
			harness.NewStep("Setup project with mixed content", func(ctx *harness.Context) error {
				files := map[string]string{
					filepath.Join(ctx.RootDir, "pkg", "alpha.go"): "package pkg\nfunc FindRulesetFile() {}\n",
					filepath.Join(ctx.RootDir, "pkg", "beta.go"):  "package pkg\nfunc Other() {}\n",
				}
				for p, c := range files {
					if err := fs.WriteString(p, c); err != nil {
						return err
					}
				}
				return nil
			}),
			harness.NewStep("Create rules with plain glob + @grep directive", func(ctx *harness.Context) error {
				rules := `pkg/**/*.go @grep: "FindRulesetFile"`
				return fs.WriteString(filepath.Join(ctx.RootDir, ".grove", "rules"), rules)
			}),
			harness.NewStep("Verify only matching file resolved", func(ctx *harness.Context) error {
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
				out := result.Stdout
				if !strings.Contains(out, "alpha.go") {
					return fmt.Errorf("@grep on plain glob dropped matching file alpha.go: %s", out)
				}
				if strings.Contains(out, "beta.go") {
					return fmt.Errorf("@grep on plain glob leaked non-matching beta.go: %s", out)
				}
				return nil
			}),
		},
	}
}
