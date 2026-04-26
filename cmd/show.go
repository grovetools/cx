package cmd

import (
	"fmt"
	"os"

	"github.com/grovetools/cx/pkg/context"
	"github.com/spf13/cobra"
)

func NewShowCmd() *cobra.Command {
	var jobFile, rulesFile string

	cmd := &cobra.Command{
		Use:   "show",
		Short: "Print the entire context file",
		Long:  `Outputs the contents of .grove/context for piping to other applications.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			mgr := context.NewManager(GetWorkDir())
			mgr.SetContext(cmd.Context())

			targetRulesFile, err := ResolveRulesFileFlag(mgr, jobFile, rulesFile)
			if err != nil {
				return err
			}

			if targetRulesFile != "" {
				// Generate from the target rules file first so context reflects it
				if err := mgr.GenerateContextFromRulesFile(targetRulesFile, true); err != nil {
					return err
				}
			} else {
				warnIfRulesStale(mgr)
			}
			return mgr.ShowContext()
		},
	}

	AddRulesFileFlags(cmd, &jobFile, &rulesFile)

	return cmd
}

func warnIfRulesStale(mgr *context.Manager) {
	rulesPath := mgr.ResolveRulesPath()
	cachedPath := mgr.ResolveCachedContextPath()
	rulesStat, err := os.Stat(rulesPath)
	if err != nil {
		return
	}
	cachedStat, err := os.Stat(cachedPath)
	if err != nil {
		return
	}
	if rulesStat.ModTime().After(cachedStat.ModTime()) {
		fmt.Fprintln(os.Stderr, "⚠ rules edited since last generate — run `cx generate` to refresh")
	}
}
