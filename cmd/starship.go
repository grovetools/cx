package cmd

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/grovetools/cx/pkg/context"
	"github.com/grovetools/core/starship"
	"github.com/grovetools/core/state"
)

func init() {
	starship.RegisterProvider(cxStatusProvider)
}

// cxStatusProvider is the status provider for grove-context.
// It displays the name of the active rule set.
func cxStatusProvider(s state.State) (string, error) {
	source, ok := s[context.StateSourceKey]
	if !ok || source == nil {
		return "", nil // No active source, display nothing.
	}

	sourcePath, ok := source.(string)
	if !ok || sourcePath == "" {
		return "", nil
	}

	// Extract the rule set name from the path (e.g., ".cx/dev.rules" -> "dev")
	ruleSetName := strings.TrimSuffix(filepath.Base(sourcePath), context.RulesExt)

	// Format for display in the prompt
	return fmt.Sprintf("(cx:%s)", ruleSetName), nil
}
