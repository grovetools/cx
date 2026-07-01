// File: grove-context/tests/e2e/scenarios_strip_comments.go
package main

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/grovetools/tend/pkg/command"
	"github.com/grovetools/tend/pkg/fs"
	"github.com/grovetools/tend/pkg/harness"
)

// Marker tokens live only inside comments in the source file, so their
// presence/absence in the generated context proves whether comment stripping
// ran. The code tokens (stripCodeMarker, stripStringSlashes) must survive.
const (
	stripCommentOrphan = "MARKER_ORPHAN_XYZZY"
	stripCommentTrail  = "MARKER_TRAILING_XYZZY"
	stripCodeMarker    = "func StripMe()"
	stripStringSlashes = "http://example.com"
)

// stripSource is a Go file with an orphan-line comment, a trailing comment,
// and a "//" that lives inside a string literal (must be preserved).
const stripSource = "package main\n" +
	"\n" +
	"// " + stripCommentOrphan + " orphan doc comment\n" +
	"func StripMe() string {\n" +
	"\turl := \"" + stripStringSlashes + "\" // " + stripCommentTrail + "\n" +
	"\treturn url\n" +
	"}\n"

// StripCommentsScenario verifies that `cx generate --strip-comments` removes
// code comments from the generated context while leaving code (and comment-like
// text inside string literals) intact, and that the default (no flag) keeps
// comments.
func StripCommentsScenario() *harness.Scenario {
	return &harness.Scenario{
		Name:        "cx-strip-comments",
		Description: "Tests that `cx generate --strip-comments` strips code comments from the generated context (default keeps them).",
		Tags:        []string{"cx", "strip-comments"},
		Steps: []harness.Step{
			harness.NewStep("Setup project with a commented Go file", func(ctx *harness.Context) error {
				if err := fs.WriteString(filepath.Join(ctx.RootDir, "main.go"), stripSource); err != nil {
					return err
				}
				rulesPath := filepath.Join(ctx.RootDir, ".grove", "rules")
				return fs.WriteString(rulesPath, "**/*.go\n")
			}),

			harness.NewStep("Default 'cx generate' keeps comments", func(ctx *harness.Context) error {
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
				if !strings.Contains(content, "<file path=\"main.go\">") {
					return fmt.Errorf("context missing main.go, got:\n%s", content)
				}
				if !strings.Contains(content, stripCommentOrphan) {
					return fmt.Errorf("expected orphan comment %q to be preserved by default, got:\n%s", stripCommentOrphan, content)
				}
				if !strings.Contains(content, stripCommentTrail) {
					return fmt.Errorf("expected trailing comment %q to be preserved by default, got:\n%s", stripCommentTrail, content)
				}
				return nil
			}),

			harness.NewStep("'cx generate --strip-comments' removes comments", func(ctx *harness.Context) error {
				cxBinary, err := FindProjectBinary()
				if err != nil {
					return err
				}
				cmd := command.New(cxBinary, "generate", "--strip-comments").Dir(ctx.RootDir)
				result := cmd.Run()
				ctx.ShowCommandOutput(cmd.String(), result.Stdout, result.Stderr)
				if result.Error != nil {
					return result.Error
				}

				content, err := fs.ReadString(findContextFileOrFallback(ctx.RootDir))
				if err != nil {
					return fmt.Errorf("could not read generated context: %w", err)
				}
				// Code and the "//" inside the string literal survive.
				if !strings.Contains(content, stripCodeMarker) {
					return fmt.Errorf("expected code %q to survive stripping, got:\n%s", stripCodeMarker, content)
				}
				if !strings.Contains(content, stripStringSlashes) {
					return fmt.Errorf("expected string literal %q to survive stripping, got:\n%s", stripStringSlashes, content)
				}
				// Comment markers are gone.
				if strings.Contains(content, stripCommentOrphan) {
					return fmt.Errorf("orphan comment %q should have been stripped, got:\n%s", stripCommentOrphan, content)
				}
				if strings.Contains(content, stripCommentTrail) {
					return fmt.Errorf("trailing comment %q should have been stripped, got:\n%s", stripCommentTrail, content)
				}
				return nil
			}),
		},
	}
}
