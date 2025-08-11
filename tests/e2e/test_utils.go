// File: grove-context/tests/e2e/test_utils.go
package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/mattsolo1/grove-tend/pkg/command"
)

// FindCxBinary is a helper to find the `cx` binary path for tests.
// It checks in the following order:
// 1. CX_BINARY environment variable
// 2. Common relative paths from test execution directory
// 3. System PATH
func FindCxBinary() (string, error) {
	// Check environment variable first
	if cxBinary := os.Getenv("CX_BINARY"); cxBinary != "" {
		return cxBinary, nil
	}

	// Try common locations relative to test execution directory
	candidates := []string{
		"./bin/cx",
		"../bin/cx",
		"../../bin/cx",
		"cx", // In PATH
	}

	for _, candidate := range candidates {
		if _, err := os.Stat(candidate); err == nil {
			// Convert to absolute path if it's not already
			if filepath.IsAbs(candidate) {
				return candidate, nil
			}
			if abs, err := filepath.Abs(candidate); err == nil {
				return abs, nil
			}
		}
	}

	// Additional fallback: try using 'which' command
	if _, err := command.RunSimple("which", "cx"); err == nil {
		return "cx", nil
	}

	return "", fmt.Errorf("cx binary not found. Build it first with 'make build' or set CX_BINARY env var")
}