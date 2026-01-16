package context

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/grovetools/core/config"
	"github.com/grovetools/core/logging"
	"github.com/grovetools/core/pkg/profiling"
	"github.com/grovetools/core/pkg/repo"
	"github.com/grovetools/core/util/pathutil"
	"github.com/sirupsen/logrus"
)

var log = logging.NewLogger("cx.context.resolve")

// patternInfo holds information about a pattern including any associated directive
type patternInfo struct {
	pattern   string
	directive string
	query     string
	isExclude bool
}

// patternMatcher holds pre-processed patterns and provides a method to classify files.
type patternMatcher struct {
	patternInfos               []patternInfo
	workDir                    string
	dirExclusions              map[string]bool
	includeBinary              bool
	hasExplicitWorktreePattern bool
}

// newPatternMatcher creates a new patternMatcher and pre-processes the patterns.
func newPatternMatcher(patternInfos []patternInfo, workDir string) *patternMatcher {
	matcher := &patternMatcher{
		patternInfos:               patternInfos,
		workDir:                    workDir,
		dirExclusions:              make(map[string]bool),
		includeBinary:              false,
		hasExplicitWorktreePattern: false,
	}

	for _, info := range patternInfos {
		pattern := info.pattern
		// Check for special pattern to include binary files
		if pattern == "!binary:exclude" || pattern == "binary:include" {
			matcher.includeBinary = true
			continue
		}

		// Check if any pattern explicitly includes .grove-worktrees
		if !info.isExclude && strings.Contains(pattern, ".grove-worktrees") {
			matcher.hasExplicitWorktreePattern = true
		}

		if info.isExclude {
			// Extract the last path component of the exclusion pattern.
			p := info.pattern
			// Treat `/**` and `/` as directory indicators before getting the base name.
			p = strings.TrimSuffix(p, "/**")
			p = strings.TrimSuffix(p, "/")

			// Get the basename of the remaining pattern.
			base := filepath.Base(p)

			// If the basename is a literal string (not a wildcard), add it to our fast-lookup map.
			// This correctly handles `!build/`, `!tests/**`, `!**/node_modules`, etc.
			if base != "." && base != "/" && !strings.ContainsAny(base, "*?") {
				matcher.dirExclusions[base] = true
			}
		}
	}

	return matcher
}

// classify determines if a file should be included based on the matcher's rules.
// It implements the "last matching pattern wins" logic.
func (pm *patternMatcher) classify(m *Manager, path, relPath string) bool {
	var lastValidMatch *patternInfo

	relToWorkDir, err := filepath.Rel(pm.workDir, path)
	isExternal := err != nil || strings.HasPrefix(relToWorkDir, "..")

	for i := range pm.patternInfos {
		info := pm.patternInfos[i] // Use index to correctly reference the item

		if info.pattern == "!binary:exclude" || info.pattern == "binary:include" {
			continue
		}

		// Floating inclusion patterns should not match external files.
		isFloatingInclusion := !info.isExclude && !strings.Contains(info.pattern, "/") && !filepath.IsAbs(info.pattern) && !strings.HasPrefix(info.pattern, "..")
		if isFloatingInclusion && isExternal {
			continue
		}
		cleanPattern := info.pattern

		match := false
		matchPath := relPath

		if filepath.IsAbs(cleanPattern) {
			matchPath = filepath.ToSlash(path)
		} else if strings.HasPrefix(cleanPattern, "../") {
			relFromWorkDir, err := filepath.Rel(pm.workDir, path)
			if err == nil {
				matchPath = filepath.ToSlash(relFromWorkDir)
			}
		}

		match = m.matchPattern(cleanPattern, matchPath)

		if match {
			isValidMatch := false
			if info.isExclude {
				isValidMatch = true
			} else { // It's an inclusion pattern
				if info.directive == "" {
					isValidMatch = true
				} else {
					filtered, err := m.applyDirectiveFilter([]string{path}, info.directive, info.query)
					if err == nil && len(filtered) > 0 {
						isValidMatch = true
					}
				}
			}

			if isValidMatch {
				lastValidMatch = &info
			}
		}
	}

	if lastValidMatch == nil {
		return false // No pattern ever matched
	}

	// Included if the last valid match was not an exclusion
	return !lastValidMatch.isExclude
}

// resolveFilesFromRulesContent resolves files based on rules content provided as a byte slice.
func (m *Manager) resolveFilesFromRulesContent(rulesContent []byte) ([]string, error) {
	// Parse the rules content directly without recursion for this case
	// This is used by commands that provide rules content directly (not from a file)
	parsed, err := m.parseRulesFileContent(rulesContent)
	if err != nil {
		return nil, fmt.Errorf("error parsing rules content: %w", err)
	}
	mainRules := parsed.hotRules
	coldRules := parsed.coldRules

	// Extract patterns from RuleInfo
	mainPatterns := make([]string, len(mainRules))
	for i, rule := range mainRules {
		pattern := rule.Pattern
		// Encode directive if present
		if rule.Directive != "" {
			pattern = pattern + "|||" + rule.Directive + "|||" + rule.DirectiveQuery
		}
		if rule.IsExclude {
			mainPatterns[i] = "!" + pattern
		} else {
			mainPatterns[i] = pattern
		}
	}

	coldPatterns := make([]string, len(coldRules))
	for i, rule := range coldRules {
		pattern := rule.Pattern
		// Encode directive if present
		if rule.Directive != "" {
			pattern = pattern + "|||" + rule.Directive + "|||" + rule.DirectiveQuery
		}
		if rule.IsExclude {
			coldPatterns[i] = "!" + pattern
		} else {
			coldPatterns[i] = pattern
		}
	}

	// Resolve files from main patterns
	mainFiles, err := m.resolveFilesFromPatterns(mainPatterns)
	if err != nil {
		return nil, fmt.Errorf("error resolving main context files: %w", err)
	}

	// Resolve files from cold patterns
	coldFiles, err := m.resolveFilesFromPatterns(coldPatterns)
	if err != nil {
		return nil, fmt.Errorf("error resolving cold context files: %w", err)
	}

	// Create a map of cold files for efficient exclusion
	coldFilesMap := make(map[string]bool)
	for _, file := range coldFiles {
		coldFilesMap[file] = true
	}

	// Filter main files to exclude any that are in cold files
	var finalMainFiles []string
	for _, file := range mainFiles {
		if !coldFilesMap[file] {
			finalMainFiles = append(finalMainFiles, file)
		}
	}

	return finalMainFiles, nil
}

// expandAllRules recursively resolves rules, including those from @default directives.
func (m *Manager) expandAllRules(rulesPath string, visited map[string]bool, importLineNum int) (hotRules, coldRules []RuleInfo, viewPaths []string, err error) {
	defer profiling.Start("context.expandAllRules").Stop()
	absRulesPath, err := filepath.Abs(rulesPath)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to get absolute path for rules: %w", err)
	}

	if visited[absRulesPath] {
		// Circular dependency detected, return to prevent infinite loop.
		return nil, nil, nil, nil
	}
	visited[absRulesPath] = true

	rulesContent, err := os.ReadFile(absRulesPath)
	if err != nil {
		if os.IsNotExist(err) {
			// If a default rules file doesn't exist, it's not an error, just return empty.
			return nil, nil, nil, nil
		}
		return nil, nil, nil, fmt.Errorf("reading rules file %s: %w", absRulesPath, err)
	}

	parsed, err := m.parseRulesFileContent(rulesContent)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("parsing rules file %s: %w", absRulesPath, err)
	}

	localHot := parsed.hotRules
	localCold := parsed.coldRules
	mainDefaults := parsed.mainDefaultPaths
	coldDefaults := parsed.coldDefaultPaths
	mainImports := parsed.mainImportedRuleSets
	coldImports := parsed.coldImportedRuleSets
	localView := parsed.viewPaths

	// Set EffectiveLineNum for local rules
	for i := range localHot {
		if importLineNum > 0 {
			localHot[i].EffectiveLineNum = importLineNum
		} else {
			localHot[i].EffectiveLineNum = localHot[i].LineNum
		}
	}
	for i := range localCold {
		if importLineNum > 0 {
			localCold[i].EffectiveLineNum = importLineNum
		} else {
			localCold[i].EffectiveLineNum = localCold[i].LineNum
		}
	}

	hotRules = append(hotRules, localHot...)
	coldRules = append(coldRules, localCold...)
	viewPaths = append(viewPaths, localView...)

	// Process concept directives
	for _, conceptID := range parsed.conceptIDs {
		resolvedFiles, err := m.resolveConcept(conceptID, visited)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: could not resolve concept '%s': %v\n", conceptID, err)
			continue
		}
		for _, file := range resolvedFiles {
			// Add each resolved file as a new rule to be processed
			hotRules = append(hotRules, RuleInfo{Pattern: file, IsExclude: false, LineNum: 0, EffectiveLineNum: 0})
		}
	}

	rulesDir := filepath.Dir(absRulesPath)

	// Process hot rule set imports
	for _, importInfo := range mainImports {
		// Handle Git ruleset imports
		if strings.HasPrefix(importInfo.ImportIdentifier, "git::") {
			// Format: git::repoURL@version::ruleset
			gitImportParts := strings.SplitN(strings.TrimPrefix(importInfo.ImportIdentifier, "git::"), "::", 2)
			if len(gitImportParts) != 2 {
				fmt.Fprintf(os.Stderr, "Warning: invalid git ruleset import format '%s'\n", importInfo.ImportIdentifier)
				continue
			}
			repoAndVersion, rulesetName := gitImportParts[0], gitImportParts[1]
			atIndex := strings.LastIndex(repoAndVersion, "@")
			repoURL := repoAndVersion
			version := ""
			if atIndex != -1 {
				repoURL = repoAndVersion[:atIndex]
				version = repoAndVersion[atIndex+1:]
			}

			// Instantiate repo manager
			repoManager, err := repo.NewManager()
			if err != nil {
				fmt.Fprintf(os.Stderr, "Warning: could not create repository manager for git import: %v\n", err)
				continue
			}

			localPath, _, err := repoManager.EnsureVersion(repoURL, version)
			if err != nil {
				m.addSkippedRule(importInfo.LineNum, importInfo.OriginalLine, fmt.Sprintf("invalid git ref: %v", err))
				continue
			}

			// Find the ruleset file within the cloned repository's .cx directories
			// Use localPath (the worktree) instead of barePath because the ruleset files
			// are in the checked-out working tree, not the bare repository
			rulesFilePath, err := FindRulesetFile(localPath, rulesetName)
			if err != nil {
				// Special case: if 'default' ruleset is requested but doesn't exist, treat it as "include all"
				if rulesetName == "default" {
					// Add a single "include all" rule for this repo
					hotRules = append(hotRules, RuleInfo{
						Pattern:          filepath.Join(localPath, "**"),
						IsExclude:        false,
						LineNum:          importInfo.LineNum,
						EffectiveLineNum: importInfo.LineNum,
					})
				} else {
					fmt.Fprintf(os.Stderr, "Warning: could not find named ruleset '%s' in repository %s: %v\n", rulesetName, repoURL, err)
				}
				continue
			}

			nestedHot, nestedCold, nestedView, err := m.expandAllRules(rulesFilePath, visited, importInfo.LineNum)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Warning: could not resolve ruleset '%s' from repository %s: %v\n", rulesetName, repoURL, err)
				continue
			}

			// Propagate directive from import to nested rules if they don't have one
			if importInfo.Directive != "" {
				for i := range nestedHot {
					if nestedHot[i].Directive == "" {
						nestedHot[i].Directive = importInfo.Directive
						nestedHot[i].DirectiveQuery = importInfo.DirectiveQuery
					}
				}
				for i := range nestedCold {
					if nestedCold[i].Directive == "" {
						nestedCold[i].Directive = importInfo.Directive
						nestedCold[i].DirectiveQuery = importInfo.DirectiveQuery
					}
				}
			}

			// Prefix patterns with the local repository path
			for i := range nestedHot {
				if !filepath.IsAbs(nestedHot[i].Pattern) {
					nestedHot[i].Pattern = filepath.Join(localPath, nestedHot[i].Pattern)
				}
			}
			for i := range nestedCold {
				if !filepath.IsAbs(nestedCold[i].Pattern) {
					nestedCold[i].Pattern = filepath.Join(localPath, nestedCold[i].Pattern)
				}
			}
			hotRules = append(hotRules, nestedHot...)
			coldRules = append(coldRules, nestedCold...) // Rules from git repo are flattened into hot/cold of importer

			for i, path := range nestedView {
				if !filepath.IsAbs(path) {
					nestedView[i] = filepath.Join(localPath, path)
				}
			}
			viewPaths = append(viewPaths, nestedView...)

			continue
		}

		parts := strings.SplitN(importInfo.ImportIdentifier, "::", 2)
		if len(parts) != 2 {
			fmt.Fprintf(os.Stderr, "Warning: invalid ruleset import format '%s'\n", importInfo.ImportIdentifier)
			continue
		}
		projectAlias, rulesetName := parts[0], parts[1]

		projectPath, resolveErr := m.getAliasResolver().Resolve(projectAlias)
		if resolveErr != nil {
			fmt.Fprintf(os.Stderr, "Warning: could not resolve project alias '%s' for rule import: %v\n", projectAlias, resolveErr)
			continue
		}

		// Validate that the resolved project path is allowed
		if allowed, reason := m.IsPathAllowed(projectPath); !allowed {
			fmt.Fprintf(os.Stderr, "Warning: skipping import from '%s': %s\n", projectAlias, reason)
			continue
		}

		// Find the ruleset file in both .cx/ and .cx.work/ directories
		rulesFilePath, err := FindRulesetFile(projectPath, rulesetName)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: could not find ruleset '%s' from project '%s': %v\n", rulesetName, projectAlias, err)
			continue
		}

		nestedHot, nestedCold, nestedView, err := m.expandAllRules(rulesFilePath, visited, importInfo.LineNum)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: could not resolve ruleset '%s' from project '%s': %v\n", rulesetName, projectAlias, err)
			continue
		}

		// Propagate directive from import to nested rules if they don't have one
		if importInfo.Directive != "" {
			for i := range nestedHot {
				if nestedHot[i].Directive == "" {
					nestedHot[i].Directive = importInfo.Directive
					nestedHot[i].DirectiveQuery = importInfo.DirectiveQuery
				}
			}
			for i := range nestedCold {
				if nestedCold[i].Directive == "" {
					nestedCold[i].Directive = importInfo.Directive
					nestedCold[i].DirectiveQuery = importInfo.DirectiveQuery
				}
			}
		}

		// The patterns from external project need to be prefixed with the project path
		// so they resolve files from that project, not the current one
		for i := range nestedHot {
			pattern := nestedHot[i].Pattern
			if !filepath.IsAbs(pattern) {
				if strings.Contains(pattern, "/") {
					// This is a path-like pattern (e.g., "src/**/*.go"), so join it directly.
					nestedHot[i].Pattern = filepath.Join(projectPath, pattern)
				} else {
					// This is a gitignore-style pattern (e.g., "*.go"), make it recursive within the project.
					nestedHot[i].Pattern = filepath.Join(projectPath, "**", pattern)
				}
			}
		}
		for i := range nestedCold {
			pattern := nestedCold[i].Pattern
			if !filepath.IsAbs(pattern) {
				if strings.Contains(pattern, "/") {
					// This is a path-like pattern (e.g., "src/**/*.go"), so join it directly.
					nestedCold[i].Pattern = filepath.Join(projectPath, pattern)
				} else {
					// This is a gitignore-style pattern (e.g., "*.go"), make it recursive within the project.
					nestedCold[i].Pattern = filepath.Join(projectPath, "**", pattern)
				}
			}
		}
		hotRules = append(hotRules, nestedHot...)
		coldRules = append(coldRules, nestedCold...)

		// Add view paths from nested rules, adjusting relative paths
		for i, path := range nestedView {
			if !filepath.IsAbs(path) {
				nestedView[i] = filepath.Join(projectPath, path)
			}
		}
		viewPaths = append(viewPaths, nestedView...)
	}

	// Process cold rule set imports
	for _, importInfo := range coldImports {
		// Handle Git ruleset imports
		if strings.HasPrefix(importInfo.ImportIdentifier, "git::") {
			// Format: git::repoURL@version::ruleset
			gitImportParts := strings.SplitN(strings.TrimPrefix(importInfo.ImportIdentifier, "git::"), "::", 2)
			if len(gitImportParts) != 2 {
				fmt.Fprintf(os.Stderr, "Warning: invalid git ruleset import format '%s'\n", importInfo.ImportIdentifier)
				continue
			}
			repoAndVersion, rulesetName := gitImportParts[0], gitImportParts[1]
			atIndex := strings.LastIndex(repoAndVersion, "@")
			repoURL := repoAndVersion
			version := ""
			if atIndex != -1 {
				repoURL = repoAndVersion[:atIndex]
				version = repoAndVersion[atIndex+1:]
			}

			repoManager, err := repo.NewManager()
			if err != nil {
				fmt.Fprintf(os.Stderr, "Warning: could not create repository manager for git import: %v\n", err)
				continue
			}

			localPath, _, err := repoManager.EnsureVersion(repoURL, version)
			if err != nil {
				m.addSkippedRule(importInfo.LineNum, importInfo.OriginalLine, fmt.Sprintf("invalid git ref: %v", err))
				continue
			}

			// Find the ruleset file within the cloned repository's .cx directories
			// Use localPath (the worktree) instead of barePath because the ruleset files
			// are in the checked-out working tree, not the bare repository
			rulesFilePath, err := FindRulesetFile(localPath, rulesetName)
			if err != nil {
				if rulesetName == "default" {
					coldRules = append(coldRules, RuleInfo{
						Pattern:          filepath.Join(localPath, "**"),
						IsExclude:        false,
						LineNum:          importInfo.LineNum,
						EffectiveLineNum: importInfo.LineNum,
					})
				} else {
					fmt.Fprintf(os.Stderr, "Warning: could not find named ruleset '%s' in repository %s: %v\n", rulesetName, repoURL, err)
				}
				continue
			}

			nestedHot, nestedCold, nestedView, err := m.expandAllRules(rulesFilePath, visited, importInfo.LineNum)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Warning: could not resolve ruleset '%s' from repository %s: %v\n", rulesetName, repoURL, err)
				continue
			}

			// For cold imports, everything from the imported ruleset goes into the cold section
			allNestedRules := append(nestedHot, nestedCold...)

			// Propagate directive from import to all nested rules
			if importInfo.Directive != "" {
				for i := range allNestedRules {
					if allNestedRules[i].Directive == "" {
						allNestedRules[i].Directive = importInfo.Directive
						allNestedRules[i].DirectiveQuery = importInfo.DirectiveQuery
					}
				}
			}

			for i := range allNestedRules {
				if !filepath.IsAbs(allNestedRules[i].Pattern) {
					allNestedRules[i].Pattern = filepath.Join(localPath, allNestedRules[i].Pattern)
				}
			}
			coldRules = append(coldRules, allNestedRules...)

			for i, path := range nestedView {
				if !filepath.IsAbs(path) {
					nestedView[i] = filepath.Join(localPath, path)
				}
			}
			viewPaths = append(viewPaths, nestedView...)
			continue
		}

		parts := strings.SplitN(importInfo.ImportIdentifier, "::", 2)
		if len(parts) != 2 {
			fmt.Fprintf(os.Stderr, "Warning: invalid ruleset import format '%s'\n", importInfo.ImportIdentifier)
			continue
		}
		projectAlias, rulesetName := parts[0], parts[1]

		projectPath, resolveErr := m.getAliasResolver().Resolve(projectAlias)
		if resolveErr != nil {
			fmt.Fprintf(os.Stderr, "Warning: could not resolve project alias '%s' for rule import: %v\n", projectAlias, resolveErr)
			continue
		}

		// Validate that the resolved project path is allowed
		if allowed, reason := m.IsPathAllowed(projectPath); !allowed {
			fmt.Fprintf(os.Stderr, "Warning: skipping import from '%s': %s\n", projectAlias, reason)
			continue
		}

		// Find the ruleset file in both .cx/ and .cx.work/ directories
		rulesFilePath, err := FindRulesetFile(projectPath, rulesetName)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: could not find ruleset '%s' from project '%s': %v\n", rulesetName, projectAlias, err)
			continue
		}

		nestedHot, nestedCold, nestedView, err := m.expandAllRules(rulesFilePath, visited, importInfo.LineNum)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: could not resolve ruleset '%s' from project '%s': %v\n", rulesetName, projectAlias, err)
			continue
		}

		allNestedRules := append(nestedHot, nestedCold...)

		// Propagate directive from import to all nested rules
		if importInfo.Directive != "" {
			for i := range allNestedRules {
				if allNestedRules[i].Directive == "" {
					allNestedRules[i].Directive = importInfo.Directive
					allNestedRules[i].DirectiveQuery = importInfo.DirectiveQuery
				}
			}
		}

		// The patterns from external project need to be prefixed with the project path
		for i := range allNestedRules {
			pattern := allNestedRules[i].Pattern
			if !filepath.IsAbs(pattern) {
				if strings.Contains(pattern, "/") {
					// This is a path-like pattern (e.g., "src/**/*.go"), so join it directly.
					allNestedRules[i].Pattern = filepath.Join(projectPath, pattern)
				} else {
					// This is a gitignore-style pattern (e.g., "*.go"), make it recursive within the project.
					allNestedRules[i].Pattern = filepath.Join(projectPath, "**", pattern)
				}
			}
		}

		// For cold imports, add everything to cold patterns
		coldRules = append(coldRules, allNestedRules...)

		// Add view paths from nested rules, adjusting relative paths
		for i, path := range nestedView {
			if !filepath.IsAbs(path) {
				nestedView[i] = filepath.Join(projectPath, path)
			}
		}
		viewPaths = append(viewPaths, nestedView...)
	}

	// Process hot defaults
	for _, defaultPath := range mainDefaults {
		resolvedPath := defaultPath
		if !filepath.IsAbs(resolvedPath) {
			resolvedPath = filepath.Join(rulesDir, resolvedPath)
		}

		// First resolve the real path and normalize for case-insensitive filesystems
		realPath, err := pathutil.NormalizeForLookup(resolvedPath)
		if err != nil {
			realPath = resolvedPath
		}

		// Validate that the default path is within an allowed workspace
		if allowed, reason := m.IsPathAllowed(realPath); !allowed {
			fmt.Fprintf(os.Stderr, "Warning: skipping @default for '%s': %s\n", defaultPath, reason)
			continue
		}

		// Load the config directly from the grove.yml file in that directory
		configFile := filepath.Join(realPath, "grove.yml")
		if _, err := os.Stat(configFile); os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "Warning: no grove.yml found at %s for @default path %s\n", configFile, defaultPath)
			continue
		}

		cfg, err := config.Load(configFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: could not load config for @default path %s (file: %s): %v\n", defaultPath, configFile, err)
			continue
		}

		var contextConfig struct {
			DefaultRulesPath string `yaml:"default_rules_path"`
		}
		if err := cfg.UnmarshalExtension("context", &contextConfig); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to unmarshal context extension for @default path %s: %v\n", defaultPath, err)
			continue
		}
		if contextConfig.DefaultRulesPath == "" {
			fmt.Fprintf(os.Stderr, "Warning: no default_rules_path found for @default path %s\n", defaultPath)
			continue
		}

		defaultRulesFile := filepath.Join(realPath, contextConfig.DefaultRulesPath)

		// Recursively resolve patterns from the default rules file
		// ALL patterns from the default (hot and cold) are added to the current HOT context.
		nestedHot, nestedCold, nestedView, err := m.expandAllRules(defaultRulesFile, visited, 0)
		if err != nil {
			return nil, nil, nil, err
		}
		// The patterns from external project need to be prefixed with the project path
		// so they resolve files from that project, not the current one
		for i := range nestedHot {
			pattern := nestedHot[i].Pattern
			if !filepath.IsAbs(pattern) {
				if strings.Contains(pattern, "/") {
					// This is a path-like pattern (e.g., "src/**/*.go"), so join it directly.
					nestedHot[i].Pattern = filepath.Join(realPath, pattern)
				} else {
					// This is a gitignore-style pattern (e.g., "*.go"), make it recursive within the project.
					nestedHot[i].Pattern = filepath.Join(realPath, "**", pattern)
				}
			}
		}
		for i := range nestedCold {
			pattern := nestedCold[i].Pattern
			if !filepath.IsAbs(pattern) {
				if strings.Contains(pattern, "/") {
					// This is a path-like pattern (e.g., "src/**/*.go"), so join it directly.
					nestedCold[i].Pattern = filepath.Join(realPath, pattern)
				} else {
					// This is a gitignore-style pattern (e.g., "*.go"), make it recursive within the project.
					nestedCold[i].Pattern = filepath.Join(realPath, "**", pattern)
				}
			}
		}
		hotRules = append(hotRules, nestedHot...)
		hotRules = append(hotRules, nestedCold...)

		// Add view paths from nested rules, adjusting relative paths
		for i, path := range nestedView {
			if !filepath.IsAbs(path) {
				nestedView[i] = filepath.Join(realPath, path)
			}
		}
		viewPaths = append(viewPaths, nestedView...)
	}

	// Process cold defaults
	for _, defaultPath := range coldDefaults {
		resolvedPath := defaultPath
		if !filepath.IsAbs(resolvedPath) {
			resolvedPath = filepath.Join(rulesDir, resolvedPath)
		}

		// First resolve the real path and normalize for case-insensitive filesystems
		realPath, err := pathutil.NormalizeForLookup(resolvedPath)
		if err != nil {
			realPath = resolvedPath
		}

		// Validate that the default path is within an allowed workspace
		if allowed, reason := m.IsPathAllowed(realPath); !allowed {
			fmt.Fprintf(os.Stderr, "Warning: skipping @default for '%s': %s\n", defaultPath, reason)
			continue
		}

		// Load the config directly from the grove.yml file in that directory
		configFile := filepath.Join(realPath, "grove.yml")
		if _, err := os.Stat(configFile); os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "Warning: no grove.yml found at %s for @default path %s\n", configFile, defaultPath)
			continue
		}

		cfg, err := config.Load(configFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: could not load config for @default path %s (file: %s): %v\n", defaultPath, configFile, err)
			continue
		}

		var contextConfig struct {
			DefaultRulesPath string `yaml:"default_rules_path"`
		}
		if err := cfg.UnmarshalExtension("context", &contextConfig); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to unmarshal context extension for @default path %s: %v\n", defaultPath, err)
			continue
		}
		if contextConfig.DefaultRulesPath == "" {
			fmt.Fprintf(os.Stderr, "Warning: no default_rules_path found for @default path %s\n", defaultPath)
			continue
		}

		defaultRulesFile := filepath.Join(realPath, contextConfig.DefaultRulesPath)

		// Recursively resolve patterns from the default rules file
		// ALL patterns from the default are added to the current COLD context.
		nestedHot, nestedCold, nestedView, err := m.expandAllRules(defaultRulesFile, visited, 0)
		if err != nil {
			return nil, nil, nil, err
		}
		// The patterns from external project need to be prefixed with the project path
		for i := range nestedHot {
			pattern := nestedHot[i].Pattern
			if !filepath.IsAbs(pattern) {
				if strings.Contains(pattern, "/") {
					// This is a path-like pattern (e.g., "src/**/*.go"), so join it directly.
					nestedHot[i].Pattern = filepath.Join(realPath, pattern)
				} else {
					// This is a gitignore-style pattern (e.g., "*.go"), make it recursive within the project.
					nestedHot[i].Pattern = filepath.Join(realPath, "**", pattern)
				}
			}
		}
		for i := range nestedCold {
			pattern := nestedCold[i].Pattern
			if !filepath.IsAbs(pattern) {
				if strings.Contains(pattern, "/") {
					// This is a path-like pattern (e.g., "src/**/*.go"), so join it directly.
					nestedCold[i].Pattern = filepath.Join(realPath, pattern)
				} else {
					// This is a gitignore-style pattern (e.g., "*.go"), make it recursive within the project.
					nestedCold[i].Pattern = filepath.Join(realPath, "**", pattern)
				}
			}
		}
		coldRules = append(coldRules, nestedHot...)
		coldRules = append(coldRules, nestedCold...)

		// Add view paths from nested rules, adjusting relative paths
		for i, path := range nestedView {
			if !filepath.IsAbs(path) {
				nestedView[i] = filepath.Join(realPath, path)
			}
		}
		viewPaths = append(viewPaths, nestedView...)
	}

	return hotRules, coldRules, viewPaths, nil
}

// ResolveFilesFromRules dynamically resolves the list of files from the active rules file
func (m *Manager) ResolveFilesFromRules() ([]string, error) {
	defer profiling.Start("context.ResolveFilesFromRules").Stop()
	// Load the active rules content (respects state-based rules)
	rulesContent, activeRulesFile, err := m.LoadRulesContent()
	if err != nil {
		return nil, fmt.Errorf("failed to load rules: %w", err)
	}
	if rulesContent == nil || activeRulesFile == "" {
		// No active or default rules found
		return []string{}, nil
	}

	// Resolve all patterns recursively from the active rules file
	hotRules, coldRules, _, err := m.expandAllRules(activeRulesFile, make(map[string]bool), 0)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve patterns: %w", err)
	}

	// Extract patterns from RuleInfo
	hotPatterns := make([]string, len(hotRules))
	for i, rule := range hotRules {
		pattern := rule.Pattern
		// Encode directive if present
		if rule.Directive != "" {
			pattern = pattern + "|||" + rule.Directive + "|||" + rule.DirectiveQuery
		}
		if rule.IsExclude {
			hotPatterns[i] = "!" + pattern
		} else {
			hotPatterns[i] = pattern
		}
	}

	coldPatterns := make([]string, len(coldRules))
	for i, rule := range coldRules {
		pattern := rule.Pattern
		// Encode directive if present
		if rule.Directive != "" {
			pattern = pattern + "|||" + rule.Directive + "|||" + rule.DirectiveQuery
		}
		if rule.IsExclude {
			coldPatterns[i] = "!" + pattern
		} else {
			coldPatterns[i] = pattern
		}
	}

	// Resolve files from hot patterns
	hotFiles, err := m.resolveFilesFromPatterns(hotPatterns)
	if err != nil {
		return nil, fmt.Errorf("error resolving hot context files: %w", err)
	}

	// Only resolve and filter cold patterns if there are any
	if len(coldPatterns) > 0 {
		// Resolve files from cold patterns
		coldFiles, err := m.resolveFilesFromPatterns(coldPatterns)
		if err != nil {
			return nil, fmt.Errorf("error resolving cold context files: %w", err)
		}

		// Create a map of cold files for efficient exclusion
		coldFilesMap := make(map[string]bool)
		for _, file := range coldFiles {
			coldFilesMap[file] = true
		}

		// Filter main files to exclude any that are also in cold files
		var finalHotFiles []string
		for _, file := range hotFiles {
			if !coldFilesMap[file] {
				finalHotFiles = append(finalHotFiles, file)
			}
		}

		return finalHotFiles, nil
	}

	// No cold patterns, return hot files as is
	return hotFiles, nil
}

// ResolveFilesFromCustomRulesFile resolves both hot and cold files from a custom rules file path.
func (m *Manager) ResolveFilesFromCustomRulesFile(rulesFilePath string) (hotFiles []string, coldFiles []string, err error) {
	// Get absolute path for the rules file
	absRulesFilePath, err := filepath.Abs(rulesFilePath)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get absolute path for rules file: %w", err)
	}

	// Check if the rules file exists
	if _, err := os.Stat(absRulesFilePath); os.IsNotExist(err) {
		return nil, nil, fmt.Errorf("rules file not found: %s", absRulesFilePath)
	}

	// Resolve all patterns recursively from the custom rules file
	hotRules, coldRules, _, err := m.expandAllRules(absRulesFilePath, make(map[string]bool), 0)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to resolve patterns from rules file: %w", err)
	}

	// Extract patterns from RuleInfo
	hotPatterns := make([]string, len(hotRules))
	for i, rule := range hotRules {
		pattern := rule.Pattern
		// Encode directive if present
		if rule.Directive != "" {
			pattern = pattern + "|||" + rule.Directive + "|||" + rule.DirectiveQuery
		}
		if rule.IsExclude {
			hotPatterns[i] = "!" + pattern
		} else {
			hotPatterns[i] = pattern
		}
	}

	coldPatterns := make([]string, len(coldRules))
	for i, rule := range coldRules {
		pattern := rule.Pattern
		// Encode directive if present
		if rule.Directive != "" {
			pattern = pattern + "|||" + rule.Directive + "|||" + rule.DirectiveQuery
		}
		if rule.IsExclude {
			coldPatterns[i] = "!" + pattern
		} else {
			coldPatterns[i] = pattern
		}
	}

	// Resolve files from hot patterns
	hotFiles, err = m.resolveFilesFromPatterns(hotPatterns)
	if err != nil {
		return nil, nil, fmt.Errorf("error resolving hot context files: %w", err)
	}

	// Resolve files from cold patterns
	if len(coldPatterns) > 0 {
		coldFiles, err = m.resolveFilesFromPatterns(coldPatterns)
		if err != nil {
			return nil, nil, fmt.Errorf("error resolving cold context files: %w", err)
		}

		// Apply cold-over-hot precedence: remove hot files that are also in cold
		coldFilesMap := make(map[string]bool)
		for _, file := range coldFiles {
			coldFilesMap[file] = true
		}

		var finalHotFiles []string
		for _, file := range hotFiles {
			if !coldFilesMap[file] {
				finalHotFiles = append(finalHotFiles, file)
			}
		}
		hotFiles = finalHotFiles
	}

	return hotFiles, coldFiles, nil
}

// ResolveColdContextFiles resolves the list of files from the "cold" section of a rules file.
func (m *Manager) ResolveColdContextFiles() ([]string, error) {
	defer profiling.Start("context.ResolveColdContextFiles").Stop()
	// Load the active rules content (respects state-based rules)
	rulesContent, activeRulesFile, err := m.LoadRulesContent()
	if err != nil {
		return nil, fmt.Errorf("failed to load rules: %w", err)
	}
	if rulesContent == nil || activeRulesFile == "" {
		// No active or default rules found
		return []string{}, nil
	}


	// Resolve all patterns recursively from the active rules file
	_, coldRules, _, err := m.expandAllRules(activeRulesFile, make(map[string]bool), 0)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve patterns for cold context: %w", err)
	}

	// Extract patterns from RuleInfo
	coldPatterns := make([]string, len(coldRules))
	for i, rule := range coldRules {
		pattern := rule.Pattern
		// Encode directive if present
		if rule.Directive != "" {
			pattern = pattern + "|||" + rule.Directive + "|||" + rule.DirectiveQuery
		}
		if rule.IsExclude {
			coldPatterns[i] = "!" + pattern
		} else {
			coldPatterns[i] = pattern
		}
	}

	// Resolve files from only the cold patterns
	coldFiles, err := m.resolveFilesFromPatterns(coldPatterns)
	if err != nil {
		return nil, fmt.Errorf("error resolving cold context files: %w", err)
	}

	// Sort for consistent output
	sort.Strings(coldFiles)
	return coldFiles, nil
}


// preProcessPatterns transforms plain directory patterns into recursive globs.
func (m *Manager) preProcessPatterns(patterns []string) []string {
	// Pre-process patterns to transform directory patterns into recursive globs
	processedPatterns := make([]string, 0, len(patterns))
	for _, pattern := range patterns {
		isExclude := strings.HasPrefix(pattern, "!")
		cleanPattern := pattern
		if isExclude {
			cleanPattern = strings.TrimPrefix(pattern, "!")
		}

		// Check if pattern contains glob characters
		hasGlob := strings.Contains(cleanPattern, "*") || strings.Contains(cleanPattern, "?")

		// Only transform plain directory patterns for INCLUSION patterns
		// Exclusion patterns like !tests should remain as-is for gitignore compatibility
		if !hasGlob && !isExclude {
			// Resolve the path to check if it exists and is a directory
			checkPath := cleanPattern
			if !filepath.IsAbs(cleanPattern) {
				checkPath = filepath.Join(m.workDir, cleanPattern)
			}
			checkPath = filepath.Clean(checkPath)

			if info, err := os.Stat(checkPath); err == nil && info.IsDir() {
				// Transform directory pattern to recursive glob
				processedPatterns = append(processedPatterns, cleanPattern+"/**")
				continue
			}
		}

		// Keep pattern as-is
		processedPatterns = append(processedPatterns, pattern)
	}
	return processedPatterns
}

// decodeDirective extracts directive information from an encoded pattern
// Returns: cleanPattern, directive, query, hasDirective
func decodeDirective(pattern string) (string, string, string, bool) {
	parts := strings.Split(pattern, "|||")
	if len(parts) == 3 {
		return parts[0], parts[1], parts[2], true
	}
	return pattern, "", "", false
}

// applyDirectiveFilter filters a list of files based on a directive (@find or @grep)
func (m *Manager) applyDirectiveFilter(files []string, directive, query string) ([]string, error) {
	var filtered []string

	if directive == "find" {
		// @find: filter by filename/path
		for _, file := range files {
			if strings.Contains(file, query) {
				filtered = append(filtered, file)
			}
		}
	} else if directive == "grep" {
		// @grep: filter by file content (parallelized for performance)
		type result struct {
			file string
			err  error
		}

		resultChan := make(chan result, len(files))
		semaphore := make(chan struct{}, 10) // Limit to 10 concurrent goroutines

		for _, file := range files {
			file := file // Capture loop variable

			go func() {
				semaphore <- struct{}{} // Acquire semaphore
				defer func() { <-semaphore }() // Release semaphore

				// Construct absolute path for reading
				filePath := file
				if !filepath.IsAbs(file) {
					filePath = filepath.Join(m.workDir, file)
				}

				// Read file content
				content, err := os.ReadFile(filePath)
				if err != nil {
					// Skip files that can't be read
					resultChan <- result{file: "", err: nil}
					return
				}

				// Check if content contains the query
				if strings.Contains(string(content), query) {
					resultChan <- result{file: file, err: nil}
				} else {
					resultChan <- result{file: "", err: nil}
				}
			}()
		}

		// Collect results
		for i := 0; i < len(files); i++ {
			res := <-resultChan
			if res.file != "" {
				filtered = append(filtered, res.file)
			}
		}
		close(resultChan)
	}

	return filtered, nil
}

// resolveFilesFromPatterns resolves files from a given set of patterns
func (m *Manager) resolveFilesFromPatterns(patterns []string) ([]string, error) {
	defer profiling.Start("context.resolveFilesFromPatterns").Stop()
	m.log.WithFields(logrus.Fields{
		"pattern_count": len(patterns),
	}).Debug("Resolving files from patterns")

	if len(patterns) == 0 {
		return []string{}, nil
	}

	// Step 1: Apply brace expansion to all incoming patterns
	var expandedPatterns []string
	for _, p := range patterns {
		expandedPatterns = append(expandedPatterns, ExpandBraces(p)...)
	}
	patterns = expandedPatterns

	// Step 2: Parse directives BEFORE preprocessing (so we work with clean base patterns)
	// Build a temporary patternInfos to extract base patterns
	type tempPatternInfo struct {
		basePattern string
		directive   string
		query       string
		isExclude   bool
	}

	tempInfos := make([]tempPatternInfo, 0, len(patterns))
	cleanPatternsForPreprocess := make([]string, 0, len(patterns))

	for _, pattern := range patterns {
		isExclude := strings.HasPrefix(pattern, "!")
		cleanPattern := pattern
		if isExclude {
			cleanPattern = strings.TrimPrefix(pattern, "!")
		}

		// Try to parse plain text directive first (e.g., "pattern @find: \"query\"")
		basePattern, directive, query, hasDirective := parseSearchDirective(cleanPattern)

		// If no plain text directive, try encoded format (e.g., "pattern|||find|||query")
		if !hasDirective {
			basePattern, directive, query, _ = decodeDirective(cleanPattern)
		}

		tempInfos = append(tempInfos, tempPatternInfo{
			basePattern: basePattern,
			directive:   directive,
			query:       query,
			isExclude:   isExclude,
		})

		// Reconstruct pattern without directive for preprocessing
		if isExclude {
			cleanPatternsForPreprocess = append(cleanPatternsForPreprocess, "!"+basePattern)
		} else {
			cleanPatternsForPreprocess = append(cleanPatternsForPreprocess, basePattern)
		}
	}

	// Step 3: Apply directory-to-glob transformation on clean base patterns
	processedPatterns := m.preProcessPatterns(cleanPatternsForPreprocess)

	// Step 4: Now build final patternInfos from processed patterns, preserving directive info
	var patternInfos []patternInfo

	for i, processedPattern := range processedPatterns {
		isExclude := strings.HasPrefix(processedPattern, "!")
		cleanProcessedPattern := processedPattern
		if isExclude {
			cleanProcessedPattern = strings.TrimPrefix(processedPattern, "!")
		}

		patternInfos = append(patternInfos, patternInfo{
			pattern:   cleanProcessedPattern,
			directive: tempInfos[i].directive,
			query:     tempInfos[i].query,
			isExclude: isExclude,
		})
	}

	// Step 5: Validate absolute/relative-up patterns before processing
	// IMPORTANT: Iterate over patternInfos directly, not a separate patterns array
	validatedPatternInfos := make([]patternInfo, 0, len(patternInfos))
	for _, info := range patternInfos {
		cleanPattern := info.pattern

		if filepath.IsAbs(cleanPattern) || strings.HasPrefix(cleanPattern, "../") {
			// Resolve path for validation. For globs, validate the base path.
			pathToValidate := cleanPattern
			if strings.ContainsAny(pathToValidate, "*?[") {
				// Find base path before glob
				// e.g., /foo/bar/**/*.go -> /foo/bar
				// e.g., ../foo/**/*.go -> ../foo
				parts := strings.Split(filepath.ToSlash(pathToValidate), "/")
				baseParts := []string{}
				for _, part := range parts {
					if strings.ContainsAny(part, "*?[") {
						break
					}
					baseParts = append(baseParts, part)
				}
				pathToValidate = strings.Join(baseParts, "/")
			}

			// Resolve relative paths
			if !filepath.IsAbs(pathToValidate) {
				pathToValidate = filepath.Join(m.workDir, pathToValidate)
			}

			if allowed, reason := m.IsPathAllowed(pathToValidate); !allowed {
				// Reconstruct pattern string for error message
				pattern := info.pattern
				if info.isExclude {
					pattern = "!" + pattern
				}
				fmt.Fprintf(os.Stderr, "Warning: skipping rule '%s': %s\n", pattern, reason)
				// Track this skipped rule (line number will be resolved later in stats)
				m.addSkippedRule(0, pattern, reason)
				continue // Skip this pattern
			}
		}
		validatedPatternInfos = append(validatedPatternInfos, info)
	}
	patternInfos = validatedPatternInfos

	// Get gitignored files for the current working directory for handling relative patterns.
	gitIgnoredForCWD, err := m.getGitIgnoredFiles(m.workDir)
	if err != nil {
		fmt.Printf("Warning: could not get gitignored files for current directory: %v\n", err)
		gitIgnoredForCWD = make(map[string]bool)
	}

	// This map will store the final list of files to include.
	uniqueFiles := make(map[string]bool)

	// Separate patterns into relative and absolute paths, preserving patternInfo
	var relativePatternInfos []patternInfo
	absolutePathInfos := make(map[string][]patternInfo) // map[absolutePath]patternInfos
	var deferredExclusionInfos []patternInfo            // Store exclusion patterns to process after inclusions
	var floatingExclusionInfos []patternInfo            // Store floating exclusions (no path separator, apply globally)

	// First pass: process inclusion and exclusion patterns
	for _, info := range patternInfos {
		// Check if this is an exact file path that exists
		// IMPORTANT: Only treat as an exact file for INCLUSION patterns.
		// Exclusion patterns should always be treated as patterns, not exact files,
		// to support gitignore-style semantics (e.g., !main.go should exclude all main.go files).
		if !info.isExclude {
			filePath := info.pattern
			if !filepath.IsAbs(filePath) {
				filePath = filepath.Join(m.workDir, filePath)
			}
			filePath = filepath.Clean(filePath)

			if fstat, err := os.Stat(filePath); err == nil && !fstat.IsDir() {
				if filepath.IsAbs(info.pattern) || strings.HasPrefix(info.pattern, "../") {
					uniqueFiles[filePath] = true
				} else {
					relPath, err := filepath.Rel(m.workDir, filePath)
					if err == nil {
						uniqueFiles[relPath] = true
					} else {
						uniqueFiles[filePath] = true
					}
				}
				continue
			}
		}

		// If not an exact file, treat as a pattern
		if info.isExclude {
			// Identify "floating" exclusions (gitignore-style patterns without path separators)
			// These should apply to ALL walks (local and external), not just the current directory
			// Also treat **/ patterns as floating since they match any directory recursively
			isFloatingExclusion := (!strings.Contains(info.pattern, "/") || strings.HasPrefix(info.pattern, "**/")) && !filepath.IsAbs(info.pattern)

			if isFloatingExclusion {
				// Floating exclusions like "!tests" or "!*.tmp" or "!**/*.md" should apply globally
				floatingExclusionInfos = append(floatingExclusionInfos, info)
			} else if filepath.IsAbs(info.pattern) || strings.HasPrefix(info.pattern, "../") {
				deferredExclusionInfos = append(deferredExclusionInfos, info)
			} else {
				// Path-specific exclusions for relative patterns
				relativePatternInfos = append(relativePatternInfos, info)
			}
			continue
		}

		if filepath.IsAbs(info.pattern) || strings.HasPrefix(info.pattern, "../") {
			basePath := info.pattern
			if !filepath.IsAbs(info.pattern) {
				basePath = filepath.Join(m.workDir, info.pattern)
			}
			basePath = filepath.Clean(basePath)

			if strings.ContainsAny(basePath, "*?[") {
				basePath = filepath.Dir(basePath)
				for strings.ContainsAny(basePath, "*?[") {
					basePath = filepath.Dir(basePath)
				}
			}

			absolutePathInfos[basePath] = append(absolutePathInfos[basePath], info)
		} else {
			relativePatternInfos = append(relativePatternInfos, info)
		}
	}

	// Second pass: add floating exclusions to ALL pattern groups (they apply globally)
	relativePatternInfos = append(relativePatternInfos, floatingExclusionInfos...)
	for basePath := range absolutePathInfos {
		absolutePathInfos[basePath] = append(absolutePathInfos[basePath], floatingExclusionInfos...)
	}

	// Add deferred (path-specific absolute/../) exclusions to relative patterns and all absolute path groups
	relativePatternInfos = append(relativePatternInfos, deferredExclusionInfos...)
	for basePath := range absolutePathInfos {
		absolutePathInfos[basePath] = append(absolutePathInfos[basePath], deferredExclusionInfos...)
	}

	// Process relative patterns
	if len(relativePatternInfos) > 0 {
		relativeMatcher := newPatternMatcher(relativePatternInfos, m.workDir)
		err = m.walkAndMatchPatterns(m.workDir, relativeMatcher, gitIgnoredForCWD, uniqueFiles, true)
		if err != nil {
			return nil, fmt.Errorf("error walking working directory: %w", err)
		}
	}

	// Process each absolute path
	for absPath, pathPatternInfos := range absolutePathInfos {
		if _, err := os.Stat(absPath); os.IsNotExist(err) {
			continue
		}

		gitIgnoredForAbsPath, err := m.getGitIgnoredFiles(absPath)
		if err != nil {
			fmt.Printf("Warning: could not get gitignored files for %s: %v\n", absPath, err)
			gitIgnoredForAbsPath = make(map[string]bool)
		}

		absoluteMatcher := newPatternMatcher(pathPatternInfos, m.workDir)
		err = m.walkAndMatchPatterns(absPath, absoluteMatcher, gitIgnoredForAbsPath, uniqueFiles, false)
		if err != nil {
			fmt.Printf("Warning: error walking absolute path %s: %v\n", absPath, err)
		}
	}

	// Convert map to a sorted slice for consistent output.
	filesToInclude := make([]string, 0, len(uniqueFiles))
	for file := range uniqueFiles {
		filesToInclude = append(filesToInclude, file)
	}
	sort.Strings(filesToInclude)

	m.log.WithFields(logrus.Fields{
		"pattern_count": len(patterns),
		"file_count":    len(filesToInclude),
	}).Debug("Resolved files from patterns")

	// Return the resolved file list
	return filesToInclude, nil
}


// walkAndMatchPatterns walks a directory and matches files against patterns
func (m *Manager) walkAndMatchPatterns(rootPath string, matcher *patternMatcher, gitIgnoredFiles map[string]bool, uniqueFiles map[string]bool, useRelativePaths bool) error {
	defer profiling.Start("context.walkAndMatchPatterns").Stop()
	// Pre-processing is now done in newPatternMatcher. We access the results from the matcher.
	dirExclusions := matcher.dirExclusions
	includeBinary := matcher.includeBinary
	hasExplicitWorktreePattern := matcher.hasExplicitWorktreePattern

	// Walk the directory tree from the specified root path.
	return filepath.WalkDir(rootPath, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// First, check if the file or directory is ignored by git. This is the most efficient check.
		// The `path` from WalkDir is absolute if the root is absolute, which it always will be.
		// Normalize for case-insensitive filesystems and symlink resolution
		normalizedPath, err := pathutil.NormalizeForLookup(path)
		if err != nil {
			normalizedPath = path
		}

		if gitIgnoredFiles[normalizedPath] {
			// If we have explicit worktree patterns and this path is under .grove-worktrees,
			// don't skip it even if it's git-ignored (user explicitly wants these files)
			if hasExplicitWorktreePattern && strings.Contains(path, string(filepath.Separator)+".grove-worktrees"+string(filepath.Separator)) {
				// Skip the git-ignore check for files under .grove-worktrees when explicitly included
			} else {
				if d.IsDir() {
					return filepath.SkipDir // Prune the walk for ignored directories.
				}
				return nil // Skip ignored files.
			}
		}

		// Always prune .git and .grove directories from the walk.
		// Only prune .grove-worktrees if no pattern explicitly includes it AND we're not already inside one
		if d.IsDir() {
			if d.Name() == ".git" || d.Name() == ".grove" {
				return filepath.SkipDir
			}
			// Skip .grove-worktrees directories UNLESS:
			// 1. We have an explicit pattern that includes .grove-worktrees, OR
			// 2. We're already walking inside a .grove-worktrees directory (rootPath contains it)
			if d.Name() == ".grove-worktrees" {
				if !hasExplicitWorktreePattern &&
					!strings.Contains(rootPath, string(filepath.Separator)+".grove-worktrees"+string(filepath.Separator)) {
					return filepath.SkipDir
				}
			}

			// Fast path: if the directory's name isn't a candidate for exclusion, continue walking.
			if !dirExclusions[d.Name()] {
				return nil
			}

			// Slow path: This directory *might* be excluded. Run the full "last match wins" logic to be sure.
			relPathForDir, err := filepath.Rel(m.workDir, path)
			if err != nil {
				relPathForDir = path // Fallback
			}
			relPathForDir = filepath.ToSlash(relPathForDir)

			var lastMatchWasExclusion *bool
			// Check against ALL patterns to respect "last match wins" for this directory path.
			for _, info := range matcher.patternInfos {
				pattern := info.pattern

				// A pattern matches a directory if it matches the name or the name with a trailing slash.
				if m.matchPattern(pattern, relPathForDir) || m.matchPattern(pattern, relPathForDir+"/") {
					isExclude := info.isExclude
					lastMatchWasExclusion = &isExclude
				}
			}

			// If the final matching pattern was an exclusion, prune the directory.
			if lastMatchWasExclusion != nil && *lastMatchWasExclusion {
				return filepath.SkipDir
			}

			return nil // Directory not excluded, continue walking.
		}

		// Get path relative to the walk root for pattern matching.
		relPath, err := filepath.Rel(rootPath, path)
		if err != nil {
			return err
		}
		// Always use forward slashes for cross-platform pattern matching consistency.
		relPath = filepath.ToSlash(relPath)

		// Skip the root directory itself.
		if relPath == "." {
			return nil
		}

		// Skip binary files unless explicitly included
		if !includeBinary && isBinaryFile(path) {
			return nil
		}

		// Use the pattern matcher to classify this file
		isIncluded := matcher.classify(m, path, relPath)

		if isIncluded {
			// Special handling for .grove-worktrees: by default, we exclude files inside these directories
			// because they often contain temporary or project-specific artifacts.
			// This exclusion is bypassed if any inclusion rule explicitly contains ".grove-worktrees",
			// indicating the user intentionally wants to include content from them.
			if strings.Contains(path, string(filepath.Separator)+".grove-worktrees"+string(filepath.Separator)) {
				isExplicitlyIncludedByRule := false
				for _, info := range matcher.patternInfos {
					if !info.isExclude && strings.Contains(info.pattern, ".grove-worktrees") {
						isExplicitlyIncludedByRule = true
						break
					}
				}
				// Also check if we're walking from a root that contains .grove-worktrees
				if !isExplicitlyIncludedByRule && strings.Contains(rootPath, string(filepath.Separator)+".grove-worktrees"+string(filepath.Separator)) {
					isExplicitlyIncludedByRule = true
				}
				if !isExplicitlyIncludedByRule {
					// Only exclude if .grove-worktrees is a descendant of the working directory
					relPath, err := filepath.Rel(m.workDir, path)
					if err == nil && strings.Contains(relPath, ".grove-worktrees") {
						// The .grove-worktrees is within our working directory, exclude it
						return nil
					}
				}
				// If explicitly included, don't exclude it
			}

			// Determine the final path to store
			var finalPath string
			if useRelativePaths {
				// For relative patterns, store path relative to workDir
				finalPath, _ = filepath.Rel(m.workDir, path)
			} else {
				// For absolute patterns, check if the path is within workDir
				if relPath, err := filepath.Rel(m.workDir, path); err == nil && !strings.HasPrefix(relPath, "..") {
					// Path is within workDir, use relative path for consistency
					finalPath = relPath
				} else {
					// Path is outside workDir, use absolute path
					finalPath = path
				}
			}

			uniqueFiles[finalPath] = true
		}

		return nil
	})
}

// matchDoubleStarPattern handles patterns with ** for recursive matching
func matchDoubleStarPattern(pattern, path string) bool {
	// Special case: pattern like "**/something/**" means "something" appears anywhere in path
	if strings.HasPrefix(pattern, "**/") && strings.HasSuffix(pattern, "/**") {
		middle := pattern[3 : len(pattern)-3]
		// Check if middle appears as a complete path component
		pathParts := strings.Split(path, "/")
		for _, part := range pathParts {
			if matched, _ := filepath.Match(middle, part); matched {
				return true
			}
		}
		return false
	}

	// Split pattern at **
	parts := strings.Split(pattern, "**")

	if len(parts) == 2 {
		prefix := strings.TrimSuffix(parts[0], "/")
		suffix := strings.TrimPrefix(parts[1], "/")

		// Check prefix match
		if prefix != "" {
			if !strings.HasPrefix(path, prefix) {
				return false
			}
			// Ensure it's a directory boundary match.
			// The path must either be identical to the prefix or have a '/' after it.
			if len(path) > len(prefix) && path[len(prefix)] != '/' {
				return false
			}
		}

		// Remove the prefix from the path for suffix matching
		pathAfterPrefix := path
		if prefix != "" {
			pathAfterPrefix = strings.TrimPrefix(path, prefix)
			pathAfterPrefix = strings.TrimPrefix(pathAfterPrefix, "/")
		}

		// Check suffix match
		if suffix != "" {
			// For patterns like "**/*.go", we need to check if the suffix matches
			// any part of the remaining path, not just the filename
			if !strings.Contains(suffix, "/") {
				// Simple suffix like "*.go" - check if the filename matches
				matched, _ := filepath.Match(suffix, filepath.Base(pathAfterPrefix))
				return matched
			} else {
				// Complex suffix with directory components
				// For example, "foo/*.go" should match "bar/baz/foo/test.go"
				// The ** means we need to try matching the suffix at all possible positions

				suffixParts := strings.Split(suffix, "/")
				pathParts := strings.Split(pathAfterPrefix, "/")

				// Try to match suffix against all possible positions in the path
				for i := 0; i <= len(pathParts)-len(suffixParts); i++ {
					match := true
					for j := 0; j < len(suffixParts); j++ {
						if matched, _ := filepath.Match(suffixParts[j], pathParts[i+j]); !matched {
							match = false
							break
						}
					}
					if match {
						return true
					}
				}
				return false
			}
		}

		// If only prefix is specified (or no suffix), it matches
		return true
	}

	// Handle multiple ** in pattern or patterns without **
	matched, _ := filepath.Match(pattern, path)
	return matched
}

// BinaryExtensions contains a map of common binary file extensions for fast checking.
var BinaryExtensions = map[string]bool{
	".exe": true, ".dll": true, ".so": true, ".dylib": true, ".a": true,
	".o": true, ".obj": true, ".lib": true, ".bin": true, ".dat": true,
	".db": true, ".sqlite": true, ".sqlite3": true,
	".jpg": true, ".jpeg": true, ".png": true, ".gif": true, ".bmp": true,
	".ico": true, ".tiff": true, ".webp": true,
	".mp3": true, ".mp4": true, ".avi": true, ".mov": true, ".wmv": true,
	".flv": true, ".webm": true, ".m4a": true, ".flac": true, ".wav": true,
	".zip": true, ".tar": true, ".gz": true, ".bz2": true, ".xz": true,
	".7z": true, ".rar": true, ".deb": true, ".rpm": true,
	".dmg": true, ".pkg": true, ".msi": true,
	".pdf": true, ".doc": true, ".docx": true, ".xls": true, ".xlsx": true,
	".ppt": true, ".pptx": true, ".odt": true, ".ods": true, ".odp": true,
	".pyc": true, ".pyo": true, ".class": true, ".jar": true, ".war": true,
	".woff": true, ".woff2": true, ".ttf": true, ".otf": true, ".eot": true,
	".wasm": true, ".node": true,
}

// isBinaryFile detects if a file is likely binary by checking the first 512 bytes
func isBinaryFile(path string) bool {
	// Check common binary file extensions first for performance
	ext := strings.ToLower(filepath.Ext(path))

	// If it's a known binary extension, return true immediately
	if BinaryExtensions[ext] {
		return true
	}

	// If file has an extension, assume it's not binary (unless in binaryExtensions)
	// This avoids checking content for most source code files
	if ext != "" {
		return false
	}

	// Check for common text files without extensions
	basename := filepath.Base(path)
	commonTextFiles := map[string]bool{
		"Makefile": true, "makefile": true, "GNUmakefile": true,
		"Dockerfile": true, "dockerfile": true,
		"README": true, "LICENSE": true, "CHANGELOG": true,
		"AUTHORS": true, "CONTRIBUTORS": true, "PATENTS": true,
		"VERSION": true, "TODO": true, "NOTICE": true,
		"Jenkinsfile": true, "Rakefile": true, "Gemfile": true,
		"Vagrantfile": true, "Brewfile": true, "Podfile": true,
		"gradlew": true, "mvnw": true,
	}

	if commonTextFiles[basename] {
		return false
	}

	// Only check content for files without extensions (like Go binaries)
	file, err := os.Open(path)
	if err != nil {
		return false
	}
	defer file.Close()

	// Read first 512 bytes to check for binary content
	buffer := make([]byte, 512)
	n, err := file.Read(buffer)
	if err != nil && err != io.EOF {
		return false
	}

	// Check for common binary file signatures
	if n >= 4 {
		// ELF header (Linux/Unix executables including Go binaries)
		if buffer[0] == 0x7f && buffer[1] == 'E' && buffer[2] == 'L' && buffer[3] == 'F' {
			return true
		}
		// Mach-O header (macOS executables including Go binaries)
		if (buffer[0] == 0xfe && buffer[1] == 0xed && buffer[2] == 0xfa && (buffer[3] == 0xce || buffer[3] == 0xcf)) ||
			(buffer[0] == 0xce && buffer[1] == 0xfa && buffer[2] == 0xed && buffer[3] == 0xfe) ||
			(buffer[0] == 0xcf && buffer[1] == 0xfa && buffer[2] == 0xed && buffer[3] == 0xfe) {
			return true
		}
		// PE header (Windows executables)
		if buffer[0] == 'M' && buffer[1] == 'Z' {
			return true
		}
	}

	// Quick check for null bytes (strong indicator of binary)
	for i := 0; i < n; i++ {
		if buffer[i] == 0 {
			return true
		}
	}

	// Count non-text characters
	nonTextCount := 0
	for i := 0; i < n; i++ {
		b := buffer[i]
		// Count non-printable characters (excluding common whitespace)
		if b < 32 && b != '\t' && b != '\n' && b != '\r' {
			nonTextCount++
		}
	}

	// If more than 30% of characters are non-text, consider it binary
	if n > 0 && float64(nonTextCount)/float64(n) > 0.3 {
		return true
	}

	return false
}
