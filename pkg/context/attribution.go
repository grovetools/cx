package context

import (
	"bufio"
	"path/filepath"
	"strings"
)

// RuleInfo holds a parsed rule with its line number.
type RuleInfo struct {
	Pattern        string
	IsExclude      bool
	LineNum        int
	Directive      string `json:"directive,omitempty"`      // e.g., "find" or "grep"
	DirectiveQuery string `json:"directiveQuery,omitempty"` // the search query
}

// AttributionResult maps a line number to the list of files it includes.
type AttributionResult map[int][]string

// ExclusionResult maps a line number to the list of files it excluded.
type ExclusionResult map[int][]string

// FilteredFileInfo tracks a file that was filtered and where it ended up
type FilteredFileInfo struct {
	File           string `json:"file"`
	WinningLineNum int    `json:"winningLineNum"`
}

// FilteredResult maps a line number to files that matched the base pattern but were filtered by directive.
type FilteredResult map[int][]FilteredFileInfo

// ResolveFilesWithAttribution walks the filesystem once and attributes each included file
// to the rule that was responsible for its inclusion. It also tracks exclusions and filtered matches.
func (m *Manager) ResolveFilesWithAttribution(rulesContent string) (AttributionResult, []RuleInfo, ExclusionResult, FilteredResult, error) {
	// Initialize alias resolver for @alias: directives
	resolver := m.getAliasResolver()

	// 1. Parse rules content into a structured list with line numbers.
	// Resolve @alias: directives as we parse
	var rules []RuleInfo
	scanner := bufio.NewScanner(strings.NewReader(rulesContent))
	lineNum := 1
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Handle Git aliases first (e.g., @a:git:owner/repo[@version])
		isGitAlias := false
		tempLineForCheck := line
		if strings.HasPrefix(tempLineForCheck, "!") {
			tempLineForCheck = strings.TrimSpace(strings.TrimPrefix(tempLineForCheck, "!"))
		}
		if strings.HasPrefix(tempLineForCheck, "@a:git:") || strings.HasPrefix(tempLineForCheck, "@alias:git:") {
			isGitAlias = true
		}

		if isGitAlias {
			// Transform Git alias to GitHub URL (same logic as in parseRulesFile)
			isExclude := false
			tempLine := line
			if strings.HasPrefix(tempLine, "!") {
				isExclude = true
				tempLine = strings.TrimPrefix(tempLine, "!")
			}
			tempLine = strings.TrimSpace(tempLine)

			prefix := "@a:git:"
			if strings.HasPrefix(tempLine, "@alias:git:") {
				prefix = "@alias:git:"
			}
			repoPart := strings.TrimPrefix(tempLine, prefix)
			githubURL := "https://github.com/" + repoPart

			if isExclude {
				line = "!" + githubURL
			} else {
				line = githubURL
			}
		} else {
			// Try to resolve workspace @alias: directives
			isAliasLine := strings.HasPrefix(line, "@alias:") || strings.HasPrefix(line, "@a:")
			if resolver != nil && isAliasLine {
				resolvedLine, err := resolver.ResolveLine(line)
				if err == nil && resolvedLine != "" {
					line = resolvedLine
					// If the resolved line is just a directory path (no glob pattern),
					// append /** to match all files in that directory
					// Check the base pattern part before any directives
					baseForCheck, _, _, _ := parseSearchDirective(line)
					if !strings.Contains(baseForCheck, "*") && !strings.Contains(baseForCheck, "?") {
						// Replace the base pattern with base + /**
						if baseForCheck == line {
							// No directive, just append /**
							line = line + "/**"
						} else {
							// Has directive, insert /** before it
							directivePart := strings.TrimPrefix(line, baseForCheck)
							line = baseForCheck + "/**" + directivePart
						}
					}
					// Convert absolute path to relative path for pattern matching
					// The resolved alias gives us an absolute path, but we need it relative to workDir
					if filepath.IsAbs(line) {
						relLine, err := filepath.Rel(m.workDir, line)
						if err == nil {
							line = filepath.ToSlash(relLine)
						}
					}
				}
				// Note: if resolution fails, line remains as the @alias: directive
				// and will be skipped below
			}
		}

		// Process the line if it's not a comment, directive (except resolved aliases), or separator
		if line != "" && !strings.HasPrefix(line, "#") && !strings.HasPrefix(line, "@") && line != "---" {
			// Check for search directives
			basePattern, directive, query, hasDirective := parseSearchDirective(line)

			// Apply brace expansion to the base pattern
			expandedPatterns := expandBraces(basePattern)

			// Create a rule for each expanded pattern
			for _, expandedPattern := range expandedPatterns {
				isExclude := strings.HasPrefix(expandedPattern, "!")
				pattern := expandedPattern
				if isExclude {
					pattern = strings.TrimPrefix(expandedPattern, "!")
				}

				ruleInfo := RuleInfo{
					Pattern:   pattern,
					IsExclude: isExclude,
					LineNum:   lineNum,
				}

				if hasDirective {
					ruleInfo.Directive = directive
					ruleInfo.DirectiveQuery = query
				}

				rules = append(rules, ruleInfo)
			}
		}
		lineNum++
	}

	// 2. Get all candidate files by first resolving only inclusion patterns
	// to find all potentially included files
	inclusionPatterns := []string{}
	for _, rule := range rules {
		if !rule.IsExclude {
			pattern := rule.Pattern
			// Encode directive if present
			if rule.Directive != "" {
				pattern = pattern + "|||" + rule.Directive + "|||" + rule.DirectiveQuery
			}
			inclusionPatterns = append(inclusionPatterns, pattern)
		}
	}

	potentialFiles, err := m.resolveFilesFromPatterns(inclusionPatterns)
	if err != nil {
		return nil, nil, nil, nil, err
	}

	// Also get the final file list with exclusions applied
	allPatterns := []string{}
	for _, rule := range rules {
		pattern := rule.Pattern
		// Encode directive if present
		if rule.Directive != "" {
			pattern = pattern + "|||" + rule.Directive + "|||" + rule.DirectiveQuery
		}

		if rule.IsExclude {
			allPatterns = append(allPatterns, "!"+pattern)
		} else {
			allPatterns = append(allPatterns, pattern)
		}
	}

	allFiles, err := m.resolveFilesFromPatterns(allPatterns)
	if err != nil {
		return nil, nil, nil, nil, err
	}

	result := make(AttributionResult)
	exclusions := make(ExclusionResult)
	filtered := make(FilteredResult)

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

		// Track which rules had base pattern match but were filtered
		filteredFromLines := []int{}

		for _, rule := range rules {
			baseMatch := m.matchPattern(rule.Pattern, relPath)

			// Track if base pattern matched but directive filtered it out
			if baseMatch && rule.Directive != "" && !rule.IsExclude {
				// Apply directive filter
				directiveFiltered, err := m.applyDirectiveFilter([]string{file}, rule.Directive, rule.DirectiveQuery)
				if err != nil || len(directiveFiltered) == 0 {
					// Base pattern matched but directive filtered it out
					// We'll record this after we know which line won
					filteredFromLines = append(filteredFromLines, rule.LineNum)
					baseMatch = false
				}
			}

			if baseMatch {
				lastMatch = rule.LineNum
				isIncluded = !rule.IsExclude
			}
		}

		// 4. If the last match was an inclusion rule, attribute the file to that line.
		if isIncluded && lastMatch != -1 {
			result[lastMatch] = append(result[lastMatch], file)

			// Record filtered matches with the winning line number
			for _, filteredLineNum := range filteredFromLines {
				filtered[filteredLineNum] = append(filtered[filteredLineNum], FilteredFileInfo{
					File:           file,
					WinningLineNum: lastMatch,
				})
			}
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

			// If the pattern matches, check if directive filter passes (if present)
			if match && rule.Directive != "" && !rule.IsExclude {
				// Apply directive filter
				filtered, err := m.applyDirectiveFilter([]string{file}, rule.Directive, rule.DirectiveQuery)
				if err != nil || len(filtered) == 0 {
					match = false
				}
			}

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

	return result, rules, exclusions, filtered, nil
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
