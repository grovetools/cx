package context

import "testing"

func TestStripComments(t *testing.T) {
	tests := []struct {
		name string
		path string
		in   string
		want string
	}{
		{
			name: "go line comment on its own line is dropped",
			path: "main.go",
			in:   "package main\n// a comment\nfunc main() {}\n",
			want: "package main\nfunc main() {}\n",
		},
		{
			name: "go trailing comment removed, code kept and trimmed",
			path: "main.go",
			in:   "x := 1 // set x\n",
			want: "x := 1\n",
		},
		{
			name: "go block comment single line dropped",
			path: "main.go",
			in:   "a\n/* block */\nb\n",
			want: "a\nb\n",
		},
		{
			name: "go multi-line block comment removed, lines preserved for surrounding code",
			path: "main.go",
			// Whitespace directly around a removed inline block comment is kept
			// verbatim (here the space left by the closing "*/ ").
			in:   "before /* c1\nc2\nc3 */ after\n",
			want: "before\n after\n",
		},
		{
			name: "go // inside double-quoted string preserved",
			path: "main.go",
			in:   "u := \"http://example.com\" // real comment\n",
			want: "u := \"http://example.com\"\n",
		},
		{
			name: "go // inside raw string preserved",
			path: "main.go",
			in:   "s := `line // not a comment`\n",
			want: "s := `line // not a comment`\n",
		},
		{
			name: "go rune literal with quote not confused",
			path: "main.go",
			in:   "c := '\\'' // trailing\nd := 'a'\n",
			want: "c := '\\''\nd := 'a'\n",
		},
		{
			name: "blank lines in code preserved",
			path: "main.go",
			in:   "a\n\nb\n",
			want: "a\n\nb\n",
		},
		{
			name: "rust lifetime not treated as char literal",
			path: "lib.rs",
			in:   "fn f<'a>(x: &'a str) -> &'a str { x } // c\n",
			want: "fn f<'a>(x: &'a str) -> &'a str { x }\n",
		},
		{
			name: "rust nested block comment",
			path: "lib.rs",
			in:   "a /* outer /* inner */ still */ b\n",
			want: "a  b\n",
		},
		{
			name: "rust raw string with hashes preserves slashes",
			path: "lib.rs",
			in:   "let s = r#\"a // b /* c */\"#; // strip me\n",
			want: "let s = r#\"a // b /* c */\"#;\n",
		},
		{
			name: "typescript single-quote string is a full string",
			path: "app.ts",
			in:   "const s = 'a // b'; // strip\n",
			want: "const s = 'a // b';\n",
		},
		{
			name: "typescript template literal preserved",
			path: "app.ts",
			in:   "const s = `x // y`; // strip\n",
			want: "const s = `x // y`;\n",
		},
		{
			name: "css block comment removed, no line comments",
			path: "a.css",
			in:   ".x { color: red; /* nice */ }\n",
			want: ".x { color: red;  }\n",
		},
		{
			name: "css double-slash is not a comment",
			path: "a.css",
			in:   ".x { background: url(http://e.com/a); }\n",
			want: ".x { background: url(http://e.com/a); }\n",
		},
		{
			name: "scss allows line comments",
			path: "a.scss",
			in:   "$x: 1; // note\n.y { color: red; }\n",
			want: "$x: 1;\n.y { color: red; }\n",
		},
		{
			name: "html comment removed",
			path: "index.html",
			in:   "<div></div>\n<!-- hidden -->\n<span></span>\n",
			want: "<div></div>\n<span></span>\n",
		},
		{
			name: "html multi-line comment",
			path: "index.html",
			in:   "<a/>\n<!-- one\ntwo -->\n<b/>\n",
			want: "<a/>\n<b/>\n",
		},
		{
			name: "unsupported extension returns content unchanged",
			path: "notes.md",
			in:   "# Title\n<!-- keep -->\n// keep\n",
			want: "# Title\n<!-- keep -->\n// keep\n",
		},
		{
			name: "no trailing newline preserved",
			path: "main.go",
			in:   "a // c",
			want: "a",
		},
		{
			name: "file that is only a comment becomes empty",
			path: "main.go",
			in:   "// just a comment\n",
			want: "",
		},
		{
			name: "indentation-only blank line preserved when original was blank",
			path: "main.go",
			in:   "a\n   \nb\n",
			want: "a\n\nb\n",
		},
		{
			name: "consecutive comment lines all dropped",
			path: "main.go",
			in:   "a\n// one\n// two\nb\n",
			want: "a\nb\n",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := string(StripComments(tc.path, []byte(tc.in)))
			if got != tc.want {
				t.Errorf("StripComments(%q)\n in:   %q\n got:  %q\n want: %q", tc.path, tc.in, got, tc.want)
			}
		})
	}
}

func TestStripCommentsIdempotent(t *testing.T) {
	// Stripping already-stripped content should be a no-op.
	src := "package main\n// c\nfunc main() { x := \"a//b\" // t\n}\n"
	once := StripComments("main.go", []byte(src))
	twice := StripComments("main.go", once)
	if string(once) != string(twice) {
		t.Errorf("not idempotent:\n once:  %q\n twice: %q", once, twice)
	}
}
