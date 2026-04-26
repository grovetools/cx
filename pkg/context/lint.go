package context

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"path/filepath"
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

var validDirectives = map[string]bool{
	"@alias": true, "@a": true,
	"@view": true, "@v": true,
	"@cmd": true, "@find": true, "@grep": true,
	"@default": true, "@concept": true,
	"@freeze-cache": true, "@no-expire": true,
	"@disable-cache": true, "@expire-time": true,
	"@include": true, "@changed": true, "@diff": true, "@tree": true,
	"@find!": true, "@grep!": true, "@grep-i": true, "@recent": true,
}

var directiveRegex = regexp.MustCompile(`@[a-zA-Z][a-zA-Z0-9-]*!?`)

// LintRulesFile parses a specific rules file and returns a list of potential issues.
func (m *Manager) LintRulesFile(rulesFilePath string) ([]LintIssue, error) {
	content, err := os.ReadFile(rulesFilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read rules file: %w", err)
	}
	return m.lintRulesContent(content)
}

// LintRules parses the active context rules and returns a list of potential issues.
func (m *Manager) LintRules() ([]LintIssue, error) {
	content, _, err := m.LoadRulesContent()
	if err != nil {
		return nil, fmt.Errorf("failed to load rules content: %w", err)
	}
	return m.lintRulesContent(content)
}

func (m *Manager) lintRulesContent(content []byte) ([]LintIssue, error) {
	if len(content) == 0 {
		return nil, nil
	}

	var issues []LintIssue

	// Directive-typo pass — operates on raw lines so unknown directives are
	// caught even when ParseToAST cannot make sense of the line.
	scanner := bufio.NewScanner(bytes.NewReader(content))
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		raw := scanner.Text()
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		for _, tok := range directiveRegex.FindAllString(trimmed, -1) {
			if !validDirectives[tok] {
				issues = append(issues, LintIssue{
					LineNum:  lineNum,
					Line:     trimmed,
					Severity: "Warning",
					Message:  fmt.Sprintf("Unrecognized directive '%s' - possible typo", tok),
				})
			}
		}
	}

	nodes, parseErrs := ParseToAST(content)

	for _, pe := range parseErrs {
		issues = append(issues, LintIssue{
			LineNum:  pe.Line,
			Severity: "Error",
			Message:  pe.Msg,
		})
	}

	for _, n := range nodes {
		raw := strings.TrimSpace(n.Raw())
		line := n.Line()

		switch node := n.(type) {
		case *GlobNode:
			issues = appendPatternIssues(issues, m, node.Pattern, raw, line)
		case *LiteralNode:
			issues = appendPatternIssues(issues, m, node.ExpectedPath, raw, line)
		case *FilterNode:
			for _, d := range node.Directives {
				if d.Name == "grep" || d.Name == "grep!" || d.Name == "grep-i" {
					if _, err := regexp.Compile(d.Query); err != nil {
						issues = append(issues, LintIssue{
							LineNum:  line,
							Line:     raw,
							Severity: "Warning",
							Message:  fmt.Sprintf("Invalid regex in @%s directive: %s", d.Name, err),
						})
					}
				}
			}
			switch child := node.Child.(type) {
			case *GlobNode:
				issues = appendPatternIssues(issues, m, child.Pattern, raw, line)
			case *LiteralNode:
				issues = appendPatternIssues(issues, m, child.ExpectedPath, raw, line)
			}
		}
	}

	return issues, nil
}

func appendPatternIssues(issues []LintIssue, m *Manager, pattern, raw string, line int) []LintIssue {
	if pattern == "" {
		return issues
	}

	if containsTraversalEscape(m.workDir, pattern) {
		return append(issues, LintIssue{
			LineNum:  line,
			Line:     raw,
			Severity: "Error",
			Message:  fmt.Sprintf("Pattern '%s' attempts to traverse outside the workspace", pattern),
		})
	}

	if pattern == "*" || pattern == ".*" {
		issues = append(issues, LintIssue{
			LineNum:  line,
			Line:     raw,
			Severity: "Warning",
			Message:  "Pattern is overly broad and may match too many files",
		})
	}

	if err := m.validateRuleSafety(pattern); err != nil {
		issues = append(issues, LintIssue{
			LineNum:  line,
			Line:     raw,
			Severity: "Warning",
			Message:  err.Error(),
		})
	}

	if strings.HasSuffix(pattern, ".rules") {
		issues = append(issues, LintIssue{
			LineNum:  line,
			Line:     raw,
			Severity: "Warning",
			Message:  "this looks like a rules file path; did you mean @a:<workspace>::<name> to import its rules?",
		})
	}

	files, err := m.resolveFilesViaAST([]RuleInfo{{Pattern: pattern, IsExclude: false}})
	if err == nil && len(files) == 0 {
		issues = append(issues, LintIssue{
			LineNum:  line,
			Line:     raw,
			Severity: "Warning",
			Message:  "Pattern matches 0 files in the workspace",
		})
	}

	return issues
}

func FormatLintIssue(issue LintIssue) string {
	return fmt.Sprintf("[%s] Line %d: %s", issue.Severity, issue.LineNum, issue.Message)
}

func LintIssuesByLine(issues []LintIssue) map[int][]LintIssue {
	m := make(map[int][]LintIssue, len(issues))
	for _, issue := range issues {
		m[issue.LineNum] = append(m[issue.LineNum], issue)
	}
	return m
}

func HighestSeverity(issues []LintIssue) string {
	severity := ""
	for _, issue := range issues {
		switch issue.Severity {
		case "Error":
			return "Error"
		case "Warning":
			severity = "Warning"
		case "Notice":
			if severity == "" {
				severity = "Notice"
			}
		}
	}
	return severity
}

func containsTraversalEscape(workDir, pattern string) bool {
	if !strings.Contains(pattern, "../") {
		return false
	}
	if workDir == "" {
		return strings.Contains(pattern, "../../")
	}
	abs := filepath.Clean(filepath.Join(workDir, pattern))
	rel, err := filepath.Rel(workDir, abs)
	if err != nil {
		return true
	}
	return strings.HasPrefix(rel, "..")
}
