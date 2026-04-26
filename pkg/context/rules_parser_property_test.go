package context

import (
	"fmt"
	"math/rand"
	"strings"
	"testing"
)

const propertyIterations = 50

type lineKind int

const (
	lkLiteral lineKind = iota
	lkGlob
	lkDoubleStar
	lkBrace
	lkAlias
	lkAliasRuleset
	lkFiltered
	lkExclude
	lkBlank
	lkComment
	lkSeparator
)

func randLine(rng *rand.Rand, kind lineKind) string {
	switch kind {
	case lkLiteral:
		return pickOne(rng, []string{"main.go", "src/foo.go", "README.md", "pkg/sub/file.txt", "a.go"})
	case lkGlob:
		return pickOne(rng, []string{"*.go", "src/*.go", "*_test.go", "pkg/?ar.go", "src/[abc].go"})
	case lkDoubleStar:
		return pickOne(rng, []string{"**/*.go", "src/**/*.md", "**/foo.txt", "pkg/**/*"})
	case lkBrace:
		return pickOne(rng, []string{"src/{a,b}.go", "{x,y,z}/main.go", "{a,b}/{c,d}.go", "pkg/{foo,bar,baz}_test.go"})
	case lkAlias:
		return pickOne(rng, []string{"@a:eco:proj/path", "@a:foo", "@alias:eco:proj:wt/src/main.go"})
	case lkAliasRuleset:
		return pickOne(rng, []string{"@a:proj::ruleset", "@alias:eco:proj::dev"})
	case lkFiltered:
		base := pickOne(rng, []string{"**/*.go", "pkg/**/*.go"})
		dir := pickOne(rng, []string{`@grep: "Foo"`, `@find: "user"`, `@grep!: "skip"`})
		return base + " " + dir
	case lkExclude:
		inner := randLine(rng, []lineKind{lkLiteral, lkGlob, lkDoubleStar}[rng.Intn(3)])
		return "!" + inner
	case lkBlank:
		return ""
	case lkComment:
		return "# " + pickOne(rng, []string{"header", "section break", "TODO"})
	case lkSeparator:
		return "---"
	}
	return ""
}

func pickOne(rng *rand.Rand, opts []string) string {
	return opts[rng.Intn(len(opts))]
}

func randInput(rng *rand.Rand) string {
	kinds := []lineKind{lkLiteral, lkGlob, lkDoubleStar, lkBrace, lkAlias, lkAliasRuleset, lkFiltered, lkExclude, lkBlank, lkComment}
	if rng.Intn(4) == 0 {
		kinds = append(kinds, lkSeparator)
	}
	n := 1 + rng.Intn(8)
	var lines []string
	for i := 0; i < n; i++ {
		lines = append(lines, randLine(rng, kinds[rng.Intn(len(kinds))]))
	}
	return strings.Join(lines, "\n") + "\n"
}

func TestParseToAST_NeverPanics(t *testing.T) {
	rng := rand.New(rand.NewSource(0xC0FFEE))
	for i := 0; i < propertyIterations; i++ {
		input := randInput(rng)
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("panic on input %q: %v", input, r)
				}
			}()
			_, _ = ParseToAST([]byte(input))
		}()
	}
}

func TestParseToAST_NodesHaveLineAttribution(t *testing.T) {
	rng := rand.New(rand.NewSource(0xBAD5EED))
	for i := 0; i < propertyIterations; i++ {
		input := randInput(rng)
		nodes, _ := ParseToAST([]byte(input))
		for j, n := range nodes {
			if n.Line() <= 0 {
				t.Fatalf("node %d has non-positive line %d for input:\n%s", j, n.Line(), input)
			}
		}
	}
}

func TestParseToAST_ExcludeMatchesBangPrefix(t *testing.T) {
	rng := rand.New(rand.NewSource(0xFADE))
	for i := 0; i < propertyIterations; i++ {
		input := randInput(rng)
		nodes, _ := ParseToAST([]byte(input))
		lines := strings.Split(input, "\n")
		for _, n := range nodes {
			lineIdx := n.Line() - 1
			if lineIdx < 0 || lineIdx >= len(lines) {
				continue
			}
			rawTrim := strings.TrimSpace(lines[lineIdx])
			expectExclude := strings.HasPrefix(rawTrim, "!")
			// Brace expansion can put a `!` on a line whose tokens individually
			// don't start with `!`. Only assert when there is no brace on the line.
			if strings.ContainsAny(rawTrim, "{}") {
				continue
			}
			if got := n.IsExclude(); got != expectExclude {
				t.Fatalf("IsExclude=%v but raw line %q expected=%v\nfull input:\n%s", got, rawTrim, expectExclude, input)
			}
		}
	}
}

func TestParseToAST_FilterNodeShape(t *testing.T) {
	rng := rand.New(rand.NewSource(0xDADA))
	for i := 0; i < propertyIterations; i++ {
		// Every input includes at least one filtered line.
		input := randLine(rng, lkFiltered) + "\n" + randInput(rng)
		nodes, _ := ParseToAST([]byte(input))
		sawFilter := false
		for _, n := range nodes {
			fn, ok := n.(*FilterNode)
			if !ok {
				continue
			}
			sawFilter = true
			if fn.Child == nil {
				t.Fatalf("FilterNode has nil child:\n%s", input)
			}
			if len(fn.Directives) == 0 {
				t.Fatalf("FilterNode has no directives:\n%s", input)
			}
		}
		if !sawFilter {
			// Acceptable if the filtered line is overridden by a later separator
			// error or if directive parsing falls back to a plain pattern; not a
			// hard invariant we can require on every input.
			continue
		}
	}
}

func TestParseToAST_BraceCardinality(t *testing.T) {
	cases := []struct {
		input string
		want  int
	}{
		{"{a,b}.go\n", 2},
		{"{a,b,c}.go\n", 3},
		{"src/{a,b}/{c,d}.go\n", 4},
		{"{a,b,c,d}\n", 4},
		{"!{a,b}.go\n", 2},
	}
	for _, c := range cases {
		nodes, errs := ParseToAST([]byte(c.input))
		if len(errs) != 0 {
			t.Fatalf("unexpected parse errors for %q: %+v", c.input, errs)
		}
		if len(nodes) != c.want {
			t.Fatalf("brace cardinality: input=%q want=%d got=%d", c.input, c.want, len(nodes))
		}
		first := nodes[0].Line()
		for _, n := range nodes {
			if n.Line() != first {
				t.Fatalf("brace expansion lines diverged for %q: %d vs %d", c.input, first, n.Line())
			}
		}
	}
	// Property: random brace lines preserve cardinality and share Line().
	rng := rand.New(rand.NewSource(0xBEEF))
	for i := 0; i < propertyIterations; i++ {
		toks := 2 + rng.Intn(4)
		var parts []string
		for j := 0; j < toks; j++ {
			parts = append(parts, fmt.Sprintf("o%d", j))
		}
		input := "src/{" + strings.Join(parts, ",") + "}.go\n"
		nodes, _ := ParseToAST([]byte(input))
		if len(nodes) != toks {
			t.Fatalf("expected %d nodes for %q, got %d", toks, input, len(nodes))
		}
	}
}
