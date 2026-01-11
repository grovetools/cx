package context

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

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
	if err := os.MkdirAll(groveDir, 0755); err != nil {
		return fmt.Errorf("error creating %s directory: %w", groveDir, err)
	}

	absRulesFilePath, err := filepath.Abs(rulesFilePath)
	if err != nil {
		return fmt.Errorf("failed to get absolute path for rules file: %w", err)
	}

	// Read and display the rules file content
	rulesContent, err := os.ReadFile(absRulesFilePath)
	if err != nil {
		return fmt.Errorf("failed to read rules file: %w", err)
	}

	// Print rules file info to stderr
	fmt.Fprintf(os.Stderr, "📋 Using rules file: %s\n", absRulesFilePath)
	content := strings.TrimSpace(string(rulesContent))
	if content != "" {
		lines := strings.Split(content, "\n")
		fmt.Fprintf(os.Stderr, "📋 Rules content (%d lines):\n", len(lines))

		// Display with indentation, limit to first 10 lines
		maxLines := 10
		displayLines := lines
		if len(lines) > maxLines {
			displayLines = lines[:maxLines]
		}

		for _, line := range displayLines {
			fmt.Fprintf(os.Stderr, "   %s\n", line)
		}

		if len(lines) > maxLines {
			fmt.Fprintf(os.Stderr, "   ... (%d more lines)\n", len(lines)-maxLines)
		}
		fmt.Fprintln(os.Stderr)
	}

	hotRules, coldRules, _, err := m.expandAllRules(absRulesFilePath, make(map[string]bool), 0)
	if err != nil {
		return fmt.Errorf("failed to resolve patterns from rules file %s: %w", rulesFilePath, err)
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
	if err := m.generateContextFromFiles(finalHotFiles, useXMLFormat); err != nil {
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
	if err := os.MkdirAll(groveDir, 0755); err != nil {
		return fmt.Errorf("error creating %s directory: %w", groveDir, err)
	}

	// Use ResolveFilesFromRules which handles @default directives
	filesToInclude, err := m.ResolveFilesFromRules()
	if err != nil {
		return fmt.Errorf("error resolving files from rules: %w", err)
	}

	// Handle case where no rules file exists
	if len(filesToInclude) == 0 {
		rulesContent, _, _ := m.LoadRulesContent()
		if rulesContent == nil {
			// Print visible warning to stderr
			fmt.Fprintf(os.Stderr, "\n⚠️  WARNING: No rules file found!\n")
			fmt.Fprintf(os.Stderr, "⚠️  Create %s with patterns to include files in the context.\n", ActiveRulesFile)
			fmt.Fprintf(os.Stderr, "⚠️  Generating empty context file.\n\n")
		}
	}

	return m.generateContextFromFiles(filesToInclude, useXMLFormat)
}

// generateContextFromFiles is a private helper that writes a list of files to the hot context file.
func (m *Manager) generateContextFromFiles(files []string, useXMLFormat bool) error {
	// Create context file
	contextPath := filepath.Join(m.workDir, ContextFile)
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

	// If no files to include, write a comment explaining why
	if len(files) == 0 {
		if useXMLFormat {
			fmt.Fprintf(ctxFile, "    <!-- No rules file found. Create %s with patterns to include files. -->\n", ActiveRulesFile)
		} else {
			fmt.Fprintf(ctxFile, "# No rules file found. Create %s with patterns to include files.\n", ActiveRulesFile)
		}
	}

	// Write concatenated content
	for _, file := range files {
		if useXMLFormat {
			// Use the existing writeFileToXML method for consistency
			if err := m.writeFileToXML(ctxFile, file, "    "); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: error writing file %s: %v\n", file, err)
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
		Field("path", ContextFile).
		Field("file_count", len(files)).
		Log(context.Background())

	return nil
}

// GenerateCachedContext generates .grove/cached-context with only the cold context files.
func (m *Manager) GenerateCachedContext() error {
	// Ensure .grove directory exists
	groveDir := filepath.Join(m.workDir, GroveDir)
	if err := os.MkdirAll(groveDir, 0755); err != nil {
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
	// Create cached context file
	cachedPath := filepath.Join(m.workDir, CachedContextFile)
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
			fmt.Fprintf(os.Stderr, "Warning: error writing file %s: %v\n", file, err)
		}
	}

	fmt.Fprintf(cachedFile, "  </cold-context>\n")
	fmt.Fprintf(cachedFile, "</context>\n")

	// Write the list to .grove/cached-context-files
	cachedListPath := filepath.Join(m.workDir, CachedContextFilesListFile)
	if err := m.WriteFilesList(cachedListPath, coldFiles); err != nil {
		return err
	}

	m.log.WithFields(logrus.Fields{
		"file_count":  len(coldFiles),
		"output_path": cachedPath,
		"list_path":   cachedListPath,
	}).Info("Generated cold context artifacts")

	m.ulog.Success("Generated cached context").
		Field("path", CachedContextFile).
		Field("file_count", len(coldFiles)).
		Log(context.Background())

	// Provide user feedback
	if len(coldFiles) > 0 {
		m.ulog.Success("Generated cached context files list").
			Field("path", CachedContextFilesListFile).
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
