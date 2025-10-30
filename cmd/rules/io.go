package rules

import (
	"fmt"
	"os"
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
		items = append(items, ruleItem{
			name:   ".grove/rules",
			path:   context.ActiveRulesFile,
			active: activeSource == "" || activeSource == context.ActiveRulesFile,
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
				items = append(items, ruleItem{
					name:   entry.Name()[:len(entry.Name())-len(context.RulesExt)],
					path:   path,
					active: path == activeSource,
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
