package cmd

import (
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
			}
			return mgr.ShowContext()
		},
	}

	AddRulesFileFlags(cmd, &jobFile, &rulesFile)

	return cmd
}
