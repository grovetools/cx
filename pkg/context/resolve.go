package context

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/grovetools/core/config"
	"github.com/grovetools/core/logging"
	"github.com/grovetools/core/pkg/daemon"
	"github.com/grovetools/core/pkg/models"
	"github.com/grovetools/core/pkg/profiling"
	"github.com/grovetools/core/util/pathutil"
)

var log = logging.NewLogger("cx.context.resolve")

// IsRelativeExternalPath checks if a pattern refers to a path outside the current directory.
func IsRelativeExternalPath(pattern string) bool {
	normPattern := filepath.ToSlash(filepath.Clean(pattern))
	return normPattern == ".." || strings.HasPrefix(normPattern, "../")
}

// patternInfo holds information about a pattern including any associated directives

// expandAllRules recursively resolves rules, including those from @default directives.
func (m *Manager) expandAllRules(rulesPath string, visited map[string]bool, importLineNum int) (hotRules, coldRules []RuleInfo, viewPaths []string, treePaths []string, err error) {
	defer profiling.Start("context.expandAllRules").Stop()
	// Resolve relative paths against workDir, not process CWD.
	if !filepath.IsAbs(rulesPath) {
		rulesPath = filepath.Join(m.workDir, rulesPath)
	}
	absRulesPath, err := filepath.Abs(rulesPath)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("failed to get absolute path for rules: %w", err)
	}

	if visited[absRulesPath] {
		// Circular dependency detected, return to prevent infinite loop.
		return nil, nil, nil, nil, nil
	}
	visited[absRulesPath] = true

	rulesContent, err := os.ReadFile(absRulesPath)
	if err != nil {
		if os.IsNotExist(err) {
			// If a default rules file doesn't exist, it's not an error, just return empty.
			return nil, nil, nil, nil, nil
		}
		return nil, nil, nil, nil, fmt.Errorf("reading rules file %s: %w", absRulesPath, err)
	}

	parsed, err := m.parseRulesFileContent(rulesContent)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("parsing rules file %s: %w", absRulesPath, err)
	}

	localHot := parsed.hotRules
	localCold := parsed.coldRules
	mainDefaults := parsed.mainDefaultPaths
	coldDefaults := parsed.coldDefaultPaths
	mainImports := parsed.mainImportedRuleSets
	coldImports := parsed.coldImportedRuleSets
	localView := parsed.viewPaths
	localTree := parsed.treePaths
	rulesDir := filepath.Dir(absRulesPath)

	// Process @include: directives before local rules so local rules can override them
	for _, includeInfo := range parsed.mainIncludes {
		includedHot, includedCold, includedView, includeErr := m.resolveInclude(includeInfo, rulesDir, visited)
		if includeErr != nil {
			fmt.Fprintf(os.Stderr, "Warning: could not resolve included ruleset '%s': %v\n", includeInfo.ImportIdentifier, includeErr)
			continue
		}
		hotRules = append(hotRules, includedHot...)
		coldRules = append(coldRules, includedCold...)
		viewPaths = append(viewPaths, includedView...)
	}

	for _, includeInfo := range parsed.coldIncludes {
		includedHot, includedCold, includedView, includeErr := m.resolveInclude(includeInfo, rulesDir, visited)
		if includeErr != nil {
			fmt.Fprintf(os.Stderr, "Warning: could not resolve included ruleset '%s': %v\n", includeInfo.ImportIdentifier, includeErr)
			continue
		}
		// For cold includes, all nested rules go to cold
		allNested := append(includedHot, includedCold...)
		if len(includeInfo.Directives) > 0 {
			for i := range allNested {
				if len(allNested[i].Directives) == 0 {
					allNested[i].Directives = includeInfo.Directives
				}
			}
		}
		coldRules = append(coldRules, allNested...)
		viewPaths = append(viewPaths, includedView...)
	}

	for i := range localHot {
		localHot[i].EffectiveLineNum = localHot[i].LineNum
	}
	for i := range localCold {
		localCold[i].EffectiveLineNum = localCold[i].LineNum
	}

	hotRules = append(hotRules, localHot...)
	coldRules = append(coldRules, localCold...)
	viewPaths = append(viewPaths, localView...)
	treePaths = append(treePaths, localTree...)

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

			// route through daemon RPC so cancellation propagates to git and clones are single-flighted across cx invocations.
			client := daemon.NewWithAutoStart(m.workDir)
			resp, err := client.EnsureRepo(m.Context(), models.RepoEnsureRequest{URL: repoURL, Version: version})
			if err != nil {
				m.addSkippedRule(importInfo.LineNum, importInfo.OriginalLine, fmt.Sprintf("invalid git ref: %v", err))
				continue
			}
			localPath := resp.WorktreePath

			// Find the ruleset file within the cloned repository's .cx directories
			// Use localPath (the worktree) instead of barePath because the ruleset files
			// are in the checked-out working tree, not the bare repository
			rulesFilePath, err := FindRulesetFileStandalone(localPath, rulesetName)
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

			nestedHot, nestedCold, nestedView, nestedTree, err := m.expandAllRules(rulesFilePath, visited, importInfo.LineNum)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Warning: could not resolve ruleset '%s' from repository %s: %v\n", rulesetName, repoURL, err)
				continue
			}

			// Propagate directives from import to nested rules if they don't have any
			if len(importInfo.Directives) > 0 {
				for i := range nestedHot {
					if len(nestedHot[i].Directives) == 0 {
						nestedHot[i].Directives = importInfo.Directives
					}
				}
				for i := range nestedCold {
					if len(nestedCold[i].Directives) == 0 {
						nestedCold[i].Directives = importInfo.Directives
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
			for i, path := range nestedTree {
				if !filepath.IsAbs(path) {
					nestedTree[i] = filepath.Join(localPath, path)
				}
			}
			treePaths = append(treePaths, nestedTree...)

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

		// Find the ruleset file (notebook presets, .cx.work/, .cx/)
		rulesFilePath, err := m.FindRulesetFile(projectPath, rulesetName)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: could not find ruleset '%s' from project '%s': %v\n", rulesetName, projectAlias, err)
			continue
		}

		nestedHot, nestedCold, nestedView, nestedTree, err := m.expandAllRules(rulesFilePath, visited, importInfo.LineNum)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: could not resolve ruleset '%s' from project '%s': %v\n", rulesetName, projectAlias, err)
			continue
		}

		// Propagate directives from import to nested rules if they don't have any
		if len(importInfo.Directives) > 0 {
			for i := range nestedHot {
				if len(nestedHot[i].Directives) == 0 {
					nestedHot[i].Directives = importInfo.Directives
				}
			}
			for i := range nestedCold {
				if len(nestedCold[i].Directives) == 0 {
					nestedCold[i].Directives = importInfo.Directives
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
		for i, path := range nestedTree {
			if !filepath.IsAbs(path) {
				nestedTree[i] = filepath.Join(projectPath, path)
			}
		}
		treePaths = append(treePaths, nestedTree...)
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

			client := daemon.NewWithAutoStart(m.workDir)
			resp, err := client.EnsureRepo(m.Context(), models.RepoEnsureRequest{URL: repoURL, Version: version})
			if err != nil {
				m.addSkippedRule(importInfo.LineNum, importInfo.OriginalLine, fmt.Sprintf("invalid git ref: %v", err))
				continue
			}
			localPath := resp.WorktreePath

			// Find the ruleset file within the cloned repository's .cx directories
			// Use localPath (the worktree) instead of barePath because the ruleset files
			// are in the checked-out working tree, not the bare repository
			rulesFilePath, err := FindRulesetFileStandalone(localPath, rulesetName)
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

			nestedHot, nestedCold, nestedView, nestedTree, err := m.expandAllRules(rulesFilePath, visited, importInfo.LineNum)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Warning: could not resolve ruleset '%s' from repository %s: %v\n", rulesetName, repoURL, err)
				continue
			}

			// For cold imports, everything from the imported ruleset goes into the cold section
			allNestedRules := append(nestedHot, nestedCold...)

			// Propagate directives from import to all nested rules
			if len(importInfo.Directives) > 0 {
				for i := range allNestedRules {
					if len(allNestedRules[i].Directives) == 0 {
						allNestedRules[i].Directives = importInfo.Directives
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
			for i, path := range nestedTree {
				if !filepath.IsAbs(path) {
					nestedTree[i] = filepath.Join(localPath, path)
				}
			}
			treePaths = append(treePaths, nestedTree...)
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

		// Find the ruleset file (notebook presets, .cx.work/, .cx/)
		rulesFilePath, err := m.FindRulesetFile(projectPath, rulesetName)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: could not find ruleset '%s' from project '%s': %v\n", rulesetName, projectAlias, err)
			continue
		}

		nestedHot, nestedCold, nestedView, nestedTree, err := m.expandAllRules(rulesFilePath, visited, importInfo.LineNum)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: could not resolve ruleset '%s' from project '%s': %v\n", rulesetName, projectAlias, err)
			continue
		}

		allNestedRules := append(nestedHot, nestedCold...)

		// Propagate directives from import to all nested rules
		if len(importInfo.Directives) > 0 {
			for i := range allNestedRules {
				if len(allNestedRules[i].Directives) == 0 {
					allNestedRules[i].Directives = importInfo.Directives
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
		for i, path := range nestedTree {
			if !filepath.IsAbs(path) {
				nestedTree[i] = filepath.Join(projectPath, path)
			}
		}
		treePaths = append(treePaths, nestedTree...)
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

		// Load the config from the grove config file in that directory
		configFile, err := config.FindConfigFile(realPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: no grove config found at %s for @default path %s\n", realPath, defaultPath)
			continue
		}

		cfg, err := config.Load(configFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: could not load config for @default path %s (file: %s): %v\n", defaultPath, configFile, err)
			continue
		}

		// Read context config from the explicit Config.Context field
		var defaultRules, defaultRulesPath string
		if cfg.Context != nil {
			defaultRules = cfg.Context.DefaultRules
			defaultRulesPath = cfg.Context.DefaultRulesPath
		}

		var defaultRulesFile string
		if defaultRules != "" {
			if resolved, findErr := m.FindRulesetFile(realPath, defaultRules); findErr == nil {
				defaultRulesFile = resolved
			} else {
				fmt.Fprintf(os.Stderr, "Warning: could not find default_rules preset '%s' for @default path %s\n", defaultRules, defaultPath)
				continue
			}
		} else if defaultRulesPath != "" {
			defaultRulesFile = filepath.Join(realPath, defaultRulesPath)
		} else {
			fmt.Fprintf(os.Stderr, "Warning: no default_rules or default_rules_path found for @default path %s\n", defaultPath)
			continue
		}

		// Recursively resolve patterns from the default rules file
		// ALL patterns from the default (hot and cold) are added to the current HOT context.
		nestedHot, nestedCold, nestedView, nestedTree, err := m.expandAllRules(defaultRulesFile, visited, 0)
		if err != nil {
			return nil, nil, nil, nil, err
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
		for i, path := range nestedTree {
			if !filepath.IsAbs(path) {
				nestedTree[i] = filepath.Join(realPath, path)
			}
		}
		treePaths = append(treePaths, nestedTree...)
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

		// Load the config from the grove config file in that directory
		configFile, err := config.FindConfigFile(realPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: no grove config found at %s for @default path %s\n", realPath, defaultPath)
			continue
		}

		cfg, err := config.Load(configFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: could not load config for @default path %s (file: %s): %v\n", defaultPath, configFile, err)
			continue
		}

		// Read context config from the explicit Config.Context field
		var defaultRules, defaultRulesPath string
		if cfg.Context != nil {
			defaultRules = cfg.Context.DefaultRules
			defaultRulesPath = cfg.Context.DefaultRulesPath
		}

		var defaultRulesFile string
		if defaultRules != "" {
			if resolved, findErr := m.FindRulesetFile(realPath, defaultRules); findErr == nil {
				defaultRulesFile = resolved
			} else {
				fmt.Fprintf(os.Stderr, "Warning: could not find default_rules preset '%s' for @default path %s\n", defaultRules, defaultPath)
				continue
			}
		} else if defaultRulesPath != "" {
			defaultRulesFile = filepath.Join(realPath, defaultRulesPath)
		} else {
			fmt.Fprintf(os.Stderr, "Warning: no default_rules or default_rules_path found for @default path %s\n", defaultPath)
			continue
		}

		// Recursively resolve patterns from the default rules file
		// ALL patterns from the default are added to the current COLD context.
		nestedHot, nestedCold, nestedView, nestedTree, err := m.expandAllRules(defaultRulesFile, visited, 0)
		if err != nil {
			return nil, nil, nil, nil, err
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
		for i, path := range nestedTree {
			if !filepath.IsAbs(path) {
				nestedTree[i] = filepath.Join(realPath, path)
			}
		}
		treePaths = append(treePaths, nestedTree...)
	}

	// When called as a nested import, all rules must be attributed to the
	// parent import's line number so the top-level AST resolver sees a
	// consistent EffectiveLineNum for the entire import.
	if importLineNum > 0 {
		for i := range hotRules {
			hotRules[i].EffectiveLineNum = importLineNum
		}
		for i := range coldRules {
			coldRules[i].EffectiveLineNum = importLineNum
		}
	}

	return hotRules, coldRules, viewPaths, treePaths, nil
}

// resolveInclude resolves a single @include: directive to its constituent rules.
// It locates the named ruleset file and recursively expands it.
func (m *Manager) resolveInclude(includeInfo ImportInfo, rulesDir string, visited map[string]bool) (hotRules, coldRules []RuleInfo, viewPaths []string, err error) {
	includeName := includeInfo.ImportIdentifier
	var rulesFilePath string

	if strings.Contains(includeName, "/") || strings.HasSuffix(includeName, RulesExt) {
		// Treat as a path (relative or absolute)
		rulesFilePath = includeName
		if !filepath.IsAbs(rulesFilePath) {
			rulesFilePath = filepath.Join(rulesDir, rulesFilePath)
		}
	} else {
		// Treat as a named preset — resolve via FindRulesetFile using the
		// logical workspace context, not the physical rules file location.
		resolvedPath, findErr := m.FindRulesetFile(m.rulesBaseDir, includeName)
		if findErr != nil {
			return nil, nil, nil, fmt.Errorf("could not find included ruleset '%s': %w", includeName, findErr)
		}
		rulesFilePath = resolvedPath
	}

	nestedHot, nestedCold, nestedView, _, err := m.expandAllRules(rulesFilePath, visited, includeInfo.LineNum)
	if err != nil {
		return nil, nil, nil, err
	}

	// Propagate search directives from the include line to nested rules that don't have their own
	if len(includeInfo.Directives) > 0 {
		for i := range nestedHot {
			if len(nestedHot[i].Directives) == 0 {
				nestedHot[i].Directives = includeInfo.Directives
			}
		}
		for i := range nestedCold {
			if len(nestedCold[i].Directives) == 0 {
				nestedCold[i].Directives = includeInfo.Directives
			}
		}
	}

	return nestedHot, nestedCold, nestedView, nil
}

// ResolveFilesFromRules dynamically resolves the list of files from the active rules file
func (m *Manager) ResolveFilesFromRules() ([]string, error) {
	files, _, err := m.ResolveFilesAndTreesFromRules()
	return files, err
}

// ResolveFilesAndTreesFromRules dynamically resolves the list of files and tree paths from the active rules file
func (m *Manager) ResolveFilesAndTreesFromRules() ([]string, []string, error) {
	defer profiling.Start("context.ResolveFilesAndTreesFromRules").Stop()
	// Load the active rules content (respects state-based rules)
	rulesContent, activeRulesFile, err := m.LoadRulesContent()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to load rules: %w", err)
	}
	if rulesContent == nil || activeRulesFile == "" {
		// No active or default rules found
		return []string{}, nil, nil
	}

	// Resolve all patterns recursively from the active rules file
	hotRules, coldRules, _, treePaths, err := m.expandAllRules(activeRulesFile, make(map[string]bool), 0)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to resolve patterns: %w", err)
	}

	hotFiles, err := m.resolveFilesViaAST(hotRules)
	if err != nil {
		return nil, nil, fmt.Errorf("error resolving hot context files: %w", err)
	}

	if len(coldRules) > 0 {
		coldFiles, err := m.resolveFilesViaAST(coldRules)
		if err != nil {
			return nil, nil, fmt.Errorf("error resolving cold context files: %w", err)
		}

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

		return finalHotFiles, deduplicateStrings(treePaths), nil
	}

	return hotFiles, deduplicateStrings(treePaths), nil
}

// deduplicateStrings returns a new slice with duplicate entries removed, preserving order.
func deduplicateStrings(items []string) []string {
	if len(items) == 0 {
		return items
	}
	seen := make(map[string]bool, len(items))
	result := make([]string, 0, len(items))
	for _, item := range items {
		if !seen[item] {
			seen[item] = true
			result = append(result, item)
		}
	}
	return result
}

// ResolveFilesFromCustomRulesFile resolves both hot and cold files from a custom rules file path.
func (m *Manager) ResolveFilesFromCustomRulesFile(rulesFilePath string) (hotFiles []string, coldFiles []string, err error) {
	// Resolve relative paths against workDir, not process CWD.
	if !filepath.IsAbs(rulesFilePath) {
		rulesFilePath = filepath.Join(m.workDir, rulesFilePath)
	}
	absRulesFilePath, err := filepath.Abs(rulesFilePath)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get absolute path for rules file: %w", err)
	}

	// Check if the rules file exists
	if _, err := os.Stat(absRulesFilePath); os.IsNotExist(err) {
		return nil, nil, fmt.Errorf("rules file not found: %s", absRulesFilePath)
	}

	// Resolve all patterns recursively from the custom rules file
	hotRules, coldRules, _, _, err := m.expandAllRules(absRulesFilePath, make(map[string]bool), 0)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to resolve patterns from rules file: %w", err)
	}

	hotFiles, err = m.resolveFilesViaAST(hotRules)
	if err != nil {
		return nil, nil, fmt.Errorf("error resolving hot context files: %w", err)
	}

	if len(coldRules) > 0 {
		coldFiles, err = m.resolveFilesViaAST(coldRules)
		if err != nil {
			return nil, nil, fmt.Errorf("error resolving cold context files: %w", err)
		}

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
	_, coldRules, _, _, err := m.expandAllRules(activeRulesFile, make(map[string]bool), 0)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve patterns for cold context: %w", err)
	}

	coldFiles, err := m.resolveFilesViaAST(coldRules)
	if err != nil {
		return nil, fmt.Errorf("error resolving cold context files: %w", err)
	}

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
				checkPath = filepath.Join(m.rulesBaseDir, cleanPattern)
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

// decodeDirectives extracts directives information from an encoded pattern
// Returns: cleanPattern, directives, hasDirectives
func decodeDirectives(pattern string) (string, []SearchDirective, bool) {
	parts := strings.Split(pattern, "|||")
	if len(parts) >= 3 && len(parts)%2 == 1 {
		basePattern := parts[0]
		var directives []SearchDirective
		for i := 1; i < len(parts); i += 2 {
			directives = append(directives, SearchDirective{Name: parts[i], Query: parts[i+1]})
		}
		return basePattern, directives, true
	}
	return pattern, nil, false
}

// matchDirective checks if a single file matches a directive filter.
// For "find", it checks if the path contains the query string.
// For "grep", it reads the file and checks if the content matches the query as a regex (or literal fallback).
func (m *Manager) matchDirective(file, directive, query string) bool {
	// Handle inverted directives (@find!:, @grep!:) by stripping the ! and inverting result
	if strings.HasSuffix(directive, "!") {
		return !m.matchDirective(file, strings.TrimSuffix(directive, "!"), query)
	}
	if directive == "find" {
		// @find: filter by filename/path using substring, glob, or regex
		// 1. Substring match (original behavior)
		if strings.Contains(file, query) {
			return true
		}
		// 2. Basename glob match
		if globMatch, _ := filepath.Match(query, filepath.Base(file)); globMatch {
			return true
		}
		// 3. Full path/recursive glob match
		if matchDoubleStarPattern(query, file) {
			return true
		}
		// 4. Regex match
		if re, err := regexp.Compile(query); err == nil && re.MatchString(filepath.ToSlash(file)) {
			return true
		}
		return false
	}
	if directive == "changed" {
		// @changed: filter files to only those in the git changed set
		changedMap, err := m.getChangedFilesCached(query)
		if err != nil {
			return false
		}
		relPath, relErr := filepath.Rel(m.workDir, file)
		if relErr != nil {
			relPath = file
		}
		relPath = filepath.ToSlash(relPath)
		return changedMap[relPath]
	}
	if directive == "grep" || directive == "grep-i" {
		filePath := file
		if !filepath.IsAbs(file) {
			filePath = filepath.Join(m.rulesBaseDir, file)
		}
		content, err := os.ReadFile(filePath)
		if err != nil {
			return false
		}
		caseInsensitive := directive == "grep-i"
		if caseInsensitive {
			compiled, err := regexp.Compile("(?i)" + query)
			if err != nil {
				return bytes.Contains(bytes.ToLower(content), []byte(strings.ToLower(query)))
			}
			return compiled.Match(content)
		}
		compiled, err := regexp.Compile(query)
		if err != nil {
			return strings.Contains(string(content), query)
		}
		return compiled.Match(content)
	}
	if directive == "recent" {
		// @recent: filter by modification time
		duration, err := parseExtendedDuration(query)
		if err != nil {
			return false
		}
		cutoff := time.Now().Add(-duration)
		filePath := file
		if !filepath.IsAbs(file) {
			filePath = filepath.Join(m.rulesBaseDir, file)
		}
		stat, err := os.Stat(filePath)
		return err == nil && stat.ModTime().After(cutoff)
	}
	return false
}

// parseExtendedDuration parses a duration string, adding support for 'd' (days) and 'w' (weeks)
// on top of Go's standard time.ParseDuration units.
func parseExtendedDuration(s string) (time.Duration, error) {
	s = strings.TrimSpace(s)
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		s = s[1 : len(s)-1]
	}
	if s == "" {
		return 0, fmt.Errorf("empty duration string")
	}
	if strings.HasSuffix(s, "d") {
		days, err := strconv.ParseFloat(s[:len(s)-1], 64)
		if err != nil {
			return 0, fmt.Errorf("invalid days value: %w", err)
		}
		return time.Duration(days * 24 * float64(time.Hour)), nil
	}
	if strings.HasSuffix(s, "w") {
		weeks, err := strconv.ParseFloat(s[:len(s)-1], 64)
		if err != nil {
			return 0, fmt.Errorf("invalid weeks value: %w", err)
		}
		return time.Duration(weeks * 7 * 24 * float64(time.Hour)), nil
	}
	return time.ParseDuration(s)
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
