package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

func NewFixCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:        "fix",
		Short:      "Removed; context is now resolved dynamically from rules",
		Hidden:     true,
		Deprecated: "edit rules files directly; context is now resolved dynamically from rules",
		RunE: func(cmd *cobra.Command, args []string) error {
			return fmt.Errorf("cx fix has been removed; edit rules files directly")
		},
	}
	return cmd
}
