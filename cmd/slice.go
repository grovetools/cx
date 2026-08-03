package cmd

import (
	"fmt"

	"github.com/grovetools/core/logging"
	"github.com/grovetools/core/pkg/workspace"
	"github.com/spf13/cobra"

	"github.com/grovetools/cx/pkg/context"
)

const (
	// sliceStatsTop and sliceStatsManifestLimit bound the --stats envelope the
	// same way `cx stats --format compact` bounds its own. Slices are small by
	// construction (a widen is a handful of files), so the manifest limit is
	// set at the compact form's ceiling rather than its default: a preflight
	// that truncated the file list would defeat its own purpose.
	sliceStatsTop           = 10
	sliceStatsManifestLimit = 1000

	// sliceStatsRulesPath fills the envelope's rules_path. A slice has no rules
	// file, and reporting a real path here would point callers at a file that
	// does not describe this resolution.
	sliceStatsRulesPath = "<cx slice>"
)

func NewSliceCmd() *cobra.Command {
	var stripComments, statsOnly bool

	cmd := &cobra.Command{
		Use:   "slice <line>...",
		Short: "Resolve ad-hoc rules lines and print the assembled context to stdout",
		Long: `Resolve each argument exactly as if it were a line in a rules file at the workspace root — globs, brace expansion, @a: aliases, @grep:/@changed: directives, exclusions — and print the matched files as <file path="…"> XML blocks on stdout.

Slice writes no state: no .grove/context, no cached-context, no files list. It is the read-only counterpart to 'cx generate', for callers that want a bundle in a pipe rather than an artifact on disk (notably an agent widening its own context).

Stdout carries the bundle and nothing else — every log line, warning and error goes to stderr — so the output can be consumed verbatim. Exits non-zero when the lines resolve to no files, so an empty slice can never be mistaken for an empty context.

Examples:
  cx slice 'pkg/context/*.go'                  # one glob
  cx slice '@a:core::pkg/models' 'cmd/*.go'    # alias line plus a glob
  cx slice --strip-comments 'pkg/**/*.go'      # code-only view
  cx slice --stats 'pkg/**/*.go'               # preflight size, print no content`,
		Args:         cobra.MinimumNArgs(1),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			// Stdout purity is the contract, and cx's loggers default to the
			// process stdout (core/logging global writer) — which is how
			// 'cx show' leaks log lines into its own output. Point every
			// logger at stderr for the life of the command, and restore it
			// after so nothing else inherits the redirection.
			previous := logging.SwapGlobalOutput(cmd.ErrOrStderr())
			defer logging.SetGlobalOutput(previous)

			mgr := context.NewManager(GetWorkDir())
			mgr.SetContext(ctx)
			mgr.SetStripComments(stripComments)

			workspaceName := ""
			if node, err := workspace.GetProjectByPath(mgr.GetWorkDir()); err == nil && node.Kind != workspace.KindNonGroveRepo {
				workspaceName = node.Identifier(":")
			}
			ulog.Info("Slice resolution context").
				Field("workspace", workspaceName).
				Field("root", mgr.GetRulesBaseDir()).
				Field("lines", len(args)).
				Log(ctx)

			files, treePaths, err := mgr.ResolveSlice(args)
			if err != nil {
				return err
			}
			if len(treePaths) > 0 {
				fmt.Fprintf(cmd.ErrOrStderr(), "Warning: ignoring %d @tree: path(s); cx slice renders files only\n", len(treePaths))
			}
			if len(files) == 0 {
				return fmt.Errorf("0 files resolved; check the patterns and the workspace root (%s)", mgr.GetRulesBaseDir())
			}

			// --stats reports on-disk bytes and cx's usual token estimate, so
			// with --strip-comments it is an upper bound on what the content
			// form would emit — the safe direction for a size gate.
			if statsOnly {
				envelope, err := buildMachineStats(mgr, workspaceName, sliceStatsRulesPath, files, nil, sliceStatsTop, sliceStatsManifestLimit)
				if err != nil {
					return err
				}
				return writeJSON(cmd, envelope)
			}

			return mgr.WriteSlice(cmd.OutOrStdout(), files)
		},
	}

	cmd.Flags().BoolVar(&stripComments, "strip-comments", false, "Strip code comments from included files (go/rust/ts/js/html/css)")
	cmd.Flags().BoolVar(&statsOnly, "stats", false, "Print the compact stats envelope instead of the content, to preflight size")

	return cmd
}
