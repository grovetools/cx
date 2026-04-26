package context

// RuleNode is the polymorphic primitive that the parser emits and downstream
// resolver phases consume. The Match method is wedge-era and Phase 3 will
// remove it; the metadata accessors are stable.
type RuleNode interface {
	Match(m *Manager, path string) bool
	Line() int
	Raw() string
	IsExclude() bool
}

// ParseError is a non-fatal grammar violation captured during parsing.
type ParseError struct {
	Line int
	Msg  string
}

// GlobNode represents a pattern containing wildcard metacharacters.
type GlobNode struct {
	Pattern  string
	LineNum  int
	RawText  string
	Excluded bool
}

func (n *GlobNode) Match(m *Manager, path string) bool { return m.matchPattern(n.Pattern, path) }
func (n *GlobNode) Line() int                          { return n.LineNum }
func (n *GlobNode) Raw() string                        { return n.RawText }
func (n *GlobNode) IsExclude() bool                    { return n.Excluded }

// LiteralNode represents an exact path with no wildcards.
type LiteralNode struct {
	ExpectedPath string
	LineNum      int
	RawText      string
	Excluded     bool
}

func (n *LiteralNode) Match(m *Manager, path string) bool {
	return m.matchPattern(n.ExpectedPath, path)
}
func (n *LiteralNode) Line() int       { return n.LineNum }
func (n *LiteralNode) Raw() string     { return n.RawText }
func (n *LiteralNode) IsExclude() bool { return n.Excluded }

// ImportNode represents an alias import (@a:target) optionally with ::ruleset.
type ImportNode struct {
	Target   string
	Ruleset  string
	LineNum  int
	RawText  string
	Excluded bool
}

func (n *ImportNode) Match(m *Manager, path string) bool { return false }
func (n *ImportNode) Line() int                          { return n.LineNum }
func (n *ImportNode) Raw() string                        { return n.RawText }
func (n *ImportNode) IsExclude() bool                    { return n.Excluded }

// CommandNode represents a @cmd: directive.
type CommandNode struct {
	Command  string
	LineNum  int
	RawText  string
	Excluded bool
}

func (n *CommandNode) Match(m *Manager, path string) bool { return false }
func (n *CommandNode) Line() int                          { return n.LineNum }
func (n *CommandNode) Raw() string                        { return n.RawText }
func (n *CommandNode) IsExclude() bool                    { return n.Excluded }

// FilterNode wraps an inner node with @grep/@find search directives.
type FilterNode struct {
	Child      RuleNode
	Directives []SearchDirective
	LineNum    int
	RawText    string
	Excluded   bool
}

func (n *FilterNode) Match(m *Manager, path string) bool {
	if n.Child == nil {
		return false
	}
	return n.Child.Match(m, path)
}
func (n *FilterNode) Line() int       { return n.LineNum }
func (n *FilterNode) Raw() string     { return n.RawText }
func (n *FilterNode) IsExclude() bool { return n.Excluded }
