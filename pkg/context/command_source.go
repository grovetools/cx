package context

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// UpdateFromCmd updates the context rules file from the output of a shell command
func (m *Manager) UpdateFromCmd(command string) error {
	// Execute the command using shell
	cmd := exec.Command("sh", "-c", command)
	cmd.Dir = m.workDir

	// Capture both stdout and stderr
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// Run the command
	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("command failed: %w\nstderr: %s", err, stderr.String())
	}

	// Parse the command output
	files := parseCmdOutput(stdout.Bytes())

	if len(files) == 0 {
		return fmt.Errorf("no files found in command output")
	}

	// Filter and validate files
	var validFiles []string
	uniqueFiles := make(map[string]bool)

	for _, file := range files {
		// Skip empty lines
		if file == "" {
			continue
		}

		// Make absolute path if relative
		absPath := file
		if !filepath.IsAbs(file) {
			absPath = filepath.Join(m.workDir, file)
		}
		absPath = filepath.Clean(absPath)

		// Check if file exists
		if info, err := os.Stat(absPath); err == nil && !info.IsDir() {
			// Get relative path from workDir if possible
			relPath, err := filepath.Rel(m.workDir, absPath)
			if err == nil && !strings.HasPrefix(relPath, "..") {
				// File is within workDir, use relative path
				if !uniqueFiles[relPath] {
					uniqueFiles[relPath] = true
					validFiles = append(validFiles, relPath)
				}
			} else {
				// File is outside workDir, use absolute path
				if !uniqueFiles[absPath] {
					uniqueFiles[absPath] = true
					validFiles = append(validFiles, absPath)
				}
			}
		}
	}

	if len(validFiles) == 0 {
		return fmt.Errorf("no valid files found in command output")
	}

	// Ensure .grove directory exists
	groveDir := filepath.Join(m.workDir, GroveDir)
	if err := os.MkdirAll(groveDir, 0755); err != nil {
		return fmt.Errorf("error creating %s directory: %w", groveDir, err)
	}

	// Write to .grove/rules
	rulesPath := filepath.Join(m.workDir, ActiveRulesFile)
	if err := m.WriteFilesList(rulesPath, validFiles); err != nil {
		return err
	}

	fmt.Printf("Updated %s with %d files from command output\n", rulesPath, len(validFiles))
	return nil
}

// parseCmdOutput parses command output into a list of file paths
func parseCmdOutput(output []byte) []string {
	var files []string
	scanner := bufio.NewScanner(bytes.NewReader(output))

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" {
			files = append(files, line)
		}
	}

	return files
}
