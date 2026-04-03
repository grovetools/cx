package context

import (
	"bufio"
	"bytes"
	"fmt"
	"regexp"
	"strings"
)

// LintIssue represents a single issue found in the rules file.
type LintIssue struct {
	LineNum  int
	Line     string
	Severity string
	Message  string
}

// validDirectives contains all recognized directives for typo-checking.
// Includes both long and short forms.
var validDirectives = map[string]bool{
	"@alias": true, "@a": true,
	"@view": true, "@v": true,
	"@cmd": true, "@find": true, "@grep": true,
	"@default": true, "@concept": true,
	"@freeze-cache": true, "@no-expire": true,
	"@disable-cache": true, "@expire-time": true,
	"@include": true, "@changed": true, "@diff": true, "@tree": true,
	"@find!": true, "@grep!": true, "@grep-i": true,
}

// directiveRegex finds tokens that look like directives (@ followed by word chars and hyphens).
var directiveRegex = regexp.MustCompile(`@[a-zA-Z][a-zA-Z0-9-]*`)

// LintRules parses the active context rules and returns a list of potential issues.
func (m *Manager) LintRules() ([]LintIssue, error) {
	content, _, err := m.LoadRulesContent()
	if err != nil {
		return nil, fmt.Errorf("failed to load rules content: %w", err)
	}
	if len(content) == 0 {
		return nil, nil
	}

	var issues []LintIssue
	scanner := bufio.NewScanner(bytes.NewReader(content))
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)

		// Skip empty lines and comments
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		// 1. Directive Typos Check
		matches := directiveRegex.FindAllString(trimmed, -1)
		for _, match := range matches {
			if !validDirectives[match] {
				issues = append(issues, LintIssue{
					LineNum:  lineNum,
					Line:     trimmed,
					Severity: "Warning",
					Message:  fmt.Sprintf("Unrecognized directive '%s' - possible typo", match),
				})
			}
		}

		parsedLine := ParseRulesLine(trimmed)

		// Determine the file pattern for safety and zero-match checks
		var basePattern string
		switch parsedLine.Type {
		case LineTypePattern:
			basePattern = parsedLine.Parts["pattern"]
		case LineTypeExclude:
			basePattern = parsedLine.Parts["pattern"]
		case LineTypeFindDirective, LineTypeGrepDirective:
			if parsedLine.Parts["inline"] == "true" {
				basePattern = parsedLine.Parts["pattern"]
			}
		}

		// 2. Rule Safety & Breadth Check
		if basePattern != "" {
			if basePattern == "*" || basePattern == ".*" {
				issues = append(issues, LintIssue{
					LineNum:  lineNum,
					Line:     trimmed,
					Severity: "Warning",
					Message:  "Pattern is overly broad and may match too many files",
				})
			}

			if err := m.validateRuleSafety(basePattern); err != nil {
				issues = append(issues, LintIssue{
					LineNum:  lineNum,
					Line:     trimmed,
					Severity: "Warning",
					Message:  err.Error(),
				})
			}
		}

		// 3. Zero Matches Check (inclusion patterns only)
		// Skip excludes and aliases to keep lint fast
		if parsedLine.Type == LineTypePattern {
			files, err := m.resolveFilesFromPatterns([]string{basePattern})
			if err == nil && len(files) == 0 {
				issues = append(issues, LintIssue{
					LineNum:  lineNum,
					Line:     trimmed,
					Severity: "Warning",
					Message:  "Pattern matches 0 files in the workspace",
				})
			}
		}
	}

	return issues, scanner.Err()
}
