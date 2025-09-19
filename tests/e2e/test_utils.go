// File: grove-context/tests/e2e/test_utils.go
package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/mattsolo1/grove-tend/pkg/project"
)

// FindProjectBinary finds the project's main binary path by reading grove.yml.
// This provides a single source of truth for locating the binary under test.
func FindProjectBinary() (string, error) {
	// The test runner is executed from the project root, so we start the search here.
	wd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("could not get working directory: %w", err)
	}

	binaryPath, err := project.GetBinaryPath(wd)
	if err != nil {
		return "", fmt.Errorf("failed to find project binary via grove.yml: %w", err)
	}

	return binaryPath, nil
}

// CleanupExistingTestSessions kills any existing tmux sessions that match tend test patterns.
// This helps ensure a clean test environment and avoids port conflicts or session collisions.
func CleanupExistingTestSessions() error {
	// List all tmux sessions
	cmd := exec.Command("tmux", "list-sessions", "-F", "#{session_name}")
	output, err := cmd.Output()
	if err != nil {
		// If tmux returns an error, it might mean no server is running
		// This is fine - nothing to clean up
		if exitErr, ok := err.(*exec.ExitError); ok {
			if strings.Contains(string(exitErr.Stderr), "no server running") {
				return nil
			}
		}
		return fmt.Errorf("failed to list tmux sessions: %w", err)
	}

	sessions := strings.Split(strings.TrimSpace(string(output)), "\n")
	cleanedCount := 0

	for _, session := range sessions {
		session = strings.TrimSpace(session)
		if session == "" {
			continue
		}

		// Check if this looks like a tend test session
		// Tend test sessions typically have patterns like "tend-test-*" or contain "cx-view"
		if strings.Contains(session, "tend-test") || 
		   strings.Contains(session, "cx-view") ||
		   strings.Contains(session, "grove-tend") {
			// Kill the session
			killCmd := exec.Command("tmux", "kill-session", "-t", session)
			if err := killCmd.Run(); err != nil {
				// Log but don't fail - session might have already ended
				fmt.Printf("   Note: Could not kill session %s: %v\n", session, err)
			} else {
				cleanedCount++
			}
		}
	}

	if cleanedCount > 0 {
		fmt.Printf("   Cleaned %d existing test session(s)\n", cleanedCount)
	}

	return nil
}