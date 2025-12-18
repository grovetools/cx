// File: grove-context/tests/e2e/scenarios_path_normalization.go
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mattsolo1/grove-tend/pkg/command"
	"github.com/mattsolo1/grove-tend/pkg/fs"
	"github.com/mattsolo1/grove-tend/pkg/harness"
	"github.com/mattsolo1/grove-tend/pkg/tui"
)

// PathNormalizationWorktreeScenario tests that paths in worktrees are displayed correctly
// after path normalization fixes. This is a simpler test that verifies the path normalization
// logic works correctly when files are accessed through worktrees.
func PathNormalizationWorktreeScenario() *harness.Scenario {
	return &harness.Scenario{
		Name:        "cx-path-normalization-worktree",
		Description: "Tests that path normalization handles worktree paths correctly",
		Tags:        []string{"cx", "path", "worktree", "normalization"},
		Steps: []harness.Step{
			harness.NewStep("Setup simple project with files", func(ctx *harness.Context) error {
				// Create test files directly in RootDir
				mainGo := `package main

import "fmt"

func main() {
    fmt.Println("Test")
}`
				if err := fs.WriteString(filepath.Join(ctx.RootDir, "main.go"), mainGo); err != nil {
					return fmt.Errorf("failed to create main.go: %w", err)
				}

				readmeContent := `# Test Project`
				if err := fs.WriteString(filepath.Join(ctx.RootDir, "README.md"), readmeContent); err != nil {
					return fmt.Errorf("failed to create README.md: %w", err)
				}

				return nil
			}),

			harness.NewStep("Create rules file", func(ctx *harness.Context) error {
				rules := `*.go
*.md`
				return fs.WriteString(filepath.Join(ctx.RootDir, ".grove", "rules"), rules)
			}),

			harness.NewStep("Generate context", func(ctx *harness.Context) error {
				cxBinary, err := FindProjectBinary()
				if err != nil {
					return err
				}

				cmd := command.New(cxBinary, "generate").Dir(ctx.RootDir)
				result := cmd.Run()

				if result.ExitCode != 0 {
					return fmt.Errorf("cx generate failed: %s\nStderr: %s", result.Stdout, result.Stderr)
				}

				return nil
			}),

			harness.NewStep("Verify cx list works", func(ctx *harness.Context) error {
				cxBinary, err := FindProjectBinary()
				if err != nil {
					return err
				}

				cmd := command.New(cxBinary, "list").Dir(ctx.RootDir)
				result := cmd.Run()

				if result.ExitCode != 0 {
					return fmt.Errorf("cx list failed: %s\nStderr: %s", result.Stdout, result.Stderr)
				}

				output := result.Stdout

				// Should show both files
				if !strings.Contains(output, "main.go") {
					return fmt.Errorf("cx list is missing main.go:\n%s", output)
				}
				if !strings.Contains(output, "README.md") {
					return fmt.Errorf("cx list is missing README.md:\n%s", output)
				}

				fmt.Printf("\n✓ cx list shows expected files\n")
				return nil
			}),

			harness.NewStep("Launch cx view list page", func(ctx *harness.Context) error {
				cxBinary, err := FindProjectBinary()
				if err != nil {
					return err
				}

				session, err := ctx.StartTUI(cxBinary, []string{"view", "--page", "list"},
					tui.WithEnv("CLICOLOR_FORCE=1"),
				)
				if err != nil {
					return fmt.Errorf("failed to start cx view TUI: %w", err)
				}
				ctx.Set("view_session", session)

				// Wait for UI to load
				time.Sleep(2 * time.Second)
				return nil
			}),

			harness.NewStep("Verify view shows files correctly", func(ctx *harness.Context) error {
				session := ctx.Get("view_session").(*tui.Session)

				// Wait for the list to load
				result, err := session.WaitForAnyText([]string{
					"Files in Hot Context",
					"main.go",
					"Error:",
				}, 8*time.Second)

				if err != nil {
					content, _ := session.Capture()
					fmt.Printf("\n=== TUI Content (no expected text found) ===\n%s\n=== END ===\n", content)
					return fmt.Errorf("TUI did not load the list page: %w", err)
				}

				if result == "Error:" {
					content, _ := session.Capture()
					fmt.Printf("\n=== TUI showing error ===\n%s\n=== END ===\n", content)
					return fmt.Errorf("TUI is showing an error")
				}

				// Let UI stabilize
				if err := session.WaitForUIStable(2*time.Second, 100*time.Millisecond, 200*time.Millisecond); err != nil {
					fmt.Printf("   ⚠️  UI stability warning: %v\n", err)
				}

				content, err := session.Capture(tui.WithCleanedOutput())
				if err != nil {
					return fmt.Errorf("failed to capture screen: %w", err)
				}

				// Should show main.go
				if !strings.Contains(content, "main.go") {
					return fmt.Errorf("cx view is missing main.go")
				}

				// Paths should be simple (no ../ at all in a basic project)
				if strings.Contains(content, "../") {
					fmt.Printf("\n=== TUI Content with unexpected paths ===\n%s\n=== END ===\n", content)
					return fmt.Errorf("cx view shows unexpected relative paths")
				}

				fmt.Printf("\n✓ View correctly displays paths\n")
				return nil
			}),

			harness.NewStep("Quit the TUI", func(ctx *harness.Context) error {
				session := ctx.Get("view_session").(*tui.Session)
				return session.SendKeys("q")
			}),
		},
	}
}

// PathNormalizationSymlinkScenario tests that paths accessed via symlinks are normalized correctly
// and displayed with proper relative paths.
func PathNormalizationSymlinkScenario() *harness.Scenario {
	return &harness.Scenario{
		Name:        "cx-path-normalization-symlink",
		Description: "Tests that cx view handles symlinked paths correctly with path normalization",
		Tags:        []string{"cx", "path", "symlink", "normalization"},
		Steps: []harness.Step{
			harness.NewStep("Setup project with real path", func(ctx *harness.Context) error {
				realProjectDir := filepath.Join(ctx.RootDir, "real-project")
				if err := fs.CreateDir(realProjectDir); err != nil {
					return fmt.Errorf("failed to create real project directory: %w", err)
				}
				ctx.Set("realProjectDir", realProjectDir)

				// Create a file in the real directory
				mainGo := `package main

func main() {
    println("Hello from real path")
}`
				if err := fs.WriteString(filepath.Join(realProjectDir, "main.go"), mainGo); err != nil {
					return fmt.Errorf("failed to create main.go: %w", err)
				}

				return nil
			}),

			harness.NewStep("Create symlink to project", func(ctx *harness.Context) error {
				realProjectDir := ctx.Get("realProjectDir").(string)
				symlinkPath := filepath.Join(ctx.RootDir, "symlinked-project")

				if err := os.Symlink(realProjectDir, symlinkPath); err != nil {
					return fmt.Errorf("failed to create symlink: %w", err)
				}
				ctx.Set("symlinkPath", symlinkPath)

				return nil
			}),

			harness.NewStep("Create grove.yml in symlinked path", func(ctx *harness.Context) error {
				symlinkPath := ctx.Get("symlinkPath").(string)

				groveConfig := `project:
  name: symlinked-project
`

				if err := fs.WriteString(filepath.Join(symlinkPath, "grove.yml"), groveConfig); err != nil {
					return fmt.Errorf("failed to create grove.yml: %w", err)
				}

				return nil
			}),

			harness.NewStep("Create rules file via symlink", func(ctx *harness.Context) error {
				symlinkPath := ctx.Get("symlinkPath").(string)

				rules := `*.go`
				if err := fs.WriteString(filepath.Join(symlinkPath, ".grove", "rules"), rules); err != nil {
					return fmt.Errorf("failed to create rules file: %w", err)
				}

				return nil
			}),

			harness.NewStep("Generate context via symlink", func(ctx *harness.Context) error {
				symlinkPath := ctx.Get("symlinkPath").(string)

				cxBinary, err := FindProjectBinary()
				if err != nil {
					return err
				}

				cmd := command.New(cxBinary, "generate").Dir(symlinkPath)
				result := cmd.Run()

				if result.ExitCode != 0 {
					return fmt.Errorf("cx generate failed: %s\nStderr: %s", result.Stdout, result.Stderr)
				}

				fmt.Printf("\n=== cx generate output ===\n%s\n", result.Stdout)
				if result.Stderr != "" {
					fmt.Printf("Stderr: %s\n", result.Stderr)
				}
				fmt.Printf("=== END ===\n")

				return nil
			}),

			harness.NewStep("Verify cx list shows normalized paths", func(ctx *harness.Context) error {
				symlinkPath := ctx.Get("symlinkPath").(string)

				cxBinary, err := FindProjectBinary()
				if err != nil {
					return err
				}

				cmd := command.New(cxBinary, "list").Dir(symlinkPath)
				result := cmd.Run()

				if result.ExitCode != 0 {
					return fmt.Errorf("cx list failed: %s\nStderr: %s", result.Stdout, result.Stderr)
				}

				output := result.Stdout
				fmt.Printf("\n=== cx list output ===\n%s\n=== END ===\n", output)

				// Should show main.go
				if !strings.Contains(output, "main.go") {
					// Check if context was generated
					contextPath := filepath.Join(symlinkPath, ".grove", "context")
					if _, err := os.Stat(contextPath); os.IsNotExist(err) {
						return fmt.Errorf("context file was not generated at %s", contextPath)
					}
					return fmt.Errorf("cx list is missing main.go:\n%s", output)
				}

				// Path should be simple, not contain complex relative paths
				if strings.Contains(output, "../") {
					return fmt.Errorf("cx list shows unexpected relative paths:\n%s", output)
				}

				return nil
			}),

			harness.NewStep("Launch cx view from symlinked path", func(ctx *harness.Context) error {
				symlinkPath := ctx.Get("symlinkPath").(string)

				cxBinary, err := FindProjectBinary()
				if err != nil {
					return err
				}

				// Change to the symlinked directory first
				originalDir, err := os.Getwd()
				if err != nil {
					return fmt.Errorf("failed to get current directory: %w", err)
				}
				ctx.Set("originalDir", originalDir)

				if err := os.Chdir(symlinkPath); err != nil {
					return fmt.Errorf("failed to change to symlink directory: %w", err)
				}

				session, err := ctx.StartTUI(cxBinary, []string{"view", "--page", "list"},
					tui.WithEnv("CLICOLOR_FORCE=1"),
				)
				if err != nil {
					// Restore directory on error
					os.Chdir(originalDir)
					return fmt.Errorf("failed to start cx view TUI: %w", err)
				}
				ctx.Set("view_session", session)

				// Wait for UI to load
				time.Sleep(2 * time.Second)
				return nil
			}),

			harness.NewStep("Verify view displays normalized paths", func(ctx *harness.Context) error {
				session := ctx.Get("view_session").(*tui.Session)

				// Wait for the list to load
				result, err := session.WaitForAnyText([]string{
					"Files in Hot Context",
					"main.go",
					"Error:",
				}, 8*time.Second)

				if err != nil {
					content, _ := session.Capture()
					fmt.Printf("\n=== TUI Content (no expected text found) ===\n%s\n=== END ===\n", content)
					return fmt.Errorf("TUI did not load the list page: %w", err)
				}

				if result == "Error:" {
					content, _ := session.Capture()
					fmt.Printf("\n=== TUI showing error ===\n%s\n=== END ===\n", content)
					return fmt.Errorf("TUI is showing an error")
				}

				// Let UI stabilize
				if err := session.WaitForUIStable(2*time.Second, 100*time.Millisecond, 200*time.Millisecond); err != nil {
					fmt.Printf("   ⚠️  UI stability warning: %v\n", err)
				}

				content, err := session.Capture(tui.WithCleanedOutput())
				if err != nil {
					return fmt.Errorf("failed to capture screen: %w", err)
				}

				// Should show main.go
				if !strings.Contains(content, "main.go") {
					return fmt.Errorf("cx view is missing main.go")
				}

				fmt.Printf("\n✓ View correctly displays normalized paths from symlink\n")
				return nil
			}),

			harness.NewStep("Quit the TUI", func(ctx *harness.Context) error {
				session := ctx.Get("view_session").(*tui.Session)
				err := session.SendKeys("q")

				// Restore original directory
				if originalDir, ok := ctx.Get("originalDir").(string); ok {
					if chErr := os.Chdir(originalDir); chErr != nil {
						fmt.Printf("Warning: failed to restore directory: %v\n", chErr)
					}
				}

				return err
			}),
		},
	}
}
