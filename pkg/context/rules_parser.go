package context

import (
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
