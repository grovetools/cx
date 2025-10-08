// File: grove-context/tests/e2e/main.go
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/mattsolo1/grove-tend/pkg/app"
	"github.com/mattsolo1/grove-tend/pkg/harness"
)

func main() {
	// A list of all E2E scenarios for grove-context.
	scenarios := []*harness.Scenario{
		// Basic Scenarios
		BasicContextGenerationScenario(),
		MissingRulesScenario(),

		// Alias Scenarios
		AliasWorkflowScenario(),
		AliasNamespacingScenario(),
		AliasSiblingResolutionScenario(),
		AliasWithStatsPerLineScenario(),

		// Hybrid Context Scenarios
		DualContextWorkflowScenario(),
		NoSeparatorBackwardCompatibilityScenario(),
		EmptyColdContextScenario(),
		CachedContextOnlyColdFilesScenario(),

		// TUI Scenarios
		TUIViewTreeScenario(), // Replaces the old generic TUIViewScenario
		TUIViewStatsScenario(),
		TUIViewRulesScenario(),
		TUIViewListScenario(),

		// Advanced Scenarios
		StatsAndValidateScenario(),
		SnapshotWorkflowScenario(),
		GitBasedContextScenario(),
		ComplexPatternScenario(),
		PlainDirectoryPatternScenario(),
		RecursiveParentPatternScenario(),
		ExclusionPatternsScenario(),
		WorktreeExclusionScenario(),
		AbsolutePathDirectoryPatternScenario(),
		ExplicitWorktreeInclusionScenario(),
		AbsolutePathFilePatternScenario(),

		// Git Repository Scenarios
		// GitRepositoryCloneScenario(),

		// Default Directive Scenarios
		DefaultDirectiveBasicScenario(),
		// DefaultDirectiveColdContextScenario(), // TODO: Fix cold context testing
		// DefaultDirectiveCircularScenario(), // TODO: Fix circular dependency test

		// Brace Expansion Scenarios
		BraceExpansionBasicScenario(),
		BraceExpansionMultipleScenario(),
		BraceExpansionWithExclusionScenario(),

		// Search Directive Scenarios
		FindDirectiveScenario(),
		GrepDirectiveScenario(),
		AliasWithDirectiveScenario(),
		CombinedDirectivesScenario(),
	}

	// Setup signal handling for graceful shutdown.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigChan
		fmt.Println("\nReceived interrupt signal, shutting down...")
		cancel()
	}()

	// Execute the custom tend application with our scenarios.
	if err := app.Execute(ctx, scenarios); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
