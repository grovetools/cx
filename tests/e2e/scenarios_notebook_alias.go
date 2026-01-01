// File: grove-context/tests/e2e/scenarios_notebook_alias.go
package main

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/mattsolo1/grove-tend/pkg/fs"
	"github.com/mattsolo1/grove-tend/pkg/harness"
)

// NotebookAliasScenario tests the @a:nb: alias for including files from notebooks.
func NotebookAliasScenario() *harness.Scenario {
	return &harness.Scenario{
		Name:        "cx-notebook-alias",
		Description: "Tests the @a:nb:[notebook]:path alias for including notebook files in context.",
		Tags:        []string{"cx", "alias", "notebook"},
		Steps: []harness.Step{
			harness.NewStep("Setup environment with multiple notebooks", func(ctx *harness.Context) error {
				// 1. Get paths for sandboxed home and config directories.
				homeDir := ctx.HomeDir()
				configDir := ctx.ConfigDir()
				groveConfigDir := filepath.Join(configDir, "grove")

				// 2. Define notebook paths within the sandboxed home.
				mainNotebookPath := filepath.Join(homeDir, "notebooks", "main")
				secondaryNotebookPath := filepath.Join(homeDir, "notebooks", "secondary")

				// 3. Create the global grove.yml with notebook definitions.
				globalYAML := fmt.Sprintf(`
notebooks:
  definitions:
    main:
      root_dir: "%s"
    secondary:
      root_dir: "%s"
  rules:
    default: "main"
`, mainNotebookPath, secondaryNotebookPath)

				if err := fs.WriteString(filepath.Join(groveConfigDir, "grove.yml"), globalYAML); err != nil {
					return fmt.Errorf("failed to write global grove.yml: %w", err)
				}

				// 4. Create the notebook directories and sample files.
				if err := fs.WriteString(filepath.Join(mainNotebookPath, "note1.md"), "Content from main notebook."); err != nil {
					return err
				}
				if err := fs.WriteString(filepath.Join(secondaryNotebookPath, "note2.md"), "Content from secondary notebook."); err != nil {
					return err
				}

				return nil
			}),
			harness.NewStep("Create rules file with notebook aliases", func(ctx *harness.Context) error {
				rules := `# This should resolve to the 'main' (default) notebook
@a:nb:note1.md

# This should resolve to the 'secondary' notebook
@a:nb:secondary:note2.md`
				return fs.WriteString(filepath.Join(ctx.RootDir, ".grove", "rules"), rules)
			}),
			harness.NewStep("Run 'cx list' and verify alias resolution", func(ctx *harness.Context) error {
				cxBinary, err := FindProjectBinary()
				if err != nil {
					return err
				}

				// Run 'cx list' with the custom config home to ensure it picks up our mock config.
				cmd := ctx.Command(cxBinary, "list").Dir(ctx.RootDir)
				result := cmd.Run()
				ctx.ShowCommandOutput(cmd.String(), result.Stdout, result.Stderr)
				if result.Error != nil {
					return result.Error
				}

				output := result.Stdout
				homeDir := ctx.HomeDir()

				// Verify the default notebook alias resolved correctly.
				expectedPath1 := filepath.Join(homeDir, "notebooks", "main", "note1.md")
				if !strings.Contains(output, expectedPath1) {
					return fmt.Errorf("output missing file from default notebook alias: expected '%s', got:\n%s", expectedPath1, output)
				}

				// Verify the explicitly named notebook alias resolved correctly.
				expectedPath2 := filepath.Join(homeDir, "notebooks", "secondary", "note2.md")
				if !strings.Contains(output, expectedPath2) {
					return fmt.Errorf("output missing file from named notebook alias: expected '%s', got:\n%s", expectedPath2, output)
				}

				return nil
			}),
		},
	}
}
