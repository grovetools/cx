package main

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/grovetools/tend/pkg/command"
	"github.com/grovetools/tend/pkg/fs"
	"github.com/grovetools/tend/pkg/harness"
	"github.com/grovetools/tend/pkg/verify"
)

// TreeDirectiveBasicScenario tests basic @tree: directive functionality including
// ASCII tree output and inherent .git/.grove exclusion.
func TreeDirectiveBasicScenario() *harness.Scenario {
	return &harness.Scenario{
		Name:        "cx-tree-basic",
		Description: "Tests basic @tree directive produces ASCII tree and excludes .git/.grove",
		Tags:        []string{"cx", "tree-directive"},
		Steps: []harness.Step{
			harness.NewStep("Setup project with nested directories", func(ctx *harness.Context) error {
				if result := command.New("git", "init").Dir(ctx.RootDir).Run(); result.Error != nil {
					return fmt.Errorf("git init failed: %w", result.Error)
				}

				if err := fs.WriteString(filepath.Join(ctx.RootDir, "src", "main.go"), "package main"); err != nil {
					return err
				}
				if err := fs.WriteString(filepath.Join(ctx.RootDir, "src", "api", "routes.go"), "package api"); err != nil {
					return err
				}
				if err := fs.WriteString(filepath.Join(ctx.RootDir, "src", "utils", "helper.go"), "package utils"); err != nil {
					return err
				}

				return fs.WriteString(filepath.Join(ctx.RootDir, ".grove", "rules"), "@tree: src/")
			}),

			harness.NewStep("Run cx generate and verify tree output", func(ctx *harness.Context) error {
				cxBinary, err := FindProjectBinary()
				if err != nil {
					return err
				}

				cmd := ctx.Command(cxBinary, "generate", "--xml=false").Dir(ctx.RootDir)
				result := cmd.Run()
				ctx.ShowCommandOutput(cmd.String(), result.Stdout, result.Stderr)
				if result.Error != nil {
					return fmt.Errorf("cx generate failed: %w", result.Error)
				}

				contextPath := findContextFileOrFallback(ctx.RootDir)
				content, err := fs.ReadString(contextPath)
				if err != nil {
					return fmt.Errorf("could not read context file: %w", err)
				}

				return ctx.Verify(func(v *verify.Collector) {
					v.Contains("tree section header present", content, "=== TREE: src/ ===")
					v.Contains("tree section footer present", content, "=== END TREE: src/ ===")
					v.Contains("api directory in tree", content, "api/")
					v.Contains("utils directory in tree", content, "utils/")
					v.Contains("routes.go in tree", content, "routes.go")
					v.Contains("helper.go in tree", content, "helper.go")
					v.Contains("tree connector present", content, "├──")
					v.Contains("tree last-item connector present", content, "└──")
					v.NotContains(".git excluded from tree", content, ".git")
					v.NotContains(".grove excluded from tree", content, ".grove")
				})
			}),
		},
	}
}

// TreeDirectiveGitignoreScenario tests that tree generation respects .gitignore patterns.
func TreeDirectiveGitignoreScenario() *harness.Scenario {
	return &harness.Scenario{
		Name:        "cx-tree-gitignore",
		Description: "Tests that @tree directive respects .gitignore",
		Tags:        []string{"cx", "tree-directive", "gitignore"},
		Steps: []harness.Step{
			harness.NewStep("Setup project with gitignored files", func(ctx *harness.Context) error {
				if result := command.New("git", "init").Dir(ctx.RootDir).Run(); result.Error != nil {
					return fmt.Errorf("git init failed: %w", result.Error)
				}

				if err := fs.WriteString(filepath.Join(ctx.RootDir, ".gitignore"), "secret.key\nnode_modules/\n"); err != nil {
					return err
				}
				if err := fs.WriteString(filepath.Join(ctx.RootDir, "src", "app.go"), "package main"); err != nil {
					return err
				}
				if err := fs.WriteString(filepath.Join(ctx.RootDir, "src", "secret.key"), "supersecret"); err != nil {
					return err
				}
				if err := fs.WriteString(filepath.Join(ctx.RootDir, "node_modules", "pkg", "index.js"), "module.exports = {}"); err != nil {
					return err
				}

				return fs.WriteString(filepath.Join(ctx.RootDir, ".grove", "rules"), "@tree: .")
			}),

			harness.NewStep("Verify gitignored paths excluded from tree", func(ctx *harness.Context) error {
				cxBinary, err := FindProjectBinary()
				if err != nil {
					return err
				}

				cmd := ctx.Command(cxBinary, "generate", "--xml=false").Dir(ctx.RootDir)
				result := cmd.Run()
				ctx.ShowCommandOutput(cmd.String(), result.Stdout, result.Stderr)
				if result.Error != nil {
					return fmt.Errorf("cx generate failed: %w", result.Error)
				}

				contextPath := findContextFileOrFallback(ctx.RootDir)
				content, err := fs.ReadString(contextPath)
				if err != nil {
					return fmt.Errorf("could not read context file: %w", err)
				}

				return ctx.Verify(func(v *verify.Collector) {
					v.Contains("tracked file app.go visible", content, "app.go")
					v.NotContains("secret.key excluded", content, "secret.key")
					v.NotContains("node_modules excluded", content, "node_modules")
				})
			}),
		},
	}
}

// TreeDirectiveCombinedScenario tests @tree: alongside standard file inclusion patterns.
func TreeDirectiveCombinedScenario() *harness.Scenario {
	return &harness.Scenario{
		Name:        "cx-tree-combined",
		Description: "Tests @tree directive combined with file inclusion patterns",
		Tags:        []string{"cx", "tree-directive"},
		Steps: []harness.Step{
			harness.NewStep("Setup project with tree and file rules", func(ctx *harness.Context) error {
				if result := command.New("git", "init").Dir(ctx.RootDir).Run(); result.Error != nil {
					return fmt.Errorf("git init failed: %w", result.Error)
				}

				if err := fs.WriteString(filepath.Join(ctx.RootDir, "src", "main.go"), "package main"); err != nil {
					return err
				}
				if err := fs.WriteString(filepath.Join(ctx.RootDir, "README.md"), "# Test Project"); err != nil {
					return err
				}

				return fs.WriteString(filepath.Join(ctx.RootDir, ".grove", "rules"), "@tree: src/\nREADME.md")
			}),

			harness.NewStep("Verify both tree and file content appear", func(ctx *harness.Context) error {
				cxBinary, err := FindProjectBinary()
				if err != nil {
					return err
				}

				cmd := ctx.Command(cxBinary, "generate", "--xml=false").Dir(ctx.RootDir)
				result := cmd.Run()
				ctx.ShowCommandOutput(cmd.String(), result.Stdout, result.Stderr)
				if result.Error != nil {
					return fmt.Errorf("cx generate failed: %w", result.Error)
				}

				contextPath := findContextFileOrFallback(ctx.RootDir)
				content, err := fs.ReadString(contextPath)
				if err != nil {
					return fmt.Errorf("could not read context file: %w", err)
				}

				return ctx.Verify(func(v *verify.Collector) {
					v.Contains("tree section present", content, "=== TREE: src/ ===")
					v.Contains("main.go in tree", content, "main.go")
					v.Contains("file content section for README", content, "=== FILE: README.md ===")
					v.Contains("README content included", content, "# Test Project")
				})
			}),
		},
	}
}

// TreeDirectiveXMLScenario tests that @tree: produces proper XML tags when using XML format.
func TreeDirectiveXMLScenario() *harness.Scenario {
	return &harness.Scenario{
		Name:        "cx-tree-xml",
		Description: "Tests @tree directive XML output format",
		Tags:        []string{"cx", "tree-directive"},
		Steps: []harness.Step{
			harness.NewStep("Setup project and generate with XML", func(ctx *harness.Context) error {
				if result := command.New("git", "init").Dir(ctx.RootDir).Run(); result.Error != nil {
					return fmt.Errorf("git init failed: %w", result.Error)
				}

				if err := fs.WriteString(filepath.Join(ctx.RootDir, "src", "main.go"), "package main"); err != nil {
					return err
				}

				return fs.WriteString(filepath.Join(ctx.RootDir, ".grove", "rules"), "@tree: src/")
			}),

			harness.NewStep("Verify XML tree tags", func(ctx *harness.Context) error {
				cxBinary, err := FindProjectBinary()
				if err != nil {
					return err
				}

				cmd := ctx.Command(cxBinary, "generate", "--xml").Dir(ctx.RootDir)
				result := cmd.Run()
				ctx.ShowCommandOutput(cmd.String(), result.Stdout, result.Stderr)
				if result.Error != nil {
					return fmt.Errorf("cx generate failed: %w", result.Error)
				}

				contextPath := findContextFileOrFallback(ctx.RootDir)
				content, err := fs.ReadString(contextPath)
				if err != nil {
					return fmt.Errorf("could not read context file: %w", err)
				}

				return ctx.Verify(func(v *verify.Collector) {
					v.Contains("XML declaration present", content, "<?xml version=")
					v.Contains("tree XML open tag", content, `<tree path="src/">`)
					v.Contains("tree XML close tag", content, "</tree>")
					v.Contains("main.go in XML tree", content, "main.go")
				})
			}),
		},
	}
}

// TreeDirectiveMultipleScenario tests multiple @tree: directives in one rules file.
func TreeDirectiveMultipleScenario() *harness.Scenario {
	return &harness.Scenario{
		Name:        "cx-tree-multiple",
		Description: "Tests multiple @tree directives in one rules file",
		Tags:        []string{"cx", "tree-directive"},
		Steps: []harness.Step{
			harness.NewStep("Setup project with two directories", func(ctx *harness.Context) error {
				if result := command.New("git", "init").Dir(ctx.RootDir).Run(); result.Error != nil {
					return fmt.Errorf("git init failed: %w", result.Error)
				}

				if err := fs.WriteString(filepath.Join(ctx.RootDir, "frontend", "app.js"), "const app = {}"); err != nil {
					return err
				}
				if err := fs.WriteString(filepath.Join(ctx.RootDir, "backend", "main.go"), "package main"); err != nil {
					return err
				}

				return fs.WriteString(filepath.Join(ctx.RootDir, ".grove", "rules"), "@tree: frontend/\n@tree: backend/")
			}),

			harness.NewStep("Verify both trees appear", func(ctx *harness.Context) error {
				cxBinary, err := FindProjectBinary()
				if err != nil {
					return err
				}

				cmd := ctx.Command(cxBinary, "generate", "--xml=false").Dir(ctx.RootDir)
				result := cmd.Run()
				ctx.ShowCommandOutput(cmd.String(), result.Stdout, result.Stderr)
				if result.Error != nil {
					return fmt.Errorf("cx generate failed: %w", result.Error)
				}

				contextPath := findContextFileOrFallback(ctx.RootDir)
				content, err := fs.ReadString(contextPath)
				if err != nil {
					return fmt.Errorf("could not read context file: %w", err)
				}

				return ctx.Verify(func(v *verify.Collector) {
					v.Contains("frontend tree header", content, "=== TREE: frontend/ ===")
					v.Contains("frontend tree footer", content, "=== END TREE: frontend/ ===")
					v.Contains("backend tree header", content, "=== TREE: backend/ ===")
					v.Contains("backend tree footer", content, "=== END TREE: backend/ ===")
					v.Contains("frontend file in tree", content, "app.js")
					v.Contains("backend file in tree", content, "main.go")
				})
			}),
		},
	}
}

// TreeDirectiveNonexistentScenario tests that a non-existent path warns but doesn't fail.
func TreeDirectiveNonexistentScenario() *harness.Scenario {
	return &harness.Scenario{
		Name:        "cx-tree-nonexistent",
		Description: "Tests @tree directive with non-existent path warns gracefully",
		Tags:        []string{"cx", "tree-directive"},
		Steps: []harness.Step{
			harness.NewStep("Setup project with nonexistent tree path", func(ctx *harness.Context) error {
				if result := command.New("git", "init").Dir(ctx.RootDir).Run(); result.Error != nil {
					return fmt.Errorf("git init failed: %w", result.Error)
				}

				if err := fs.WriteString(filepath.Join(ctx.RootDir, "main.go"), "package main"); err != nil {
					return err
				}

				return fs.WriteString(filepath.Join(ctx.RootDir, ".grove", "rules"), "@tree: nonexistent_dir/\nmain.go")
			}),

			harness.NewStep("Verify command succeeds and file content still generated", func(ctx *harness.Context) error {
				cxBinary, err := FindProjectBinary()
				if err != nil {
					return err
				}

				cmd := ctx.Command(cxBinary, "generate", "--xml=false").Dir(ctx.RootDir)
				result := cmd.Run()
				ctx.ShowCommandOutput(cmd.String(), result.Stdout, result.Stderr)

				if result.Error != nil {
					return fmt.Errorf("cx generate should not fail for non-existent tree path: %w", result.Error)
				}

				contextPath := findContextFileOrFallback(ctx.RootDir)
				content, err := fs.ReadString(contextPath)
				if err != nil {
					return fmt.Errorf("could not read context file: %w", err)
				}

				return ctx.Verify(func(v *verify.Collector) {
					v.Contains("main.go file content present", content, "=== FILE: main.go ===")
					v.NotContains("non-existent tree not in output", content, "=== TREE: nonexistent_dir/ ===")
				})
			}),
		},
	}
}

// TreeDirectiveResolveScenario tests that cx resolve strips the @tree: prefix.
func TreeDirectiveResolveScenario() *harness.Scenario {
	return &harness.Scenario{
		Name:        "cx-tree-resolve",
		Description: "Tests cx resolve strips @tree: prefix for editor plugins",
		Tags:        []string{"cx", "tree-directive"},
		Steps: []harness.Step{
			harness.NewStep("Setup project with src directory", func(ctx *harness.Context) error {
				if result := command.New("git", "init").Dir(ctx.RootDir).Run(); result.Error != nil {
					return fmt.Errorf("git init failed: %w", result.Error)
				}

				return fs.WriteString(filepath.Join(ctx.RootDir, "src", "main.go"), "package main")
			}),

			harness.NewStep("Verify cx resolve strips @tree: prefix", func(ctx *harness.Context) error {
				cxBinary, err := FindProjectBinary()
				if err != nil {
					return err
				}

				// @tree: src/main.go should strip prefix and resolve the file
				cmd := command.New(cxBinary, "resolve", "@tree: src/main.go").Dir(ctx.RootDir)
				result := cmd.Run()
				ctx.ShowCommandOutput(cmd.String(), result.Stdout, result.Stderr)

				if result.Error != nil {
					return fmt.Errorf("cx resolve failed: %w", result.Error)
				}

				output := strings.TrimSpace(result.Stdout)

				return ctx.Verify(func(v *verify.Collector) {
					v.NotContains("no @tree prefix in output", output, "@tree")
					v.Contains("resolved file contains src/main.go", output, "src/main.go")
				})
			}),
		},
	}
}

// TreeDirectiveAliasScenario tests that @tree: works with alias references like @a:project.
func TreeDirectiveAliasScenario() *harness.Scenario {
	return &harness.Scenario{
		Name:        "cx-tree-alias",
		Description: "Tests @tree directive with alias resolution",
		Tags:        []string{"cx", "tree-directive", "alias"},
		Steps: []harness.Step{
			harness.NewStep("Setup multi-project environment", func(ctx *harness.Context) error {
				// Create a groves directory with an external project
				grovesDir := filepath.Join(ctx.RootDir, "mock-groves")

				// Configure grove discovery
				groveConfigDir := filepath.Join(ctx.ConfigDir(), "grove")
				groveConfig := fmt.Sprintf("groves:\n  test:\n    path: %s\n    enabled: true\n", grovesDir)
				if err := fs.WriteString(filepath.Join(groveConfigDir, "grove.yml"), groveConfig); err != nil {
					return err
				}

				// External project: "lib-ext"
				libExtDir := filepath.Join(grovesDir, "lib-ext")
				if err := fs.WriteString(filepath.Join(libExtDir, "grove.yml"), "name: lib-ext"); err != nil {
					return err
				}
				if err := fs.WriteString(filepath.Join(libExtDir, "src", "core.go"), "package src"); err != nil {
					return err
				}
				if err := fs.WriteString(filepath.Join(libExtDir, "src", "util", "helper.go"), "package util"); err != nil {
					return err
				}
				if result := command.New("git", "init").Dir(libExtDir).Run(); result.Error != nil {
					return fmt.Errorf("git init lib-ext failed: %w", result.Error)
				}

				// Main project
				if result := command.New("git", "init").Dir(ctx.RootDir).Run(); result.Error != nil {
					return fmt.Errorf("git init failed: %w", result.Error)
				}
				if err := fs.WriteString(filepath.Join(ctx.RootDir, "grove.yml"), "name: test-main"); err != nil {
					return err
				}
				return fs.WriteString(filepath.Join(ctx.RootDir, ".grove", "rules"), "@tree: @a:lib-ext\nmain.go")
			}),

			harness.NewStep("Verify alias tree generates correctly", func(ctx *harness.Context) error {
				cxBinary, err := FindProjectBinary()
				if err != nil {
					return err
				}

				// Write a dummy main.go so there's at least one file pattern match
				if err := fs.WriteString(filepath.Join(ctx.RootDir, "main.go"), "package main"); err != nil {
					return err
				}

				cmd := ctx.Command(cxBinary, "generate", "--xml=false").Dir(ctx.RootDir)
				result := cmd.Run()
				ctx.ShowCommandOutput(cmd.String(), result.Stdout, result.Stderr)
				if result.Error != nil {
					return fmt.Errorf("cx generate failed: %w", result.Error)
				}

				contextPath := findContextFileOrFallback(ctx.RootDir)
				content, err := fs.ReadString(contextPath)
				if err != nil {
					return fmt.Errorf("could not read context file: %w", err)
				}

				return ctx.Verify(func(v *verify.Collector) {
					v.Contains("tree section for aliased project", content, "=== TREE:")
					v.Contains("src directory in aliased tree", content, "src/")
					v.Contains("core.go in aliased tree", content, "core.go")
					v.Contains("helper.go in aliased tree", content, "helper.go")
					v.Contains("util directory in aliased tree", content, "util/")
					v.NotContains("no glob pattern in tree header", content, "**")
				})
			}),
		},
	}
}
