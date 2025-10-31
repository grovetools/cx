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

	"github.com/mattsolo1/grove-core/config"
	"github.com/mattsolo1/grove-core/pkg/repo"
	"github.com/mattsolo1/grove-core/state"
)

// parsedRules holds the fully parsed contents of a single rules file,
// including all rules, directives, and import statements.
type parsedRules struct {
	hotRules             []RuleInfo
	coldRules            []RuleInfo
	mainDefaultPaths     []string
	coldDefaultPaths     []string
	mainImportedRuleSets []ImportInfo
	coldImportedRuleSets []ImportInfo
	viewPaths            []string
	freezeCache          bool
	disableExpiration    bool
	disableCache         bool
	expireTime           time.Duration
}

// RuleStatus represents the current state of a rule
type RuleStatus int

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
	rulesPath = filepath.Join(m.workDir, ActiveRulesFile)

	// Load grove.yml to check for default rules
	cfg, err := config.LoadFrom(m.workDir)
	if err != nil || cfg == nil {
		// No config, so no default rules
		return nil, rulesPath
	}

	// Use custom extension approach since the Context field may not exist in grove-core yet
	var contextConfig struct {
		DefaultRulesPath string `yaml:"default_rules_path"`
	}

	if err := cfg.UnmarshalExtension("context", &contextConfig); err != nil {
		// Extension doesn't exist or failed to unmarshal, no default rules
		return nil, rulesPath
	}

	if contextConfig.DefaultRulesPath != "" {
		// Project root is where grove.yml is found
		configPath, _ := config.FindConfigFile(m.workDir)
		projectRoot := filepath.Dir(configPath)
		if projectRoot == "" {
			projectRoot = m.workDir
		}
		defaultRulesPath := filepath.Join(projectRoot, contextConfig.DefaultRulesPath)

		content, err := os.ReadFile(defaultRulesPath)
		if err != nil {
			// Don't error out, just warn and act as if no default exists
			fmt.Fprintf(os.Stderr, "Warning: could not read default_rules_path %s: %v\n", defaultRulesPath, err)
			return nil, rulesPath
		}
		return content, rulesPath
	}

	return nil, rulesPath
}

// GetDefaultRuleName returns the name of the default rule set from grove.yml.
// For example, if default_rules_path is ".cx/dev-no-tests.rules", it returns "dev-no-tests".
// Returns empty string if no default is configured.
func (m *Manager) GetDefaultRuleName() string {
	// Load grove.yml to check for default rules
	cfg, err := config.LoadFrom(m.workDir)
	if err != nil || cfg == nil {
		return ""
	}

	// Use custom extension approach
	var contextConfig struct {
		DefaultRulesPath string `yaml:"default_rules_path"`
	}

	if err := cfg.UnmarshalExtension("context", &contextConfig); err != nil {
		return ""
	}

	if contextConfig.DefaultRulesPath != "" {
		// Extract just the filename without extension
		base := filepath.Base(contextConfig.DefaultRulesPath)
		// Remove .rules extension if present
		name := strings.TrimSuffix(base, ".rules")
		return name
	}

	return ""
}

// LoadRulesContent finds and reads the active rules file, falling back to grove.yml defaults.
// It returns the content of the rules, the path of the file read (if any), and an error.
func (m *Manager) LoadRulesContent() (content []byte, path string, err error) {
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

	// 2. Look for local .grove/rules
	localRulesPath := filepath.Join(m.workDir, ActiveRulesFile)
	if _, err := os.Stat(localRulesPath); err == nil {
		content, err := os.ReadFile(localRulesPath)
		if err != nil {
			return nil, "", fmt.Errorf("reading local rules file %s: %w", localRulesPath, err)
		}
		return content, localRulesPath, nil
	}

	// 3. If not found, look for legacy .grovectx
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

	// Note: For now, we'll use a custom extension approach since the Context field
	// may not exist in grove-core yet
	var contextConfig struct {
		DefaultRulesPath string `yaml:"default_rules_path"`
	}

	if err := cfg.UnmarshalExtension("context", &contextConfig); err != nil {
		// Extension doesn't exist or failed to unmarshal, no default rules
		return nil, "", nil
	}

	if contextConfig.DefaultRulesPath != "" {
		// Project root is where grove.yml is found
		configPath, _ := config.FindConfigFile(m.workDir)
		projectRoot := filepath.Dir(configPath)
		if projectRoot == "" {
			projectRoot = m.workDir
		}
		defaultRulesPath := filepath.Join(projectRoot, contextConfig.DefaultRulesPath)

		content, err := os.ReadFile(defaultRulesPath)
		if err != nil {
			// Don't error out, just warn and act as if no default exists
			fmt.Fprintf(os.Stderr, "Warning: could not read default_rules_path %s: %v\n", defaultRulesPath, err)
			return nil, "", nil
		}
		// Return the content, but the path is the *local* path where it *should* be written.
		return content, localRulesPath, nil
	}

	// 5. No local or default rules found
	return nil, "", nil
}

// ExpandBraces recursively expands shell-style brace patterns.
// Example: "path/{a,b}/{c,d}" -> ["path/a/c", "path/a/d", "path/b/c", "path/b/d"]
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

// parseSearchDirective parses a line for search directives (@find: or @grep:)
// Returns: basePattern, directive, query, hasDirective
// Example: "pkg/**/*.go @find: \"manager\"" -> "pkg/**/*.go", "find", "manager", true
func parseSearchDirective(line string) (basePattern, directive, query string, hasDirective bool) {
	// Look for @find: or @grep: followed by a quoted string
	findIdx := strings.Index(line, " @find: ")
	grepIdx := strings.Index(line, " @grep: ")

	var directiveIdx int
	var directiveName string

	if findIdx != -1 && (grepIdx == -1 || findIdx < grepIdx) {
		directiveIdx = findIdx
		directiveName = "find"
	} else if grepIdx != -1 {
		directiveIdx = grepIdx
		directiveName = "grep"
	} else {
		// No directive found
		return line, "", "", false
	}

	// Extract the base pattern (everything before the directive)
	basePattern = strings.TrimSpace(line[:directiveIdx])

	// Extract the query (everything after the directive keyword and colon)
	queryPart := strings.TrimSpace(line[directiveIdx+len(" @"+directiveName+": "):])

	// The query should be in quotes
	if len(queryPart) >= 2 && queryPart[0] == '"' {
		// Find the closing quote
		endQuote := strings.Index(queryPart[1:], "\"")
		if endQuote != -1 {
			query = queryPart[1 : endQuote+1]
			return basePattern, directiveName, query, true
		}
	}

	// If we couldn't parse the query properly, treat it as no directive
	return line, "", "", false
}

// parseRulesFileContent parses the rules file content and extracts patterns, directives, and default paths.
// It does not handle recursion for imports or defaults.
func (m *Manager) parseRulesFileContent(rulesContent []byte) (*parsedRules, error) {
	results := &parsedRules{}
	if len(rulesContent) == 0 {
		return results, nil
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
	var globalDirective string
	var globalQuery string
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
		// Handle global @find: directive
		if strings.HasPrefix(line, "@find:") {
			queryPart := strings.TrimSpace(strings.TrimPrefix(line, "@find:"))
			// Parse the quoted query
			if len(queryPart) >= 2 && queryPart[0] == '"' {
				endQuote := strings.Index(queryPart[1:], "\"")
				if endQuote != -1 {
					globalDirective = "find"
					globalQuery = queryPart[1 : endQuote+1]
				}
			}
			continue
		}
		// Handle global @grep: directive
		if strings.HasPrefix(line, "@grep:") {
			queryPart := strings.TrimSpace(strings.TrimPrefix(line, "@grep:"))
			// Parse the quoted query
			if len(queryPart) >= 2 && queryPart[0] == '"' {
				endQuote := strings.Index(queryPart[1:], "\"")
				if endQuote != -1 {
					globalDirective = "grep"
					globalQuery = queryPart[1 : endQuote+1]
				}
			}
			continue
		}
		if line == "---" {
			inColdSection = true
			continue
		}
		if line != "" && !strings.HasPrefix(line, "#") {
			// Check for ruleset imports first (using :: delimiter)
			// Format: @alias:project-name::ruleset-name or @a:project-name::ruleset-name
			isRuleSetAlias := false
			if strings.HasPrefix(line, "@alias:") || strings.HasPrefix(line, "@a:") {
				// Extract the alias part
				aliasPart := line
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
								ImportIdentifier: importIdentifier,
								LineNum:          lineNum,
							})
						} else {
							results.mainImportedRuleSets = append(results.mainImportedRuleSets, ImportInfo{
								ImportIdentifier: importIdentifier,
								LineNum:          lineNum,
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
			tempLineForCheck := line
			if strings.HasPrefix(tempLineForCheck, "!") {
				tempLineForCheck = strings.TrimSpace(strings.TrimPrefix(tempLineForCheck, "!"))
			}
			if strings.HasPrefix(tempLineForCheck, "@a:git:") || strings.HasPrefix(tempLineForCheck, "@alias:git:") {
				isGitAlias = true
			}

			// Resolve alias if present (supports both @alias: and @a:), before further processing
			processedLine := line
			if isGitAlias {
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
					processedLine = "!" + githubURL
				} else {
					processedLine = githubURL
				}
			} else if resolver != nil && (strings.Contains(line, "@alias:") || strings.Contains(line, "@a:")) {
				resolvedLine, resolveErr := resolver.ResolveLine(line)
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
				basePathForValidation, _, _, _ := parseSearchDirective(pathToValidate)
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
				baseForCheck, _, _, _ := parseSearchDirective(processedLine)
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

				if isGitURL, repoURL, version := m.ParseGitRule(cleanLine); isGitURL {
					// Clone/update the repository
					localPath, _, cloneErr := repoManager.Ensure(repoURL, version)
					if cloneErr != nil {
						fmt.Fprintf(os.Stderr, "Warning: could not clone repository %s: %v\n", repoURL, cloneErr)
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

			// Check for search directives before brace expansion
			basePattern, directive, query, hasDirective := parseSearchDirective(processedLine)

			// If no inline directive but there's a global directive, use that
			if !hasDirective && globalDirective != "" {
				directive = globalDirective
				query = globalQuery
				hasDirective = true
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
				if hasDirective {
					ruleInfo.Directive = directive
					ruleInfo.DirectiveQuery = query
				}

				if inColdSection {
					results.coldRules = append(results.coldRules, ruleInfo)
				} else {
					results.hotRules = append(results.hotRules, ruleInfo)
				}
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return results, nil
}

// findActiveRulesFile returns the path to the active rules file if it exists
func (m *Manager) findActiveRulesFile() string {
	// Check for new rules file location first
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
	// Check if source file exists
	if _, err := os.Stat(sourcePath); os.IsNotExist(err) {
		return fmt.Errorf("source rules file not found: %s", sourcePath)
	}

	// Read content from source
	content, err := os.ReadFile(sourcePath)
	if err != nil {
		return fmt.Errorf("error reading source rules file %s: %w", sourcePath, err)
	}

	// Ensure .grove directory exists
	groveDir := filepath.Join(m.workDir, GroveDir)
	if err := os.MkdirAll(groveDir, 0755); err != nil {
		return fmt.Errorf("error creating %s directory: %w", groveDir, err)
	}

	// Write to active rules file, overwriting if it exists
	activeRulesPath := filepath.Join(m.workDir, ActiveRulesFile)
	if err := os.WriteFile(activeRulesPath, content, 0644); err != nil {
		return fmt.Errorf("error writing active rules file: %w", err)
	}

	fmt.Printf("Set active rules from %s\n", sourcePath)
	return nil
}

// WriteRulesTo writes the current active rules to a specified file path
func (m *Manager) WriteRulesTo(destPath string) error {
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
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return fmt.Errorf("error creating destination directory: %w", err)
	}

	// Write to destination file, overwriting if it exists
	if err := os.WriteFile(destPath, content, 0644); err != nil {
		return fmt.Errorf("error writing to destination file: %w", err)
	}

	return nil
}

// AppendRule adds a rule to the active rules file in the specified context
// contextType can be "hot", "cold", or "exclude".
func (m *Manager) AppendRule(rulePath, contextType string) error {
	// Validate the rule safety before adding
	if err := m.validateRuleSafety(rulePath); err != nil {
		return fmt.Errorf("safety validation failed: %w", err)
	}

	// First, remove any existing rules for this path to prevent duplicates
	// This makes the function idempotent and handles state changes
	if err := m.RemoveRuleForPath(rulePath); err != nil {
		// Non-fatal error, log and continue
		fmt.Fprintf(os.Stderr, "Warning: could not remove existing rules: %v\n", err)
	}

	// Find or create the rules file
	rulesFilePath := m.findActiveRulesFile()
	if rulesFilePath == "" {
		// Create .grove/rules file
		groveDir := filepath.Join(m.workDir, GroveDir)
		if err := os.MkdirAll(groveDir, 0755); err != nil {
			return fmt.Errorf("error creating %s directory: %w", groveDir, err)
		}
		rulesFilePath = filepath.Join(m.workDir, ActiveRulesFile)
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
	return os.WriteFile(rulesFilePath, []byte(content), 0644)
}

// ToggleViewDirective adds or removes a `@view:` directive from the rules file.
func (m *Manager) ToggleViewDirective(path string) error {
	rulesFilePath := m.findActiveRulesFile()
	if rulesFilePath == "" {
		// Create .grove/rules file if it doesn't exist
		groveDir := filepath.Join(m.workDir, GroveDir)
		if err := os.MkdirAll(groveDir, 0755); err != nil {
			return fmt.Errorf("error creating %s directory: %w", groveDir, err)
		}
		rulesFilePath = filepath.Join(m.workDir, ActiveRulesFile)
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

	return os.WriteFile(rulesFilePath, []byte(newContent), 0644)
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

	return os.WriteFile(rulesFilePath, []byte(newContent), 0644)
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

	return os.WriteFile(rulesFilePath, []byte(newContent), 0644)
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
