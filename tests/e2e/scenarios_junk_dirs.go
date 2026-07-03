// File: grove-context/tests/e2e/scenarios_junk_dirs.go
package main

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/grovetools/tend/pkg/command"
	"github.com/grovetools/tend/pkg/fs"
	"github.com/grovetools/tend/pkg/harness"
)

// JunkDirExclusionScenario verifies that a directory glob (`terraform/`) does
// not implicitly ingest well-known junk directories (`.terraform/`), while an
// explicit rule naming the junk directory re-includes it.
func JunkDirExclusionScenario() *harness.Scenario {
	const (
		realSource = "resource \"null_resource\" \"x\" {}\n"
		junkMarker = "JUNK_PROVIDER_BLOB_XYZZY"
	)

	writeProject := func(ctx *harness.Context) error {
		files := map[string]string{
			"terraform/main.tf":                      realSource,
			"terraform/variables.tf":                 "variable \"y\" {}\n",
			"terraform/.terraform/providers/aws.bin": junkMarker,
			"terraform/.terraform/terraform.tfstate": "{\"" + junkMarker + "\": true}\n",
		}
		for rel, content := range files {
			if err := fs.WriteString(filepath.Join(ctx.RootDir, rel), content); err != nil {
				return err
			}
		}
		return nil
	}

	return &harness.Scenario{
		Name:        "cx-junk-dirs",
		Description: "Tests that directory globs skip junk dirs (.terraform) implicitly but honor an explicit re-include rule.",
		Tags:        []string{"cx", "junk-dirs"},
		Steps: []harness.Step{
			harness.NewStep("Directory glob skips .terraform junk", func(ctx *harness.Context) error {
				if err := writeProject(ctx); err != nil {
					return err
				}
				rulesPath := filepath.Join(ctx.RootDir, ".grove", "rules")
				if err := fs.WriteString(rulesPath, "terraform/\n"); err != nil {
					return err
				}

				cxBinary, err := FindProjectBinary()
				if err != nil {
					return err
				}
				cmd := command.New(cxBinary, "generate").Dir(ctx.RootDir)
				result := cmd.Run()
				ctx.ShowCommandOutput(cmd.String(), result.Stdout, result.Stderr)
				if result.Error != nil {
					return result.Error
				}

				content, err := fs.ReadString(findContextFileOrFallback(ctx.RootDir))
				if err != nil {
					return fmt.Errorf("could not read generated context: %w", err)
				}
				if !strings.Contains(content, "main.tf") {
					return fmt.Errorf("expected real terraform source in context, got:\n%s", content)
				}
				if strings.Contains(content, junkMarker) {
					return fmt.Errorf("junk .terraform blob leaked into context:\n%s", content)
				}
				return nil
			}),

			harness.NewStep("Explicit rule re-includes .terraform", func(ctx *harness.Context) error {
				rulesPath := filepath.Join(ctx.RootDir, ".grove", "rules")
				if err := fs.WriteString(rulesPath, "terraform/\nterraform/.terraform/**\n"); err != nil {
					return err
				}

				cxBinary, err := FindProjectBinary()
				if err != nil {
					return err
				}
				cmd := command.New(cxBinary, "generate").Dir(ctx.RootDir)
				result := cmd.Run()
				ctx.ShowCommandOutput(cmd.String(), result.Stdout, result.Stderr)
				if result.Error != nil {
					return result.Error
				}

				content, err := fs.ReadString(findContextFileOrFallback(ctx.RootDir))
				if err != nil {
					return fmt.Errorf("could not read generated context: %w", err)
				}
				if !strings.Contains(content, junkMarker) {
					return fmt.Errorf("expected explicit re-include to pull .terraform blob into context, got:\n%s", content)
				}
				return nil
			}),
		},
	}
}
