// File: grove-context/tests/e2e/scenarios_search_directives.go
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
					if err := os.MkdirAll(dir, 0755); err != nil {
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
					if err := os.MkdirAll(dir, 0755); err != nil {
						return err
					}
				}

				// Create files - some containing "UserManager" in the content
				files := map[string]string{
					filepath.Join(ctx.RootDir, "pkg", "managers", "user.go"):    "package managers\n\ntype UserManager struct {\n\tID int\n}",
					filepath.Join(ctx.RootDir, "pkg", "managers", "file.go"):    "package managers\n\ntype FileHandler struct {\n\tPath string\n}",
					filepath.Join(ctx.RootDir, "pkg", "api", "user_api.go"):     "package api\n\nimport \"myapp/pkg/managers\"\n\nfunc GetUser() *managers.UserManager {\n\treturn nil\n}",
					filepath.Join(ctx.RootDir, "pkg", "api", "file_api.go"):     "package api\n\nfunc GetFile() string {\n\treturn \"\"\n}",
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
					if err := os.MkdirAll(dir, 0755); err != nil {
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
					if err := os.MkdirAll(dir, 0755); err != nil {
						return err
					}
				}

				files := map[string]string{
					filepath.Join(ctx.RootDir, "src", "handler.go"):       "package src\n\nfunc HandleRequest() {}",
					filepath.Join(ctx.RootDir, "src", "server.go"):        "package src\n\nfunc Start() {}",
					filepath.Join(ctx.RootDir, "tests", "handler_test.go"): "package tests\n\nfunc TestHandler() {}",
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
				if err := os.MkdirAll(filepath.Join(ctx.RootDir, "src"), 0755); err != nil {
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
					if err := os.MkdirAll(dir, 0755); err != nil {
						return err
					}
				}

				files := map[string]string{
					filepath.Join(ctx.RootDir, "src", "manager.go"):         "package src\n\ntype Manager struct {}",
					filepath.Join(ctx.RootDir, "src", "handler.go"):         "package src\n\ntype Handler struct {}",
					filepath.Join(ctx.RootDir, "tests", "utils_test.go"):    "package tests\n\nfunc TestHelper() {}",
					filepath.Join(ctx.RootDir, "tests", "helper.go"):        "package tests\n\nfunc Helper() {}",
					filepath.Join(ctx.RootDir, "config", "config.yaml"):     "app:\n  name: test",
					filepath.Join(ctx.RootDir, "config", "secrets.yaml"):    "secrets:\n  key: value",
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
