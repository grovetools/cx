package main

import (
	"encoding/json"
	"fmt"
	"path/filepath"

	"github.com/grovetools/tend/pkg/fs"
	"github.com/grovetools/tend/pkg/harness"
)

// AliasListJSONScenario asserts that `cx alias list --json` emits valid JSON
// containing the expected structured fields.
func AliasListJSONScenario() *harness.Scenario {
	return &harness.Scenario{
		Name:        "cx-alias-list-json",
		Description: "alias list --json emits well-formed JSON with alias/path fields",
		Tags:        []string{"cx", "alias", "json"},
		Steps: []harness.Step{
			harness.NewStep("Setup ecosystem with one workspace", func(ctx *harness.Context) error {
				grovesDir := filepath.Join(ctx.RootDir, "mock-groves")
				groveConfigDir := filepath.Join(ctx.ConfigDir(), "grove")
				groveConfig := fmt.Sprintf("groves:\n  test:\n    path: %s\n    enabled: true\n", grovesDir)
				if err := fs.WriteString(filepath.Join(groveConfigDir, "grove.yml"), groveConfig); err != nil {
					return err
				}
				wsDir := filepath.Join(grovesDir, "alpha")
				if err := fs.WriteString(filepath.Join(wsDir, "grove.yml"), "name: alpha\n"); err != nil {
					return err
				}
				if err := fs.WriteString(filepath.Join(wsDir, "main.go"), "package alpha"); err != nil {
					return err
				}
				return fs.WriteString(filepath.Join(ctx.RootDir, "main.go"), "package main")
			}),
			harness.NewStep("Run 'cx alias list --json' and parse", func(ctx *harness.Context) error {
				cxBinary, err := FindProjectBinary()
				if err != nil {
					return err
				}
				cmd := ctx.Command(cxBinary, "alias", "list", "--json").Dir(ctx.RootDir)
				result := cmd.Run()
				ctx.ShowCommandOutput(cmd.String(), result.Stdout, result.Stderr)
				if result.Error != nil {
					return fmt.Errorf("alias list --json failed: %w", result.Error)
				}

				var entries []map[string]any
				if err := json.Unmarshal([]byte(result.Stdout), &entries); err != nil {
					return fmt.Errorf("expected valid JSON, got error %v; stdout=%q", err, result.Stdout)
				}
				if len(entries) == 0 {
					return fmt.Errorf("expected at least one alias entry; stdout=%q", result.Stdout)
				}
				for _, e := range entries {
					if _, ok := e["alias"].(string); !ok {
						return fmt.Errorf("entry missing 'alias' string: %#v", e)
					}
					if _, ok := e["path"].(string); !ok {
						return fmt.Errorf("entry missing 'path' string: %#v", e)
					}
					if _, ok := e["is_current"].(bool); !ok {
						return fmt.Errorf("entry missing 'is_current' bool: %#v", e)
					}
				}
				return nil
			}),
			harness.NewStep("Verify cx-repos hidden by default", func(ctx *harness.Context) error {
				cxBinary, err := FindProjectBinary()
				if err != nil {
					return err
				}
				cmd := ctx.Command(cxBinary, "alias", "list", "--json").Dir(ctx.RootDir)
				result := cmd.Run()
				if result.Error != nil {
					return result.Error
				}
				var entries []map[string]any
				if err := json.Unmarshal([]byte(result.Stdout), &entries); err != nil {
					return err
				}
				for _, e := range entries {
					if eco, _ := e["ecosystem"].(string); eco == "cx-repos" {
						return fmt.Errorf("cx-repos should be hidden by default: %#v", e)
					}
				}
				return nil
			}),
		},
	}
}
