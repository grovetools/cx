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

			projects, err := workspace.GetProjects(logger)
			if err != nil {
				return fmt.Errorf("failed to discover workspaces: %w", err)
			}

			if jsonOutput {
				// Create a custom struct to include the identifier in the JSON output.
				type workspaceJSON struct {
					*workspace.WorkspaceNode
					Identifier string `json:"identifier"`
				}

				jsonProjects := make([]workspaceJSON, len(projects))
				for i, p := range projects {
					jsonProjects[i] = workspaceJSON{
						WorkspaceNode: p,
						Identifier:    p.Identifier(),
					}
				}

				jsonData, err := json.MarshalIndent(jsonProjects, "", "  ")
				if err != nil {
					return fmt.Errorf("failed to marshal project info to JSON: %w", err)
				}
				fmt.Println(string(jsonData))
			} else {
				// Simple text output for human consumption.
				for _, p := range projects {
					fmt.Printf("- %s (%s)\n", p.Identifier(), p.Path)
				}
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output workspace information in JSON format")

	return cmd
}
