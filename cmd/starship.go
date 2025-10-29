package cmd

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/mattsolo1/grove-context/pkg/context"
	"github.com/mattsolo1/grove-core/starship"
	"github.com/mattsolo1/grove-core/state"
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
	ruleSetName := strings.TrimSuffix(filepath.Base(sourcePath), rulesExt)

	// Format for display in the prompt
	return fmt.Sprintf("(cx:%s)", ruleSetName), nil
}
