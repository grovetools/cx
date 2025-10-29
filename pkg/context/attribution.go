package context

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// RuleInfo holds a parsed rule with its line number and origin.
type RuleInfo struct {
	Pattern          string
	IsExclude        bool
	LineNum          int    // The line number in its original source file
	EffectiveLineNum int    // The line number in the root file that caused this rule to be included
	Directive        string `json:"directive,omitempty"`      // e.g., "find" or "grep"
	DirectiveQuery   string `json:"directiveQuery,omitempty"` // the search query
}

// ImportInfo holds information about a ruleset import with its line number.
type ImportInfo struct {
	ImportIdentifier string // e.g., "project:ruleset"
	LineNum          int    // The line number where this import appears
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
	// 1. Write rulesContent to a temporary file so we can use expandAllRules
	tmpFile, err := os.CreateTemp("", "grove-rules-*.rules")
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("failed to create temp rules file: %w", err)
	}
	defer os.Remove(tmpFile.Name())
	defer tmpFile.Close()

	if _, err := tmpFile.WriteString(rulesContent); err != nil {
		return nil, nil, nil, nil, fmt.Errorf("failed to write temp rules file: %w", err)
	}
	tmpFile.Close()

	// 2. Use expandAllRules to get all rules with proper import handling
	hotRules, coldRules, _, err := m.expandAllRules(tmpFile.Name(), make(map[string]bool), 0)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("failed to expand rules: %w", err)
	}

	// 3. Combine hot and cold rules into a single list (for attribution purposes, we treat them the same)
	allRules := append(hotRules, coldRules...)

	// 4. Parse the original rulesContent separately to build the RuleInfo list for backward compatibility
	// This preserves the original line numbers from the input content
	var rawRules []RuleInfo
	scanner := bufio.NewScanner(strings.NewReader(rulesContent))
	lineNum := 1
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Skip comments, directives, and separators
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "@") || line == "---" {
			lineNum++
			continue
		}

		// Check for search directives
		basePattern, directive, query, hasDirective := parseSearchDirective(line)

		// Apply brace expansion to the base pattern
		expandedPatterns := ExpandBraces(basePattern)

		// Create a rule for each expanded pattern
		for _, expandedPattern := range expandedPatterns {
			isExclude := strings.HasPrefix(expandedPattern, "!")
			pattern := expandedPattern
			if isExclude {
				pattern = strings.TrimPrefix(expandedPattern, "!")
			}

			ruleInfo := RuleInfo{
				Pattern:          pattern,
				IsExclude:        isExclude,
				LineNum:          lineNum,
				EffectiveLineNum: lineNum,
			}

			if hasDirective {
				ruleInfo.Directive = directive
				ruleInfo.DirectiveQuery = query
			}

			rawRules = append(rawRules, ruleInfo)
		}
		lineNum++
	}

	// 5. Extract patterns from expanded rules for file resolution
	inclusionPatterns := []string{}
	for _, rule := range allRules {
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
	for _, rule := range allRules {
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

	// 6. For each included file, find the last matching rule (for attribution).
	// Use EffectiveLineNum for attribution to handle imported rulesets correctly.
	for _, file := range allFiles {
		lastMatchEffectiveLineNum := -1
		isIncluded := false

		// Get path relative to workDir for matching
		relPath, err := filepath.Rel(m.workDir, file)
		if err != nil {
			relPath = file
		}
		relPath = filepath.ToSlash(relPath)

		// Track which rules had base pattern match but were filtered
		filteredFromLines := []int{}

		for _, rule := range allRules {
			// For absolute path patterns (e.g., from Git repos), match against absolute path
			// For relative patterns, match against relative path
			pathToMatch := relPath
			if filepath.IsAbs(rule.Pattern) {
				pathToMatch = filepath.ToSlash(file)
			}

			// Skip patterns that shouldn't match external files
			// If the file is outside workDir (relPath starts with ..) and the pattern is not
			// absolute and doesn't start with .., then skip this pattern
			isExternalFile := strings.HasPrefix(relPath, "..")
			isExternalPattern := filepath.IsAbs(rule.Pattern) || strings.HasPrefix(rule.Pattern, "..")
			if isExternalFile && !isExternalPattern {
				continue
			}

			baseMatch := m.matchPattern(rule.Pattern, pathToMatch)

			// Track if base pattern matched but directive filtered it out
			if baseMatch && rule.Directive != "" && !rule.IsExclude {
				// Apply directive filter
				directiveFiltered, err := m.applyDirectiveFilter([]string{file}, rule.Directive, rule.DirectiveQuery)
				if err != nil || len(directiveFiltered) == 0 {
					// Base pattern matched but directive filtered it out
					// We'll record this after we know which line won
					filteredFromLines = append(filteredFromLines, rule.EffectiveLineNum)
					baseMatch = false
				}
			}

			if baseMatch {
				lastMatchEffectiveLineNum = rule.EffectiveLineNum
				isIncluded = !rule.IsExclude
			}
		}

		// 7. If the last match was an inclusion rule, attribute the file to that line.
		if isIncluded && lastMatchEffectiveLineNum != -1 {
			result[lastMatchEffectiveLineNum] = append(result[lastMatchEffectiveLineNum], file)

			// Record filtered matches with the winning line number
			for _, filteredLineNum := range filteredFromLines {
				filtered[filteredLineNum] = append(filtered[filteredLineNum], FilteredFileInfo{
					File:           file,
					WinningLineNum: lastMatchEffectiveLineNum,
				})
			}
		}
	}

	// 8. For each potential file, determine if it was excluded and by which rule
	for _, file := range potentialFiles {
		// Get path relative to workDir for matching
		relPath, err := filepath.Rel(m.workDir, file)
		if err != nil {
			relPath = file
		}
		relPath = filepath.ToSlash(relPath)

		lastMatchEffectiveLineNum := -1
		wasExcluded := false

		// Check if this file matches any rule
		for _, rule := range allRules {
			// For absolute path patterns (e.g., from Git repos), match against absolute path
			// For relative patterns, match against relative path
			pathToMatch := relPath
			if filepath.IsAbs(rule.Pattern) {
				pathToMatch = filepath.ToSlash(file)
			}

			// Skip patterns that shouldn't match external files
			isExternalFile := strings.HasPrefix(relPath, "..")
			isExternalPattern := filepath.IsAbs(rule.Pattern) || strings.HasPrefix(rule.Pattern, "..")
			if isExternalFile && !isExternalPattern {
				continue
			}

			match := m.matchPattern(rule.Pattern, pathToMatch)

			// If the pattern matches, check if directive filter passes (if present)
			if match && rule.Directive != "" && !rule.IsExclude {
				// Apply directive filter
				filteredResult, err := m.applyDirectiveFilter([]string{file}, rule.Directive, rule.DirectiveQuery)
				if err != nil || len(filteredResult) == 0 {
					match = false
				}
			}

			if match {
				lastMatchEffectiveLineNum = rule.EffectiveLineNum
				wasExcluded = rule.IsExclude
			}
		}

		// If the last matching rule was an exclusion, track it
		if wasExcluded && lastMatchEffectiveLineNum != -1 {
			exclusions[lastMatchEffectiveLineNum] = append(exclusions[lastMatchEffectiveLineNum], file)
		}
	}

	return result, rawRules, exclusions, filtered, nil
}

// matchPattern matches a file path against a pattern using gitignore-style matching
func (m *Manager) matchPattern(pattern, relPath string) bool {
	// Normalize for case-insensitive filesystems (macOS/Windows)
	normalizedPattern := strings.ToLower(pattern)
	normalizedPath := strings.ToLower(relPath)

	// Handle ** patterns
	if strings.Contains(normalizedPattern, "**") {
		return matchDoubleStarPattern(normalizedPattern, normalizedPath)
	}

	// Handle single * or ? patterns
	if matched, _ := filepath.Match(normalizedPattern, normalizedPath); matched {
		return true
	}

	// If pattern doesn't contain /, it matches against the basename or any directory component.
	if !strings.Contains(normalizedPattern, "/") {
		// Check basename
		if matched, _ := filepath.Match(normalizedPattern, filepath.Base(normalizedPath)); matched {
			return true
		}
		// Check directory components
		parts := strings.Split(normalizedPath, "/")
		for _, part := range parts {
			if matched, _ := filepath.Match(normalizedPattern, part); matched {
				return true
			}
		}
	}

	return false
}
