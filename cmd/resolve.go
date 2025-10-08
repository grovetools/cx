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

			mgr := context.NewManager("")

			// First, resolve any potential alias in the line.
			// The alias resolver is lazily initialized within the manager.
			resolvedPattern, err := mgr.ResolveLineForRulePreview(ruleLine)
			if err != nil {
				// If alias resolution fails, it's a non-fatal warning for the user.
				// Print to stderr so it can be captured by the calling plugin.
				fmt.Fprintf(os.Stderr, "Warning: could not resolve alias: %v\n", err)
				// Fallback to using the original line as the pattern.
				resolvedPattern = ruleLine
			}

			// Use the manager's file resolution logic with the single pattern.
			// Note: ResolveFilesFromPatterns expects a slice.
			files, err := mgr.ResolveFilesFromPatterns([]string{resolvedPattern})
			if err != nil {
				return fmt.Errorf("error resolving files for pattern '%s': %w", resolvedPattern, err)
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
