package context

import (
	"bufio"
	"bytes"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// LineType represents the type of a rules file line
type LineType int

const (
	LineTypeUnknown LineType = iota
	LineTypeComment
	LineTypeSeparator
	LineTypeExclude
	LineTypeGitURL
	LineTypeGitURLRuleset
	LineTypeRulesetImport
	LineTypeAliasPattern
	LineTypeViewDirective
	LineTypeTreeDirective
	LineTypeCmdDirective
	LineTypeIncludeDirective
	LineTypeFindDirective
	LineTypeFindInvertedDirective
	LineTypeGrepDirective
	LineTypeGrepInvertedDirective
	LineTypeCombinedDirective
	LineTypeGrepIDirective
	LineTypeChangedDirective
	LineTypeDiffDirective
	LineTypeRecentDirective
	LineTypeOtherDirective
	LineTypePattern
	LineTypeEmpty
)

// ParsedLine represents a parsed line from a rules file
type ParsedLine struct {
	Type    LineType
	Content string
	Parts   map[string]string // For storing parsed components
}

var (
	// Comment: lines starting with #
	commentRegex = regexp.MustCompile(`^\s*#.*$`)

	// Separator: --- for hot/cold context
	separatorRegex = regexp.MustCompile(`^---\s*$`)

	// Exclusion: lines starting with !
	excludeRegex = regexp.MustCompile(`^\s*!.*$`)

	// Git URLs with ruleset: lines starting with git@ or http(s):// and containing ::
	gitURLRulesetRegex = regexp.MustCompile(`^\s*(git@|https?://)\S+::\S+\s*$`)

	// Git URLs: lines starting with git@ or http(s)://
	gitURLRegex = regexp.MustCompile(`^\s*(git@|https?://).*$`)

	// Ruleset import: @alias:project::ruleset or @a:project::ruleset
	rulesetImportRegex = regexp.MustCompile(`^\s*@(alias|a):\s*\S+::\S+\s*$`)

	// Alias pattern: @alias:workspace/path or @a:workspace/path
	// Note: We check for ruleset imports first, so this won't match those
	aliasPatternRegex = regexp.MustCompile(`^\s*@(alias|a):\s*\S+`)

	// View directive: @view: or @v:
	viewDirectiveRegex = regexp.MustCompile(`^\s*@(view|v):`)

	// Tree directive: @tree:
	treeDirectiveRegex = regexp.MustCompile(`^\s*@tree:`)

	// Command directive: @cmd:
	cmdDirectiveRegex = regexp.MustCompile(`^\s*@cmd:`)

	// Include directive: @include:
	includeDirectiveRegex = regexp.MustCompile(`^\s*@include:`)

	// Find directive: @find: (standalone or inline)
	findDirectiveRegex = regexp.MustCompile(`@find:`)

	// Find inverted directive: @find!: (standalone or inline)
	findInvertedDirectiveRegex = regexp.MustCompile(`@find!:`)

	// Grep directive: @grep: (standalone or inline)
	grepDirectiveRegex = regexp.MustCompile(`@grep:`)

	// Grep inverted directive: @grep!: (standalone or inline)
	grepInvertedDirectiveRegex = regexp.MustCompile(`@grep!:`)

	// Grep-i directive: @grep-i: (standalone or inline, case-insensitive grep)
	grepIDirectiveRegex = regexp.MustCompile(`@grep-i:`)

	// Changed directive: @changed: (standalone or inline)
	changedDirectiveRegex = regexp.MustCompile(`@changed:`)

	// Recent directive: @recent: (standalone or inline)
	recentDirectiveRegex = regexp.MustCompile(`@recent:`)

	// Diff directive: @diff: (standalone)
	diffDirectiveRegex = regexp.MustCompile(`^\s*@diff:`)

	// Other directives: @default, @freeze-cache, @no-expire, @disable-cache, @expire-time
	otherDirectiveRegex = regexp.MustCompile(`^\s*@(default|freeze-cache|no-expire|disable-cache|expire-time):?`)
)

// ParseRulesLine parses a single line from a rules file and returns its type and parsed components
func ParseRulesLine(line string) ParsedLine {
	trimmed := strings.TrimSpace(line)

	// Empty line
	if trimmed == "" {
		return ParsedLine{
			Type:    LineTypeEmpty,
			Content: line,
			Parts:   make(map[string]string),
		}
	}

	// Comment
	if commentRegex.MatchString(line) {
		return ParsedLine{
			Type:    LineTypeComment,
			Content: line,
			Parts:   make(map[string]string),
		}
	}

	// Separator
	if separatorRegex.MatchString(line) {
		return ParsedLine{
			Type:    LineTypeSeparator,
			Content: line,
			Parts:   make(map[string]string),
		}
	}

	// Exclusion pattern
	if excludeRegex.MatchString(line) {
		return ParsedLine{
			Type:    LineTypeExclude,
			Content: line,
			Parts:   map[string]string{"pattern": strings.TrimSpace(strings.TrimPrefix(trimmed, "!"))},
		}
	}

	// Git URL with ruleset (must be checked BEFORE regular Git URL)
	if gitURLRulesetRegex.MatchString(line) {
		parts := parseGitURLRuleset(trimmed)
		return ParsedLine{
			Type:    LineTypeGitURLRuleset,
			Content: line,
			Parts:   parts,
		}
	}

	// Git URL
	if gitURLRegex.MatchString(line) {
		return ParsedLine{
			Type:    LineTypeGitURL,
			Content: line,
			Parts:   map[string]string{"url": trimmed},
		}
	}

	// Check for ruleset import FIRST (more specific pattern)
	// Ruleset import: @alias:project::ruleset
	if rulesetImportRegex.MatchString(line) {
		parts := parseRulesetImport(trimmed)
		return ParsedLine{
			Type:    LineTypeRulesetImport,
			Content: line,
			Parts:   parts,
		}
	}

	// Alias pattern: @alias:workspace/path (checked after ruleset imports)
	if aliasPatternRegex.MatchString(line) {
		// Additional check: skip if it contains :: (shouldn't happen due to order, but safety)
		if !strings.Contains(trimmed, "::") {
			parts := parseAliasPattern(trimmed)
			return ParsedLine{
				Type:    LineTypeAliasPattern,
				Content: line,
				Parts:   parts,
			}
		}
	}

	// View directive
	if viewDirectiveRegex.MatchString(line) {
		parts := parseViewDirective(trimmed)
		return ParsedLine{
			Type:    LineTypeViewDirective,
			Content: line,
			Parts:   parts,
		}
	}

	// Tree directive
	if treeDirectiveRegex.MatchString(line) {
		parts := parseTreeDirective(trimmed)
		return ParsedLine{
			Type:    LineTypeTreeDirective,
			Content: line,
			Parts:   parts,
		}
	}

	// Command directive
	if cmdDirectiveRegex.MatchString(line) {
		return ParsedLine{
			Type:    LineTypeCmdDirective,
			Content: line,
			Parts:   map[string]string{"command": strings.TrimSpace(strings.TrimPrefix(trimmed, "@cmd:"))},
		}
	}

	// Combined find+grep directive (must be checked before individual directives)
	hasFind := findDirectiveRegex.MatchString(line)
	hasGrep := grepDirectiveRegex.MatchString(line)
	if hasFind && hasGrep {
		findParts := parseSearchDirectiveLine(trimmed, "@find:")
		grepParts := parseSearchDirectiveLine(trimmed, "@grep:")
		parts := make(map[string]string)
		for k, v := range findParts {
			parts["find_"+k] = v
		}
		for k, v := range grepParts {
			parts["grep_"+k] = v
		}
		return ParsedLine{
			Type:    LineTypeCombinedDirective,
			Content: line,
			Parts:   parts,
		}
	}

	// Include directive
	if includeDirectiveRegex.MatchString(line) {
		return ParsedLine{
			Type:    LineTypeIncludeDirective,
			Content: line,
			Parts:   map[string]string{"ruleset": strings.TrimSpace(strings.TrimPrefix(trimmed, "@include:"))},
		}
	}

	// Find inverted directive (must check before @find:)
	if findInvertedDirectiveRegex.MatchString(line) {
		parts := parseSearchDirectiveLine(trimmed, "@find!:")
		return ParsedLine{
			Type:    LineTypeFindInvertedDirective,
			Content: line,
			Parts:   parts,
		}
	}

	// Grep inverted directive (must check before @grep:)
	if grepInvertedDirectiveRegex.MatchString(line) {
		parts := parseSearchDirectiveLine(trimmed, "@grep!:")
		return ParsedLine{
			Type:    LineTypeGrepInvertedDirective,
			Content: line,
			Parts:   parts,
		}
	}

	// Find directive (standalone or inline)
	if hasFind {
		parts := parseSearchDirectiveLine(trimmed, "@find:")
		return ParsedLine{
			Type:    LineTypeFindDirective,
			Content: line,
			Parts:   parts,
		}
	}

	// Grep-i directive (standalone or inline) - must be checked before @grep:
	if grepIDirectiveRegex.MatchString(line) {
		parts := parseSearchDirectiveLine(trimmed, "@grep-i:")
		return ParsedLine{
			Type:    LineTypeGrepIDirective,
			Content: line,
			Parts:   parts,
		}
	}

	// Grep directive (standalone or inline)
	if hasGrep {
		parts := parseSearchDirectiveLine(trimmed, "@grep:")
		return ParsedLine{
			Type:    LineTypeGrepDirective,
			Content: line,
			Parts:   parts,
		}
	}

	// Changed directive (standalone or inline)
	if changedDirectiveRegex.MatchString(line) {
		parts := parseSearchDirectiveLine(trimmed, "@changed:")
		return ParsedLine{
			Type:    LineTypeChangedDirective,
			Content: line,
			Parts:   parts,
		}
	}

	// Recent directive (standalone or inline)
	if recentDirectiveRegex.MatchString(line) {
		parts := parseSearchDirectiveLine(trimmed, "@recent:")
		return ParsedLine{
			Type:    LineTypeRecentDirective,
			Content: line,
			Parts:   parts,
		}
	}

	// Diff directive (standalone)
	if diffDirectiveRegex.MatchString(line) {
		parts := map[string]string{"ref": strings.TrimSpace(strings.TrimPrefix(trimmed, "@diff:"))}
		return ParsedLine{
			Type:    LineTypeDiffDirective,
			Content: line,
			Parts:   parts,
		}
	}

	// Other directives
	if otherDirectiveRegex.MatchString(line) {
		return ParsedLine{
			Type:    LineTypeOtherDirective,
			Content: line,
			Parts:   make(map[string]string),
		}
	}

	// Regular pattern
	return ParsedLine{
		Type:    LineTypePattern,
		Content: line,
		Parts:   map[string]string{"pattern": trimmed},
	}
}

// parseGitURLRuleset parses a Git URL with ruleset like https://github.com/owner/repo::ruleset
func parseGitURLRuleset(line string) map[string]string {
	parts := make(map[string]string)

	// Split by ::
	if idx := strings.Index(line, "::"); idx != -1 {
		parts["url"] = strings.TrimSpace(line[:idx])
		parts["delimiter"] = "::"
		parts["ruleset"] = strings.TrimSpace(line[idx+2:])
	} else {
		// Fallback if :: not found (shouldn't happen due to regex)
		parts["url"] = line
	}

	return parts
}

// parseRulesetImport parses a ruleset import like @alias:project::ruleset
func parseRulesetImport(line string) map[string]string {
	parts := make(map[string]string)

	// Identify and store the directive
	directive := "@a:"
	if strings.HasPrefix(line, "@alias:") {
		directive = "@alias:"
	}
	parts["directive"] = directive

	// Remove @alias: or @a: prefix
	line = strings.TrimPrefix(line, "@alias:")
	line = strings.TrimPrefix(line, "@a:")
	line = strings.TrimSpace(line)

	// Split by ::
	if idx := strings.Index(line, "::"); idx != -1 {
		parts["alias"] = strings.TrimSpace(line[:idx])
		parts["delimiter"] = "::"
		parts["ruleset"] = strings.TrimSpace(line[idx+2:])
	}

	return parts
}

// parseAliasPattern parses an alias pattern like @alias:workspace/path
func parseAliasPattern(line string) map[string]string {
	parts := make(map[string]string)

	// Identify and store the directive
	directive := "@a:"
	if strings.HasPrefix(line, "@alias:") {
		directive = "@alias:"
	}
	parts["directive"] = directive

	// Remove @alias: or @a: prefix
	line = strings.TrimPrefix(line, "@alias:")
	line = strings.TrimPrefix(line, "@a:")
	line = strings.TrimSpace(line)

	// Separate alias from path
	pathIdx := strings.Index(line, "/")
	aliasPart := line
	if pathIdx != -1 {
		aliasPart = line[:pathIdx]
		parts["path"] = line[pathIdx:]
	}

	// Split alias into components (up to 3 components: ecosystem:repo:worktree)
	aliasComponents := strings.Split(aliasPart, ":")
	for i, comp := range aliasComponents {
		if i < 3 && comp != "" { // Max 3 components
			parts["component"+string(rune('1'+i))] = comp
		}
	}

	parts["full_alias"] = aliasPart

	return parts
}

// parseTreeDirective parses a tree directive like @tree: /path/to/dir or @tree: @a:project
func parseTreeDirective(line string) map[string]string {
	parts := make(map[string]string)

	line = strings.TrimPrefix(line, "@tree:")
	line = strings.TrimSpace(line)

	if strings.HasPrefix(line, "@alias:") || strings.HasPrefix(line, "@a:") {
		aliasValue := strings.TrimPrefix(line, "@alias:")
		aliasValue = strings.TrimPrefix(aliasValue, "@a:")
		parts["type"] = "alias"
		parts["value"] = strings.TrimSpace(aliasValue)
	} else {
		parts["type"] = "path"
		parts["value"] = line
	}

	return parts
}

// parseViewDirective parses a view directive like @view: @a:project or @v: /path/to/dir
func parseViewDirective(line string) map[string]string {
	parts := make(map[string]string)

	// Remove @view: or @v: prefix
	line = strings.TrimPrefix(line, "@view:")
	line = strings.TrimPrefix(line, "@v:")
	line = strings.TrimSpace(line)

	// Check if it contains an alias reference
	if strings.HasPrefix(line, "@alias:") || strings.HasPrefix(line, "@a:") {
		aliasValue := strings.TrimPrefix(line, "@alias:")
		aliasValue = strings.TrimPrefix(aliasValue, "@a:")
		parts["type"] = "alias"
		parts["value"] = strings.TrimSpace(aliasValue)
	} else {
		parts["type"] = "path"
		parts["value"] = line
	}

	return parts
}

// parseSearchDirectiveLine parses find/grep directives (standalone or inline)
func parseSearchDirectiveLine(line, prefix string) map[string]string {
	parts := make(map[string]string)

	// Check if it's inline (has pattern before the directive)
	if !strings.HasPrefix(strings.TrimSpace(line), prefix) {
		parts["inline"] = "true"
		// Split pattern and search parts
		if idx := strings.Index(line, prefix); idx != -1 {
			parts["pattern"] = strings.TrimSpace(line[:idx])
			parts["search"] = strings.TrimSpace(line[idx+len(prefix):])
		}
	} else {
		parts["inline"] = "false"
		parts["search"] = strings.TrimSpace(strings.TrimPrefix(line, prefix))
	}

	// Extract quoted query if present, otherwise use unquoted search as query
	if strings.HasPrefix(parts["search"], "\"") {
		if end := strings.Index(parts["search"][1:], "\""); end != -1 {
			parts["query"] = parts["search"][1 : end+1]
		}
	} else if prefix == "@changed:" {
		// @changed: does not require quotes around the ref
		parts["query"] = parts["search"]
	}
	if parts["query"] == "" && parts["search"] != "" {
		parts["query"] = parts["search"]
	}

	return parts
}

// stripInlineComments removes a trailing " # ..." comment unless the # is
// inside a double-quoted span (preserves @grep: "foo # bar").
func stripInlineComments(line string) string {
	inQuote := false
	for i := 0; i < len(line); i++ {
		c := line[i]
		if c == '"' {
			inQuote = !inQuote
			continue
		}
		if !inQuote && c == '#' && i > 0 && (line[i-1] == ' ' || line[i-1] == '\t') {
			return strings.TrimRight(line[:i], " \t")
		}
	}
	return line
}

// canonicalizePath applies pure string-only path normalization: ~ home
// expansion, leading ./ strip, trailing slash → /**.
func canonicalizePath(p string) string {
	if p == "" {
		return p
	}
	if strings.HasPrefix(p, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			p = filepath.Join(home, p[2:])
		}
	} else if p == "~" {
		if home, err := os.UserHomeDir(); err == nil {
			p = home
		}
	}
	for strings.HasPrefix(p, "./") {
		p = p[2:]
	}
	if strings.HasSuffix(p, "/") && p != "/" {
		p = p + "**"
	}
	return p
}

// classifyAlias splits an alias body into (target, ruleset). target is the
// part before "::" (or the whole body if "::" is absent).
func classifyAlias(body string) (target, ruleset string) {
	if idx := strings.Index(body, "::"); idx != -1 {
		return body[:idx], body[idx+2:]
	}
	return body, ""
}

// hasGlobMeta reports whether a pattern has wildcard metacharacters.
func hasGlobMeta(p string) bool {
	return strings.ContainsAny(p, "*?[")
}

// ParseToAST is the pure parser: bytes in, AST + non-fatal errors out. It
// performs no I/O beyond os.UserHomeDir for ~ expansion.
func ParseToAST(content []byte) ([]RuleNode, []ParseError) {
	var nodes []RuleNode
	var errs []ParseError

	scanner := bufio.NewScanner(bytes.NewReader(content))
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	lineNum := 0
	seenSeparator := false

	for scanner.Scan() {
		lineNum++
		raw := scanner.Text()
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		if trimmed == "---" {
			if seenSeparator {
				errs = append(errs, ParseError{Line: lineNum, Msg: "multiple '---' separators found; rules support exactly one hot/cold separator"})
				continue
			}
			seenSeparator = true
			continue
		}

		// 1. Brace-expand FIRST.
		expanded := ExpandBraces(trimmed)
		for _, tok := range expanded {
			tok = strings.TrimSpace(tok)
			if tok == "" {
				continue
			}
			// 2. Strip inline # comment (but preserve quoted #).
			tok = strings.TrimSpace(stripInlineComments(tok))
			if tok == "" {
				continue
			}

			// Detect exclusion.
			excluded := false
			if strings.HasPrefix(tok, "!") {
				excluded = true
				tok = strings.TrimSpace(tok[1:])
			}

			// Capital-letter directive guard.
			if strings.HasPrefix(tok, "@") && len(tok) > 1 {
				rest := tok[1:]
				if rest != "" && rest[0] >= 'A' && rest[0] <= 'Z' && strings.Contains(rest, ":") {
					errs = append(errs, ParseError{Line: lineNum, Msg: "unrecognized directive '" + tok[:strings.Index(tok, ":")+1] + "' - did you mean lowercase?"})
					continue
				}
			}

			// Dangling ::name.
			if strings.HasPrefix(tok, "::") {
				errs = append(errs, ParseError{Line: lineNum, Msg: "invalid ruleset import '" + tok + "' - missing workspace prefix"})
				continue
			}

			// Multi-alias-per-line detection (after brace expansion).
			if countAliasMarkers(tok) > 1 {
				errs = append(errs, ParseError{Line: lineNum, Msg: "multiple aliases on a single line are not supported"})
				continue
			}

			// 3. Extract directives.
			basePattern, directives, hasDirectives := parseSearchDirectives(tok)

			// 4. Classify and canonicalize per-type.
			var node RuleNode
			switch {
			case strings.HasPrefix(basePattern, "@a:") || strings.HasPrefix(basePattern, "@alias:"):
				body := strings.TrimPrefix(basePattern, "@alias:")
				body = strings.TrimPrefix(body, "@a:")
				body = strings.TrimSpace(body)
				if body == "" {
					errs = append(errs, ParseError{Line: lineNum, Msg: "empty alias target provided"})
					continue
				}
				target, ruleset := classifyAlias(body)
				// Apply trailing-slash → /** to the target's path portion.
				if ruleset == "" {
					target = canonicalizeAliasTarget(target)
				}
				node = &ImportNode{
					Target:   target,
					Ruleset:  ruleset,
					LineNum:  lineNum,
					RawText:  raw,
					Excluded: excluded,
				}
			case strings.HasPrefix(basePattern, "@cmd:"):
				cmd := strings.TrimSpace(strings.TrimPrefix(basePattern, "@cmd:"))
				node = &CommandNode{Command: cmd, LineNum: lineNum, RawText: raw, Excluded: excluded}
			default:
				canonical := canonicalizePath(basePattern)
				if hasGlobMeta(canonical) {
					node = &GlobNode{Pattern: canonical, LineNum: lineNum, RawText: raw, Excluded: excluded}
				} else {
					node = &LiteralNode{ExpectedPath: canonical, LineNum: lineNum, RawText: raw, Excluded: excluded}
				}
			}

			if hasDirectives {
				node = &FilterNode{
					Child:      node,
					Directives: directives,
					LineNum:    lineNum,
					RawText:    raw,
					Excluded:   excluded,
				}
			}
			nodes = append(nodes, node)
		}
	}

	return nodes, errs
}

// canonicalizeAliasTarget applies trailing-slash → /** to alias targets.
func canonicalizeAliasTarget(t string) string {
	if strings.HasSuffix(t, "/") && t != "/" {
		return t + "**"
	}
	return t
}

// countAliasMarkers counts independent @a:/@alias: occurrences in a line.
func countAliasMarkers(s string) int {
	n := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '@' {
			rest := s[i:]
			if strings.HasPrefix(rest, "@a:") || strings.HasPrefix(rest, "@alias:") {
				n++
			}
		}
	}
	return n
}
