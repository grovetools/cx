package cmd

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/grovetools/cx/pkg/context"
)

func NewListCmd() *cobra.Command {
	var jobFile, rulesFile string
	var relPaths bool

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List files in context",
		Long:  `Lists the absolute paths of all files in the context. Use --rel for paths relative to the rules base directory.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			mgr := context.NewManager(GetWorkDir())
			mgr.SetContext(cmd.Context())

			targetRulesFile, err := ResolveRulesFileFlag(mgr, jobFile, rulesFile)
			if err != nil {
				return err
			}

			var files []string
			if targetRulesFile != "" {
				// ResolveFilesFromCustomRulesFile returns rulesBaseDir-relative
				// paths (flattenAttrResult's form, which internal consumers
				// depend on); the command layer projects them to absolute so
				// the output matches the documented help text and reveals which
				// worktree each file rooted into.
				hotFiles, _, resolveErr := mgr.ResolveFilesFromCustomRulesFile(targetRulesFile)
				if resolveErr != nil {
					return fmt.Errorf("failed to resolve files from rules file: %w", resolveErr)
				}
				files = hotFiles
			} else {
				// ListFiles already yields absolute paths.
				files, err = mgr.ListFiles()
				if err != nil {
					return err
				}
			}

			if len(files) == 0 && targetRulesFile == "" {
				if _, rulesPath, _ := mgr.LoadRulesContent(); rulesPath == "" {
					fmt.Fprintln(cmd.ErrOrStderr(), "hint: no context rules found — create one with 'cx edit' (see 'cx rules where')")
					return nil
				}
			}

			base := mgr.GetRulesBaseDir()
			for _, file := range files {
				fmt.Println(projectListPath(file, base, relPaths))
			}
			return nil
		},
	}

	AddRulesFileFlags(cmd, &jobFile, &rulesFile)
	cmd.Flags().BoolVar(&relPaths, "rel", false, "print paths relative to the rules base directory instead of absolute")

	return cmd
}

// projectListPath renders a resolved file path in the requested form. Inputs
// may be absolute or relative to base (the two branches above differ); this
// normalizes both so a single --rel flag controls the whole output.
func projectListPath(file, base string, rel bool) string {
	if rel {
		if filepath.IsAbs(file) {
			if r, err := filepath.Rel(base, file); err == nil && !strings.HasPrefix(r, "..") {
				return r
			}
		}
		return file
	}
	if !filepath.IsAbs(file) {
		return filepath.Join(base, file)
	}
	return file
}
