package cmd

import (
	"fmt"

	"github.com/grovetools/core/pkg/workspace"
	"github.com/spf13/cobra"

	"github.com/grovetools/cx/pkg/context"
)

var useXMLFormat bool = true

func NewGenerateCmd() *cobra.Command {
	var jobFile, rulesFile string

	cmd := &cobra.Command{
		Use:   "generate",
		Short: "Generate the context file from the active rules",
		Long:  `Resolves the active rules file (run 'cx rules where' to see which one) and generates a concatenated context file with all matched files.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			mgr := context.NewManager(GetWorkDir())
			mgr.SetContext(ctx)

			targetRulesFile, err := ResolveRulesFileFlag(mgr, jobFile, rulesFile)
			if err != nil {
				return err
			}

			if targetRulesFile == "" {
				if _, rulesPath, _ := mgr.LoadRulesContent(); rulesPath == "" {
					fmt.Fprintln(cmd.ErrOrStderr(), "hint: no context rules found — create one with 'cx edit' (see 'cx rules where')")
					return nil
				}
			}

			if node, err := workspace.GetProjectByPath(mgr.GetWorkDir()); err == nil && node.Kind != workspace.KindNonGroveRepo {
				rulesDisplay := targetRulesFile
				if rulesDisplay == "" {
					rulesDisplay = mgr.ResolveRulesPath()
				}
				ulog.Info("Resolution context").
					Field("workspace", node.Identifier(":")).
					Field("rules", rulesDisplay).
					Pretty(fmt.Sprintf("Workspace: %s | Rules: %s", node.Identifier(":"), rulesDisplay)).
					Log(ctx)
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
