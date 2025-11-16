package rules

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/mattsolo1/grove-context/pkg/context"
	"github.com/mattsolo1/grove-core/state"
	"github.com/mattsolo1/grove-core/tui/theme"
)

// --- TUI Commands ---

type rulesLoadedMsg struct {
	items []ruleItem
	err   error
}

func loadRulesCmd() tea.Msg {
	var items []ruleItem
	activeSource, _ := state.GetString(context.StateSourceKey)

	// Check if .grove/rules exists and add it as the first option
	if _, err := os.Stat(context.ActiveRulesFile); err == nil {
		content, err := os.ReadFile(context.ActiveRulesFile)
		if err != nil {
			content = []byte(theme.DefaultTheme.Error.Render(fmt.Sprintf("Error reading file: %v", err)))
		}
		items = append(items, ruleItem{
			name:    ".grove/rules",
			path:    context.ActiveRulesFile,
			active:  activeSource == "" || activeSource == context.ActiveRulesFile,
			content: string(content),
		})
	}

	// Helper function to load rules from a directory
	loadRulesFromDir := func(dir string) error {
		entries, err := os.ReadDir(dir)
		if err != nil {
			if os.IsNotExist(err) {
				return nil // Directory doesn't exist, that's ok
			}
			return fmt.Errorf("reading %s directory: %w", dir, err)
		}

		for _, entry := range entries {
			if !entry.IsDir() && filepath.Ext(entry.Name()) == context.RulesExt {
				path := filepath.Join(dir, entry.Name())
				content, err := os.ReadFile(path)
				if err != nil {
					content = []byte(theme.DefaultTheme.Error.Render(fmt.Sprintf("Error reading file: %v", err)))
				}
				items = append(items, ruleItem{
					name:    entry.Name()[:len(entry.Name())-len(context.RulesExt)],
					path:    path,
					active:  path == activeSource,
					content: string(content),
				})
			}
		}
		return nil
	}

	// Load named rule sets from .cx/ directory
	if err := loadRulesFromDir(context.RulesDir); err != nil {
		return rulesLoadedMsg{err: err}
	}

	// Load named rule sets from .cx.work/ directory
	if err := loadRulesFromDir(context.RulesWorkDir); err != nil {
		return rulesLoadedMsg{err: err}
	}

	// New: Load plan rules
	mgr := context.NewManager("")
	planRules, err := mgr.ListPlanRules()
	if err != nil {
		// Non-fatal error, just log to stderr for debugging
		fmt.Fprintf(os.Stderr, "Warning: could not load plan-specific rules: %v\n", err)
	} else {
		for _, rule := range planRules {
			content, readErr := os.ReadFile(rule.Path)
			if readErr != nil {
				content = []byte(theme.DefaultTheme.Error.Render(fmt.Sprintf("Error reading file: %v", readErr)))
			}
			items = append(items, ruleItem{
				name:        rule.Name,
				path:        rule.Path,
				active:      rule.Path == activeSource,
				content:     string(content),
				planContext: fmt.Sprintf("plan:%s (ws:%s)", rule.PlanName, rule.WorkspaceName),
				isPlanRule:  true,
			})
		}
	}

	return rulesLoadedMsg{items: items, err: nil}
}

func setRuleCmd(item ruleItem) tea.Cmd {
	return func() tea.Msg {
		sourcePath := item.path

		// If selecting .grove/rules, unset the state (fall back to default)
		if sourcePath == context.ActiveRulesFile {
			if err := state.Delete(context.StateSourceKey); err != nil {
				fmt.Fprintf(os.Stderr, "Error updating state: %v\n", err)
				return tea.Quit()
			}
			fmt.Println(theme.DefaultTheme.Success.Render("✓ Using .grove/rules (default)"))
			return tea.Quit()
		}

		// For named rule sets in .cx/, set the state
		if _, err := os.Stat(sourcePath); os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "Error: rule set '%s' not found at %s\n", item.name, sourcePath)
			return tea.Quit()
		}

		if err := state.Set(context.StateSourceKey, sourcePath); err != nil {
			fmt.Fprintf(os.Stderr, "Error updating state: %v\n", err)
			return tea.Quit()
		}

		// Warn user if a .grove/rules file exists, as it will now be ignored.
		if _, err := os.Stat(context.ActiveRulesFile); err == nil {
			fmt.Fprintf(os.Stderr, "Warning: %s exists but will be ignored while '%s' is active.\n", context.ActiveRulesFile, item.name)
		}

		fmt.Println(theme.DefaultTheme.Success.Render(fmt.Sprintf("✓ Active context rules set to '%s'", item.name)))
		return tea.Quit()
	}
}

func loadRuleCmd(item ruleItem) tea.Cmd {
	return func() tea.Msg {
		sourcePath := item.path

		// Can't load .grove/rules into itself
		if sourcePath == context.ActiveRulesFile {
			fmt.Fprintf(os.Stderr, "Error: Cannot load .grove/rules into itself\n")
			return tea.Quit()
		}

		// Read the source file
		content, err := os.ReadFile(sourcePath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error reading rule set: %v\n", err)
			return tea.Quit()
		}

		// Ensure .grove directory exists
		if err := os.MkdirAll(filepath.Dir(context.ActiveRulesFile), 0755); err != nil {
			fmt.Fprintf(os.Stderr, "Error creating .grove directory: %v\n", err)
			return tea.Quit()
		}

		// Write to .grove/rules
		if err := os.WriteFile(context.ActiveRulesFile, content, 0644); err != nil {
			fmt.Fprintf(os.Stderr, "Error writing to .grove/rules: %v\n", err)
			return tea.Quit()
		}

		// Unset any active rule set state so .grove/rules becomes active
		if err := state.Delete(context.StateSourceKey); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: could not unset active rule set in state: %v\n", err)
		}

		fmt.Println(theme.DefaultTheme.Success.Render(fmt.Sprintf("✓ Loaded '%s' into .grove/rules as working copy", item.name)))
		return tea.Quit()
	}
}

func performLoadCmd(item ruleItem) tea.Cmd {
	return func() tea.Msg {
		sourcePath := item.path

		// Read the source file
		content, err := os.ReadFile(sourcePath)
		if err != nil {
			return loadCompleteMsg{err: err}
		}

		// Ensure .grove directory exists
		if err := os.MkdirAll(filepath.Dir(context.ActiveRulesFile), 0755); err != nil {
			return loadCompleteMsg{err: err}
		}

		// Write to .grove/rules
		if err := os.WriteFile(context.ActiveRulesFile, content, 0644); err != nil {
			return loadCompleteMsg{err: err}
		}

		// Unset any active rule set state so .grove/rules becomes active
		_ = state.Delete(context.StateSourceKey)

		return loadCompleteMsg{err: nil}
	}
}

func performSetCmd(item ruleItem) tea.Cmd {
	return func() tea.Msg {
		sourcePath := item.path

		// If selecting .grove/rules, unset the state (fall back to default)
		if sourcePath == context.ActiveRulesFile {
			if err := state.Delete(context.StateSourceKey); err != nil {
				return setCompleteMsg{err: err}
			}
			return setCompleteMsg{err: nil}
		}

		// For named rule sets, set the state
		if _, err := os.Stat(sourcePath); os.IsNotExist(err) {
			return setCompleteMsg{err: fmt.Errorf("rule set not found at %s", sourcePath)}
		}

		if err := state.Set(context.StateSourceKey, sourcePath); err != nil {
			return setCompleteMsg{err: err}
		}

		return setCompleteMsg{err: nil}
	}
}

func editRuleCmd(item ruleItem) tea.Cmd {
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "vi" // fallback to vi if EDITOR not set
	}

	// Check if file exists
	if _, err := os.Stat(item.path); os.IsNotExist(err) {
		return func() tea.Msg {
			fmt.Fprintf(os.Stderr, "Error: rule set '%s' not found at %s\n", item.name, item.path)
			return tea.Quit()
		}
	}

	// Use tea.ExecProcess to properly suspend the TUI and run the editor
	c := exec.Command(editor, item.path)
	return tea.ExecProcess(c, func(err error) tea.Msg {
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error opening editor: %v\n", err)
		}
		return tea.Quit()
	})
}

func performSaveCmd(name string, toWork bool) tea.Cmd {
	return func() tea.Msg {
		mgr := context.NewManager("")
		content, _, err := mgr.LoadRulesContent()
		if err != nil {
			return saveCompleteMsg{err: fmt.Errorf("failed to load active rules: %w", err)}
		}
		if content == nil {
			return saveCompleteMsg{err: fmt.Errorf("no active rules found")}
		}

		destDir := context.RulesDir
		if toWork {
			destDir = context.RulesWorkDir
		}

		if err := os.MkdirAll(destDir, 0755); err != nil {
			return saveCompleteMsg{err: fmt.Errorf("failed to create %s directory: %w", destDir, err)}
		}

		destPath := filepath.Join(destDir, name+context.RulesExt)
		if err := os.WriteFile(destPath, content, 0644); err != nil {
			return saveCompleteMsg{err: fmt.Errorf("failed to save rule set: %w", err)}
		}

		return saveCompleteMsg{err: nil}
	}
}

func performDeleteCmd(item ruleItem, force bool) tea.Cmd {
	return func() tea.Msg {
		// Check if it's in the version-controlled directory
		isVersionControlled := filepath.Dir(item.path) == context.RulesDir

		if isVersionControlled && !force {
			return deleteCompleteMsg{err: fmt.Errorf("rule set '%s' is in %s/ and is likely version-controlled. Press 'd' again to force delete", item.name, context.RulesDir)}
		}

		// Check if this is the currently active rule set
		activeSource, _ := state.GetString(context.StateSourceKey)
		if activeSource == item.path {
			// Unset it first before deleting
			_ = state.Delete(context.StateSourceKey)
		}

		if err := os.Remove(item.path); err != nil {
			return deleteCompleteMsg{err: fmt.Errorf("failed to remove rule set: %w", err)}
		}

		return deleteCompleteMsg{err: nil}
	}
}
