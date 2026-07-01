package context

import (
	"path/filepath"
	"strings"
	"unicode/utf8"
)

// Comment stripping produces a "code-only" view of a source file for LLM
// context: comments are removed while string and character literals are left
// untouched (so a "//" or "/*" that lives inside a string never triggers a
// strip). Lines that consisted solely of a comment are dropped entirely;
// trailing comments are removed and the code kept (with trailing whitespace
// trimmed). Unsupported file types are returned verbatim.
//
// The stripper is deliberately lexical (a small hand-rolled scanner), not a
// full parser — it understands just enough of each language family's literal
// and comment syntax to avoid corrupting code. Currently supported: Go, Rust,
// TypeScript/JavaScript, HTML, and CSS (plus the SCSS/LESS and JS/TS
// dialects), matching the initial scope for this feature.

// quoteKind describes how a language treats the single-quote (') character.
type quoteKind int

const (
	quoteNone   quoteKind = iota // ' is not special
	quoteChar                    // ' delimits a short character/rune literal (Go, Rust)
	quoteString                  // ' delimits an ordinary string literal (JS/TS, CSS)
)

// langSpec captures the minimal lexical rules needed to strip comments while
// preserving string/char literals for one language family.
type langSpec struct {
	lineComments   []string  // tokens that begin a line comment, e.g. "//"
	blockStart     string    // block-comment open, e.g. "/*" ("" = none)
	blockEnd       string    // block-comment close, e.g. "*/"
	nestedBlock    bool      // block comments nest (Rust)
	doubleQuote    bool      // "..." string literals (with backslash escapes)
	singleQuote    quoteKind // how ' is treated
	backtick       bool      // `...` string literals
	backtickEscape bool      // backtick strings honor backslash escapes (JS/TS); Go raw strings do not
	rustRawStrings bool      // r"...", r#"..."# raw strings
}

// specForExt returns the lexical spec for a lower-cased file extension, and
// whether comment stripping is supported for it at all.
func specForExt(ext string) (langSpec, bool) {
	switch ext {
	case ".go":
		return langSpec{
			lineComments: []string{"//"},
			blockStart:   "/*", blockEnd: "*/",
			doubleQuote: true, singleQuote: quoteChar, backtick: true,
		}, true
	case ".rs":
		return langSpec{
			lineComments: []string{"//"},
			blockStart:   "/*", blockEnd: "*/", nestedBlock: true,
			doubleQuote: true, singleQuote: quoteChar, rustRawStrings: true,
		}, true
	case ".ts", ".tsx", ".js", ".jsx", ".mjs", ".cjs", ".mts", ".cts":
		return langSpec{
			lineComments: []string{"//"},
			blockStart:   "/*", blockEnd: "*/",
			doubleQuote: true, singleQuote: quoteString, backtick: true, backtickEscape: true,
		}, true
	case ".css":
		return langSpec{
			blockStart: "/*", blockEnd: "*/",
			doubleQuote: true, singleQuote: quoteString,
		}, true
	case ".scss", ".sass", ".less":
		return langSpec{
			lineComments: []string{"//"},
			blockStart:   "/*", blockEnd: "*/",
			doubleQuote: true, singleQuote: quoteString,
		}, true
	case ".html", ".htm":
		// HTML comments are modeled as a plain block comment; no string
		// literals participate (attribute values never contain "<!--").
		return langSpec{blockStart: "<!--", blockEnd: "-->"}, true
	default:
		return langSpec{}, false
	}
}

// StripComments removes comments from source content, choosing comment/string
// rules from the file extension of path. String and character literals are
// preserved. For unsupported extensions the content is returned unchanged.
func StripComments(path string, content []byte) []byte {
	spec, ok := specForExt(strings.ToLower(filepath.Ext(path)))
	if !ok {
		return content
	}
	stripped := scanAndStrip(spec, content)
	return dropCommentOnlyLines(content, stripped)
}

// scanAndStrip walks content byte-by-byte, emitting code and string/char
// literals while dropping comment bodies. Newlines are always preserved
// (including those inside block comments) so the output has a 1:1 line
// correspondence with the input — dropCommentOnlyLines relies on this.
func scanAndStrip(spec langSpec, content []byte) []byte {
	n := len(content)
	out := make([]byte, 0, n)
	i := 0
	for i < n {
		c := content[i]

		// String / char literals first, so a "//" or "/*" inside them is
		// treated as data, not a comment.
		switch {
		case spec.doubleQuote && c == '"':
			i = scanString(content, i, '"', true, &out)
			continue
		case spec.backtick && c == '`':
			i = scanString(content, i, '`', spec.backtickEscape, &out)
			continue
		case spec.singleQuote == quoteString && c == '\'':
			i = scanString(content, i, '\'', true, &out)
			continue
		case spec.singleQuote == quoteChar && c == '\'':
			if end, ok := matchCharLiteral(content, i); ok {
				out = append(out, content[i:end]...)
				i = end
				continue
			}
			// Not a char literal (e.g. a Rust lifetime like 'a) — emit as code.
			out = append(out, c)
			i++
			continue
		case spec.rustRawStrings && (c == 'r') && isRustRawStart(content, i):
			i = scanRustRawString(content, i, &out)
			continue
		}

		// Line comment?
		if matchLineComment(spec, content, i) {
			for i < n && content[i] != '\n' {
				i++
			}
			continue // the newline (if any) is emitted on the next iteration
		}

		// Block comment?
		if spec.blockStart != "" && hasPrefixAt(content, i, spec.blockStart) {
			i = skipBlockComment(spec, content, i, &out)
			continue
		}

		out = append(out, c)
		i++
	}
	return out
}

// scanString emits a string literal starting at content[i] (the opening
// delimiter) through its closing delimiter, honoring backslash escapes when
// escapes is true. Returns the index just past the close (or len on EOF).
func scanString(content []byte, i int, delim byte, escapes bool, out *[]byte) int {
	n := len(content)
	*out = append(*out, content[i]) // opening delimiter
	i++
	for i < n {
		c := content[i]
		if escapes && c == '\\' && i+1 < n {
			*out = append(*out, c, content[i+1])
			i += 2
			continue
		}
		*out = append(*out, c)
		i++
		if c == delim {
			return i
		}
	}
	return i
}

// matchCharLiteral reports whether content[i:] (with content[i] == '\”) is a
// single character/rune literal and, if so, the index just past its closing
// quote. It returns false for Rust lifetimes ('a, 'static) and anything else
// that is not a well-formed single-quote literal.
func matchCharLiteral(content []byte, i int) (int, bool) {
	n := len(content)
	j := i + 1
	if j >= n {
		return 0, false
	}
	if content[j] == '\\' {
		// Escape: skip the backslash and the first escaped byte, then look for
		// the closing quote within a small window (covers \n, \', \\, \xNN,
		// \u{...}, etc.).
		j += 2
		for k := 0; k < 16 && j < n; k++ {
			if content[j] == '\'' {
				return j + 1, true
			}
			j++
		}
		return 0, false
	}
	// Non-escape: exactly one rune, then the closing quote.
	r, sz := utf8.DecodeRune(content[j:])
	if r == utf8.RuneError && sz <= 1 {
		return 0, false
	}
	j += sz
	if j < n && content[j] == '\'' {
		return j + 1, true
	}
	return 0, false
}

// isRustRawStart reports whether content[i] ('r') begins a Rust raw string
// literal (r"...", r#"..."#, and the br"..." byte variant).
func isRustRawStart(content []byte, i int) bool {
	// Reject a plain identifier ending in 'r' (e.g. "for"), but allow the
	// byte-string prefix "br".
	if i > 0 && isIdentByte(content[i-1]) && content[i-1] != 'b' {
		return false
	}
	j := i + 1
	n := len(content)
	for j < n && content[j] == '#' {
		j++
	}
	return j < n && content[j] == '"'
}

// scanRustRawString emits a Rust raw string starting at content[i] ('r') and
// returns the index just past its close. Raw strings have no escapes; the
// closer is a quote followed by the same number of '#' as the opener.
func scanRustRawString(content []byte, i int, out *[]byte) int {
	n := len(content)
	j := i + 1
	hashes := 0
	for j < n && content[j] == '#' {
		hashes++
		j++
	}
	j++ // past the opening quote (guaranteed present by isRustRawStart)
	for j < n {
		if content[j] == '"' {
			k := j + 1
			cnt := 0
			for cnt < hashes && k < n && content[k] == '#' {
				cnt++
				k++
			}
			if cnt == hashes {
				j = k
				break
			}
		}
		j++
	}
	*out = append(*out, content[i:j]...)
	return j
}

// matchLineComment reports whether a line-comment token begins at content[i].
func matchLineComment(spec langSpec, content []byte, i int) bool {
	for _, tok := range spec.lineComments {
		if hasPrefixAt(content, i, tok) {
			return true
		}
	}
	return false
}

// skipBlockComment consumes a block comment starting at content[i] and returns
// the index just past its close. Newlines inside the comment are emitted to
// keep the line count stable. Honors nesting when spec.nestedBlock is set.
func skipBlockComment(spec langSpec, content []byte, i int, out *[]byte) int {
	n := len(content)
	i += len(spec.blockStart)
	depth := 1
	for i < n {
		if content[i] == '\n' {
			*out = append(*out, '\n')
			i++
			continue
		}
		if spec.nestedBlock && hasPrefixAt(content, i, spec.blockStart) {
			depth++
			i += len(spec.blockStart)
			continue
		}
		if hasPrefixAt(content, i, spec.blockEnd) {
			i += len(spec.blockEnd)
			depth--
			if depth == 0 {
				return i
			}
			continue
		}
		i++
	}
	return i
}

// dropCommentOnlyLines removes lines that became blank solely because a
// comment was stripped, and trims trailing whitespace left by removed trailing
// comments. It relies on scanAndStrip preserving newlines (orig and stripped
// have the same line count); if that invariant is somehow violated it falls
// back to returning stripped unchanged.
func dropCommentOnlyLines(orig, stripped []byte) []byte {
	origStr := string(orig)
	strippedStr := string(stripped)

	origLines := strings.Split(origStr, "\n")
	outLines := strings.Split(strippedStr, "\n")
	if len(origLines) != len(outLines) {
		return stripped
	}

	trailingNL := strings.HasSuffix(origStr, "\n")
	count := len(outLines)

	kept := make([]string, 0, count)
	for idx := 0; idx < count; idx++ {
		// The trailing "" element that Split produces for a final newline is
		// not a real line — skip it (the newline is re-added below).
		if trailingNL && idx == count-1 && outLines[idx] == "" && origLines[idx] == "" {
			continue
		}
		ol := outLines[idx]
		// A line that became blank only because its comment was removed
		// (the original had non-whitespace content) is dropped entirely.
		if strings.TrimSpace(ol) == "" && strings.TrimSpace(origLines[idx]) != "" {
			continue
		}
		kept = append(kept, strings.TrimRight(ol, " \t"))
	}

	if len(kept) == 0 {
		return []byte{}
	}
	res := strings.Join(kept, "\n")
	if trailingNL {
		res += "\n"
	}
	return []byte(res)
}

// isIdentByte reports whether b is an ASCII identifier byte.
func isIdentByte(b byte) bool {
	return b == '_' ||
		(b >= 'a' && b <= 'z') ||
		(b >= 'A' && b <= 'Z') ||
		(b >= '0' && b <= '9')
}

// hasPrefixAt reports whether content[i:] begins with prefix.
func hasPrefixAt(content []byte, i int, prefix string) bool {
	if i+len(prefix) > len(content) {
		return false
	}
	for k := 0; k < len(prefix); k++ {
		if content[i+k] != prefix[k] {
			return false
		}
	}
	return true
}
