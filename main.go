package main

import (
    "os"
    "github.com/mattsolo1/grove-core/cli"
    "github.com/mattsolo1/grove-context/cmd"
)

func main() {
    rootCmd := cli.NewStandardCommand(
        "cx",
        "LLM context management (formerly grove cx)",
    )
    
    // Add subcommands
    rootCmd.AddCommand(cmd.NewDashboardCmd())
    rootCmd.AddCommand(cmd.NewEditCmd())
    rootCmd.AddCommand(cmd.NewGenerateCmd()) 
    rootCmd.AddCommand(cmd.NewShowCmd())
    rootCmd.AddCommand(cmd.NewListCmd())
    rootCmd.AddCommand(cmd.NewListCacheCmd())
    rootCmd.AddCommand(cmd.NewSaveCmd())
    rootCmd.AddCommand(cmd.NewLoadCmd())
    rootCmd.AddCommand(cmd.NewDiffCmd())
    rootCmd.AddCommand(cmd.NewListSnapshotsCmd())
    rootCmd.AddCommand(cmd.NewValidateCmd())
    rootCmd.AddCommand(cmd.NewFixCmd())
    rootCmd.AddCommand(cmd.NewStatsCmd())
    rootCmd.AddCommand(cmd.NewFromGitCmd())
    rootCmd.AddCommand(cmd.NewVersionCmd())
    
    if err := rootCmd.Execute(); err != nil {
        os.Exit(1)
    }
}