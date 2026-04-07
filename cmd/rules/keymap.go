// Package rules is a thin compatibility shim that re-exports KeymapInfo from
// the embeddable TUI package at cx/pkg/tui/rules. The actual TUI implementation
// lives there so it can be imported and embedded by other ecosystems (e.g.,
// grove-terminal) which cannot import a sibling cx/internal/ package.
//
// This file exists solely so that github.com/grovetools/cx/cmd/rules continues
// to be importable by the grove keys-registry-generator without requiring a
// cross-repo coordination commit.
package rules

import (
	"github.com/grovetools/core/tui/keymap"
	tuirules "github.com/grovetools/cx/pkg/tui/rules"
)

// KeymapInfo returns the keymap metadata for the cx rules picker TUI.
// Used by the grove keys registry generator to aggregate all TUI keybindings.
func KeymapInfo() keymap.TUIInfo {
	return tuirules.KeymapInfo()
}
