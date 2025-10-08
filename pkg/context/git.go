package context

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// GitOptions contains options for git-based context generation
type GitOptions struct {
	Since   string // Include files changed since date/commit
	Branch  string // Include files changed in branch (e.g., main..HEAD)
	Staged  bool   // Include only staged files
	Commits int    // Include files from last N commits
}

// UpdateFromGit updates the context files list based on git history
func (m *Manager) UpdateFromGit(opts GitOptions) error {
	// Ensure we're in a git repository
	if err := checkGitRepo(); err != nil {
		return err
	}

	// Collect files based on options
	var files []string
	var err error

	switch {
	case opts.Staged:
		files, err = getGitStagedFiles()
	case opts.Since != "":
		files, err = getGitFilesSince(opts.Since)
	case opts.Branch != "":
		files, err = getGitFilesInBranch(opts.Branch)
	case opts.Commits > 0:
		files, err = getGitFilesFromCommits(opts.Commits)
	default:
		return fmt.Errorf("no git option specified")
	}

	if err != nil {
		return fmt.Errorf("error getting git files: %w", err)
	}

	if len(files) == 0 {
		return fmt.Errorf("no files found matching git criteria")
	}

	// Remove duplicates
	uniqueFiles := make(map[string]bool)
	for _, file := range files {
		uniqueFiles[file] = true
	}

	// Convert map to slice
	var fileList []string
	for file := range uniqueFiles {
		// Exclude files in .grove-worktrees directories, but only if the .grove-worktrees
		// is a descendant of the working directory (not an ancestor)
		relPath, err := filepath.Rel(m.workDir, file)
		if err == nil && strings.Contains(relPath, ".grove-worktrees") {
			// The .grove-worktrees is within our working directory, exclude it
			continue
		}

		// Check if file still exists (might have been deleted)
		if _, err := os.Stat(file); err == nil {
			fileList = append(fileList, file)
		}
	}

	if len(fileList) == 0 {
		return fmt.Errorf("no existing files found matching git criteria")
	}

	// Ensure .grove directory exists
	groveDir := filepath.Join(m.workDir, GroveDir)
	if err := os.MkdirAll(groveDir, 0755); err != nil {
		return fmt.Errorf("error creating %s directory: %w", groveDir, err)
	}

	// Write to .grove/rules as explicit file paths
	rulesPath := filepath.Join(m.workDir, ActiveRulesFile)
	if err := m.WriteFilesList(rulesPath, fileList); err != nil {
		return err
	}

	fmt.Printf("Updated %s with %d explicit file paths from git\n", rulesPath, len(fileList))
	return nil
}

// checkGitRepo verifies we're in a git repository
func checkGitRepo() error {
	cmd := exec.Command("git", "rev-parse", "--git-dir")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("not in a git repository")
	}
	return nil
}

// getGitStagedFiles returns files in the staging area
func getGitStagedFiles() ([]string, error) {
	cmd := exec.Command("git", "diff", "--cached", "--name-only")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get staged files: %w", err)
	}

	return parseGitFileList(output), nil
}

// getGitFilesSince returns files changed since a specific date or commit
func getGitFilesSince(since string) ([]string, error) {
	// Try to interpret as a commit first
	cmd := exec.Command("git", "rev-parse", since)
	if err := cmd.Run(); err == nil {
		// It's a commit, use rev-list
		return getGitFilesFromCommitRange(since + "..HEAD")
	}

	// Otherwise treat as a date
	cmd = exec.Command("git", "log", "--since="+since, "--name-only", "--pretty=format:")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get files since %s: %w", since, err)
	}

	return parseGitFileList(output), nil
}

// getGitFilesInBranch returns files changed in a branch compared to another
func getGitFilesInBranch(branch string) ([]string, error) {
	// If branch doesn't contain "..", assume comparison with current branch
	if !strings.Contains(branch, "..") {
		branch = branch + "..HEAD"
	}

	return getGitFilesFromCommitRange(branch)
}

// getGitFilesFromCommits returns files changed in the last N commits
func getGitFilesFromCommits(n int) ([]string, error) {
	cmd := exec.Command("git", "log", "-"+strconv.Itoa(n), "--name-only", "--pretty=format:")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get files from last %d commits: %w", n, err)
	}

	return parseGitFileList(output), nil
}

// getGitFilesFromCommitRange returns files changed in a commit range
func getGitFilesFromCommitRange(commitRange string) ([]string, error) {
	cmd := exec.Command("git", "diff", "--name-only", commitRange)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get files in range %s: %w", commitRange, err)
	}

	return parseGitFileList(output), nil
}

// parseGitFileList parses git output into a list of file paths
func parseGitFileList(output []byte) []string {
	var files []string
	scanner := bufio.NewScanner(bytes.NewReader(output))

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" {
			// Normalize path separators
			file := filepath.Clean(line)
			files = append(files, file)
		}
	}

	return files
}

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

// GetGitInfo returns information about the current git state
func GetGitInfo() (branch string, hasChanges bool, err error) {
	// Get current branch
	cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	output, err := cmd.Output()
	if err != nil {
		return "", false, fmt.Errorf("failed to get current branch: %w", err)
	}
	branch = strings.TrimSpace(string(output))

	// Check for uncommitted changes
	cmd = exec.Command("git", "status", "--porcelain")
	output, err = cmd.Output()
	if err != nil {
		return branch, false, fmt.Errorf("failed to get git status: %w", err)
	}
	hasChanges = len(output) > 0

	return branch, hasChanges, nil
}
