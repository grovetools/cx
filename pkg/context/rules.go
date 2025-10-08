package context

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mattsolo1/grove-core/config"
	"github.com/mattsolo1/grove-core/pkg/repo"
)

// RuleStatus represents the current state of a rule
type RuleStatus int

const (
	RuleNotFound RuleStatus = iota // Rule doesn't exist
	RuleHot                        // Rule exists in hot context
	RuleCold                       // Rule exists in cold context
	RuleExcluded                   // Rule exists as exclusion
)

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

// LoadRulesContent finds and reads the active rules file, falling back to grove.yml defaults.
// It returns the content of the rules, the path of the file read (if any), and an error.
func (m *Manager) LoadRulesContent() (content []byte, path string, err error) {
	// 1. Look for local .grove/rules
	localRulesPath := filepath.Join(m.workDir, ActiveRulesFile)
	if _, err := os.Stat(localRulesPath); err == nil {
		content, err := os.ReadFile(localRulesPath)
		if err != nil {
			return nil, "", fmt.Errorf("reading local rules file %s: %w", localRulesPath, err)
		}
		return content, localRulesPath, nil
	}

	// 2. If not found, look for legacy .grovectx
	legacyRulesPath := filepath.Join(m.workDir, RulesFile)
	if _, err := os.Stat(legacyRulesPath); err == nil {
		content, err := os.ReadFile(legacyRulesPath)
		if err != nil {
			return nil, "", fmt.Errorf("reading legacy rules file %s: %w", legacyRulesPath, err)
		}
		return content, legacyRulesPath, nil
	}

	// 3. If not found, check grove.yml for a default
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

	// 4. No local or default rules found
	return nil, "", nil
}

// expandBraces recursively expands shell-style brace patterns.
// Example: "path/{a,b}/{c,d}" -> ["path/a/c", "path/a/d", "path/b/c", "path/b/d"]
func expandBraces(pattern string) []string {
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
		expandedSuffixes := expandBraces(prefix + option + suffix)
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

// parseRulesFile parses the rules file content and extracts patterns, directives, and default paths
func (m *Manager) parseRulesFile(rulesContent []byte) (mainPatterns, coldPatterns, mainDefaultPaths, coldDefaultPaths, viewPaths []string, freezeCache, disableExpiration, disableCache bool, expireTime time.Duration, err error) {
	if len(rulesContent) == 0 {
		return nil, nil, nil, nil, nil, false, false, false, 0, nil
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

	scanner := bufio.NewScanner(bytes.NewReader(rulesContent))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "@freeze-cache" {
			freezeCache = true
			continue
		}
		if line == "@no-expire" {
			disableExpiration = true
			continue
		}
		if line == "@disable-cache" {
			disableCache = true
			continue
		}
		if strings.HasPrefix(line, "@expire-time ") {
			// Parse the duration argument
			durationStr := strings.TrimSpace(strings.TrimPrefix(line, "@expire-time "))
			if durationStr != "" {
				parsedDuration, parseErr := time.ParseDuration(durationStr)
				if parseErr != nil {
					return nil, nil, nil, nil, nil, false, false, false, 0, fmt.Errorf("invalid duration format for @expire-time: %w", parseErr)
				}
				expireTime = parsedDuration
			}
			continue
		}
		// Support both @view: and @v: (short form)
		if strings.HasPrefix(line, "@view:") || strings.HasPrefix(line, "@v:") {
			// Normalize to @view: for processing
			normalizedLine := line
			if strings.HasPrefix(line, "@v:") {
				normalizedLine = "@view:" + strings.TrimPrefix(line, "@v:")
			}

			path := strings.TrimSpace(strings.TrimPrefix(normalizedLine, "@view:"))
			if path != "" {
				// Check if the value itself is an alias directive (with or without spacing)
				if resolver != nil && (strings.HasPrefix(path, "@alias:") || strings.HasPrefix(path, "@a:")) {
					// The value of @view is an alias. Resolve it by calling ResolveLine on just the alias part
					resolvedPath, resolveErr := resolver.ResolveLine(path)
					if resolveErr != nil {
						fmt.Fprintf(os.Stderr, "Warning: could not resolve chained alias in line '%s': %v\n", line, resolveErr)
						continue // Skip this line if alias resolution fails
					}
					// Use the resolved path
					path = strings.TrimSpace(resolvedPath)
				}
				viewPaths = append(viewPaths, path)
			}
			continue
		}
		if strings.HasPrefix(line, "@default:") {
			path := strings.TrimSpace(strings.TrimPrefix(line, "@default:"))
			if path != "" {
				if inColdSection {
					coldDefaultPaths = append(coldDefaultPaths, path)
				} else {
					mainDefaultPaths = append(mainDefaultPaths, path)
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
			// Resolve alias if present (supports both @alias: and @a:), before further processing
			processedLine := line
			if resolver != nil && (strings.Contains(line, "@alias:") || strings.Contains(line, "@a:")) {
				resolvedLine, resolveErr := resolver.ResolveLine(line)
				if resolveErr != nil {
					fmt.Fprintf(os.Stderr, "Warning: could not resolve alias in line '%s': %v\n", line, resolveErr)
					continue // Skip this line if alias resolution fails
				}
				processedLine = resolvedLine

				// If the resolved line is just a directory path (no glob pattern),
				// append /** to match all files in that directory
				// Check the base pattern part before any directives
				baseForCheck, _, _, _ := parseSearchDirective(processedLine)
				if !strings.Contains(baseForCheck, "*") && !strings.Contains(baseForCheck, "?") {
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
			}

			// Process Git URLs
			if repoManager != nil {
				isExclude := strings.HasPrefix(processedLine, "!")
				cleanLine := processedLine
				if isExclude {
					cleanLine = strings.TrimPrefix(processedLine, "!")
				}

				if isGitURL, repoURL, version := m.parseGitRule(cleanLine); isGitURL {
					// Clone/update the repository
					localPath, _, cloneErr := repoManager.Ensure(repoURL, version)
					if cloneErr != nil {
						fmt.Fprintf(os.Stderr, "Warning: could not clone repository %s: %v\n", repoURL, cloneErr)
						continue
					}

					// Replace the Git URL with the local path pattern
					processedLine = localPath + "/**"
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
			expandedPatterns := expandBraces(basePattern)

			// Add each expanded pattern to the appropriate list
			// If there's a directive, encode it in the pattern using a special marker
			for _, expandedPattern := range expandedPatterns {
				finalPattern := expandedPattern
				if hasDirective {
					// Encode directive info: pattern|||directive|||query
					// We use ||| as a separator since it's unlikely to appear in file paths
					finalPattern = expandedPattern + "|||" + directive + "|||" + query
				}

				if inColdSection {
					coldPatterns = append(coldPatterns, finalPattern)
				} else {
					mainPatterns = append(mainPatterns, finalPattern)
				}
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, nil, nil, nil, nil, false, false, false, 0, err
	}
	return mainPatterns, coldPatterns, mainDefaultPaths, coldDefaultPaths, viewPaths, freezeCache, disableExpiration, disableCache, expireTime, nil
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
	_, _, _, _, _, freezeCache, _, _, _, err := m.parseRulesFile(rulesContent)
	if err != nil {
		return false, fmt.Errorf("error parsing rules file for cache directive: %w", err)
	}

	return freezeCache, nil
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
	_, _, _, _, _, _, disableExpiration, _, _, err := m.parseRulesFile(rulesContent)
	if err != nil {
		return false, fmt.Errorf("error parsing rules file for cache directive: %w", err)
	}

	return disableExpiration, nil
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
	_, _, _, _, _, _, _, _, expireTime, err := m.parseRulesFile(rulesContent)
	if err != nil {
		return 0, fmt.Errorf("error parsing rules file for expire time: %w", err)
	}

	return expireTime, nil
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
	_, _, _, _, _, _, _, disableCache, _, err := m.parseRulesFile(rulesContent)
	if err != nil {
		return false, fmt.Errorf("error parsing rules file for cache directive: %w", err)
	}

	return disableCache, nil
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
