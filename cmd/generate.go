package cmd

import (
	stdctx "context"

	"github.com/grovetools/cx/pkg/context"
	"github.com/spf13/cobra"
)

var (
	useXMLFormat bool = true
)

func NewGenerateCmd() *cobra.Command {
	var jobFile, rulesFile string

	cmd := &cobra.Command{
		Use:   "generate",
		Short: "Generate .grove/context from .grove/context-files",
		Long:  `Reads the .grove/context-files list and generates a concatenated .grove/context file with all specified files.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := stdctx.Background()
			mgr := context.NewManager(".")

			targetRulesFile, err := ResolveRulesFileFlag(mgr, jobFile, rulesFile)
			if err != nil {
				return err
			}

			ulog.Progress("Generating context file").Log(ctx)

			if targetRulesFile != "" {
				if err := mgr.GenerateContextFromRulesFile(targetRulesFile, useXMLFormat); err != nil {
					return err
				}
			} else {
				if err := mgr.GenerateContext(useXMLFormat); err != nil {
					return err
				}
			}

			ulog.Success("Context file generated successfully").Log(ctx)

			// Only generate cached context for active scratchpad (not snapshot inspections)
			if targetRulesFile == "" {
				ulog.Progress("Generating cached context file").Log(ctx)

				if err := mgr.GenerateCachedContext(); err != nil {
					return err
				}

				ulog.Success("Cached context file generated successfully").Log(ctx)
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&useXMLFormat, "xml", true, "Use XML-style delimiters (default: true)")
	AddRulesFileFlags(cmd, &jobFile, &rulesFile)

	return cmd
}
