package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/mattsolo1/grove-context/pkg/context"
	"github.com/spf13/cobra"
)

func NewResolveCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "resolve [rule]",
		Short: "Resolve a single rule pattern to a list of files",
		Long:  `Accepts a single inclusion rule (glob or alias) and prints the list of files it resolves to. Primarily for use by editor integrations.`,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ruleLine := args[0]

			// Do not process exclusion rules, as they don't resolve to a file list on their own.
			if strings.HasPrefix(strings.TrimSpace(ruleLine), "!") {
				// Print nothing and exit successfully.
				return nil
			}

			mgr := context.NewManager(".")

			// First, resolve the line. This one call now handles simple globs, aliases,
			// and ruleset imports, returning a potentially multi-line string of patterns.
			resolvedPatternsStr, err := mgr.ResolveLineForRulePreview(ruleLine)
			if err != nil {
				// If resolution fails, it's a non-fatal warning for the user.
				// Print to stderr so it can be captured by the calling plugin.
				fmt.Fprintf(os.Stderr, "Warning: could not resolve rule: %v\n", err)
				// Fallback to using the original line as the pattern.
				resolvedPatternsStr = ruleLine
			}

			// Split the result into individual patterns (for ruleset imports)
			patterns := strings.Split(resolvedPatternsStr, "\n")

			// Use the manager's file resolution logic with the patterns.
			// Note: ResolveFilesFromPatterns expects a slice and now handles brace expansion internally.
			files, err := mgr.ResolveFilesFromPatterns(patterns)
			if err != nil {
				return fmt.Errorf("error resolving files for rule '%s': %w", ruleLine, err)
			}

			// Print the list of files to stdout, one per line.
			for _, file := range files {
				fmt.Println(file)
			}

			return nil
		},
	}
	return cmd
}
