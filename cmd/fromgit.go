package cmd

import (
	"fmt"
	"github.com/spf13/cobra"
	"github.com/grovetools/cx/pkg/context"
	"github.com/grovetools/core/logging"
)

var (
	fromGitLog = logging.NewLogger("grove-context")
	fromGitPrettyLog = logging.NewPrettyLogger()
)

func NewFromGitCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "from-git",
		Short: "Generate context based on git history",
		Long:  `Generate context from files in git history based on various criteria like commits, branches, or dates.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			mgr := context.NewManager("")
			
			// Get flags
			since, _ := cmd.Flags().GetString("since")
			branch, _ := cmd.Flags().GetString("branch")
			staged, _ := cmd.Flags().GetBool("staged")
			commits, _ := cmd.Flags().GetInt("commits")
			appendRules, _ := cmd.Flags().GetBool("append")
			force, _ := cmd.Flags().GetBool("force")

			if appendRules && force {
				return fmt.Errorf("cannot use both --append and --force flags together")
			}

			// Validate that at least one option is specified
			if since == "" && branch == "" && !staged && commits == 0 {
				return fmt.Errorf("specify at least one option: --since, --branch, --staged, or --commits")
			}

			fromGitLog.Info("Updating context from git history")
			fromGitPrettyLog.InfoPretty("Updating context from git history...")

			// Create git options
			opts := context.GitOptions{
				Since:   since,
				Branch:  branch,
				Staged:  staged,
				Commits: commits,
				Append:  appendRules,
				Force:   force,
			}
			
			// Update from git
			if err := mgr.UpdateFromGit(opts); err != nil {
				return err
			}
			
			// Show what was added
			files, err := mgr.ListFiles()
			if err == nil {
				fromGitPrettyLog.Blank()
				fromGitPrettyLog.InfoPretty("Files added to context:")
				for _, file := range files {
					fromGitPrettyLog.Path("  ", file)
				}
			}
			
			return nil
		},
	}
	
	cmd.Flags().String("since", "", "Include files changed since date/commit")
	cmd.Flags().String("branch", "", "Include files changed in branch (e.g., main..HEAD)")
	cmd.Flags().Bool("staged", false, "Include only staged files")
	cmd.Flags().Int("commits", 0, "Include files from last N commits")
	cmd.Flags().BoolP("append", "a", false, "Append to existing rules instead of overwriting")
	cmd.Flags().BoolP("force", "f", false, "Force overwrite of existing rules without prompting")

	return cmd
}