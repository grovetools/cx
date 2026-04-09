package view

import "github.com/grovetools/core/tui/components/pager"

// Page is the interface for a full-screen view in the cx TUI. It is
// a type alias for pager.Page, elevated into core so cx, memory, nb,
// skills, and flow can share a single tab dispatcher. Extra methods
// (e.g. Keys()) may be defined on individual page implementations
// without affecting pager compatibility — Go interface satisfaction
// is structural.
type Page = pager.Page
