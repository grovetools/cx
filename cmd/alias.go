package cmd

import (
	stdctx "context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
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

type aliasOutput struct {
	Alias     string `json:"alias"`
	Path      string `json:"path"`
	Ecosystem string `json:"ecosystem,omitempty"`
	IsCurrent bool   `json:"is_current"`
}

func newAliasListCmd() *cobra.Command {
	var (
		jsonOut       bool
		ecosystemFlag string
		nameFlag      string
		showAll       bool
	)

	cmd := &cobra.Command{
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

			ecosystems := make(map[string][]*workspace.WorkspaceNode)
			var standalone []*workspace.WorkspaceNode

			for _, node := range resolver.Provider.All() {
				ecoName := ""
				if node.RootEcosystemPath != "" {
					rootNode := resolver.Provider.FindByPath(node.RootEcosystemPath)
					ecoName = "Unknown"
					if rootNode != nil {
						ecoName = rootNode.Name
					}
				}

				if !showAll && ecoName == "cx-repos" {
					continue
				}
				if ecosystemFlag != "" && ecoName != ecosystemFlag {
					continue
				}
				if nameFlag != "" && !strings.Contains(node.Identifier(":"), nameFlag) {
					continue
				}

				if ecoName != "" {
					ecosystems[ecoName] = append(ecosystems[ecoName], node)
				} else {
					standalone = append(standalone, node)
				}
			}

			if jsonOut {
				return emitAliasJSON(ecosystems, standalone, currentNode)
			}

			if currentNode != nil && currentNode.Kind != workspace.KindNonGroveRepo {
				ulog.Info("Current context").
					Pretty(fmt.Sprintf("Current workspace: %s (from %s)\n", currentNode.Identifier(":"), currentNode.Path)).
					Log(ctx)
			}

			w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)

			var ecoNames []string
			for name := range ecosystems {
				ecoNames = append(ecoNames, name)
			}
			sort.Strings(ecoNames)

			for _, ecoName := range ecoNames {
				fmt.Fprintf(w, "\nEcosystem: %s\n", ecoName)
				nodes := ecosystems[ecoName]
				sort.Slice(nodes, func(i, j int) bool { return nodes[i].Path < nodes[j].Path })
				for _, node := range nodes {
					indicator := ""
					if currentNode != nil && node.Path == currentNode.Path {
						indicator = "(current)"
					}
					fmt.Fprintf(w, "  %s\t%s\t%s\n", "@a:"+node.Identifier(":"), node.Path, indicator)
				}
			}

			if len(standalone) > 0 {
				fmt.Fprintf(w, "\nStandalone:\n")
				sort.Slice(standalone, func(i, j int) bool { return standalone[i].Path < standalone[j].Path })
				for _, node := range standalone {
					indicator := ""
					if currentNode != nil && node.Path == currentNode.Path {
						indicator = "(current)"
					}
					fmt.Fprintf(w, "  %s\t%s\t%s\n", "@a:"+node.Identifier(":"), node.Path, indicator)
				}
			}

			w.Flush()
			return nil
		},
	}

	cmd.Flags().BoolVar(&jsonOut, "json", false, "Output as JSON")
	cmd.Flags().StringVar(&ecosystemFlag, "ecosystem", "", "Filter by ecosystem name")
	cmd.Flags().StringVar(&nameFlag, "name", "", "Filter by substring match on alias identifier")
	cmd.Flags().BoolVar(&showAll, "all", false, "Include internal ecosystems (e.g. cx-repos)")
	return cmd
}

func emitAliasJSON(ecosystems map[string][]*workspace.WorkspaceNode, standalone []*workspace.WorkspaceNode, currentNode *workspace.WorkspaceNode) error {
	out := []aliasOutput{}

	var ecoNames []string
	for name := range ecosystems {
		ecoNames = append(ecoNames, name)
	}
	sort.Strings(ecoNames)
	for _, ecoName := range ecoNames {
		nodes := ecosystems[ecoName]
		sort.Slice(nodes, func(i, j int) bool { return nodes[i].Path < nodes[j].Path })
		for _, node := range nodes {
			out = append(out, aliasOutput{
				Alias:     "@a:" + node.Identifier(":"),
				Path:      node.Path,
				Ecosystem: ecoName,
				IsCurrent: currentNode != nil && node.Path == currentNode.Path,
			})
		}
	}
	sort.Slice(standalone, func(i, j int) bool { return standalone[i].Path < standalone[j].Path })
	for _, node := range standalone {
		out = append(out, aliasOutput{
			Alias:     "@a:" + node.Identifier(":"),
			Path:      node.Path,
			IsCurrent: currentNode != nil && node.Path == currentNode.Path,
		})
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}
