package rules

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/grovetools/core/pkg/workspace"
	"github.com/grovetools/core/state"
	"github.com/grovetools/core/tui/theme"
	"github.com/grovetools/cx/pkg/context"
)

// --- TUI Commands ---

type rulesLoadedMsg struct {
	items []ruleItem
	err   error
}

func loadRulesCmd() tea.Msg {
	var items []ruleItem
	activeSource, _ := state.GetString(context.StateSourceKey)

	mgr := context.NewManager("")
	seen := make(map[string]bool)

	// Check for active rules file: notebook location first, then legacy .grove/rules
	rulesFileChecked := false
	if node, err := workspace.GetProjectByPath(mgr.GetWorkDir()); err == nil {
		if nbRulesFile, err := mgr.Locator().GetContextRulesFile(node); err == nil {
			if _, statErr := os.Stat(nbRulesFile); statErr == nil {
				content, err := os.ReadFile(nbRulesFile)
				if err != nil {
					content = []byte(theme.DefaultTheme.Error.Render(fmt.Sprintf("Error reading file: %v", err)))
				}
				items = append(items, ruleItem{
					name:    "rules",
					path:    nbRulesFile,
					active:  activeSource == "" || activeSource == nbRulesFile,
					content: string(content),
				})
				rulesFileChecked = true
			}
		}
	}
	if !rulesFileChecked {
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
	}

	// Helper function to load rules from a directory, deduplicating by name
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
				name := entry.Name()[:len(entry.Name())-len(context.RulesExt)]
				if seen[name] {
					continue
				}
				seen[name] = true
				path := filepath.Join(dir, entry.Name())
				content, err := os.ReadFile(path)
				if err != nil {
					content = []byte(theme.DefaultTheme.Error.Render(fmt.Sprintf("Error reading file: %v", err)))
				}
				items = append(items, ruleItem{
					name:    name,
					path:    path,
					active:  path == activeSource,
					content: string(content),
				})
			}
		}
		return nil
	}

	// Load from notebook presets directories first
	if node, err := workspace.GetProjectByPath(mgr.GetWorkDir()); err == nil {
		if presetsDir, err := mgr.Locator().GetContextPresetsDir(node); err == nil {
			if err := loadRulesFromDir(presetsDir); err != nil {
				return rulesLoadedMsg{err: err}
			}
		}
		if workDir, err := mgr.Locator().GetContextPresetsWorkDir(node); err == nil {
			if err := loadRulesFromDir(workDir); err != nil {
				return rulesLoadedMsg{err: err}
			}
		}
	}

	// Load from legacy .cx/ directory
	if err := loadRulesFromDir(context.RulesDir); err != nil {
		return rulesLoadedMsg{err: err}
	}

	// Load from legacy .cx.work/ directory
	if err := loadRulesFromDir(context.RulesWorkDir); err != nil {
		return rulesLoadedMsg{err: err}
	}

	// Load plan rules (only from the active plan)
	planRules, err := mgr.ListPlanRules()
	if err != nil {
		// Non-fatal error, just log to stderr for debugging
		fmt.Fprintf(os.Stderr, "Warning: could not load plan-specific rules: %v\n", err)
	} else {
		activePlan := mgr.GetActivePlanName()
		for _, rule := range planRules {
			// Only include rules from the active plan
			if activePlan != "" && rule.PlanName != activePlan {
				continue
			}
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
			fmt.Println(theme.DefaultTheme.Success.Render("Using .grove/rules (default)"))
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

		fmt.Println(theme.DefaultTheme.Success.Render(fmt.Sprintf("Active context rules set to '%s'", item.name)))
		return tea.Quit()
	}
}

func loadRuleCmd(item ruleItem) tea.Cmd {
	return func() tea.Msg {
		// Check for zombie worktree - refuse to create rules in deleted worktrees
		if context.IsZombieWorktreeCwd() {
			fmt.Fprintf(os.Stderr, "Error: Cannot create rules file in deleted worktree\n")
			return tea.Quit()
		}

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

		// Resolve the active rules write path (plan-scoped > notebook > local)
		mgr := context.NewManager("")
		rulesPath := mgr.ResolveRulesWritePath()

		// Write to resolved rules path
		if err := os.WriteFile(rulesPath, content, 0644); err != nil {
			fmt.Fprintf(os.Stderr, "Error writing rules: %v\n", err)
			return tea.Quit()
		}

		// Unset any active rule set state so the resolved path becomes active
		if err := state.Delete(context.StateSourceKey); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: could not unset active rule set in state: %v\n", err)
		}

		fmt.Println(theme.DefaultTheme.Success.Render(fmt.Sprintf("%s Loaded '%s' to %s as working copy", theme.IconSuccess, item.name, rulesPath)))
		return tea.Quit()
	}
}

func performLoadCmd(item ruleItem) tea.Cmd {
	return func() tea.Msg {
		// Check for zombie worktree - refuse to create rules in deleted worktrees
		if context.IsZombieWorktreeCwd() {
			return loadCompleteMsg{err: fmt.Errorf("cannot create rules file: worktree has been deleted")}
		}

		sourcePath := item.path

		// Read the source file
		content, err := os.ReadFile(sourcePath)
		if err != nil {
			return loadCompleteMsg{err: err}
		}

		// Resolve the active rules write path (plan-scoped > notebook > local)
		mgr := context.NewManager("")
		rulesPath := mgr.ResolveRulesWritePath()

		// Write to resolved rules path
		if err := os.WriteFile(rulesPath, content, 0644); err != nil {
			return loadCompleteMsg{err: err}
		}

		// Unset any active rule set state so the resolved path becomes active
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

		// Prioritize notebook location
		if node, nodeErr := workspace.GetProjectByPath(mgr.GetWorkDir()); nodeErr == nil {
			if toWork {
				if nbDir, locErr := mgr.Locator().GetContextPresetsWorkDir(node); locErr == nil {
					destDir = nbDir
				}
			} else {
				if nbDir, locErr := mgr.Locator().GetContextPresetsDir(node); locErr == nil {
					destDir = nbDir
				}
			}
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
			return deleteCompleteMsg{err: fmt.Errorf("rule set '%s' is in %s/ and is likely version-controlled. Press 'dd' again to force delete", item.name, context.RulesDir)}
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
