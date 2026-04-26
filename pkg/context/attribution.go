package context

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/grovetools/core/pkg/profiling"
)

// SearchDirective represents a single search directive (@find: or @grep:) with its query.
type SearchDirective struct {
	Name  string `json:"name"`            // e.g., "find" or "grep"
	Query string `json:"query,omitempty"` // the search query
}

// RuleInfo holds a parsed rule with its line number and origin.
type RuleInfo struct {
	Pattern          string
	IsExclude        bool
	LineNum          int               // The line number in its original source file
	EffectiveLineNum int               // The line number in the root file that caused this rule to be included
	Directives       []SearchDirective `json:"directives,omitempty"` // search directives (@find:/@grep:)
}

// ImportInfo holds information about a ruleset import with its line number.
type ImportInfo struct {
	OriginalLine     string            `json:"originalLine,omitempty"` // The full original line text
	ImportIdentifier string            // e.g., "project:ruleset"
	LineNum          int               // The line number where this import appears
	Directives       []SearchDirective `json:"directives,omitempty"` // search directives (@find:/@grep:)
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
	defer profiling.Start("context.ResolveFilesWithAttribution").Stop()
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
	hotRules, coldRules, _, _, err := m.expandAllRules(tmpFile.Name(), make(map[string]bool), 0)
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
		basePattern, directives, hasDirectives := parseSearchDirectives(line)

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

			if hasDirectives {
				ruleInfo.Directives = directives
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
			// Encode directives if present
			pattern = encodeDirectives(pattern, rule.Directives)
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
		// Encode directives if present
		pattern = encodeDirectives(pattern, rule.Directives)

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

	// Phase 3A: single-pass node-based attribution. Build synthetic AST nodes
	// from the expanded rules and run ResolveAST against the legacy-discovered
	// file set. This deletes the second-pass m.matchPattern re-derivation.
	syntheticNodes := ruleInfosToNodes(allRules)
	allFilesUnion := make([]string, 0, len(allFiles)+len(potentialFiles))
	seenUnion := make(map[string]bool)
	for _, f := range allFiles {
		if !seenUnion[f] {
			seenUnion[f] = true
			allFilesUnion = append(allFilesUnion, f)
		}
	}
	for _, f := range potentialFiles {
		if !seenUnion[f] {
			seenUnion[f] = true
			allFilesUnion = append(allFilesUnion, f)
		}
	}
	astCtx := newProdResolutionContext(m).withFileSet(allFilesUnion)
	astAttr, astExcl, astFilt := ResolveAST(syntheticNodes, astCtx)

	// Restrict attribution to files that survived legacy exclusion processing.
	includedSet := make(map[string]bool, len(allFiles))
	for _, f := range allFiles {
		includedSet[f] = true
	}
	result := make(AttributionResult)
	for line, files := range astAttr {
		for _, f := range files {
			if includedSet[f] {
				result[line] = append(result[line], f)
			}
		}
	}
	_ = astExcl
	_ = astFilt
	// Restrict exclusions to files that were not finally included.
	exclusions := make(ExclusionResult)
	for line, files := range astExcl {
		for _, f := range files {
			if !includedSet[f] {
				exclusions[line] = append(exclusions[line], f)
			}
		}
	}
	// Restrict filtered (superseded inclusion) tracking to finally-included files.
	filtered := make(FilteredResult)
	for line, infos := range astFilt {
		for _, fi := range infos {
			if includedSet[fi.File] {
				filtered[line] = append(filtered[line], fi)
			}
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

	// For absolute path patterns without globs, check if this is a directory pattern
	// that should match files inside it. This handles the case where a rule like
	// "/path/to/dir" should match "/path/to/dir/file.txt"
	if filepath.IsAbs(normalizedPattern) && !strings.ContainsAny(normalizedPattern, "*?[") {
		// Check if the path is under this directory
		if strings.HasPrefix(normalizedPath, normalizedPattern+"/") {
			return true
		}
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
