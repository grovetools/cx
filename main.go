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
    
    if err := rootCmd.Execute(); err != nil {
        os.Exit(1)
    }
}