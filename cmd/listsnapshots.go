package cmd

import (
	"github.com/spf13/cobra"
	"github.com/mattsolo1/grove-context/pkg/context"
)

var (
	sortBy     string
	descending bool
)

func NewListSnapshotsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list-snapshots",
		Short: "List all saved context snapshots",
		Long:  `View all saved context snapshots with metadata including name, date, size, and file count.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			mgr := context.NewManager("")
			
			snapshots, err := mgr.ListSnapshots()
			if err != nil {
				return err
			}
			
			context.SortSnapshots(snapshots, sortBy, descending)
			context.PrintSnapshots(snapshots)
			
			return nil
		},
	}
	
	cmd.Flags().StringVar(&sortBy, "sort", "date", "Sort by: date, name, size, tokens, files")
	cmd.Flags().BoolVar(&descending, "desc", true, "Sort in descending order")
	
	return cmd
}