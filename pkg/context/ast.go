package context

// RuleNode is the minimal polymorphic primitive that downstream phases of the
// rule-engine refactor will build on. For the wedge it has a single
// responsibility: decide whether a given path matches the node's rule.
//
// The Manager is passed through so concrete nodes can delegate to existing
// path-normalization logic (matchPattern) without us having to lift that
// behaviour out of the manager during the wedge.
type RuleNode interface {
	Match(m *Manager, path string) bool
}

// GlobNode represents a pattern that contains wildcard metacharacters
// (*, ?, [, **). Matching is delegated to Manager.matchPattern so the wedge
// preserves the exact semantics of the legacy matcher.
type GlobNode struct {
	Pattern string
}

func (n *GlobNode) Match(m *Manager, path string) bool {
	return m.matchPattern(n.Pattern, path)
}

// LiteralNode represents an exact file or directory path with no wildcard
// metacharacters. For the wedge we still delegate to Manager.matchPattern to
// preserve case-insensitivity and trailing-slash semantics; the type
// distinction is what matters here so that the resolver no longer needs an
// out-of-band fast path for literal inclusions.
type LiteralNode struct {
	ExpectedPath string
}

func (n *LiteralNode) Match(m *Manager, path string) bool {
	return m.matchPattern(n.ExpectedPath, path)
}
