package cmd

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mattsolo1/grove-context/pkg/context"
	"github.com/spf13/cobra"
)

// resolveWithContext resolves a pattern in the context of a rules file.
// It reads the rules file up to the given line number, builds the set of files
// that would be included by earlier rules, then applies the pattern against those files.
func resolveWithContext(mgr *context.Manager, rulesFile string, lineNumber int, pattern string) ([]string, error) {
	// Read the rules file up to the specified line
	file, err := os.Open(rulesFile)
	if err != nil {
		return nil, fmt.Errorf("failed to open rules file: %w", err)
	}
	defer file.Close()

	var rulesBeforeLine []string
	scanner := bufio.NewScanner(file)
	currentLine := 0
	for scanner.Scan() {
		currentLine++
		if currentLine >= lineNumber {
			break
		}
		rulesBeforeLine = append(rulesBeforeLine, scanner.Text())
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("failed to read rules file: %w", err)
	}

	// Build rules content from lines before the current line
	rulesContent := strings.Join(rulesBeforeLine, "\n")

	// Use attribution to get all potential files from earlier rules
	attribution, _, _, _, err := mgr.ResolveFilesWithAttribution(rulesContent)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve context: %w", err)
	}

	// Collect all files that were included by earlier rules
	var potentialFiles []string
	for _, files := range attribution {
		potentialFiles = append(potentialFiles, files...)
	}

	// Now apply the pattern against these files
	var matchedFiles []string
	workDir, _ := filepath.Abs(".")
	for _, file := range potentialFiles {
		// Get path relative to workDir for matching
		relPath, err := filepath.Rel(workDir, file)
		if err != nil {
			relPath = file
		}
		relPath = filepath.ToSlash(relPath)

		// Floating inclusion patterns should only apply to local files.
		isFloatingInclusion := !strings.HasPrefix(pattern, "!") && !strings.Contains(pattern, "/")
		isExternalFile := strings.HasPrefix(relPath, "..")
		if isFloatingInclusion && isExternalFile {
			continue // Skip this match.
		}

		// Determine which path to match against
		pathToMatch := relPath
		if filepath.IsAbs(pattern) {
			pathToMatch = filepath.ToSlash(file)
		}

		// Use simple pattern matching logic
		if matchPattern(pattern, pathToMatch) {
			matchedFiles = append(matchedFiles, file)
		}
	}

	return matchedFiles, nil
}

// matchPattern matches a file path against a pattern using gitignore-style matching
func matchPattern(pattern, relPath string) bool {
	// Normalize for case-insensitive filesystems (macOS/Windows)
	normalizedPattern := strings.ToLower(pattern)
	normalizedPath := strings.ToLower(relPath)

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

func NewResolveCmd() *cobra.Command {
	var rulesFile string
	var lineNumber int

	cmd := &cobra.Command{
		Use:   "resolve [rule]",
		Short: "Resolve a single rule pattern to a list of files",
		Long:  `Accepts a single inclusion rule (glob or alias) and prints the list of files it resolves to. Primarily for use by editor integrations.`,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ruleLine := args[0]

			// Strip @view: prefix to allow Neovim's <leader>f? to work on view rules
			trimmedLine := strings.TrimSpace(ruleLine)
			if strings.HasPrefix(trimmedLine, "@view:") {
				trimmedLine = strings.TrimSpace(strings.TrimPrefix(trimmedLine, "@view:"))
			} else if strings.HasPrefix(trimmedLine, "@v:") {
				trimmedLine = strings.TrimSpace(strings.TrimPrefix(trimmedLine, "@v:"))
			}

			// Do not process exclusion rules, as they don't resolve to a file list on their own.
			if strings.HasPrefix(strings.TrimSpace(trimmedLine), "!") {
				// Print nothing and exit successfully.
				return nil
			}

			mgr := context.NewManager(".")

			var files []string
			var err error

			// If rules file and line number are provided, use context-aware resolution
			if rulesFile != "" && lineNumber > 0 {
				files, err = resolveWithContext(mgr, rulesFile, lineNumber, trimmedLine)
				if err != nil {
					return fmt.Errorf("error resolving with context: %w", err)
				}
			} else {
				// Original behavior: resolve pattern in isolation
				// First, resolve the line. This one call now handles simple globs, aliases,
				// and ruleset imports, returning a potentially multi-line string of patterns.
				resolvedPatternsStr, err := mgr.ResolveLineForRulePreview(trimmedLine)
				if err != nil {
					// If resolution fails, it's a non-fatal warning for the user.
					// Print to stderr so it can be captured by the calling plugin.
					fmt.Fprintf(os.Stderr, "Warning: could not resolve rule: %v\n", err)
					// Fallback to using the original line as the pattern.
					resolvedPatternsStr = ruleLine
				}

				// Split the result into individual patterns (for ruleset imports)
				patterns := strings.Split(resolvedPatternsStr, "\n")

				// Use the manager's file resolution logic with the patterns.
				// Note: ResolveFilesFromPatterns expects a slice and now handles brace expansion internally.
				files, err = mgr.ResolveFilesFromPatterns(patterns)
				if err != nil {
					return fmt.Errorf("error resolving files for rule '%s': %w", ruleLine, err)
				}
			}

			// Print the list of files to stdout, one per line.
			for _, file := range files {
				fmt.Println(file)
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&rulesFile, "rules-file", "", "Path to rules file for context-aware resolution")
	cmd.Flags().IntVar(&lineNumber, "line-number", 0, "Line number in rules file (1-indexed) for context-aware resolution")

	return cmd
}
