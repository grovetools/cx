package cmd

import (
	"github.com/spf13/cobra"

	"github.com/grovetools/cx/pkg/context"
)

// GlobalWorkDir holds the value of the --dir / -C persistent flag.
var GlobalWorkDir string

// GetWorkDir returns the global --dir flag value, or empty string to let NewManager use CWD.
func GetWorkDir() string {
	return GlobalWorkDir
}

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
