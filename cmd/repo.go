package cmd

import (
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	"github.com/mattsolo1/grove-context/pkg/repo"
	"github.com/spf13/cobra"
)

func NewRepoCmd() *cobra.Command {
	repoCmd := &cobra.Command{
		Use:   "repo",
		Short: "Manage Git repositories used in context",
		Long:  `Commands for managing Git repositories that are cloned and used in grove context.`,
	}

	repoCmd.AddCommand(newRepoListCmd())
	repoCmd.AddCommand(newRepoSyncCmd())
	repoCmd.AddCommand(newRepoAuditCmd())

	return repoCmd
}

func newRepoListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all tracked repositories",
		Long:  `List all Git repositories that have been cloned and are tracked in the manifest.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			manager, err := repo.NewManager()
			if err != nil {
				return fmt.Errorf("failed to create repository manager: %w", err)
			}

			repos, err := manager.List()
			if err != nil {
				return fmt.Errorf("failed to list repositories: %w", err)
			}

			if len(repos) == 0 {
				fmt.Println("No repositories tracked yet.")
				fmt.Println("Add a Git URL to your rules file to start tracking repositories.")
				return nil
			}

			// Create a tabwriter for formatted output
			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "URL\tVERSION\tCOMMIT\tSTATUS\tLAST SYNCED")
			fmt.Fprintln(w, "---\t-------\t------\t------\t-----------")

			for _, r := range repos {
				version := r.PinnedVersion
				if version == "" {
					version = "default"
				}
				
				commit := r.ResolvedCommit
				if len(commit) > 7 {
					commit = commit[:7]
				}
				
				lastSynced := "never"
				if !r.LastSyncedAt.IsZero() {
					lastSynced = formatTimeSince(r.LastSyncedAt)
				}
				
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
					r.URL,
					version,
					commit,
					r.Audit.Status,
					lastSynced,
				)
			}
			
			w.Flush()
			return nil
		},
	}
}

func newRepoSyncCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "sync",
		Short: "Sync all tracked repositories",
		Long:  `Fetch the latest changes for all tracked repositories and checkout their pinned versions.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			manager, err := repo.NewManager()
			if err != nil {
				return fmt.Errorf("failed to create repository manager: %w", err)
			}

			fmt.Println("Syncing all tracked repositories...")
			
			if err := manager.Sync(); err != nil {
				return fmt.Errorf("failed to sync repositories: %w", err)
			}

			fmt.Println("All repositories synced successfully.")
			return nil
		},
	}
}

func newRepoAuditCmd() *cobra.Command {
	var status string
	
	cmd := &cobra.Command{
		Use:   "audit <url>",
		Short: "Update audit status for a repository",
		Long:  `Update the audit status for a specific repository in the manifest.`,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			repoURL := args[0]
			
			if status == "" {
				return fmt.Errorf("--status flag is required")
			}
			
			// Validate status value
			validStatuses := map[string]bool{
				"not_audited": true,
				"audited":     true,
				"failed":      true,
				"in_progress": true,
				"passed":      true,
			}
			
			if !validStatuses[status] {
				return fmt.Errorf("invalid status: %s. Valid values are: not_audited, audited, failed, in_progress, passed", status)
			}
			
			manager, err := repo.NewManager()
			if err != nil {
				return fmt.Errorf("failed to create repository manager: %w", err)
			}

			if err := manager.UpdateAuditStatus(repoURL, status); err != nil {
				return fmt.Errorf("failed to update audit status: %w", err)
			}

			fmt.Printf("Updated audit status for %s to '%s'\n", repoURL, status)
			return nil
		},
	}
	
	cmd.Flags().StringVar(&status, "status", "", "Audit status (not_audited, audited, failed, in_progress, passed)")
	
	return cmd
}

func formatTimeSince(t time.Time) string {
	duration := time.Since(t)
	
	if duration < time.Minute {
		return "just now"
	} else if duration < time.Hour {
		minutes := int(duration.Minutes())
		if minutes == 1 {
			return "1 minute ago"
		}
		return fmt.Sprintf("%d minutes ago", minutes)
	} else if duration < 24*time.Hour {
		hours := int(duration.Hours())
		if hours == 1 {
			return "1 hour ago"
		}
		return fmt.Sprintf("%d hours ago", hours)
	} else if duration < 30*24*time.Hour {
		days := int(duration.Hours() / 24)
		if days == 1 {
			return "1 day ago"
		}
		return fmt.Sprintf("%d days ago", days)
	} else {
		return t.Format("2006-01-02")
	}
}