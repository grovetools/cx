package cmd

import (
	"github.com/grovetools/cx/pkg/context"
	"github.com/spf13/cobra"
)

// AddRulesFileFlags adds standard --job and --rules-file flags to a command.
func AddRulesFileFlags(cmd *cobra.Command, jobFile, rulesFile *string) {
	cmd.Flags().StringVar(jobFile, "job", "", "Resolve rules from job file frontmatter")
	cmd.Flags().StringVar(rulesFile, "rules-file", "", "Use an explicit rules file directly")
}

// ResolveRulesFileFlag resolves the target rules file from the provided flags.
// Returns empty string if neither flag is set.
func ResolveRulesFileFlag(mgr *context.Manager, jobFile, rulesFile string) (string, error) {
	if jobFile != "" {
		return mgr.GetRulesFileFromJob(jobFile)
	}
	return rulesFile, nil
}
