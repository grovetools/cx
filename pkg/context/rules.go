package context

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/grovetools/core/config"
	"github.com/grovetools/core/logging"
	"github.com/grovetools/core/pkg/repo"
	"github.com/grovetools/core/pkg/workspace"
	"github.com/grovetools/core/state"
)

var rulesLog = logging.NewLogger("cx.context.rules")

// parsedRules holds the fully parsed contents of a single rules file,
// including all rules, directives, and import statements.
type parsedRules struct {
	hotRules             []RuleInfo
	coldRules            []RuleInfo
	mainDefaultPaths     []string
	coldDefaultPaths     []string
	mainImportedRuleSets []ImportInfo
	coldImportedRuleSets []ImportInfo
	mainIncludes         []ImportInfo
	coldIncludes         []ImportInfo
	viewPaths            []string
	treePaths            []string
	conceptIDs           []string // Concept IDs to resolve via @concept: directive
	freezeCache          bool
	disableExpiration    bool
	disableCache         bool
	expireTime           time.Duration
}

// RuleStatus represents the current state of a rule
type RuleStatus int

// GitRule represents a parsed Git alias rule from the rules file.
type GitRule struct {
	RepoURL     string
	Version     string
	Ruleset     string
	ContextType RuleStatus
	IsExclude   bool
}

const (
	RuleNotFound RuleStatus = iota // Rule doesn't exist
	RuleHot                        // Rule exists in hot context
	RuleCold                       // Rule exists in cold context
	RuleExcluded                   // Rule exists as exclusion

	// StateSourceKey is the key used in grove-core state to store the path to the active rule set.
	StateSourceKey = "context.active_rules_source"
)

// SkippedRule represents a rule that was skipped during parsing along with the reason why
type SkippedRule struct {
	LineNum int
	Rule    string
	Reason  string
}

// LoadDefaultRulesContent loads only the default rules from grove.yml, ignoring any local rules files.
// It returns the default rules content and the path where rules should be written.
func (m *Manager) LoadDefaultRulesContent() (content []byte, rulesPath string) {
	rulesPath = m.ResolveRulesWritePath()

	// Load grove.yml to check for default rules
	cfg, err := config.LoadFrom(m.workDir)
	if err != nil || cfg == nil {
		// No config, so no default rules
		return nil, rulesPath
	}

	if cfg.Context == nil {
		return nil, rulesPath
	}

	// Project root is where the config file is found
	configPath, _ := config.FindConfigFile(m.workDir)
	projectRoot := filepath.Dir(configPath)
	if projectRoot == "" {
		projectRoot = m.workDir
	}

	// Preferred: default_rules takes just a preset name (e.g., "dev-no-tests")
	if cfg.Context.DefaultRules != "" {
		if resolvedPath, findErr := m.FindRulesetFile(projectRoot, cfg.Context.DefaultRules); findErr == nil {
			content, err := os.ReadFile(resolvedPath)
			if err == nil {
				return content, rulesPath
			}
		}
		fmt.Fprintf(os.Stderr, "Warning: could not find default_rules preset '%s'\n", cfg.Context.DefaultRules)
		return nil, rulesPath
	}

	// Legacy: default_rules_path takes a relative path (e.g., ".cx/dev-no-tests.rules")
	if cfg.Context.DefaultRulesPath != "" {
		// Try resolving as a preset name first
		base := filepath.Base(cfg.Context.DefaultRulesPath)
		presetName := strings.TrimSuffix(base, RulesExt)
		if resolvedPath, findErr := m.FindRulesetFile(projectRoot, presetName); findErr == nil {
			content, err := os.ReadFile(resolvedPath)
			if err == nil {
				return content, rulesPath
			}
		}

		// Fallback: try as a direct relative path from project root
		defaultRulesPath := filepath.Join(projectRoot, cfg.Context.DefaultRulesPath)
		content, err := os.ReadFile(defaultRulesPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: could not read default_rules_path %s: %v\n", defaultRulesPath, err)
			return nil, rulesPath
		}
		return content, rulesPath
	}

	return nil, rulesPath
}

// GetDefaultRuleName returns the name of the default rule set from grove.yml.
// For example, if default_rules is "dev-no-tests", it returns "dev-no-tests".
// For legacy default_rules_path like ".cx/dev-no-tests.rules", it extracts "dev-no-tests".
// Returns empty string if no default is configured.
func (m *Manager) GetDefaultRuleName() string {
	// Load grove.yml to check for default rules
	cfg, err := config.LoadFrom(m.workDir)
	if err != nil || cfg == nil {
		return ""
	}

	if cfg.Context == nil {
		return ""
	}

	// Preferred: default_rules is already just the name
	if cfg.Context.DefaultRules != "" {
		return cfg.Context.DefaultRules
	}

	// Legacy: extract name from default_rules_path
	if cfg.Context.DefaultRulesPath != "" {
		base := filepath.Base(cfg.Context.DefaultRulesPath)
		return strings.TrimSuffix(base, ".rules")
	}

	return ""
}

// parseGitRuleForModification is a helper to parse a rule line to check if it's a Git rule.
// It handles both direct URLs and @a:git: aliases.
func (m *Manager) parseGitRuleForModification(rule string) (isGit bool, repoURL, version, ruleset string) {
	cleanRule := strings.TrimSpace(rule)
	if strings.HasPrefix(cleanRule, "!") {
		cleanRule = strings.TrimPrefix(cleanRule, "!")
	}

	// Handle @a:git: or @alias:git: alias
	isGitAlias := false
	if strings.HasPrefix(cleanRule, "@a:git:") || strings.HasPrefix(cleanRule, "@alias:git:") {
		isGitAlias = true
	}

	lineForParsing := cleanRule
	if isGitAlias {
		prefix := "@a:git:"
		if strings.HasPrefix(cleanRule, "@alias:git:") {
			prefix = "@alias:git:"
		}
		repoPart := strings.TrimPrefix(cleanRule, prefix)

		// Check for ruleset within alias (e.g., @a:git:owner/repo::ruleset)
		if strings.Contains(repoPart, "::") {
			parts := strings.SplitN(repoPart, "::", 2)
			repoPart = parts[0]
			// The ruleset will be picked up by ParseGitRule on the full line
		}

		// Check if repoPart is already a full URL or use GitHub as default
		if strings.HasPrefix(repoPart, "http://") ||
			strings.HasPrefix(repoPart, "https://") ||
			strings.HasPrefix(repoPart, "git@") ||
			strings.HasPrefix(repoPart, "file://") {
			// Use as-is - already a full URL
			lineForParsing = repoPart
		} else {
			// Shorthand - default to GitHub
			lineForParsing = "https://github.com/" + repoPart
		}
	}

	return m.ParseGitRule(lineForParsing)
}

// ListGitRules parses the active rules file and returns a slice of all Git-based rules.
func (m *Manager) ListGitRules() ([]GitRule, error) {
	content, _, err := m.LoadRulesContent()
	if err != nil {
		return nil, err
	}
	if content == nil {
		return nil, nil // No rules file, no git rules.
	}

	var gitRules []GitRule
	inColdSection := false
	scanner := bufio.NewScanner(bytes.NewReader(content))

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		if line == "---" {
			inColdSection = true
			continue
		}
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		isExclude := strings.HasPrefix(line, "!")

		// Use the helper to parse the line
		isGit, repoURL, version, ruleset := m.parseGitRuleForModification(line)

		if isGit {
			status := RuleHot
			if inColdSection {
				status = RuleCold
			}
			if isExclude {
				status = RuleExcluded
			}

			gitRules = append(gitRules, GitRule{
				RepoURL:     repoURL,
				Version:     version,
				Ruleset:     ruleset,
				ContextType: status,
				IsExclude:   isExclude,
			})
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return gitRules, nil
}

// MatchesGitRule checks if a project matches a Git rule by comparing repo URL and version.
// It handles exact version matches, partial commit hash matches, and branch-to-commit resolution.
// Parameters:
//   - rule: the Git rule to match against
//   - projectRepoURL: the project's repository URL (e.g., "https://github.com/owner/repo")
//   - projectVersion: the project's current branch or version string
//   - projectHeadCommit: the project's current HEAD commit hash
//   - projectPath: the project's filesystem path (used for git ref resolution)
func MatchesGitRule(rule GitRule, projectRepoURL, projectVersion, projectHeadCommit, projectPath string) bool {
	// Must match repo URL
	if rule.RepoURL != projectRepoURL {
		return false
	}

	// Check exact version match
	if rule.Version == projectVersion {
		return true
	}

	// Handle partial commit hash matching (rule version is prefix of project version or vice versa)
	if len(rule.Version) >= 7 {
		if strings.HasPrefix(projectVersion, rule.Version) || strings.HasPrefix(rule.Version, projectVersion) {
			return true
		}
	}

	// Handle branch-to-commit matching for detached HEAD worktrees:
	// If the rule specifies a branch (like "master") and the project is at a commit,
	// resolve the branch to its commit hash and compare
	if projectHeadCommit != "" && projectPath != "" {
		// Use exec directly here to avoid circular dependency with grove-core
		cmd := exec.Command("git", "-C", projectPath, "rev-parse", rule.Version)
		output, err := cmd.Output()
		if err == nil {
			ruleCommit := strings.TrimSpace(string(output))
			if ruleCommit != "" {
				// Compare with at least 12 chars for safety
				minLen := 12
				if len(ruleCommit) < minLen {
					minLen = len(ruleCommit)
				}
				if len(projectHeadCommit) < minLen {
					minLen = len(projectHeadCommit)
				}
				if strings.HasPrefix(ruleCommit, projectHeadCommit[:minLen]) ||
					strings.HasPrefix(projectHeadCommit, ruleCommit[:minLen]) {
					return true
				}
			}
		}
	}

	return false
}

// LoadRulesContent finds and reads the active rules file, falling back to grove.yml defaults.
// It returns the content of the rules, the path of the file read (if any), and an error.
func (m *Manager) LoadRulesContent() (content []byte, path string, err error) {
	// 0. Instance-level override — bypasses all discovery logic.
	if m.rulesFileOverride != "" {
		if _, err := os.Stat(m.rulesFileOverride); err == nil {
			content, err := os.ReadFile(m.rulesFileOverride)
			if err != nil {
				return nil, "", fmt.Errorf("reading override rules file %s: %w", m.rulesFileOverride, err)
			}
			return content, m.rulesFileOverride, nil
		}
		return nil, "", nil // Override file doesn't exist yet — match existing fallback behavior
	}

	// 1. Check state for an active rule set from .cx/
	activeSource, _ := state.GetString(StateSourceKey)
	if activeSource != "" {
		// The path in state is relative to the project root (m.workDir)
		rulesPath := filepath.Join(m.workDir, activeSource)
		if _, err := os.Stat(rulesPath); err == nil {
			content, err := os.ReadFile(rulesPath)
			if err != nil {
				return nil, "", fmt.Errorf("reading active rules file %s: %w", rulesPath, err)
			}
			return content, rulesPath, nil
		}
		// If the file in state doesn't exist, fall through to default behavior.
		// A warning could be logged here in a future iteration.
	}

	// 2. Plan-scoped rules — preferred and exclusive when a plan is active.
	// We never fall through to workspace-level notebook rules from inside a
	// worktree with an active plan: the plan dir is the single source of truth
	// for that worktree. If the plan-scoped file doesn't exist yet, return
	// (nil, "", nil) so the caller can create it at the plan-scoped path.
	if planName := m.GetActivePlanName(); planName != "" {
		if planRulesPath := m.GetPlanRulesPath(planName); planRulesPath != "" {
			if _, err := os.Stat(planRulesPath); err == nil {
				content, err := os.ReadFile(planRulesPath)
				if err != nil {
					return nil, "", fmt.Errorf("reading plan rules file %s: %w", planRulesPath, err)
				}
				return content, planRulesPath, nil
			}
			return nil, "", nil
		}
	}

	// 3. Check the notebook-scoped rules file (preferred location).
	// This must be checked BEFORE local .grove/rules so the notebook is the
	// single source of truth — .grove/rules is legacy and should only be
	// consulted as a fallback for projects that haven't migrated yet.
	if node, wsErr := workspace.GetProjectByPath(m.workDir); wsErr == nil {
		if nbRulesFile, locErr := m.locator.GetContextRulesFile(node); locErr == nil {
			if _, statErr := os.Stat(nbRulesFile); statErr == nil {
				content, err := os.ReadFile(nbRulesFile)
				if err != nil {
					return nil, "", fmt.Errorf("reading notebook rules file %s: %w", nbRulesFile, err)
				}
				// Warn if a stale .grove/rules also exists — it's now ignored.
				localRulesPath := filepath.Join(m.workDir, ActiveRulesFile)
				if _, err := os.Stat(localRulesPath); err == nil {
					rulesLog.WithField("legacy_file", localRulesPath).
						WithField("active_file", nbRulesFile).
						Warn("ignoring legacy .grove/rules — notebook rules file is now active; delete .grove/rules to silence this warning")
				}
				return content, nbRulesFile, nil
			}
		}
	}

	// 4. Look for local .grove/rules (legacy fallback)
	localRulesPath := filepath.Join(m.workDir, ActiveRulesFile)
	if _, err := os.Stat(localRulesPath); err == nil {
		content, err := os.ReadFile(localRulesPath)
		if err != nil {
			return nil, "", fmt.Errorf("reading local rules file %s: %w", localRulesPath, err)
		}
		return content, localRulesPath, nil
	}

	// 5. If not found, look for legacy .grovectx
	legacyRulesPath := filepath.Join(m.workDir, RulesFile)
	if _, err := os.Stat(legacyRulesPath); err == nil {
		content, err := os.ReadFile(legacyRulesPath)
		if err != nil {
			return nil, "", fmt.Errorf("reading legacy rules file %s: %w", legacyRulesPath, err)
		}
		return content, legacyRulesPath, nil
	}

	// 4. If not found, check grove.yml for a default
	cfg, err := config.LoadFrom(m.workDir)
	if err != nil || cfg == nil {
		// No config, so no default rules
		return nil, "", nil
	}

	if cfg.Context == nil {
		return nil, "", nil
	}

	// Project root is where the config file is found
	configPath, _ := config.FindConfigFile(m.workDir)
	projectRoot := filepath.Dir(configPath)
	if projectRoot == "" {
		projectRoot = m.workDir
	}

	// Preferred: default_rules takes just a preset name
	if cfg.Context.DefaultRules != "" {
		if resolvedPath, findErr := m.FindRulesetFile(projectRoot, cfg.Context.DefaultRules); findErr == nil {
			content, err := os.ReadFile(resolvedPath)
			if err == nil {
				return content, localRulesPath, nil
			}
		}
		fmt.Fprintf(os.Stderr, "Warning: could not find default_rules preset '%s'\n", cfg.Context.DefaultRules)
		return nil, "", nil
	}

	// Legacy: default_rules_path takes a relative path
	if cfg.Context.DefaultRulesPath != "" {
		// Try as preset name first
		base := filepath.Base(cfg.Context.DefaultRulesPath)
		presetName := strings.TrimSuffix(base, RulesExt)
		if resolvedPath, findErr := m.FindRulesetFile(projectRoot, presetName); findErr == nil {
			content, err := os.ReadFile(resolvedPath)
			if err == nil {
				return content, localRulesPath, nil
			}
		}

		// Fallback: direct relative path
		defaultRulesPath := filepath.Join(projectRoot, cfg.Context.DefaultRulesPath)
		content, err := os.ReadFile(defaultRulesPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: could not read default_rules_path %s: %v\n", defaultRulesPath, err)
			return nil, "", nil
		}
		return content, localRulesPath, nil
	}

	// 5. No local or default rules found
	return nil, "", nil
}

// ExpandBraces recursively expands shell-style brace patterns.
// Example: "path/{a,b}/{c,d}" -> ["path/a/c", "path/a/d", "path/b/c", "path/b/d"]
// expandHomeAndDot expands a leading ~ and strips a leading ./ on a rule line,
// preserving any leading ! exclusion marker.
func expandHomeAndDot(line string) string {
	excl := ""
	if strings.HasPrefix(line, "!") {
		excl = "!"
		line = line[1:]
	}
	if strings.HasPrefix(line, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			line = filepath.Join(home, line[2:])
		}
	} else if line == "~" {
		if home, err := os.UserHomeDir(); err == nil {
			line = home
		}
	}
	for strings.HasPrefix(line, "./") {
		line = line[2:]
	}
	return excl + line
}

func ExpandBraces(pattern string) []string {
	// Find the first opening brace
	leftBrace := strings.Index(pattern, "{")
	if leftBrace == -1 {
		// No braces found, return the pattern as-is
		return []string{pattern}
	}

	// Find the matching closing brace, keeping track of nesting
	rightBrace := -1
	braceDepth := 0
	for i := leftBrace + 1; i < len(pattern); i++ {
		if pattern[i] == '{' {
			braceDepth++
		} else if pattern[i] == '}' {
			if braceDepth == 0 {
				rightBrace = i
				break
			}
			braceDepth--
		}
	}

	if rightBrace == -1 {
		// Unmatched brace, return pattern as-is
		return []string{pattern}
	}

	// Extract the parts
	prefix := pattern[:leftBrace]
	suffix := pattern[rightBrace+1:]
	middle := pattern[leftBrace+1 : rightBrace]

	// Split the middle part by commas (but not commas inside nested braces)
	options := splitByComma(middle)

	var results []string
	for _, option := range options {
		// Recursively expand the rest of the pattern for each option
		expandedSuffixes := ExpandBraces(prefix + option + suffix)
		results = append(results, expandedSuffixes...)
	}

	return results
}

// splitByComma splits a string by commas, but not commas inside nested braces
func splitByComma(s string) []string {
	var results []string
	var current strings.Builder
	braceDepth := 0

	for _, ch := range s {
		if ch == '{' {
			braceDepth++
			current.WriteRune(ch)
		} else if ch == '}' {
			braceDepth--
			current.WriteRune(ch)
		} else if ch == ',' && braceDepth == 0 {
			// Found a comma at the top level
			results = append(results, current.String())
			current.Reset()
		} else {
			current.WriteRune(ch)
		}
	}

	// Add the last part
	if current.Len() > 0 {
		results = append(results, current.String())
	}

	return results
}

// parseSearchDirectives parses a line for search directives (@find:, @grep:, @grep-i:, or @changed:)
// Returns: basePattern, directives, hasDirectives
// Supports multiple directives on the same line acting as AND filters.
// Example: "pkg/**/*.go @find: \"api\" @grep: \"User\"" -> "pkg/**/*.go", [{Name: "find", Query: "api"}, {Name: "grep", Query: "User"}], true
func parseSearchDirectives(line string) (basePattern string, directives []SearchDirective, hasDirectives bool) {
	// Known directive markers
	dirMarkers := []struct {
		marker string
		name   string
	}{
		{" @grep-i: ", "grep-i"},
		{" @find!: ", "find!"},
		{" @grep!: ", "grep!"},
		{" @find: ", "find"},
		{" @grep: ", "grep"},
		{" @changed: ", "changed"},
		{" @recent: ", "recent"},
	}

	// Find the position of the first directive across all markers
	firstDirIdx := -1
	for _, dm := range dirMarkers {
		idx := strings.Index(line, dm.marker)
		if idx != -1 && (firstDirIdx == -1 || idx < firstDirIdx) {
			firstDirIdx = idx
		}
	}

	if firstDirIdx == -1 {
		return line, nil, false
	}

	basePattern = strings.TrimSpace(line[:firstDirIdx])
	remainder := line[firstDirIdx:]

	for {
		// Find the earliest directive in remainder
		bestIdx := -1
		var bestName, bestMarker string
		for _, dm := range dirMarkers {
			idx := strings.Index(remainder, dm.marker)
			if idx != -1 && (bestIdx == -1 || idx < bestIdx) {
				bestIdx = idx
				bestName = dm.name
				bestMarker = dm.marker
			}
		}
		if bestIdx == -1 {
			break
		}

		queryPart := strings.TrimSpace(remainder[bestIdx+len(bestMarker):])

		// For @changed:, quotes are optional
		if bestName == "changed" {
			query := queryPart
			// Consume until next directive or end
			for _, dm := range dirMarkers {
				if idx := strings.Index(query, dm.marker); idx != -1 {
					query = strings.TrimSpace(query[:idx])
					break
				}
			}
			if len(query) >= 2 && query[0] == '"' && query[len(query)-1] == '"' {
				query = query[1 : len(query)-1]
			}
			directives = append(directives, SearchDirective{Name: bestName, Query: query})
			advance := bestIdx + len(bestMarker) + len(query)
			if advance < len(remainder) {
				remainder = remainder[advance:]
			} else {
				break
			}
			continue
		}

		// Try quoted query
		if len(queryPart) >= 2 && queryPart[0] == '"' {
			endQuote := strings.Index(queryPart[1:], "\"")
			if endQuote != -1 {
				query := queryPart[1 : endQuote+1]
				directives = append(directives, SearchDirective{Name: bestName, Query: query})
				advance := bestIdx + len(bestMarker) + endQuote + 2
				if advance < len(remainder) {
					remainder = remainder[advance:]
				} else {
					break
				}
				continue
			}
		}

		// Unquoted query — consume until next directive or end of string
		query := queryPart
		for _, dm := range dirMarkers {
			if idx := strings.Index(query, dm.marker); idx != -1 {
				query = strings.TrimSpace(query[:idx])
				break
			}
		}
		if query != "" {
			directives = append(directives, SearchDirective{Name: bestName, Query: query})
			advance := bestIdx + len(bestMarker) + len(query)
			if advance < len(remainder) {
				remainder = remainder[advance:]
			} else {
				break
			}
		} else {
			break
		}
	}

	return basePattern, directives, len(directives) > 0
}

// encodeDirectives appends directives to a pattern in |||name|||query format.
func encodeDirectives(pattern string, directives []SearchDirective) string {
	for _, d := range directives {
		pattern = pattern + "|||" + d.Name + "|||" + d.Query
	}
	return pattern
}

// parseRulesFileContent parses the rules file content and extracts patterns, directives, and default paths.
// It does not handle recursion for imports or defaults.
func (m *Manager) parseRulesFileContent(rulesContent []byte) (*parsedRules, error) {
	results := &parsedRules{}
	if len(rulesContent) == 0 {
		return results, nil
	}

	// Surface ParseError issues from the pure parser (Phase 2 shim).
	if _, parseErrs := ParseToAST(rulesContent); len(parseErrs) > 0 {
		for _, e := range parseErrs {
			fmt.Fprintf(os.Stderr, "Error: line %d: %s\n", e.Line, e.Msg)
		}
	}

	// Create repo manager for processing Git URLs
	repoManager, repoErr := repo.NewManager()
	if repoErr != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not create repository manager: %v\n", repoErr)
	}

	// Initialize alias resolver for @alias: directives
	resolver := m.getAliasResolver()

	inColdSection := false
	// Track global search directives
	var globalDirectives []SearchDirective
	lineNum := 0 // Track line numbers

	scanner := bufio.NewScanner(bytes.NewReader(rulesContent))
	for scanner.Scan() {
		lineNum++ // Increment line number for each line
		line := strings.TrimSpace(scanner.Text())
		if line == "@freeze-cache" {
			results.freezeCache = true
			continue
		}
		if line == "@no-expire" {
			results.disableExpiration = true
			continue
		}
		if line == "@disable-cache" {
			results.disableCache = true
			continue
		}
		if strings.HasPrefix(line, "@expire-time ") {
			// Parse the duration argument
			durationStr := strings.TrimSpace(strings.TrimPrefix(line, "@expire-time "))
			if durationStr != "" {
				parsedDuration, parseErr := time.ParseDuration(durationStr)
				if parseErr != nil {
					return nil, fmt.Errorf("invalid duration format for @expire-time: %w", parseErr)
				}
				results.expireTime = parsedDuration
			}
			continue
		}
		// Support both @view: and @v: (short form)
		if strings.HasPrefix(line, "@view:") || strings.HasPrefix(line, "@v:") {
			var rulePart string
			if strings.HasPrefix(line, "@view:") {
				rulePart = strings.TrimSpace(strings.TrimPrefix(line, "@view:"))
			} else {
				rulePart = strings.TrimSpace(strings.TrimPrefix(line, "@v:"))
			}

			if rulePart != "" {
				// Resolve the rule part, which can be a complex rule itself
				resolvedPatterns, err := m.ResolveLineForRulePreview(rulePart)
				if err != nil {
					fmt.Fprintf(os.Stderr, "Warning: could not resolve view rule '%s': %v\n", rulePart, err)
					results.viewPaths = append(results.viewPaths, rulePart) // Fallback to unresolved
				} else {
					// A ruleset import can return multiple patterns
					for _, p := range strings.Split(resolvedPatterns, "\n") {
						if p != "" {
							results.viewPaths = append(results.viewPaths, p)
						}
					}
				}
			}
			continue
		}
		// Handle @tree:
		if strings.HasPrefix(line, "@tree:") {
			rulePart := strings.TrimSpace(strings.TrimPrefix(line, "@tree:"))
			if rulePart != "" {
				// For aliases, resolve directly to the project base path (not a glob pattern)
				if resolver != nil && (strings.HasPrefix(rulePart, "@alias:") || strings.HasPrefix(rulePart, "@a:")) {
					aliasPart := strings.TrimPrefix(rulePart, "@alias:")
					aliasPart = strings.TrimPrefix(aliasPart, "@a:")
					projectPath, resolveErr := resolver.Resolve(aliasPart)
					if resolveErr != nil {
						fmt.Fprintf(os.Stderr, "Warning: could not resolve alias for tree rule '%s': %v\n", rulePart, resolveErr)
						results.treePaths = append(results.treePaths, rulePart)
					} else {
						results.treePaths = append(results.treePaths, projectPath)
					}
				} else {
					results.treePaths = append(results.treePaths, rulePart)
				}
			}
			continue
		}
		if strings.HasPrefix(line, "@default:") {
			path := strings.TrimSpace(strings.TrimPrefix(line, "@default:"))
			if path != "" {
				if inColdSection {
					results.coldDefaultPaths = append(results.coldDefaultPaths, path)
				} else {
					results.mainDefaultPaths = append(results.mainDefaultPaths, path)
				}
			}
			continue
		}
		if strings.HasPrefix(line, "@concept:") {
			conceptID := strings.TrimSpace(strings.TrimPrefix(line, "@concept:"))
			if conceptID != "" {
				results.conceptIDs = append(results.conceptIDs, conceptID)
			}
			continue
		}
		if strings.HasPrefix(line, "@include:") {
			rulePart, directives, _ := parseSearchDirectives(line)
			includeName := strings.TrimSpace(strings.TrimPrefix(rulePart, "@include:"))
			if includeName != "" {
				info := ImportInfo{
					OriginalLine:     line,
					ImportIdentifier: includeName,
					LineNum:          lineNum,
					Directives:       directives,
				}
				if inColdSection {
					results.coldIncludes = append(results.coldIncludes, info)
				} else {
					results.mainIncludes = append(results.mainIncludes, info)
				}
			}
			continue
		}
		// Handle global @recent: directive
		if strings.HasPrefix(line, "@recent:") {
			queryPart := strings.TrimSpace(strings.TrimPrefix(line, "@recent:"))
			if len(queryPart) >= 2 && queryPart[0] == '"' && queryPart[len(queryPart)-1] == '"' {
				queryPart = queryPart[1 : len(queryPart)-1]
			}
			if queryPart != "" {
				globalDirectives = append(globalDirectives, SearchDirective{Name: "recent", Query: queryPart})
			}
			continue
		}
		// Handle global @find!: directive (inverted find)
		if strings.HasPrefix(line, "@find!:") {
			queryPart := strings.TrimSpace(strings.TrimPrefix(line, "@find!:"))
			if len(queryPart) >= 2 && queryPart[0] == '"' {
				if endQuote := strings.Index(queryPart[1:], "\""); endQuote != -1 {
					globalDirectives = append(globalDirectives, SearchDirective{Name: "find!", Query: queryPart[1 : endQuote+1]})
					continue
				}
			}
			if queryPart != "" {
				globalDirectives = append(globalDirectives, SearchDirective{Name: "find!", Query: queryPart})
			}
			continue
		}
		// Handle global @grep!: directive (inverted grep)
		if strings.HasPrefix(line, "@grep!:") {
			queryPart := strings.TrimSpace(strings.TrimPrefix(line, "@grep!:"))
			if len(queryPart) >= 2 && queryPart[0] == '"' {
				if endQuote := strings.Index(queryPart[1:], "\""); endQuote != -1 {
					globalDirectives = append(globalDirectives, SearchDirective{Name: "grep!", Query: queryPart[1 : endQuote+1]})
					continue
				}
			}
			if queryPart != "" {
				globalDirectives = append(globalDirectives, SearchDirective{Name: "grep!", Query: queryPart})
			}
			continue
		}
		// Handle global @find: directive
		if strings.HasPrefix(line, "@find:") {
			queryPart := strings.TrimSpace(strings.TrimPrefix(line, "@find:"))
			// Parse the quoted or unquoted query
			if len(queryPart) >= 2 && queryPart[0] == '"' {
				if endQuote := strings.Index(queryPart[1:], "\""); endQuote != -1 {
					globalDirectives = append(globalDirectives, SearchDirective{Name: "find", Query: queryPart[1 : endQuote+1]})
					continue
				}
			}
			if queryPart != "" {
				globalDirectives = append(globalDirectives, SearchDirective{Name: "find", Query: queryPart})
			}
			continue
		}
		// Handle global @grep-i: directive (must be checked before @grep:)
		if strings.HasPrefix(line, "@grep-i:") {
			queryPart := strings.TrimSpace(strings.TrimPrefix(line, "@grep-i:"))
			// Parse the quoted query
			if len(queryPart) >= 2 && queryPart[0] == '"' {
				endQuote := strings.Index(queryPart[1:], "\"")
				if endQuote != -1 {
					globalDirectives = append(globalDirectives, SearchDirective{Name: "grep-i", Query: queryPart[1 : endQuote+1]})
				}
			}
			continue
		}
		// Handle global @grep: directive
		if strings.HasPrefix(line, "@grep:") {
			queryPart := strings.TrimSpace(strings.TrimPrefix(line, "@grep:"))
			// Parse the quoted or unquoted query
			if len(queryPart) >= 2 && queryPart[0] == '"' {
				if endQuote := strings.Index(queryPart[1:], "\""); endQuote != -1 {
					globalDirectives = append(globalDirectives, SearchDirective{Name: "grep", Query: queryPart[1 : endQuote+1]})
					continue
				}
			}
			if queryPart != "" {
				globalDirectives = append(globalDirectives, SearchDirective{Name: "grep", Query: queryPart})
			}
			continue
		}
		// Handle standalone @changed: directive — expands to list of changed files
		if strings.HasPrefix(line, "@changed:") {
			ref := strings.TrimSpace(strings.TrimPrefix(line, "@changed:"))
			if len(ref) >= 2 && ref[0] == '"' && ref[len(ref)-1] == '"' {
				ref = ref[1 : len(ref)-1]
			}
			files, err := m.getChangedFiles(ref)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to get changed files for %q: %v\n", ref, err)
			} else {
				for _, file := range files {
					ruleInfo := RuleInfo{Pattern: file, IsExclude: false, LineNum: lineNum}
					if inColdSection {
						results.coldRules = append(results.coldRules, ruleInfo)
					} else {
						results.hotRules = append(results.hotRules, ruleInfo)
					}
				}
			}
			continue
		}
		// Handle standalone @diff: directive — generates a .patch file
		if strings.HasPrefix(line, "@diff:") {
			ref := strings.TrimSpace(strings.TrimPrefix(line, "@diff:"))
			diffFile, err := m.generateDiffFile(ref)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to generate diff for %q: %v\n", ref, err)
			} else if diffFile != "" {
				ruleInfo := RuleInfo{Pattern: diffFile, IsExclude: false, LineNum: lineNum}
				if inColdSection {
					results.coldRules = append(results.coldRules, ruleInfo)
				} else {
					results.hotRules = append(results.hotRules, ruleInfo)
				}
			}
			continue
		}
		if line == "---" {
			if inColdSection {
				// Already past one separator; ignore additional ones (warning emitted by ParseToAST shim).
				continue
			}
			inColdSection = true
			continue
		}
		if line != "" && !strings.HasPrefix(line, "#") {
			// Strip inline ` # comment` if no quoted directive contains a #.
			if !strings.Contains(line, `"`) {
				if cleaned := stripInlineComments(line); cleaned != "" {
					line = cleaned
				}
			}
			// First, parse out any inline search directives from the line.
			// The rest of the logic will operate on the `rulePart`.
			rulePart, directives, hasInlineDirectives := parseSearchDirectives(line)

			// Expand ~ and strip leading ./ on rulePart (preserve ! prefix).
			rulePart = expandHomeAndDot(rulePart)

			// Trailing-slash → /** for plain paths (not aliases/git URLs/directives).
			if !strings.HasPrefix(rulePart, "@") &&
				!strings.HasPrefix(rulePart, "!@") &&
				!strings.HasPrefix(rulePart, "http://") &&
				!strings.HasPrefix(rulePart, "https://") &&
				!strings.HasPrefix(rulePart, "git@") &&
				strings.HasSuffix(rulePart, "/") && rulePart != "/" {
				rulePart = rulePart + "**"
			}

			// Expand braces before type-specific processing so alias patterns
			// like @a:eco:{a,b}/path resolve each alternative independently.
			expandedRuleParts := ExpandBraces(rulePart)
			for _, rulePart := range expandedRuleParts {

				isGitAliasRuleset := false
				if strings.HasPrefix(rulePart, "@alias:git:") || strings.HasPrefix(rulePart, "@a:git:") {
					// Extract the git alias part
					tempLine := rulePart
					prefix := "@a:git:"
					if strings.HasPrefix(tempLine, "@alias:git:") {
						prefix = "@alias:git:"
					}
					gitPart := strings.TrimPrefix(tempLine, prefix)

					// Check if it has a ruleset specifier (::)
					if strings.Contains(gitPart, "::") {

						isGitAliasRuleset = true
						// Split on :: to get repo part and ruleset name
						parts := strings.SplitN(gitPart, "::", 2)
						if len(parts) == 2 {
							repoPart := parts[0]    // e.g., "owner/repo" or "owner/repo@version"
							rulesetName := parts[1] // e.g., "default"

							// Check if repoPart is already a full URL or use GitHub as default
							var fullURL string
							if strings.HasPrefix(repoPart, "http://") ||
								strings.HasPrefix(repoPart, "https://") ||
								strings.HasPrefix(repoPart, "git@") ||
								strings.HasPrefix(repoPart, "file://") {
								// Use as-is - already a full URL
								fullURL = repoPart
							} else {
								// Shorthand - default to GitHub
								fullURL = "https://github.com/" + repoPart
							}

							// Parse to extract version if present
							_, repoURL, version, _ := m.ParseGitRule(fullURL)

							// Create git import identifier
							importIdentifier := fmt.Sprintf("git::%s@%s::%s", repoURL, version, rulesetName)

							if inColdSection {
								results.coldImportedRuleSets = append(results.coldImportedRuleSets, ImportInfo{
									OriginalLine:     line,
									ImportIdentifier: importIdentifier,
									LineNum:          lineNum,
									Directives:       directives,
								})
							} else {
								results.mainImportedRuleSets = append(results.mainImportedRuleSets, ImportInfo{
									OriginalLine:     line,
									ImportIdentifier: importIdentifier,
									LineNum:          lineNum,
									Directives:       directives,
								})
							}
						}
					}
				}

				if isGitAliasRuleset {
					continue // Skip further processing for this line
				}

				// Check for regular ruleset imports (using :: delimiter)
				// Format: @alias:project-name::ruleset-name or @a:project-name::ruleset-name
				isRuleSetAlias := false
				if strings.HasPrefix(rulePart, "@alias:") || strings.HasPrefix(rulePart, "@a:") {
					// Extract the alias part
					aliasPart := rulePart
					if strings.HasPrefix(aliasPart, "@alias:") {
						aliasPart = strings.TrimPrefix(aliasPart, "@alias:")
					} else {
						aliasPart = strings.TrimPrefix(aliasPart, "@a:")
					}

					// Check for '::' to identify a ruleset import
					if strings.Contains(aliasPart, "::") {
						isRuleSetAlias = true
						// Preserve '::' delimiter for the resolver to correctly parse multi-part aliases
						parts := strings.SplitN(aliasPart, "::", 2)
						if len(parts) == 2 {
							importIdentifier := strings.Join(parts, "::")

							if inColdSection {
								results.coldImportedRuleSets = append(results.coldImportedRuleSets, ImportInfo{
									OriginalLine:     line,
									ImportIdentifier: importIdentifier,
									LineNum:          lineNum,
									Directives:       directives,
								})
							} else {
								results.mainImportedRuleSets = append(results.mainImportedRuleSets, ImportInfo{
									OriginalLine:     line,
									ImportIdentifier: importIdentifier,
									LineNum:          lineNum,
									Directives:       directives,
								})
							}
						}
					}
				}

				if isRuleSetAlias {
					continue // Skip further processing for this line
				}

				// Check for command expressions
				if strings.HasPrefix(line, "@cmd:") {
					cmdExpr := strings.TrimPrefix(line, "@cmd:")
					cmdExpr = strings.TrimSpace(cmdExpr)

					// Execute the command and get file paths
					if cmdFiles, cmdErr := m.executeCommandExpression(cmdExpr); cmdErr == nil {
						// Add each file from command output as a pattern
						for _, file := range cmdFiles {
							if inColdSection {
								results.coldRules = append(results.coldRules, RuleInfo{Pattern: file, IsExclude: false, LineNum: lineNum})
							} else {
								results.hotRules = append(results.hotRules, RuleInfo{Pattern: file, IsExclude: false, LineNum: lineNum})
							}
						}
					} else {
						fmt.Fprintf(os.Stderr, "Warning: command expression failed: %s: %v\n", cmdExpr, cmdErr)
					}
					continue
				}

				// Handle Git aliases before workspace aliases.
				// e.g., @a:git:owner/repo[@version]
				isGitAlias := false
				tempLineForCheck := rulePart
				if strings.HasPrefix(tempLineForCheck, "!") {
					tempLineForCheck = strings.TrimSpace(strings.TrimPrefix(tempLineForCheck, "!"))
				}
				if strings.HasPrefix(tempLineForCheck, "@a:git:") || strings.HasPrefix(tempLineForCheck, "@alias:git:") {
					isGitAlias = true
				}

				// Resolve alias if present (supports both @alias: and @a:), before further processing
				processedLine := rulePart
				if isGitAlias {
					isExclude := false
					tempLine := rulePart
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

					// Check if repoPart is already a full URL or use GitHub as default
					var fullURL string
					if strings.HasPrefix(repoPart, "http://") ||
						strings.HasPrefix(repoPart, "https://") ||
						strings.HasPrefix(repoPart, "git@") ||
						strings.HasPrefix(repoPart, "file://") {
						// Use as-is - already a full URL
						fullURL = repoPart
					} else {
						// Shorthand - default to GitHub
						fullURL = "https://github.com/" + repoPart
					}

					if isExclude {
						processedLine = "!" + fullURL
					} else {
						processedLine = fullURL
					}
				} else if resolver != nil && (strings.Contains(rulePart, "@alias:") || strings.Contains(rulePart, "@a:")) {
					if strings.HasSuffix(rulePart, "/") {
						rulePart = rulePart + "**"
					}
					resolvedLine, resolveErr := resolver.ResolveLine(rulePart)
					if resolveErr != nil {
						fmt.Fprintf(os.Stderr, "Warning: could not resolve alias in line '%s': %v\n", line, resolveErr)
						continue // Skip this line if alias resolution fails
					}

					// Extract the project path from the resolved line to validate against workspace exclusions
					// The resolved line will be something like "/path/to/project/**" or "!/path/to/project/src/**/*.go"
					pathToValidate := resolvedLine
					if strings.HasPrefix(pathToValidate, "!") {
						pathToValidate = strings.TrimPrefix(pathToValidate, "!")
					}
					// Parse out any search directives to get the base path
					basePathForValidation, _, _ := parseSearchDirectives(pathToValidate)
					// Extract just the directory part (remove glob patterns)
					// For patterns like "/path/to/project/**" or "/path/to/project/src/**/*.go"
					// we want to extract the project root path
					pathParts := strings.Split(basePathForValidation, string(filepath.Separator))
					var projectPath string
					for i, part := range pathParts {
						if strings.Contains(part, "*") || strings.Contains(part, "?") {
							// Found the first glob part, take everything before it
							projectPath = strings.Join(pathParts[:i], string(filepath.Separator))
							break
						}
					}
					if projectPath == "" {
						// No glob found, the entire path might be a directory
						projectPath = basePathForValidation
					}

					// Validate that the resolved project path is allowed
					if allowed, reason := m.IsPathAllowed(projectPath); !allowed {
						fmt.Fprintf(os.Stderr, "Warning: skipping rule '%s': %s\n", line, reason)
						m.addSkippedRule(lineNum, line, reason)
						continue
					}

					processedLine = resolvedLine

					// If the resolved line is just a directory path (no glob pattern),
					// append /** to match all files in that directory
					// Check the base pattern part before any directives
					baseForCheck, _, _ := parseSearchDirectives(processedLine)
					if !strings.Contains(baseForCheck, "*") && !strings.Contains(baseForCheck, "?") {
						// Check if the path is actually a directory before appending /**
						checkPath := baseForCheck
						// Handle exclusion prefix
						if strings.HasPrefix(checkPath, "!") {
							checkPath = strings.TrimPrefix(checkPath, "!")
						}
						if info, err := os.Stat(checkPath); err == nil && info.IsDir() {
							// It's a directory, append /**
							// Replace the base pattern with base + /**
							if baseForCheck == processedLine {
								// No directive, just append /**
								processedLine = processedLine + "/**"
							} else {
								// Has directive, insert /** before it
								directivePart := strings.TrimPrefix(processedLine, baseForCheck)
								processedLine = baseForCheck + "/**" + directivePart
							}
						}
						// If it's a file or doesn't exist, keep as-is
					}
				}

				// Process Git URLs
				if repoManager != nil {
					isExclude := strings.HasPrefix(processedLine, "!")
					cleanLine := processedLine
					if isExclude {
						cleanLine = strings.TrimPrefix(processedLine, "!")
					}

					if isGitURL, repoURL, version, ruleset := m.ParseGitRule(cleanLine); isGitURL {
						// If a ruleset is specified, treat this as a special import, not a pattern
						if ruleset != "" {
							importIdentifier := fmt.Sprintf("git::%s@%s::%s", repoURL, version, ruleset)
							if isExclude {
								// Exclusions on git ruleset imports are not yet supported.
								fmt.Fprintf(os.Stderr, "Warning: exclusion prefix '!' on git ruleset import is not supported: %s\n", processedLine)
							} else {
								if inColdSection {
									results.coldImportedRuleSets = append(results.coldImportedRuleSets, ImportInfo{
										OriginalLine:     line,
										ImportIdentifier: importIdentifier,
										LineNum:          lineNum,
									})
								} else {
									results.mainImportedRuleSets = append(results.mainImportedRuleSets, ImportInfo{
										OriginalLine:     line,
										ImportIdentifier: importIdentifier,
										LineNum:          lineNum,
									})
								}
							}
							continue // This is an import, so we're done with this line.
						}

						// Ensure the repository worktree exists for the specified version
						localPath, _, cloneErr := repoManager.EnsureVersion(m.Context(), repoURL, version)
						if cloneErr != nil {
							fmt.Fprintf(os.Stderr, "Warning: could not ensure repository version %s: %v\n", repoURL, cloneErr)
							continue
						}

						// Extract any path pattern that comes after the repo URL
						// e.g., https://github.com/owner/repo@v1.0.0/**/*.yml -> /**/*.yml
						// We need to find what remains after protocol://domain/owner/repo[@version]
						pathPattern := "/**"

						// Build the full repo reference (URL with optional version)
						repoRef := repoURL
						if version != "" {
							repoRef = repoURL + "@" + version
						}

						// Check if cleanLine has additional path after the repo reference
						if len(cleanLine) > len(repoRef) && strings.HasPrefix(cleanLine, repoRef) {
							pathPattern = cleanLine[len(repoRef):]
						}

						// Replace the Git URL with the local path pattern
						processedLine = localPath + pathPattern
						if isExclude {
							processedLine = "!" + processedLine
						}
					}
				}

				// We already parsed inline directives at the start.
				// Now we check if we should apply a global directive.
				basePattern := processedLine

				// If no inline directives but there are global directives, use them
				if !hasInlineDirectives && len(globalDirectives) > 0 {
					directives = globalDirectives
					hasInlineDirectives = true // Mark that directives are now active
				}

				// Apply brace expansion to the base pattern
				expandedPatterns := ExpandBraces(basePattern)

				// Add each expanded pattern to the appropriate list
				for _, expandedPattern := range expandedPatterns {
					// Detect if this is an exclusion pattern
					isExclude := strings.HasPrefix(expandedPattern, "!")
					cleanPattern := expandedPattern
					if isExclude {
						cleanPattern = strings.TrimPrefix(expandedPattern, "!")
					}

					ruleInfo := RuleInfo{
						Pattern:   cleanPattern,
						IsExclude: isExclude,
						LineNum:   lineNum,
					}
					if hasInlineDirectives {
						ruleInfo.Directives = directives
					}

					if inColdSection {
						results.coldRules = append(results.coldRules, ruleInfo)
					} else {
						results.hotRules = append(results.hotRules, ruleInfo)
					}
				}
			} // end expandedRuleParts loop
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return results, nil
}

// findActiveRulesFile returns the path to the active rules file if it exists
func (m *Manager) findActiveRulesFile() string {
	if m.rulesFileOverride != "" {
		return m.rulesFileOverride
	}
	// Check plan-scoped rules first
	if planName := m.GetActivePlanName(); planName != "" {
		if planRulesPath := m.GetPlanRulesPath(planName); planRulesPath != "" {
			if _, err := os.Stat(planRulesPath); err == nil {
				return planRulesPath
			}
		}
	}

	// Check notebook location
	if node, err := workspace.GetProjectByPath(m.workDir); err == nil {
		if nbRulesFile, err := m.locator.GetContextRulesFile(node); err == nil {
			if _, statErr := os.Stat(nbRulesFile); statErr == nil {
				return nbRulesFile
			}
		}
	}

	// Check for new rules file location
	activeRulesPath := filepath.Join(m.workDir, ActiveRulesFile)
	if _, err := os.Stat(activeRulesPath); err == nil {
		return activeRulesPath
	}

	// Check for old .grovectx file for backward compatibility
	oldRulesPath := filepath.Join(m.workDir, RulesFile)
	if _, err := os.Stat(oldRulesPath); err == nil {
		return oldRulesPath
	}

	return ""
}

// ShouldFreezeCache checks if the @freeze-cache directive is present in the rules file.
func (m *Manager) ShouldFreezeCache() (bool, error) {
	rulesContent, _, err := m.LoadRulesContent()
	if err != nil {
		return false, err
	}
	if rulesContent == nil {
		return false, nil
	}
	parsed, err := m.parseRulesFileContent(rulesContent)
	if err != nil {
		return false, fmt.Errorf("error parsing rules file for cache directive: %w", err)
	}

	return parsed.freezeCache, nil
}

// ShouldDisableExpiration checks if the @no-expire directive is present in the rules file.
func (m *Manager) ShouldDisableExpiration() (bool, error) {
	rulesContent, _, err := m.LoadRulesContent()
	if err != nil {
		return false, err
	}
	if rulesContent == nil {
		return false, nil
	}
	parsed, err := m.parseRulesFileContent(rulesContent)
	if err != nil {
		return false, fmt.Errorf("error parsing rules file for cache directive: %w", err)
	}

	return parsed.disableExpiration, nil
}

// GetExpireTime returns the custom expiration duration if @expire-time directive is present.
// Returns 0 if no custom expiration time is set.
func (m *Manager) GetExpireTime() (time.Duration, error) {
	rulesContent, _, err := m.LoadRulesContent()
	if err != nil {
		return 0, err
	}
	if rulesContent == nil {
		return 0, nil
	}
	parsed, err := m.parseRulesFileContent(rulesContent)
	if err != nil {
		return 0, fmt.Errorf("error parsing rules file for expire time: %w", err)
	}

	return parsed.expireTime, nil
}

// ShouldDisableCache checks if the @disable-cache directive is present in the rules file.
func (m *Manager) ShouldDisableCache() (bool, error) {
	rulesContent, _, err := m.LoadRulesContent()
	if err != nil {
		return false, err
	}
	if rulesContent == nil {
		return false, nil
	}
	parsed, err := m.parseRulesFileContent(rulesContent)
	if err != nil {
		return false, fmt.Errorf("error parsing rules file for cache directive: %w", err)
	}

	return parsed.disableCache, nil
}

// SetActiveRules copies a rules file to the active rules location
func (m *Manager) SetActiveRules(sourcePath string) error {
	// Check for zombie worktree - refuse to create rules in deleted worktrees
	if IsZombieWorktree(m.workDir) {
		return fmt.Errorf("cannot create rules file: worktree has been deleted")
	}

	// Check if source file exists
	if _, err := os.Stat(sourcePath); os.IsNotExist(err) {
		return fmt.Errorf("source rules file not found: %s", sourcePath)
	}

	// Read content from source
	content, err := os.ReadFile(sourcePath)
	if err != nil {
		return fmt.Errorf("error reading source rules file %s: %w", sourcePath, err)
	}

	// Write to active rules file (handles plan-scoped paths and creates parent dirs)
	activeRulesPath := m.ResolveRulesWritePath()
	if err := os.WriteFile(activeRulesPath, content, 0o644); err != nil {
		return fmt.Errorf("error writing active rules file: %w", err)
	}

	fmt.Printf("Set active rules from %s\n", sourcePath)
	return nil
}

// WriteRulesTo writes the current active rules to a specified file path
func (m *Manager) WriteRulesTo(destPath string) error {
	// Check for zombie worktree - refuse to create rules in deleted worktrees
	if IsZombieWorktree(destPath) {
		return fmt.Errorf("cannot create rules file: worktree has been deleted")
	}

	// Find the active rules file
	rulesFilePath := m.findActiveRulesFile()
	if rulesFilePath == "" {
		return fmt.Errorf("no active rules file found")
	}

	// Read content from active rules
	content, err := os.ReadFile(rulesFilePath)
	if err != nil {
		return fmt.Errorf("error reading active rules file: %w", err)
	}

	// Ensure destination directory exists
	destDir := filepath.Dir(destPath)
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return fmt.Errorf("error creating destination directory: %w", err)
	}

	// Write to destination file, overwriting if it exists
	if err := os.WriteFile(destPath, content, 0o644); err != nil {
		return fmt.Errorf("error writing to destination file: %w", err)
	}

	return nil
}

// removeGitRulesForRepo scans the active rules file and removes all rules
// (hot, cold, or excluded) that pertain to the given repository URL.
func (m *Manager) removeGitRulesForRepo(repoURL string) error {
	rulesFilePath := m.findActiveRulesFile()
	if rulesFilePath == "" {
		return nil // No file, nothing to remove
	}

	content, err := os.ReadFile(rulesFilePath)
	if err != nil {
		// If file doesn't exist, it's not an error
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("reading rules file: %w", err)
	}

	lines := strings.Split(string(content), "\n")
	var newLines []string

	for _, line := range lines {
		isGit, lineRepoURL, _, _ := m.parseGitRuleForModification(strings.TrimSpace(line))
		if isGit && lineRepoURL == repoURL {
			// This is a rule for the repo we want to remove, so skip it.
			continue
		}
		newLines = append(newLines, line)
	}

	// Clean up and write back to file
	newLines = cleanupRulesLines(newLines)

	newContent := strings.Join(newLines, "\n")
	if len(newLines) > 0 && !strings.HasSuffix(newContent, "\n") {
		newContent += "\n"
	}
	return os.WriteFile(rulesFilePath, []byte(newContent), 0o644)
}

// AppendRule adds a rule to the active rules file in the specified context
// contextType can be "hot", "cold", or "exclude".
func (m *Manager) AppendRule(rulePath, contextType string) error {
	// Check for zombie worktree - refuse to create rules in deleted worktrees
	if IsZombieWorktree(m.workDir) {
		return fmt.Errorf("cannot create rules file: worktree has been deleted")
	}

	// Validate the rule safety before adding
	if err := m.validateRuleSafety(rulePath); err != nil {
		return fmt.Errorf("safety validation failed: %w", err)
	}

	// Check if the new rule is a Git rule. If so, remove all existing rules for that repo.
	// Otherwise, use the existing path-based removal logic.
	isGit, repoURL, _, _ := m.parseGitRuleForModification(rulePath)
	if isGit {
		if err := m.removeGitRulesForRepo(repoURL); err != nil {
			// Non-fatal error, log and continue
			fmt.Fprintf(os.Stderr, "Warning: could not remove existing git rules for %s: %v\n", repoURL, err)
		}
	} else {
		// First, remove any existing rules for this path to prevent duplicates
		// This makes the function idempotent and handles state changes
		if err := m.RemoveRuleForPath(rulePath); err != nil {
			// Non-fatal error, log and continue
			fmt.Fprintf(os.Stderr, "Warning: could not remove existing rules: %v\n", err)
		}
	}

	// Find or create the rules file
	rulesFilePath := m.findActiveRulesFile()
	if rulesFilePath == "" {
		rulesFilePath = m.ResolveRulesWritePath()
	}

	// Read existing content
	var lines []string
	if content, err := os.ReadFile(rulesFilePath); err == nil {
		scanner := bufio.NewScanner(strings.NewReader(string(content)))
		for scanner.Scan() {
			lines = append(lines, scanner.Text())
		}
	}

	// Prepare the new rule
	var newRule string
	switch contextType {
	case "exclude", "exclude-cold":
		newRule = "!" + rulePath
	default:
		newRule = rulePath
	}

	// Find separator line index
	separatorIndex := -1
	for i, line := range lines {
		if strings.TrimSpace(line) == "---" {
			separatorIndex = i
			break
		}
	}

	// Insert the new rule based on context type
	switch contextType {
	case "hot", "exclude":
		if separatorIndex >= 0 {
			// Insert before separator
			lines = insertAt(lines, separatorIndex, newRule)
		} else {
			// No separator, append to end
			lines = append(lines, newRule)
		}
	case "cold", "exclude-cold":
		if separatorIndex >= 0 {
			// Append after separator
			lines = append(lines, newRule)
		} else {
			// No separator, add one first then the rule
			lines = append(lines, "---", newRule)
		}
	}

	// Write back to file
	content := strings.Join(lines, "\n")
	if len(lines) > 0 && !strings.HasSuffix(content, "\n") {
		content += "\n"
	}
	return os.WriteFile(rulesFilePath, []byte(content), 0o644)
}

// ToggleViewDirective adds or removes a `@view:` directive from the rules file.
func (m *Manager) ToggleViewDirective(path string) error {
	// Check for zombie worktree - refuse to create rules in deleted worktrees
	if IsZombieWorktree(m.workDir) {
		return fmt.Errorf("cannot create rules file: worktree has been deleted")
	}

	rulesFilePath := m.findActiveRulesFile()
	if rulesFilePath == "" {
		rulesFilePath = m.ResolveRulesWritePath()
	}

	content, err := os.ReadFile(rulesFilePath)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("error reading rules file: %w", err)
	}

	lines := strings.Split(string(content), "\n")
	var newLines []string
	found := false
	viewDirective := "@view: " + path

	for _, line := range lines {
		if strings.TrimSpace(line) == viewDirective {
			found = true
			continue // Remove the line
		}
		newLines = append(newLines, line)
	}

	if !found {
		// Add the directive to the top
		newLines = append([]string{viewDirective}, newLines...)
	}

	// Clean up empty lines at the end
	for len(newLines) > 0 && strings.TrimSpace(newLines[len(newLines)-1]) == "" {
		newLines = newLines[:len(newLines)-1]
	}

	newContent := strings.Join(newLines, "\n")
	if len(newLines) > 0 {
		newContent += "\n" // Ensure trailing newline
	}

	return os.WriteFile(rulesFilePath, []byte(newContent), 0o644)
}

// GetRuleStatus checks the current status of a rule in the rules file
func (m *Manager) GetRuleStatus(rulePath string) RuleStatus {
	rulesFilePath := m.findActiveRulesFile()
	if rulesFilePath == "" {
		return RuleNotFound
	}

	content, err := os.ReadFile(rulesFilePath)
	if err != nil {
		return RuleNotFound
	}

	// Check for exclusion rule
	excludeRule := "!" + rulePath
	// Check for normal rule
	normalRule := rulePath

	lines := strings.Split(string(content), "\n")
	inColdSection := false

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "---" {
			inColdSection = true
			continue
		}

		if line == excludeRule {
			return RuleExcluded
		}

		if line == normalRule {
			if inColdSection {
				return RuleCold
			} else {
				return RuleHot
			}
		}
	}

	return RuleNotFound
}

// RemoveRule removes a specific rule from the rules file
func (m *Manager) RemoveRule(rulePath string) error {
	rulesFilePath := m.findActiveRulesFile()
	if rulesFilePath == "" {
		// No rules file exists, nothing to remove
		return nil
	}

	content, err := os.ReadFile(rulesFilePath)
	if err != nil {
		return fmt.Errorf("error reading rules file: %w", err)
	}

	lines := strings.Split(string(content), "\n")
	var newLines []string

	// Rules to potentially remove
	excludeRule := "!" + rulePath
	normalRule := rulePath

	for _, line := range lines {
		trimmedLine := strings.TrimSpace(line)
		// Skip the lines that match our rule (either normal or exclude form)
		if trimmedLine == excludeRule || trimmedLine == normalRule {
			continue
		}
		newLines = append(newLines, line)
	}

	// Clean up empty lines and unnecessary separators
	newLines = cleanupRulesLines(newLines)

	// Write back to file
	newContent := strings.Join(newLines, "\n")
	if len(newLines) > 0 && !strings.HasSuffix(newContent, "\n") {
		newContent += "\n"
	}

	return os.WriteFile(rulesFilePath, []byte(newContent), 0o644)
}

// RemoveRuleForPath removes any rule that corresponds to the given repository path.
// Unlike RemoveRule which requires an exact match, this function will find and remove
// rules in various formats (path, !path, path/**, !path/**) that match the repository.
func (m *Manager) RemoveRuleForPath(path string) error {
	rulesFilePath := m.findActiveRulesFile()
	if rulesFilePath == "" {
		// No rules file exists, nothing to remove
		return nil
	}

	content, err := os.ReadFile(rulesFilePath)
	if err != nil {
		return fmt.Errorf("error reading rules file: %w", err)
	}

	lines := strings.Split(string(content), "\n")
	var newLines []string

	// Clean the input path
	path = strings.TrimSpace(path)
	path = strings.TrimSuffix(path, "/")

	// Generate all possible patterns to look for based on the path
	patternsToRemove := []string{
		path,               // exact path
		"!" + path,         // excluded path
		path + "/**",       // recursive include
		"!" + path + "/**", // recursive exclude
		path + "/*",        // single level include
		"!" + path + "/*",  // single level exclude
	}

	// Also check for relative paths starting with ./ or ../
	if !filepath.IsAbs(path) {
		patternsToRemove = append(patternsToRemove,
			"./"+path,
			"!./"+path,
			"./"+path+"/**",
			"!./"+path+"/**",
		)
	}

	// Check each line and skip if it matches any of our patterns
	for _, line := range lines {
		trimmedLine := strings.TrimSpace(line)
		shouldRemove := false

		for _, pattern := range patternsToRemove {
			if trimmedLine == pattern {
				shouldRemove = true
				break
			}
		}

		if !shouldRemove {
			newLines = append(newLines, line)
		}
	}

	// Clean up empty lines and unnecessary separators
	newLines = cleanupRulesLines(newLines)

	// Write back to file
	newContent := strings.Join(newLines, "\n")
	if len(newLines) > 0 && !strings.HasSuffix(newContent, "\n") {
		newContent += "\n"
	}

	return os.WriteFile(rulesFilePath, []byte(newContent), 0o644)
}

// validateRuleSafety checks if a rule is safe to add
func (m *Manager) validateRuleSafety(rulePath string) error {
	// Skip validation for exclusion rules
	if strings.HasPrefix(rulePath, "!") {
		rulePath = strings.TrimPrefix(rulePath, "!")
	}

	// Count parent directory traversals
	traversalCount := strings.Count(rulePath, "../")
	if traversalCount > 2 {
		return fmt.Errorf("rule '%s' contains too many parent directory traversals (max 2 allowed)", rulePath)
	}

	// Check for patterns that could match everything
	if rulePath == "**" || rulePath == "/**" || strings.HasPrefix(rulePath, "../../../") {
		return fmt.Errorf("rule '%s' is too broad and could include system files", rulePath)
	}

	// Resolve the actual path to check boundaries
	absPath := filepath.Join(m.workDir, rulePath)
	absPath = filepath.Clean(absPath)

	// Get home directory for boundary checking
	homeDir, err := os.UserHomeDir()
	if err != nil {
		homeDir = ""
	}

	// Check if the rule would go above the home directory
	if homeDir != "" && len(absPath) < len(homeDir) {
		// Path is shorter than home dir, meaning it's above it
		homeParts := strings.Split(homeDir, string(filepath.Separator))
		absParts := strings.Split(absPath, string(filepath.Separator))
		if len(absParts) < len(homeParts)-1 { // Allow one level above home
			return fmt.Errorf("rule '%s' would include directories too far above home directory", rulePath)
		}
	}

	// Check against system directories (both Unix and Windows)
	dangerousPaths := []string{
		"/etc", "/usr", "/bin", "/sbin", "/System", "/Library",
		"/proc", "/sys", "/dev", "/root",
		"C:\\Windows", "C:\\Program Files", "C:\\ProgramData",
	}

	for _, dangerous := range dangerousPaths {
		if absPath == dangerous || strings.HasPrefix(absPath, dangerous+string(filepath.Separator)) {
			return fmt.Errorf("rule '%s' would include system directory '%s'", rulePath, dangerous)
		}
	}

	// Check if it's trying to include hidden system directories
	if strings.Contains(rulePath, "/.") && traversalCount > 0 {
		// Be extra careful with hidden directories when going up
		if strings.Contains(rulePath, "/.Trash") || strings.Contains(rulePath, "/.cache") ||
			strings.Contains(rulePath, "/.config") {
			return fmt.Errorf("rule '%s' would include hidden system directories", rulePath)
		}
	}

	return nil
}

// insertAt inserts a string at the specified index in a slice
func insertAt(slice []string, index int, value string) []string {
	if index < 0 || index > len(slice) {
		return slice
	}

	result := make([]string, len(slice)+1)
	copy(result, slice[:index])
	result[index] = value
	copy(result[index+1:], slice[index:])
	return result
}

// cleanupRulesLines removes unnecessary separators and empty lines
func cleanupRulesLines(lines []string) []string {
	// Remove trailing empty lines
	for len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) == "" {
		lines = lines[:len(lines)-1]
	}

	// If only separator remains, remove it
	if len(lines) == 1 && strings.TrimSpace(lines[0]) == "---" {
		return []string{}
	}

	// Remove separator if there are no cold context rules after it
	hasColdRules := false
	separatorIndex := -1

	for i, line := range lines {
		if strings.TrimSpace(line) == "---" {
			separatorIndex = i
		} else if separatorIndex >= 0 && strings.TrimSpace(line) != "" {
			hasColdRules = true
			break
		}
	}

	// Remove separator if no cold rules follow
	if separatorIndex >= 0 && !hasColdRules {
		result := make([]string, 0, len(lines)-1)
		for i, line := range lines {
			if i != separatorIndex {
				result = append(result, line)
			}
		}
		lines = result
	}

	return lines
}

// executeCommandExpression executes a shell command and returns the file paths from its output
func (m *Manager) executeCommandExpression(cmdExpr string) ([]string, error) {
	// Execute the command using shell
	cmd := exec.Command("sh", "-c", cmdExpr)
	cmd.Dir = m.workDir

	// Capture output
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("command failed: %w", err)
	}

	// Parse output into file paths
	var files []string
	scanner := bufio.NewScanner(bytes.NewReader(output))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" {
			// Make absolute path if relative
			absPath := line
			if !filepath.IsAbs(line) {
				absPath = filepath.Join(m.workDir, line)
			}
			absPath = filepath.Clean(absPath)

			// Check if file exists
			if info, err := os.Stat(absPath); err == nil && !info.IsDir() {
				// Get relative path from workDir if possible
				relPath, err := filepath.Rel(m.workDir, absPath)
				if err == nil && !strings.HasPrefix(relPath, "..") {
					// File is within workDir, use relative path
					files = append(files, relPath)
				} else {
					// File is outside workDir, use absolute path
					files = append(files, absPath)
				}
			}
		}
	}

	return files, nil
}
