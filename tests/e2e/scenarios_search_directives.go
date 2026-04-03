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
					if err := os.MkdirAll(dir, 0755); err != nil {
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
					if err := os.MkdirAll(dir, 0755); err != nil {
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
					if err := os.MkdirAll(dir, 0755); err != nil {
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
					if err := os.MkdirAll(dir, 0755); err != nil {
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
					if err := os.MkdirAll(dir, 0755); err != nil {
						return err
					}
				}

				files := map[string]string{
					filepath.Join(ctx.RootDir, "hot", "api", "user.go"):  "package api\n\ntype UserManager struct {}",
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
					if err := os.MkdirAll(dir, 0755); err != nil {
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
