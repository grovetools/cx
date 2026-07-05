package view

import (
	"testing"

	"github.com/grovetools/core/tui/keymap"
)

// TestKeymapAuditCoverage asserts that every per-page keymap and the merged
// composite advertise exactly the keys they honor: every enabled binding
// appears in a section, and no help label contradicts its keys. If one fails,
// the Sections() list or the Base disable list is wrong — fix the keymap.
func TestKeymapAuditCoverage(t *testing.T) {
	cases := []struct {
		name string
		km   keymap.SectionedKeyMap
	}{
		{"pager", newPagerKeyMap(nil)},
		{"stats", newStatsKeyMap(nil)},
		{"tree", newTreeKeyMap(nil)},
		{"view", newViewKeyMap(nil)},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if gaps := keymap.AuditCoverage(c.km); len(gaps) != 0 {
				for _, g := range gaps {
					t.Errorf("audit gap: field=%s kind=%s detail=%s", g.Field, g.Kind, g.Detail)
				}
			}
		})
	}
}
