package cmd

import (
	"fmt"
	"os"

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
	// Defect C: a job .md passed to --rules-file would reach the rules parser and
	// fail with a cryptic "multiple '---' separators" error (its YAML frontmatter
	// has two '---' fences). Detect it and point the user at --job, which resolves
	// the job's rules_file. Explicit .rules files are unaffected.
	if rulesFile != "" {
		content, _ := os.ReadFile(rulesFile)
		if context.IsJobFile(rulesFile, content) {
			return "", fmt.Errorf("%s looks like a job file with YAML frontmatter; use --job %s to resolve its rules_file, or pass a .rules file directly to --rules-file", rulesFile, rulesFile)
		}
	}
	return rulesFile, nil
}
