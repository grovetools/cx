package context

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseToAST(t *testing.T) {
	home, _ := os.UserHomeDir()

	type expectedNode struct {
		kind     string // "literal", "glob", "import", "command", "filter"
		path     string // ExpectedPath / Pattern / Target / Command
		line     int
		excluded bool
		ruleset  string
	}

	tests := []struct {
		name      string
		input     string
		wantNodes []expectedNode
		wantErrs  []int // expected error line numbers (any errors at these lines)
	}{
		{
			name:  "home expansion",
			input: "~/src/main.go\n",
			wantNodes: []expectedNode{
				{kind: "literal", path: filepath.Join(home, "src/main.go"), line: 1},
			},
		},
		{
			name:  "inline comment stripped",
			input: "foo.go # comment\n",
			wantNodes: []expectedNode{
				{kind: "literal", path: "foo.go", line: 1},
			},
		},
		{
			name:  "brace top-level alias",
			input: "{@a:foo,@a:bar}\n",
			wantNodes: []expectedNode{
				{kind: "import", path: "foo", line: 1},
				{kind: "import", path: "bar", line: 1},
			},
		},
		{
			name:  "brace inside alias path",
			input: "@a:eco:proj/{a,b}/*.go\n",
			wantNodes: []expectedNode{
				{kind: "import", path: "eco:proj/a/*.go", line: 1},
				{kind: "import", path: "eco:proj/b/*.go", line: 1},
			},
		},
		{
			name:  "alias trailing slash",
			input: "@a:eco:proj/\n",
			wantNodes: []expectedNode{
				{kind: "import", path: "eco:proj/**", line: 1},
			},
		},
		{
			name:  "directory trailing slash glob",
			input: "cx/\n",
			wantNodes: []expectedNode{
				{kind: "glob", path: "cx/**", line: 1},
			},
		},
		{
			name:  "leading dot slash stripped",
			input: "./foo.go\n",
			wantNodes: []expectedNode{
				{kind: "literal", path: "foo.go", line: 1},
			},
		},
		{
			name:     "multiple separators second errors",
			input:    "a.go\n---\nb.go\n---\nc.go\n",
			wantErrs: []int{4},
			wantNodes: []expectedNode{
				{kind: "literal", path: "a.go", line: 1},
				{kind: "literal", path: "b.go", line: 3},
				{kind: "literal", path: "c.go", line: 5},
			},
		},
		{
			name:     "capital alias errors",
			input:    "@A:foo\n",
			wantErrs: []int{1},
		},
		{
			name:     "dangling ruleset errors",
			input:    "::name\n",
			wantErrs: []int{1},
		},
		{
			name:     "multi alias per line errors",
			input:    "@a:foo @a:bar\n",
			wantErrs: []int{1},
		},
		{
			name:     "empty alias target errors",
			input:    "@a:\n",
			wantErrs: []int{1},
		},
		{
			name:  "exclude prefix",
			input: "!Makefile\n",
			wantNodes: []expectedNode{
				{kind: "literal", path: "Makefile", line: 1, excluded: true},
			},
		},
		{
			name:  "ruleset import",
			input: "@a:proj::default\n",
			wantNodes: []expectedNode{
				{kind: "import", path: "proj", ruleset: "default", line: 1},
			},
		},
		{
			name:  "alias_brace_expansion",
			input: "@a:eco:{a,b}/main.go\n",
			wantNodes: []expectedNode{
				{kind: "import", path: "eco:a/main.go", line: 1},
				{kind: "import", path: "eco:b/main.go", line: 1},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nodes, errs := ParseToAST([]byte(tt.input))

			// Verify expected error lines are present
			for _, wantLine := range tt.wantErrs {
				found := false
				for _, e := range errs {
					if e.Line == wantLine {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("expected ParseError on line %d, got errs=%v", wantLine, errs)
				}
			}

			if len(tt.wantNodes) != len(nodes) {
				t.Fatalf("node count: want %d, got %d (nodes=%+v)", len(tt.wantNodes), len(nodes), nodes)
			}
			for i, want := range tt.wantNodes {
				got := nodes[i]
				switch want.kind {
				case "literal":
					n, ok := got.(*LiteralNode)
					if !ok {
						t.Fatalf("node[%d]: want LiteralNode, got %T", i, got)
					}
					if n.ExpectedPath != want.path {
						t.Errorf("node[%d] LiteralNode.ExpectedPath = %q, want %q", i, n.ExpectedPath, want.path)
					}
					if n.LineNum != want.line {
						t.Errorf("node[%d] LineNum = %d, want %d", i, n.LineNum, want.line)
					}
					if n.Excluded != want.excluded {
						t.Errorf("node[%d] Excluded = %v, want %v", i, n.Excluded, want.excluded)
					}
				case "glob":
					n, ok := got.(*GlobNode)
					if !ok {
						t.Fatalf("node[%d]: want GlobNode, got %T", i, got)
					}
					if n.Pattern != want.path {
						t.Errorf("node[%d] GlobNode.Pattern = %q, want %q", i, n.Pattern, want.path)
					}
					if n.LineNum != want.line {
						t.Errorf("node[%d] LineNum = %d, want %d", i, n.LineNum, want.line)
					}
				case "import":
					n, ok := got.(*ImportNode)
					if !ok {
						t.Fatalf("node[%d]: want ImportNode, got %T (path=%q)", i, got, want.path)
					}
					if n.Target != want.path {
						t.Errorf("node[%d] ImportNode.Target = %q, want %q", i, n.Target, want.path)
					}
					if n.Ruleset != want.ruleset {
						t.Errorf("node[%d] ImportNode.Ruleset = %q, want %q", i, n.Ruleset, want.ruleset)
					}
					if n.LineNum != want.line {
						t.Errorf("node[%d] LineNum = %d, want %d", i, n.LineNum, want.line)
					}
				}
			}
		})
	}
}

func TestStripInlineComments(t *testing.T) {
	cases := []struct {
		in, out string
	}{
		{"foo.go # bar", "foo.go"},
		{"foo.go", "foo.go"},
		{"# whole line comment", "# whole line comment"},
		{`@grep: "foo # bar"`, `@grep: "foo # bar"`},
		{"a.go #c1 #c2", "a.go"},
	}
	for _, c := range cases {
		got := stripInlineComments(c.in)
		if strings.TrimSpace(got) != strings.TrimSpace(c.out) {
			t.Errorf("stripInlineComments(%q) = %q, want %q", c.in, got, c.out)
		}
	}
}
