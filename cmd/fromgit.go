package cmd

import (
	"fmt"
	"github.com/spf13/cobra"
	"github.com/mattsolo1/grove-context/pkg/context"
	"github.com/mattsolo1/grove-core/cli"
)

func NewFromGitCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "from-git",
		Short: "Generate context based on git history",
		Long:  `Generate context from files in git history based on various criteria like commits, branches, or dates.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			logger := cli.GetLogger(cmd)
			mgr := context.NewManager("")
			
			// Get flags
			since, _ := cmd.Flags().GetString("since")
			branch, _ := cmd.Flags().GetString("branch")
			staged, _ := cmd.Flags().GetBool("staged")
			commits, _ := cmd.Flags().GetInt("commits")
			
			// Validate that at least one option is specified
			if since == "" && branch == "" && !staged && commits == 0 {
				return fmt.Errorf("specify at least one option: --since, --branch, --staged, or --commits")
			}
			
			logger.Info("Updating context from git history...")
			
			// Create git options
			opts := context.GitOptions{
				Since:   since,
				Branch:  branch,
				Staged:  staged,
				Commits: commits,
			}
			
			// Update from git
			if err := mgr.UpdateFromGit(opts); err != nil {
				return err
			}
			
			// Show what was added
			files, err := mgr.ListFiles()
			if err == nil {
				fmt.Printf("\nFiles added to context:\n")
				for _, file := range files {
					fmt.Printf("  %s\n", file)
				}
			}
			
			return nil
		},
	}
	
	cmd.Flags().String("since", "", "Include files changed since date/commit")
	cmd.Flags().String("branch", "", "Include files changed in branch (e.g., main..HEAD)")
	cmd.Flags().Bool("staged", false, "Include only staged files")
	cmd.Flags().Int("commits", 0, "Include files from last N commits")
	
	return cmd
}