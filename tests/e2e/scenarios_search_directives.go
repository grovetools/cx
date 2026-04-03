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
					if err := os.MkdirAll(dir, 0755); err != nil {
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
					if err := os.MkdirAll(dir, 0755); err != nil {
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
					if err := os.MkdirAll(filepath.Join(ctx.RootDir, d), 0755); err != nil {
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
					if err := os.MkdirAll(dir, 0755); err != nil {
						return err
					}
				}

				files := map[string]string{
					filepath.Join(ctx.RootDir, "src", "manager.go"):     "package src\n\ntype Manager struct{}",
					filepath.Join(ctx.RootDir, "src", "old_manager.go"): "package src\n\ntype OldManager struct{}",
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
				if err := os.MkdirAll(dir, 0755); err != nil {
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
