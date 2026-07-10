package context

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// IsLegacyRulesSpec reports whether spec is an explicit, legacy rules path.
// Bare names select a named preset; paths (including a bare *.rules filename)
// retain docgen's historical path resolution semantics.
func IsLegacyRulesSpec(spec string) bool {
	return filepath.IsAbs(spec) || strings.ContainsAny(spec, `/\\`) || strings.HasSuffix(spec, RulesExt)
}

// ResolveRulesSpec resolves a rules_file value for a repository. Bare names are
// notebook-aware preset names. Legacy paths remain supported: absolute paths
// are used directly, .cx/ paths are repo-relative, and other paths are
// relative to the repository's docs directory.
func ResolveRulesSpec(mgr *Manager, repoPath, spec string) (string, error) {
	spec = strings.TrimSpace(spec)
	if spec == "" {
		return "", fmt.Errorf("rules_file is empty")
	}
	if !IsLegacyRulesSpec(spec) {
		return mgr.FindRulesetFile(repoPath, spec)
	}

	var path string
	switch {
	case filepath.IsAbs(spec):
		path = spec
	case strings.HasPrefix(spec, ".cx/") || strings.HasPrefix(spec, ".cx\\"):
		path = filepath.Join(repoPath, spec)
	default:
		path = filepath.Join(repoPath, "docs", spec)
	}
	path, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("resolve legacy rules path %q: %w", spec, err)
	}
	if _, err := os.Stat(path); err != nil {
		return "", fmt.Errorf("legacy rules_file %q not found at %s: %w", spec, path, err)
	}
	return path, nil
}
