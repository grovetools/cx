package main

import (
    "os"
    "github.com/yourorg/grove-core/cli"
    "github.com/yourorg/grove-context/cmd"
)

func main() {
    rootCmd := cli.NewStandardCommand(
        "cx",
        "LLM context management (formerly grove cx)",
    )
    
    // Add subcommands
    rootCmd.AddCommand(cmd.NewUpdateCmd())
    rootCmd.AddCommand(cmd.NewGenerateCmd()) 
    rootCmd.AddCommand(cmd.NewShowCmd())
    rootCmd.AddCommand(cmd.NewInfoCmd())
    rootCmd.AddCommand(cmd.NewListCmd())
    rootCmd.AddCommand(cmd.NewSaveCmd())
    rootCmd.AddCommand(cmd.NewLoadCmd())
    rootCmd.AddCommand(cmd.NewDiffCmd())
    rootCmd.AddCommand(cmd.NewListSnapshotsCmd())
    rootCmd.AddCommand(cmd.NewValidateCmd())
    rootCmd.AddCommand(cmd.NewFixCmd())
    rootCmd.AddCommand(cmd.NewStatsCmd())
    rootCmd.AddCommand(cmd.NewFromGitCmd())
    
    if err := rootCmd.Execute(); err != nil {
        os.Exit(1)
    }
}