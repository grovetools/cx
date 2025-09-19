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
			mgr := context.NewManager("")
			rulesContent, rulesPath, err := mgr.LoadRulesContent()
			if err != nil {
				return err
			}

			// If no local rules file exists, create one before editing
			if _, err := os.Stat(rulesPath); os.IsNotExist(err) {
				// Ensure .grove directory exists
				groveDir := filepath.Dir(rulesPath)
				if err := os.MkdirAll(groveDir, 0755); err != nil {
					return fmt.Errorf("error creating %s directory: %w", groveDir, err)
				}
				
				// Use default content if available, otherwise use boilerplate
				if rulesContent == nil {
					rulesContent = []byte("# Context rules file\n# Add patterns to include files, one per line\n# Use ! prefix to exclude\n# Examples:\n#   *.go\n#   !*_test.go\n#   src/**/*.js\n\n*\n")
					fmt.Printf("Created new rules file with boilerplate: %s\n", rulesPath)
				} else {
					fmt.Printf("Created new rules file from project default: %s\n", rulesPath)
				}

				if err := os.WriteFile(rulesPath, rulesContent, 0644); err != nil {
					return fmt.Errorf("error creating %s: %w", rulesPath, err)
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
			
			// Get absolute path to rules file
			absRulesPath, err := filepath.Abs(rulesPath)
			if err != nil {
				return fmt.Errorf("error getting absolute path: %w", err)
			}
			
			// Save current directory
			originalDir, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("error getting current directory: %w", err)
			}
			
			// Change to git root if found
			if gitRoot != "" {
				if err := os.Chdir(gitRoot); err != nil {
					return fmt.Errorf("error changing to git root: %w", err)
				}
				defer os.Chdir(originalDir) // Restore original directory when done
			}
			
			// Open the file in the editor with absolute path
			editorCmd := exec.Command(editor, absRulesPath)
			editorCmd.Stdin = os.Stdin
			editorCmd.Stdout = os.Stdout
			editorCmd.Stderr = os.Stderr
			
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