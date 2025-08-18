package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	
	"github.com/spf13/cobra"
	"github.com/mattsolo1/grove-context/pkg/context"
)

func NewEditCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "edit",
		Short: "Open the rules file in your editor",
		Long:  `Opens .grove/rules in your system's default editor (specified by $EDITOR environment variable).`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Determine the rule file path
			rulesPath := filepath.Join(context.GroveDir, "rules")
			
			// Ensure .grove directory exists
			groveDir := context.GroveDir
			if err := os.MkdirAll(groveDir, 0755); err != nil {
				return fmt.Errorf("error creating %s directory: %w", groveDir, err)
			}
			
			// Check if rules file exists, if not check for .grovectx
			if _, err := os.Stat(rulesPath); os.IsNotExist(err) {
				// Check for old .grovectx file
				oldRulesPath := context.RulesFile
				if _, err := os.Stat(oldRulesPath); err == nil {
					// Copy .grovectx to .grove/rules
					content, err := os.ReadFile(oldRulesPath)
					if err != nil {
						return fmt.Errorf("error reading %s: %w", oldRulesPath, err)
					}
					if err := os.WriteFile(rulesPath, content, 0644); err != nil {
						return fmt.Errorf("error writing %s: %w", rulesPath, err)
					}
					fmt.Printf("Migrated %s to %s\n", oldRulesPath, rulesPath)
				} else {
					// Create new rules file with default content
					defaultContent := []byte("# Context rules file\n# Add patterns to include files, one per line\n# Use ! prefix to exclude\n# Examples:\n#   *.go\n#   !*_test.go\n#   src/**/*.js\n\n*\n")
					if err := os.WriteFile(rulesPath, defaultContent, 0644); err != nil {
						return fmt.Errorf("error creating %s: %w", rulesPath, err)
					}
					fmt.Printf("Created new rules file: %s\n", rulesPath)
				}
			}
			
			// Get editor from environment
			editor := os.Getenv("EDITOR")
			if editor == "" {
				// Default editor based on OS
				switch runtime.GOOS {
				case "windows":
					editor = "notepad"
				default:
					editor = "vim"
				}
			}
			
			// Find git root directory
			gitRoot := findGitRoot()
			
			// Open the file in the editor
			editorCmd := exec.Command(editor, rulesPath)
			editorCmd.Stdin = os.Stdin
			editorCmd.Stdout = os.Stdout
			editorCmd.Stderr = os.Stderr
			
			// Set working directory to git root if found
			if gitRoot != "" {
				editorCmd.Dir = gitRoot
			}
			
			if err := editorCmd.Run(); err != nil {
				return fmt.Errorf("error opening editor: %w", err)
			}
			
			return nil
		},
	}
	
	return cmd
}

// findGitRoot finds the root directory of the git repository
func findGitRoot() string {
	// Try using git rev-parse
	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
	output, err := cmd.Output()
	if err == nil {
		return strings.TrimSpace(string(output))
	}
	
	// Fallback: walk up the directory tree looking for .git
	dir, err := os.Getwd()
	if err != nil {
		return ""
	}
	
	for {
		if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
			return dir
		}
		
		parent := filepath.Dir(dir)
		if parent == dir {
			// Reached the root
			break
		}
		dir = parent
	}
	
	return ""
}