package context

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ValidationResult contains the results of context validation
type ValidationResult struct {
	TotalFiles       int
	AccessibleFiles  int
	MissingFiles     []string
	Duplicates       map[string]int
	PermissionIssues []string
}

// ValidateContext checks the integrity of all files in the context
func (m *Manager) ValidateContext(files []string) (*ValidationResult, error) {

	result := &ValidationResult{
		TotalFiles: len(files),
		Duplicates: make(map[string]int),
	}
	
	if len(files) == 0 {
		return result, nil
	}
	
	// Track file occurrences
	fileCount := make(map[string]int)
	
	for _, file := range files {
		// Normalize path
		absPath, err := filepath.Abs(file)
		if err != nil {
			result.MissingFiles = append(result.MissingFiles, file)
			continue
		}
		
		// Count occurrences
		fileCount[absPath]++
		
		// Check if file exists
		info, err := os.Stat(absPath)
		if err != nil {
			if os.IsNotExist(err) {
				result.MissingFiles = append(result.MissingFiles, file)
			} else if os.IsPermission(err) {
				result.PermissionIssues = append(result.PermissionIssues, file)
			} else {
				// Other errors treated as missing
				result.MissingFiles = append(result.MissingFiles, file)
			}
			continue
		}
		
		// Check if it's a regular file
		if info.IsDir() {
			result.MissingFiles = append(result.MissingFiles, file+" (is a directory)")
			continue
		}
		
		// Check read permission
		testFile, err := os.Open(absPath)
		if err != nil {
			if os.IsPermission(err) {
				result.PermissionIssues = append(result.PermissionIssues, file)
			} else {
				result.MissingFiles = append(result.MissingFiles, file)
			}
			continue
		}
		testFile.Close()
		
		result.AccessibleFiles++
	}
	
	// Find duplicates
	for path, count := range fileCount {
		if count > 1 {
			// Convert back to relative path for display
			relPath, err := filepath.Rel(".", path)
			if err != nil {
				relPath = path
			}
			result.Duplicates[relPath] = count
		}
	}
	
	return result, nil
}

// PrintValidationResults displays validation results in a formatted way
func (r *ValidationResult) Print() {
	fmt.Println("Validating context files...")
	fmt.Println()
	
	// Missing files
	if len(r.MissingFiles) > 0 {
		fmt.Printf("✗ Missing files (%d):\n", len(r.MissingFiles))
		for _, file := range r.MissingFiles {
			fmt.Printf("  - %s", file)
			if strings.HasSuffix(file, " (is a directory)") {
				fmt.Print(" (remove from context)")
			} else {
				fmt.Print(" (remove from context)")
			}
			fmt.Println()
		}
		fmt.Println()
	}
	
	// Permission issues
	if len(r.PermissionIssues) > 0 {
		fmt.Printf("✗ Permission denied (%d):\n", len(r.PermissionIssues))
		for _, file := range r.PermissionIssues {
			fmt.Printf("  - %s (check file permissions)\n", file)
		}
		fmt.Println()
	}
	
	// Duplicates
	if len(r.Duplicates) > 0 {
		fmt.Printf("⚠ Duplicates found (%d):\n", len(r.Duplicates))
		for file, count := range r.Duplicates {
			fmt.Printf("  - %s appears %d times\n", file, count)
		}
		fmt.Println()
	}
	
	// Summary
	fmt.Printf("✓ Accessible files: %d/%d\n", r.AccessibleFiles, r.TotalFiles)
	
	// Total issues
	totalIssues := len(r.MissingFiles) + len(r.Duplicates) + len(r.PermissionIssues)
	if totalIssues > 0 {
		fmt.Printf("✗ Issues found: %d\n", totalIssues)
		fmt.Println("\nCheck your rules file and ensure all referenced files exist.")
	} else {
		fmt.Printf("\n✓ All %d files are valid and accessible\n", r.TotalFiles)
	}
}

// FixContext removes invalid files and duplicates from the context (deprecated - no longer applicable with dynamic resolution)
func (m *Manager) FixContext() error {
	fmt.Println("Note: 'fix' command is deprecated. Context is now dynamically resolved from rules.")
	fmt.Println("To fix issues, edit your rules file directly.")
	return nil
}