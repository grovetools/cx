package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"github.com/mattsolo1/grove-core/pkg/workspace"
	"github.com/sirupsen/logrus"

	"github.com/spf13/cobra"
)

// NewWorkspaceCmd creates the 'workspace' command and its subcommands.
func NewWorkspaceCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "workspace",
		Short: "Interact with discovered workspaces",
		Long:  `List and manage workspaces (projects, ecosystems, worktrees) discovered by Grove.`,
	}

	cmd.AddCommand(newWorkspaceListCmd())

	return cmd
}

// newWorkspaceListCmd creates the 'workspace list' command.
func newWorkspaceListCmd() *cobra.Command {
	var jsonOutput bool

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all discovered workspaces",
		Long:  `Outputs a list of all projects, ecosystems, and worktrees discovered from your Grove configuration.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Use a temporary logger for the discovery process.
			logger := logrus.New()

			// When JSON output is requested, completely silence the logger
			// to ensure clean JSON output
			if jsonOutput {
				logger.SetOutput(io.Discard)
			} else {
				logger.SetLevel(logrus.WarnLevel)
			}

			discoveryService := workspace.NewDiscoveryService(logger)
			discoveryResult, err := discoveryService.DiscoverAll()
			if err != nil {
				return fmt.Errorf("failed to discover workspaces: %w", err)
			}

			projectInfos := workspace.TransformToProjectInfo(discoveryResult)

			if jsonOutput {
				// For JSON output, we enrich the data with the identifier used for aliases.
				type jsonProjectInfo struct {
					*workspace.ProjectInfo
					Identifier string `json:"identifier"`
				}

				output := make([]jsonProjectInfo, len(projectInfos))
				for i, p := range projectInfos {
					output[i] = jsonProjectInfo{
						ProjectInfo: p,
						Identifier:  p.Identifier(),
					}
				}

				jsonData, err := json.MarshalIndent(output, "", "  ")
				if err != nil {
					return fmt.Errorf("failed to marshal project info to JSON: %w", err)
				}
				fmt.Println(string(jsonData))
			} else {
				// Simple text output for human consumption.
				for _, p := range projectInfos {
					fmt.Printf("- %s (%s)\n", p.Identifier(), p.Path)
				}
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output workspace information in JSON format")

	return cmd
}
