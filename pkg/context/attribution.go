package context

import (
	"bufio"
	"path/filepath"
	"strings"
)

// RuleInfo holds a parsed rule with its line number.
type RuleInfo struct {
	Pattern   string
	IsExclude bool
	LineNum   int
}

// AttributionResult maps a line number to the list of files it includes.
type AttributionResult map[int][]string

// ExclusionResult maps a line number to the list of files it excluded.
type ExclusionResult map[int][]string

// ResolveFilesWithAttribution walks the filesystem once and attributes each included file
// to the rule that was responsible for its inclusion. It also tracks exclusions.
func (m *Manager) ResolveFilesWithAttribution(rulesContent string) (AttributionResult, []RuleInfo, ExclusionResult, error) {
	// Initialize alias resolver for @alias: directives
	resolver := NewAliasResolver()

	// 1. Parse rules content into a structured list with line numbers.
	// Resolve @alias: directives as we parse
	var rules []RuleInfo
	scanner := bufio.NewScanner(strings.NewReader(rulesContent))
	lineNum := 1
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Try to resolve @alias: directives
		isAliasLine := strings.HasPrefix(line, "@alias:") || strings.HasPrefix(line, "@a:")
		if resolver != nil && isAliasLine {
			resolvedLine, err := resolver.ResolveLine(line)
			if err == nil && resolvedLine != "" {
				line = resolvedLine
				// If the resolved line is just a directory path (no glob pattern),
				// append /** to match all files in that directory
				if !strings.Contains(line, "*") && !strings.Contains(line, "?") {
					line = line + "/**"
				}
			}
			// Note: if resolution fails, line remains as the @alias: directive
			// and will be skipped below
		}

		// Process the line if it's not a comment, directive (except resolved aliases), or separator
		if line != "" && !strings.HasPrefix(line, "#") && !strings.HasPrefix(line, "@") && line != "---" {
			isExclude := strings.HasPrefix(line, "!")
			pattern := line
			if isExclude {
				pattern = strings.TrimPrefix(line, "!")
			}
			rules = append(rules, RuleInfo{Pattern: pattern, IsExclude: isExclude, LineNum: lineNum})
		}
		lineNum++
	}

	// 2. Get all candidate files by first resolving only inclusion patterns
	// to find all potentially included files
	inclusionPatterns := []string{}
	for _, rule := range rules {
		if !rule.IsExclude {
			inclusionPatterns = append(inclusionPatterns, rule.Pattern)
		}
	}

	potentialFiles, err := m.resolveFilesFromPatterns(inclusionPatterns)
	if err != nil {
		return nil, nil, nil, err
	}

	// Also get the final file list with exclusions applied
	allPatterns := []string{}
	for _, rule := range rules {
		if rule.IsExclude {
			allPatterns = append(allPatterns, "!"+rule.Pattern)
		} else {
			allPatterns = append(allPatterns, rule.Pattern)
		}
	}

	allFiles, err := m.resolveFilesFromPatterns(allPatterns)
	if err != nil {
		return nil, nil, nil, err
	}

	result := make(AttributionResult)
	exclusions := make(ExclusionResult)

	// 3. For each included file, find the last matching rule (for attribution).
	for _, file := range allFiles {
		lastMatch := -1
		isIncluded := false

		// Get path relative to workDir for matching
		relPath, err := filepath.Rel(m.workDir, file)
		if err != nil {
			relPath = file
		}
		relPath = filepath.ToSlash(relPath)

		for _, rule := range rules {
			match := m.matchPattern(rule.Pattern, relPath)

			if match {
				lastMatch = rule.LineNum
				isIncluded = !rule.IsExclude
			}
		}

		// 4. If the last match was an inclusion rule, attribute the file to that line.
		if isIncluded && lastMatch != -1 {
			result[lastMatch] = append(result[lastMatch], file)
		}
	}

	// 5. For each potential file, determine if it was excluded and by which rule
	for _, file := range potentialFiles {
		// Get path relative to workDir for matching
		relPath, err := filepath.Rel(m.workDir, file)
		if err != nil {
			relPath = file
		}
		relPath = filepath.ToSlash(relPath)

		lastMatch := -1
		wasExcluded := false

		// Check if this file matches any rule
		for _, rule := range rules {
			match := m.matchPattern(rule.Pattern, relPath)

			if match {
				lastMatch = rule.LineNum
				wasExcluded = rule.IsExclude
			}
		}

		// If the last matching rule was an exclusion, track it
		if wasExcluded && lastMatch != -1 {
			exclusions[lastMatch] = append(exclusions[lastMatch], file)
		}
	}

	return result, rules, exclusions, nil
}

// matchPattern matches a file path against a pattern using gitignore-style matching
func (m *Manager) matchPattern(pattern, relPath string) bool {
	// Handle ** patterns
	if strings.Contains(pattern, "**") {
		return matchDoubleStarPattern(pattern, relPath)
	}

	// Handle single * or ? patterns
	if matched, _ := filepath.Match(pattern, relPath); matched {
		return true
	}

	// If pattern doesn't contain /, try matching just the basename
	if !strings.Contains(pattern, "/") {
		if matched, _ := filepath.Match(pattern, filepath.Base(relPath)); matched {
			return true
		}
	}

	return false
}
