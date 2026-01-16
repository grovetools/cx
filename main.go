package main

import (
	"io"
	"os"

	"github.com/grovetools/core/cli"
	"github.com/grovetools/core/logging"
	"github.com/grovetools/core/pkg/profiling"
	// "github.com/grovetools/core/tui"
	"github.com/grovetools/cx/cmd"
	"github.com/grovetools/cx/cmd/view"
	"github.com/sirupsen/logrus"
)

func main() {
	// Check if --json flag is present to suppress logging early
	for _, arg := range os.Args {
		if arg == "--json" {
			// Suppress all logging output when JSON format is requested
			logrus.StandardLogger().SetOutput(io.Discard)
			// Also suppress the logging package's output
			log := logging.NewLogger("cx")
			log.Logger.SetOutput(io.Discard)
			break
		}
	}

	rootCmd := cli.NewStandardCommand(
		"cx",
		"LLM context management (formerly grove cx)",
	)

	// Setup profiling
	profiler := profiling.NewCobraProfiler()
	profiler.AddFlags(rootCmd)
	rootCmd.PersistentPreRunE = profiler.PreRun
	rootCmd.PersistentPostRun = profiler.PostRun

	// Add subcommands
	rootCmd.AddCommand(cmd.NewEditCmd())
	rootCmd.AddCommand(cmd.NewResetCmd())
	rootCmd.AddCommand(cmd.NewRulesCmd())
	rootCmd.AddCommand(cmd.NewSetRulesCmd())
	rootCmd.AddCommand(cmd.NewWriteRulesCmd())
	rootCmd.AddCommand(cmd.NewGenerateCmd())
	rootCmd.AddCommand(cmd.NewShowCmd())
	rootCmd.AddCommand(cmd.NewListCmd())
	rootCmd.AddCommand(cmd.NewListCacheCmd())
	rootCmd.AddCommand(cmd.NewDiffCmd())
	rootCmd.AddCommand(cmd.NewValidateCmd())
	rootCmd.AddCommand(cmd.NewFixCmd())
	rootCmd.AddCommand(cmd.NewStatsCmd())
	rootCmd.AddCommand(cmd.NewFromGitCmd())
	rootCmd.AddCommand(cmd.NewFromCmdCmd())
	rootCmd.AddCommand(view.NewViewCmd())
	rootCmd.AddCommand(cmd.NewVersionCmd())
	rootCmd.AddCommand(cmd.NewRepoCmd())
	rootCmd.AddCommand(cmd.NewWorkspaceCmd())
	rootCmd.AddCommand(cmd.NewResolveCmd())

	if err := cli.Execute(rootCmd); err != nil {
		os.Exit(1)
	}
}

