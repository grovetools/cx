package cmd

import (
	stdctx "context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"text/tabwriter"

	"github.com/grovetools/core/pkg/alias"
	"github.com/grovetools/core/pkg/workspace"
	"github.com/spf13/cobra"
)

// NewAliasCmd creates the 'alias' command and its subcommands.
func NewAliasCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "alias",
		Short: "Manage and list context aliases",
		Long:  `View available @a: aliases that can be used in your context rules.`,
	}
	cmd.AddCommand(newAliasListCmd())
	return cmd
}

func newAliasListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all valid @a: context aliases",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := stdctx.Background()
			workDir := GetWorkDir()
			if workDir == "" {
				workDir, _ = os.Getwd()
			}
			workDir, _ = filepath.Abs(workDir)

			resolver := alias.NewAliasResolver()
			resolver.InitProvider()
			if resolver.Provider == nil {
				return fmt.Errorf("failed to initialize workspace provider")
			}

			currentNode, _ := workspace.GetProjectByPath(workDir)

			if currentNode != nil && currentNode.Kind != workspace.KindNonGroveRepo {
				ulog.Info("Current context").
					Pretty(fmt.Sprintf("Current workspace: %s (from %s)\n", currentNode.Identifier(":"), currentNode.Path)).
					Log(ctx)
			}

			// Group nodes by ecosystem
			ecosystems := make(map[string][]*workspace.WorkspaceNode)
			var standalone []*workspace.WorkspaceNode

			for _, node := range resolver.Provider.All() {
				if node.RootEcosystemPath != "" {
					rootNode := resolver.Provider.FindByPath(node.RootEcosystemPath)
					ecoName := "Unknown"
					if rootNode != nil {
						ecoName = rootNode.Name
					}
					ecosystems[ecoName] = append(ecosystems[ecoName], node)
				} else {
					standalone = append(standalone, node)
				}
			}

			w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)

			// Print ecosystems
			var ecoNames []string
			for name := range ecosystems {
				ecoNames = append(ecoNames, name)
			}
			sort.Strings(ecoNames)

			for _, ecoName := range ecoNames {
				fmt.Fprintf(w, "\nEcosystem: %s\n", ecoName)

				nodes := ecosystems[ecoName]
				sort.Slice(nodes, func(i, j int) bool {
					return nodes[i].Path < nodes[j].Path
				})

				for _, node := range nodes {
					aliasStr := "@a:" + node.Identifier(":")
					indicator := ""
					if currentNode != nil && node.Path == currentNode.Path {
						indicator = "(current)"
					}
					fmt.Fprintf(w, "  %s\t%s\t%s\n", aliasStr, node.Path, indicator)
				}
			}

			// Print standalone
			if len(standalone) > 0 {
				fmt.Fprintf(w, "\nStandalone:\n")
				sort.Slice(standalone, func(i, j int) bool {
					return standalone[i].Path < standalone[j].Path
				})
				for _, node := range standalone {
					aliasStr := "@a:" + node.Identifier(":")
					indicator := ""
					if currentNode != nil && node.Path == currentNode.Path {
						indicator = "(current)"
					}
					fmt.Fprintf(w, "  %s\t%s\t%s\n", aliasStr, node.Path, indicator)
				}
			}

			w.Flush()
			return nil
		},
	}
}
