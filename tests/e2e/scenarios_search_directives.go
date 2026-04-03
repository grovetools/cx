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
					if err := os.MkdirAll(dir, 0755); err != nil {
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
					if err := os.MkdirAll(dir, 0755); err != nil {
						return err
					}
				}

				files := map[string]string{
					filepath.Join(ctx.RootDir, "pkg", "api", "generated.go"): "package api\n// Code generated by tool. DO NOT EDIT.\ntype Gen struct{}",
					filepath.Join(ctx.RootDir, "pkg", "api", "manual.go"):   "package api\n\ntype Manual struct{}",
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
					filepath.Join(ctx.RootDir, "src", "user_test.go"):  "package src",
					filepath.Join(ctx.RootDir, "src", "user.go"):       "package src",
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
