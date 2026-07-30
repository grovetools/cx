package view

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/grovetools/core/tui/keymap"
)

func chordKey(s string) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
}

// TestNamespaceMembership pins the canon-60 t… members per page. cx-view is
// multi-page: the list page owns `ts`, the tree page owns th/tc/ti, and the
// merged export is their union. A page picking up a member it cannot dispatch
// (or losing one it can) breaks the container's per-page narrowing in
// pagerModel.activeNamespaces.
func TestNamespaceMembership(t *testing.T) {
	cases := []struct {
		name string
		ns   []keymap.Namespace
		want []string
	}{
		{"list", newPagerKeyMap(nil).Namespaces(), []string{"ts"}},
		{"tree", newTreeKeyMap(nil).Namespaces(), []string{"th", "tc", "ti"}},
		{"merged", newViewKeyMap(nil).Namespaces(), []string{"ts", "th", "tc", "ti"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if len(c.ns) != 1 || c.ns[0].Prefix != "t" {
				t.Fatalf("want exactly one t… namespace, got %+v", c.ns)
			}
			var got []string
			for _, b := range c.ns[0].Bindings {
				if len(b.Keys()) != 1 {
					t.Errorf("chord-only (E4) violated: %v carries %d keys", b.Keys(), len(b.Keys()))
					continue
				}
				got = append(got, b.Keys()[0])
			}
			if len(got) != len(c.want) {
				t.Fatalf("members = %v, want %v", got, c.want)
			}
			for i := range got {
				if got[i] != c.want[i] {
					t.Errorf("member[%d] = %q, want %q", i, got[i], c.want[i])
				}
			}
		})
	}
}

// TestTreeTogglesAreChordOnly walks the tree page's prefix and each completion
// through the shared host: bare `t` must arm (never fire), and each second key
// must resolve to its chord. It also guards the two keys the canon retired —
// flat `h` and flat `c` must no longer resolve to anything.
func TestTreeTogglesAreChordOnly(t *testing.T) {
	km := newTreeKeyMap(nil)

	for _, tc := range []struct{ second, want string }{
		{"h", "th"}, {"c", "tc"}, {"i", "ti"},
	} {
		host := keymap.NewWhichKeyHost(nil, km.Namespaces()...)
		if res, _, cmd := host.ProcessChord(chordKey("t")); res != keymap.ChordPending {
			t.Fatalf("`t` alone: got %v, want ChordPending", res)
		} else if cmd == nil {
			t.Error("an armed namespace prefix must return the show-delay tick cmd")
		}
		res, matched, _ := host.ProcessChord(chordKey(tc.second))
		if res != keymap.ChordMatched {
			t.Fatalf("`t`+%q: got %v, want ChordMatched", tc.second, res)
		}
		if got := matched.Keys()[0]; got != tc.want {
			t.Errorf("`t`+%q matched %q, want %q", tc.second, got, tc.want)
		}
	}

	// The retired flat keys must not resolve on their own any more.
	for _, k := range []string{"h", "c", "H", "."} {
		host := keymap.NewWhichKeyHost(nil, km.Namespaces()...)
		if res, _, _ := host.ProcessChord(chordKey(k)); res != keymap.ChordNone {
			t.Errorf("flat %q still resolves (%v); canon 60 E4 drops it", k, res)
		}
	}
}

// TestStraySecondKeyIsConsumed is the which-key idiom: with `t` armed on the
// tree page, a non-continuation key closes the menu and is swallowed rather
// than firing its flat action. `s` is the interesting one — it is the rules
// page's flat select-rule-set key, which must not fire mid-chord.
func TestStraySecondKeyIsConsumed(t *testing.T) {
	host := keymap.NewWhichKeyHost(nil, newTreeKeyMap(nil).Namespaces()...)
	if res, _, _ := host.ProcessChord(chordKey("t")); res != keymap.ChordPending {
		t.Fatalf("`t` alone: got %v, want ChordPending", res)
	}
	if res, _, _ := host.ProcessChord(chordKey("s")); res != keymap.ChordConsumed {
		t.Errorf("stray `s` while armed: got %v, want ChordConsumed", res)
	}
	if host.IsPending() {
		t.Error("buffer should be cleared after a stray key")
	}
}

// TestPageScopedSplit covers the intra-TUI `s` fix. The rules page keeps flat
// `s` (select rule set) and must declare no namespace at all, so no prefix can
// ever arm there and eat it; the stats page's old flat `s` (switch focus) is
// gone, replaced by `tab`.
func TestPageScopedSplit(t *testing.T) {
	if got := newPagerKeyMap(nil).SelectRules.Keys(); len(got) != 1 || got[0] != "s" {
		t.Errorf("rules-page select_rules keys = %v, want [s] (stays flat)", got)
	}

	stats := newStatsKeyMap(nil)
	for _, k := range stats.SwitchFocus.Keys() {
		if k == "s" {
			t.Errorf("stats switch_focus still binds `s`: %v", stats.SwitchFocus.Keys())
		}
	}
	if got := stats.SwitchFocus.Keys(); len(got) == 0 || got[0] != "tab" {
		t.Errorf("stats switch_focus keys = %v, want tab first", got)
	}

	// The rules and stats pages own no chord members, which is what makes the
	// container's guard (pagerModel.namespaceArmable) refuse to arm on them.
	m, err := NewWithStartPage("rules", t.TempDir(), "", nil, false)
	if err != nil {
		t.Fatalf("NewWithStartPage: %v", err)
	}
	defer m.Close()
	if ns := m.activeNamespaces(); len(ns) != 0 {
		t.Errorf("rules page namespaces = %+v, want none", ns)
	}
	if m.namespaceArmable() {
		t.Error("namespaces must not be armable on the rules page")
	}
}

// TestRefreshIsCtrlROnly pins canon 60 §5.5: flat `r`=refresh is retired.
func TestRefreshIsCtrlROnly(t *testing.T) {
	got := newTreeKeyMap(nil).Refresh.Keys()
	if len(got) != 1 || got[0] != "ctrl+r" {
		t.Errorf("tree refresh keys = %v, want [ctrl+r]", got)
	}
}
