package context

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/grovetools/core/util/pathutil"
	"github.com/sirupsen/logrus"
)

// GenerateContextFromRulesFile generates context from an explicit rules file path.
func (m *Manager) GenerateContextFromRulesFile(rulesFilePath string, useXMLFormat bool) error {
	m.log.WithFields(logrus.Fields{
		"rules_file": rulesFilePath,
		"xml_format": useXMLFormat,
	}).Info("Generating context from custom rules file")

	// Ensure .grove directory exists for output files
	groveDir := filepath.Join(m.workDir, GroveDir)
	if err := os.MkdirAll(groveDir, 0o755); err != nil {
		return fmt.Errorf("error creating %s directory: %w", groveDir, err)
	}

	absRulesFilePath, err := filepath.Abs(rulesFilePath)
	if err != nil {
		return fmt.Errorf("failed to get absolute path for rules file: %w", err)
	}

	// Read the rules file content
	rulesContent, err := os.ReadFile(absRulesFilePath)
	if err != nil {
		return fmt.Errorf("failed to read rules file: %w", err)
	}

	// Log rules file info using structured logging (respects TUI mode)
	content := strings.TrimSpace(string(rulesContent))
	lineCount := 0
	if content != "" {
		lineCount = len(strings.Split(content, "\n"))
	}
	m.ulog.Info("Using custom rules file").
		Field("path", absRulesFilePath).
		Field("lines", lineCount).
		Log(context.Background())

	hotRules, coldRules, _, treePaths, err := m.expandAllRules(absRulesFilePath, make(map[string]bool), 0)
	if err != nil {
		return fmt.Errorf("failed to resolve patterns from rules file %s: %w", rulesFilePath, err)
	}

	// Extract patterns from RuleInfo
	hotPatterns := make([]string, len(hotRules))
	for i, rule := range hotRules {
		pattern := rule.Pattern
		// Encode directive if present
		pattern = encodeDirectives(pattern, rule.Directives)
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
		pattern = encodeDirectives(pattern, rule.Directives)
		if rule.IsExclude {
			coldPatterns[i] = "!" + pattern
		} else {
			coldPatterns[i] = pattern
		}
	}

	hotFiles, err := m.resolveFilesFromPatterns(hotPatterns)
	if err != nil {
		return fmt.Errorf("error resolving hot context files: %w", err)
	}

	coldFiles, err := m.resolveFilesFromPatterns(coldPatterns)
	if err != nil {
		return fmt.Errorf("error resolving cold context files: %w", err)
	}

	// Cold-over-hot precedence
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

	// Generate context files
	if err := m.generateContextFromFilesAndTrees(finalHotFiles, treePaths, useXMLFormat); err != nil {
		return err
	}

	if err := m.generateCachedContextFromFiles(coldFiles); err != nil {
		return err
	}

	return nil
}

// GenerateContext creates the context file from the files list
func (m *Manager) GenerateContext(useXMLFormat bool) error {
	m.log.WithFields(logrus.Fields{
		"workdir":      m.workDir,
		"xml_format":   useXMLFormat,
		"context_file": ContextFile,
	}).Debug("Generating hot context file")

	// Ensure .grove directory exists
	groveDir := filepath.Join(m.workDir, GroveDir)
	if err := os.MkdirAll(groveDir, 0o755); err != nil {
		return fmt.Errorf("error creating %s directory: %w", groveDir, err)
	}

	// Use ResolveFilesAndTreesFromRules which handles @default and @tree directives
	filesToInclude, treePaths, err := m.ResolveFilesAndTreesFromRules()
	if err != nil {
		return fmt.Errorf("error resolving files from rules: %w", err)
	}

	// Handle case where no rules file exists
	if len(filesToInclude) == 0 && len(treePaths) == 0 {
		rulesContent, _, _ := m.LoadRulesContent()
		if rulesContent == nil {
			// Log warning using structured logging (respects TUI mode)
			m.ulog.Warn("No rules file found").
				Field("hint", fmt.Sprintf("Create %s with patterns to include files", ActiveRulesFile)).
				Log(context.Background())
		}
	}

	return m.generateContextFromFilesAndTrees(filesToInclude, treePaths, useXMLFormat)
}

// generateContextFromFilesAndTrees is a private helper that writes trees and a list of files to the hot context file.
func (m *Manager) generateContextFromFilesAndTrees(files []string, treePaths []string, useXMLFormat bool) error {
	// Resolve context file path (plan-scoped > notebook > local)
	contextPath := m.ResolveContextWritePath()
	ctxFile, err := os.Create(contextPath)
	if err != nil {
		return fmt.Errorf("error creating %s: %w", contextPath, err)
	}
	defer ctxFile.Close()

	// Write XML header and opening tags if using XML format
	if useXMLFormat {
		fmt.Fprintf(ctxFile, "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n")
		fmt.Fprintf(ctxFile, "<context>\n")
		fmt.Fprintf(ctxFile, "  <hot-context files=\"%d\" description=\"Files to be used for reference/background context to carry out the user's question/task to be provided later\">\n", len(files))
	}

	// If no files or trees to include, write a comment explaining why
	if len(files) == 0 && len(treePaths) == 0 {
		if useXMLFormat {
			fmt.Fprintf(ctxFile, "    <!-- No rules file found. Create %s with patterns to include files. -->\n", ActiveRulesFile)
		} else {
			fmt.Fprintf(ctxFile, "# No rules file found. Create %s with patterns to include files.\n", ActiveRulesFile)
		}
	}

	// Write trees first
	for _, tp := range treePaths {
		treeStr, err := m.GenerateTreeString(tp)
		if err != nil {
			m.ulog.Warn("Error generating tree").
				Field("path", tp).
				Err(err).
				Log(context.Background())
			continue
		}

		if useXMLFormat {
			fmt.Fprintf(ctxFile, "    <tree path=\"%s\">\n%s    </tree>\n", tp, treeStr)
		} else {
			fmt.Fprintf(ctxFile, "=== TREE: %s ===\n%s=== END TREE: %s ===\n\n", tp, treeStr, tp)
		}
	}

	// Write concatenated content
	for _, file := range files {
		if useXMLFormat {
			// Use the existing writeFileToXML method for consistency
			if err := m.writeFileToXML(ctxFile, file, "    "); err != nil {
				m.ulog.Warn("Error writing file to context").
					Field("file", file).
					Err(err).
					Log(context.Background())
			}
		} else {
			// Classic delimiter style
			fmt.Fprintf(ctxFile, "=== FILE: %s ===\n", file)

			// Read and write file content
			filePath := file
			if !filepath.IsAbs(file) {
				filePath = filepath.Join(m.workDir, file)
			}
			content, err := os.ReadFile(filePath)
			if err != nil {
				fmt.Fprintf(ctxFile, "Error reading file: %v\n", err)
				fmt.Fprintf(ctxFile, "=== END FILE: %s ===\n\n", file)
				continue
			}

			ctxFile.Write(content)

			// Write end marker
			fmt.Fprintf(ctxFile, "\n=== END FILE: %s ===\n\n", file)
		}
	}

	// Close XML tags if using XML format
	if useXMLFormat {
		fmt.Fprintf(ctxFile, "  </hot-context>\n")
		fmt.Fprintf(ctxFile, "</context>\n")
	}

	m.log.WithFields(logrus.Fields{
		"file_count":  len(files),
		"output_path": contextPath,
	}).Info("Generated hot context file")

	m.ulog.Success("Generated context file").
		Field("path", contextPath).
		Field("file_count", len(files)).
		Log(context.Background())

	return nil
}

// GenerateCachedContext generates .grove/cached-context with only the cold context files.
func (m *Manager) GenerateCachedContext() error {
	// Ensure .grove directory exists
	groveDir := filepath.Join(m.workDir, GroveDir)
	if err := os.MkdirAll(groveDir, 0o755); err != nil {
		return fmt.Errorf("error creating %s directory: %w", groveDir, err)
	}

	// Get ONLY cold context files
	coldFiles, err := m.ResolveColdContextFiles()
	if err != nil {
		return fmt.Errorf("error resolving cold context files: %w", err)
	}

	return m.generateCachedContextFromFiles(coldFiles)
}

// generateCachedContextFromFiles is a private helper that writes a list of files to the cold context files.
func (m *Manager) generateCachedContextFromFiles(coldFiles []string) error {
	// Resolve cached context file paths (plan-scoped > notebook > local)
	cachedPath := m.ResolveCachedContextWritePath()
	cachedListPath := m.ResolveCachedContextFilesListWritePath()
	cachedFile, err := os.Create(cachedPath)
	if err != nil {
		return fmt.Errorf("error creating %s: %w", cachedPath, err)
	}
	defer cachedFile.Close()

	// If no cold files, we can just create an empty file or a small XML structure.
	// Let's keep the structure for consistency.
	fmt.Fprintf(cachedFile, "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n")
	fmt.Fprintf(cachedFile, "<context>\n")
	fmt.Fprintf(cachedFile, "  <cold-context files=\"%d\">\n", len(coldFiles))

	// Write cold context files
	for _, file := range coldFiles {
		if err := m.writeFileToXML(cachedFile, file, "    "); err != nil {
			m.ulog.Warn("Error writing file to cached context").
				Field("file", file).
				Err(err).
				Log(context.Background())
		}
	}

	fmt.Fprintf(cachedFile, "  </cold-context>\n")
	fmt.Fprintf(cachedFile, "</context>\n")

	// Write the list of cached context files
	if err := m.WriteFilesList(cachedListPath, coldFiles); err != nil {
		return err
	}

	m.log.WithFields(logrus.Fields{
		"file_count":  len(coldFiles),
		"output_path": cachedPath,
		"list_path":   cachedListPath,
	}).Info("Generated cold context artifacts")

	m.ulog.Success("Generated cached context").
		Field("path", cachedPath).
		Field("file_count", len(coldFiles)).
		Log(context.Background())

	// Provide user feedback
	if len(coldFiles) > 0 {
		m.ulog.Success("Generated cached context files list").
			Field("path", cachedListPath).
			Field("file_count", len(coldFiles)).
			Log(context.Background())
	}

	return nil
}

// writeFileToXML writes a file's content to the XML output with proper indentation
func (m *Manager) writeFileToXML(w io.Writer, file string, indent string) error {
	fmt.Fprintf(w, "%s<file path=\"%s\">\n", indent, file)

	// Read file content
	filePath := file
	if !filepath.IsAbs(file) {
		filePath = filepath.Join(m.workDir, file)
	}

	content, err := os.ReadFile(filePath)
	if err != nil {
		fmt.Fprintf(w, "%s  <error>%v</error>\n", indent, err)
		fmt.Fprintf(w, "%s</file>\n", indent)
		return err
	}

	// Write content directly without extra indentation (content already has its own)
	w.Write(content)

	// Ensure there's a newline before the closing tag
	if len(content) > 0 && content[len(content)-1] != '\n' {
		fmt.Fprintf(w, "\n")
	}

	fmt.Fprintf(w, "%s</file>\n", indent)
	return nil
}

// GenerateTreeString generates an ascii tree representation of the directory structure
func (m *Manager) GenerateTreeString(rootPath string) (string, error) {
	absPath := rootPath
	if !filepath.IsAbs(rootPath) {
		absPath = filepath.Join(m.workDir, rootPath)
	}
	absPath = filepath.Clean(absPath)

	gitIgnored, err := m.getGitIgnoredFiles(absPath)
	if err != nil {
		gitIgnored = make(map[string]bool)
	}

	var sb strings.Builder
	baseName := filepath.Base(absPath)
	if baseName == "." || baseName == string(filepath.Separator) {
		baseName = filepath.Base(m.workDir)
	}
	sb.WriteString(baseName + "/\n")

	err = m.buildTreeString(&sb, absPath, "", gitIgnored)
	return sb.String(), err
}

func (m *Manager) buildTreeString(sb *strings.Builder, dirPath string, prefix string, gitIgnored map[string]bool) error {
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return err
	}

	// Filter entries to drop gitignored items and internal Grove/git directories
	var validEntries []os.DirEntry
	for _, entry := range entries {
		name := entry.Name()
		if name == ".git" || name == ".grove" {
			continue
		}

		fullPath := filepath.Join(dirPath, name)
		normPath, normErr := pathutil.NormalizeForLookup(fullPath)
		if normErr != nil {
			normPath = fullPath
		}

		if gitIgnored[normPath] {
			continue
		}
		validEntries = append(validEntries, entry)
	}

	for i, entry := range validEntries {
		isLast := i == len(validEntries)-1

		connector := "├── "
		childPrefix := prefix + "│   "
		if isLast {
			connector = "└── "
			childPrefix = prefix + "    "
		}

		name := entry.Name()
		if entry.IsDir() {
			name += "/"
		}

		sb.WriteString(prefix + connector + name + "\n")

		if entry.IsDir() {
			if err := m.buildTreeString(sb, filepath.Join(dirPath, entry.Name()), childPrefix, gitIgnored); err != nil {
				sb.WriteString(childPrefix + "[error reading directory]\n")
			}
		}
	}

	return nil
}
