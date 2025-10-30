// File: grove-context/tests/e2e/scenarios_absolute.go
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mattsolo1/grove-tend/pkg/fs"
	"github.com/mattsolo1/grove-tend/pkg/harness"
)

// AbsolutePathDirectoryPatternScenario tests that plain absolute directory paths are correctly included.
func AbsolutePathDirectoryPatternScenario() *harness.Scenario {
	return &harness.Scenario{
		Name:        "cx-absolute-path-directory-pattern",
		Description: "Tests that a plain absolute directory path in rules includes its contents.",
		Tags:        []string{"cx", "rules", "patterns", "absolute-path"},
		Steps: []harness.Step{
			harness.NewStep("Create an external directory with files", func(ctx *harness.Context) error {
				// Create a directory completely outside the test's RootDir to simulate a real absolute path.
				externalDir, err := os.MkdirTemp("", "grove-e2e-abs-")
				if err != nil {
					return fmt.Errorf("failed to create external temp dir: %w", err)
				}
				// Store external dir path in a temporary file so we can access it in later steps
				externalDirFile := filepath.Join(ctx.RootDir, ".external_dir_path")
				if err := fs.WriteString(externalDirFile, externalDir); err != nil {
					os.RemoveAll(externalDir)
					return err
				}

				// Create a file within this external directory.
				if err := fs.WriteString(filepath.Join(externalDir, "external_file.go"), "package external"); err != nil {
					os.RemoveAll(externalDir)
					return err
				}

				// Create local grove.yml with allowed_paths configuration
				groveConfig := fmt.Sprintf(`context:
  allowed_paths:
    - %s
`, externalDir)
				groveYmlPath := filepath.Join(ctx.RootDir, "grove.yml")
				if err := fs.WriteString(groveYmlPath, groveConfig); err != nil {
					os.RemoveAll(externalDir)
					return err
				}

				// Note: The external directory will be cleaned up after the test completes
				return nil
			}),
			harness.NewStep("Create rules file with an absolute path", func(ctx *harness.Context) error {
				// Read the external dir path from the file we saved
				externalDirBytes, err := os.ReadFile(filepath.Join(ctx.RootDir, ".external_dir_path"))
				if err != nil {
					return fmt.Errorf("failed to read external dir path: %w", err)
				}
				externalDir := string(externalDirBytes)
				// The rule is just the absolute path to the directory, without any globs.
				rules := externalDir
				return fs.WriteString(filepath.Join(ctx.RootDir, ".grove", "rules"), rules)
			}),
			harness.NewStep("Run 'cx list' and verify the external file is included", func(ctx *harness.Context) error {
				// Read the external dir path
				externalDirBytes, err := os.ReadFile(filepath.Join(ctx.RootDir, ".external_dir_path"))
				if err != nil {
					return fmt.Errorf("failed to read external dir path: %w", err)
				}
				externalDir := string(externalDirBytes)
				defer os.RemoveAll(externalDir) // Clean up after test

				cx, _ := FindProjectBinary()
				cmd := ctx.Command(cx, "list").Dir(ctx.RootDir)
				result := cmd.Run()
				ctx.ShowCommandOutput(cmd.String(), result.Stdout, result.Stderr)
				if result.Error != nil {
					return result.Error
				}

				output := result.Stdout
				expectedFilePath := filepath.Join(externalDir, "external_file.go")

				// 'cx list' outputs absolute paths, so we can check for the full expected path.
				if !strings.Contains(output, expectedFilePath) {
					return fmt.Errorf("expected 'cx list' to include '%s', but it was not found in the output:\n%s", expectedFilePath, output)
				}
				return nil
			}),
		},
	}
}

// AbsolutePathFilePatternScenario tests that absolute file paths are correctly included.
func AbsolutePathFilePatternScenario() *harness.Scenario {
	return &harness.Scenario{
		Name:        "cx-absolute-path-file-pattern",
		Description: "Tests that an absolute file path in rules includes the specific file.",
		Tags:        []string{"cx", "rules", "patterns", "absolute-path"},
		Steps: []harness.Step{
			harness.NewStep("Create external files", func(ctx *harness.Context) error {
				// Create a directory completely outside the test's RootDir
				externalDir, err := os.MkdirTemp("", "grove-e2e-abs-file-")
				if err != nil {
					return fmt.Errorf("failed to create external temp dir: %w", err)
				}

				// Store external dir path in a temporary file
				externalDirFile := filepath.Join(ctx.RootDir, ".external_dir_path")
				if err := fs.WriteString(externalDirFile, externalDir); err != nil {
					os.RemoveAll(externalDir)
					return err
				}

				// Create multiple files to test that only the specified file is included
				if err := fs.WriteString(filepath.Join(externalDir, "target_file.go"), "package target\n\nfunc TargetFunction() {}"); err != nil {
					os.RemoveAll(externalDir)
					return err
				}
				if err := fs.WriteString(filepath.Join(externalDir, "other_file.go"), "package other\n\nfunc OtherFunction() {}"); err != nil {
					os.RemoveAll(externalDir)
					return err
				}
				if err := fs.WriteString(filepath.Join(externalDir, "third_file.go"), "package third\n\nfunc ThirdFunction() {}"); err != nil {
					os.RemoveAll(externalDir)
					return err
				}

				// Create local grove.yml with allowed_paths configuration
				groveConfig := fmt.Sprintf(`context:
  allowed_paths:
    - %s
`, externalDir)
				groveYmlPath := filepath.Join(ctx.RootDir, "grove.yml")
				if err := fs.WriteString(groveYmlPath, groveConfig); err != nil {
					os.RemoveAll(externalDir)
					return err
				}

				return nil
			}),
			harness.NewStep("Create rules file with absolute file paths", func(ctx *harness.Context) error {
				// Read the external dir path
				externalDirBytes, err := os.ReadFile(filepath.Join(ctx.RootDir, ".external_dir_path"))
				if err != nil {
					return fmt.Errorf("failed to read external dir path: %w", err)
				}
				externalDir := string(externalDirBytes)

				// Add only the absolute path to target_file.go
				targetFile := filepath.Join(externalDir, "target_file.go")
				rules := targetFile
				return fs.WriteString(filepath.Join(ctx.RootDir, ".grove", "rules"), rules)
			}),
			harness.NewStep("Run 'cx list' and verify only the target file is included", func(ctx *harness.Context) error {
				// Read the external dir path
				externalDirBytes, err := os.ReadFile(filepath.Join(ctx.RootDir, ".external_dir_path"))
				if err != nil {
					return fmt.Errorf("failed to read external dir path: %w", err)
				}
				externalDir := string(externalDirBytes)
				defer os.RemoveAll(externalDir) // Clean up after test

				cx, _ := FindProjectBinary()
				cmd := ctx.Command(cx, "list").Dir(ctx.RootDir)
				result := cmd.Run()
				ctx.ShowCommandOutput(cmd.String(), result.Stdout, result.Stderr)
				if result.Error != nil {
					return result.Error
				}

				output := result.Stdout
				targetFilePath := filepath.Join(externalDir, "target_file.go")
				otherFilePath := filepath.Join(externalDir, "other_file.go")
				thirdFilePath := filepath.Join(externalDir, "third_file.go")

				// Verify the target file is included
				if !strings.Contains(output, targetFilePath) {
					return fmt.Errorf("expected 'cx list' to include '%s', but it was not found in the output:\n%s", targetFilePath, output)
				}

				// Verify the other files are NOT included
				if strings.Contains(output, otherFilePath) {
					return fmt.Errorf("'cx list' should not include '%s', but it was found in the output:\n%s", otherFilePath, output)
				}
				if strings.Contains(output, thirdFilePath) {
					return fmt.Errorf("'cx list' should not include '%s', but it was found in the output:\n%s", thirdFilePath, output)
				}

				return nil
			}),
		},
	}
}
