package cmd

import (
	"fmt"
	"os"

	"github.com/grovetools/cx/pkg/context"
	"github.com/spf13/cobra"
)

func NewLintCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "lint",
		Short: "Validate rules syntax and check for potential issues",
		Long:  `Analyzes the active rules file for syntax errors, directive typos, overly broad patterns, and patterns that match zero files.`,
		Example: `  # Lint the active rules file
  cx lint

  # Use in CI to catch unsafe rules (exits 1 on errors)
  cx lint && echo "rules ok"`,
		RunE: func(cmd *cobra.Command, args []string) error {
			mgr := context.NewManager("")
			issues, err := mgr.LintRules()
			if err != nil {
				return fmt.Errorf("failed to lint rules: %w", err)
			}

			if len(issues) == 0 {
				fmt.Println("Rules look good! No issues found.")
				return nil
			}

			hasErrors := false
			fmt.Printf("Found %d issue(s) in rules file:\n\n", len(issues))
			for _, issue := range issues {
				fmt.Printf("[%s] Line %d: %s\n", issue.Severity, issue.LineNum, issue.Message)
				fmt.Printf("    > %s\n\n", issue.Line)
				if issue.Severity == "Error" {
					hasErrors = true
				}
			}

			if hasErrors {
				os.Exit(1)
			}
			return nil
		},
	}
}
