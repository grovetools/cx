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
func (m *Manager) ValidateContext() (*ValidationResult, error) {
	// Read file list
	files, err := m.ReadFilesList(FilesListFile)
	if err != nil {
		return nil, fmt.Errorf("error reading %s: %w", FilesListFile, err)
	}

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
		fmt.Println("\nRun 'grove cx update' to regenerate from rules.")
	} else {
		fmt.Printf("\n✓ All %d files are valid and accessible\n", r.TotalFiles)
	}
}

// FixContext removes invalid files and duplicates from the context
func (m *Manager) FixContext() error {
	// Read file list
	files, err := m.ReadFilesList(FilesListFile)
	if err != nil {
		return fmt.Errorf("error reading %s: %w", FilesListFile, err)
	}

	if len(files) == 0 {
		fmt.Println("No files in context.")
		return nil
	}

	// Validate and collect valid, unique files
	validFiles := make(map[string]bool)
	var fixedFiles []string
	removedCount := 0
	duplicatesRemoved := 0

	for _, file := range files {
		// Normalize path
		absPath, err := filepath.Abs(file)
		if err != nil {
			removedCount++
			continue
		}
		
		// Skip if already added (duplicate)
		if validFiles[absPath] {
			duplicatesRemoved++
			continue
		}
		
		// Check if file exists and is accessible
		info, err := os.Stat(absPath)
		if err != nil {
			removedCount++
			continue
		}
		
		// Skip directories
		if info.IsDir() {
			removedCount++
			continue
		}
		
		// Check read permission
		testFile, err := os.Open(absPath)
		if err != nil {
			removedCount++
			continue
		}
		testFile.Close()
		
		// Add to valid files
		validFiles[absPath] = true
		fixedFiles = append(fixedFiles, file)
	}

	// Write back the cleaned list
	if err := m.WriteFilesList(FilesListFile, fixedFiles); err != nil {
		return fmt.Errorf("error writing fixed file list: %w", err)
	}

	fmt.Printf("Fixed context file list:\n")
	fmt.Printf("  Removed %d invalid/missing files\n", removedCount)
	fmt.Printf("  Removed %d duplicate entries\n", duplicatesRemoved)
	fmt.Printf("  %d valid files remain\n", len(fixedFiles))
	fmt.Printf("Updated %s with %d files\n", FilesListFile, len(fixedFiles))
	
	return nil
}