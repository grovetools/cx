package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/mattsolo1/grove-context/pkg/context"
	"github.com/mattsolo1/grove-core/state"
	"github.com/mattsolo1/grove-core/tui/components/help"
	"github.com/mattsolo1/grove-core/tui/components/table"
	"github.com/mattsolo1/grove-core/tui/keymap"
	core_theme "github.com/mattsolo1/grove-core/tui/theme"
	"github.com/spf13/cobra"
)

const (
	rulesDir     = ".cx"
	rulesWorkDir = ".cx.work"
	rulesExt     = ".rules"
	activeRules  = ".grove/rules"
)

// --- TUI Model ---

type ruleItem struct {
	name   string
	path   string
	active bool
}

type rulesPickerModel struct {
	items         []ruleItem
	selectedIndex int
	keys          pickerKeyMap
	help          help.Model
	width, height int
	err           error
	quitting      bool
}

func newRulesPickerModel() *rulesPickerModel {
	return &rulesPickerModel{
		keys: defaultPickerKeyMap,
		help: help.New(defaultPickerKeyMap),
	}
}

func (m *rulesPickerModel) Init() tea.Cmd {
	return loadRulesCmd
}

func (m *rulesPickerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		return m, nil

	case rulesLoadedMsg:
		if msg.err != nil {
			m.err = msg.err
			return m, tea.Quit
		}
		m.items = msg.items
		// Set initial selection to the active rule
		for i, item := range m.items {
			if item.active {
				m.selectedIndex = i
				break
			}
		}
		return m, nil

	case tea.KeyMsg:
		if m.help.ShowAll {
			m.help.Toggle()
			return m, nil
		}
		switch {
		case key.Matches(msg, m.keys.Quit):
			m.quitting = true
			return m, tea.Quit
		case key.Matches(msg, m.keys.Select):
			if len(m.items) > 0 && m.selectedIndex < len(m.items) {
				m.quitting = true
				return m, setRuleCmd(m.items[m.selectedIndex])
			}
		case key.Matches(msg, m.keys.Load):
			if len(m.items) > 0 && m.selectedIndex < len(m.items) {
				m.quitting = true
				return m, loadRuleCmd(m.items[m.selectedIndex])
			}
		case key.Matches(msg, m.keys.Up):
			if m.selectedIndex > 0 {
				m.selectedIndex--
			}
		case key.Matches(msg, m.keys.Down):
			if m.selectedIndex < len(m.items)-1 {
				m.selectedIndex++
			}
		case key.Matches(msg, m.keys.Help):
			m.help.ShowAll = !m.help.ShowAll
			return m, nil
		}
	}

	return m, nil
}

func (m *rulesPickerModel) View() string {
	if m.quitting {
		return ""
	}
	if m.err != nil {
		return fmt.Sprintf("Error loading rule sets: %v\n", m.err)
	}

	// Build table data
	var rows [][]string
	for _, item := range m.items {
		status := " "
		if item.active {
			status = "✓"
		}
		rows = append(rows, []string{
			status,
			item.name,
			item.path,
		})
	}

	// Render header
	header := core_theme.DefaultTheme.Header.Render("Select an Active Rule Set")

	// Render table with selection and highlight the Name column (index 1)
	tableView := table.SelectableTableWithOptions(
		[]string{"", "Name", "Path"},
		rows,
		m.selectedIndex,
		table.SelectableTableOptions{
			HighlightColumn: 1, // Highlight the Name column
		},
	)

	// Render help
	helpContent := m.help.View()

	return fmt.Sprintf("%s\n\n%s\n\n%s", header, tableView, helpContent)
}

// --- TUI Commands ---

type rulesLoadedMsg struct {
	items []ruleItem
	err   error
}

func loadRulesCmd() tea.Msg {
	var items []ruleItem
	activeSource, _ := state.GetString(context.StateSourceKey)

	// Check if .grove/rules exists and add it as the first option
	if _, err := os.Stat(activeRules); err == nil {
		items = append(items, ruleItem{
			name:   ".grove/rules",
			path:   activeRules,
			active: activeSource == "" || activeSource == activeRules,
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
			if !entry.IsDir() && filepath.Ext(entry.Name()) == rulesExt {
				path := filepath.Join(dir, entry.Name())
				items = append(items, ruleItem{
					name:   entry.Name()[:len(entry.Name())-len(rulesExt)],
					path:   path,
					active: path == activeSource,
				})
			}
		}
		return nil
	}

	// Load named rule sets from .cx/ directory
	if err := loadRulesFromDir(rulesDir); err != nil {
		return rulesLoadedMsg{err: err}
	}

	// Load named rule sets from .cx.work/ directory
	if err := loadRulesFromDir(rulesWorkDir); err != nil {
		return rulesLoadedMsg{err: err}
	}

	return rulesLoadedMsg{items: items, err: nil}
}

func setRuleCmd(item ruleItem) tea.Cmd {
	return func() tea.Msg {
		sourcePath := item.path

		// If selecting .grove/rules, unset the state (fall back to default)
		if sourcePath == activeRules {
			if err := state.Delete(context.StateSourceKey); err != nil {
				fmt.Fprintf(os.Stderr, "Error updating state: %v\n", err)
				return tea.Quit()
			}
			fmt.Println(core_theme.DefaultTheme.Success.Render("✓ Using .grove/rules (default)"))
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
		if _, err := os.Stat(activeRules); err == nil {
			fmt.Fprintf(os.Stderr, "Warning: %s exists but will be ignored while '%s' is active.\n", activeRules, item.name)
		}

		fmt.Println(core_theme.DefaultTheme.Success.Render(fmt.Sprintf("✓ Active context rules set to '%s'", item.name)))
		return tea.Quit()
	}
}

func loadRuleCmd(item ruleItem) tea.Cmd {
	return func() tea.Msg {
		sourcePath := item.path

		// Can't load .grove/rules into itself
		if sourcePath == activeRules {
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
		if err := os.MkdirAll(filepath.Dir(activeRules), 0755); err != nil {
			fmt.Fprintf(os.Stderr, "Error creating .grove directory: %v\n", err)
			return tea.Quit()
		}

		// Write to .grove/rules
		if err := os.WriteFile(activeRules, content, 0644); err != nil {
			fmt.Fprintf(os.Stderr, "Error writing to .grove/rules: %v\n", err)
			return tea.Quit()
		}

		// Unset any active rule set state so .grove/rules becomes active
		if err := state.Delete(context.StateSourceKey); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: could not unset active rule set in state: %v\n", err)
		}

		fmt.Println(core_theme.DefaultTheme.Success.Render(fmt.Sprintf("✓ Loaded '%s' into .grove/rules as working copy", item.name)))
		return tea.Quit()
	}
}

// --- TUI Keymap ---

type pickerKeyMap struct {
	keymap.Base
	Select key.Binding
	Load   key.Binding
}

func (k pickerKeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Select, k.Load, k.Quit}
}

func (k pickerKeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Up, k.Down, k.Select, k.Load},
		{k.Help, k.Quit},
	}
}

var defaultPickerKeyMap = pickerKeyMap{
	Base: keymap.NewBase(),
	Select: key.NewBinding(
		key.WithKeys("enter"),
		key.WithHelp("enter", "select"),
	),
	Load: key.NewBinding(
		key.WithKeys("l"),
		key.WithHelp("l", "load to .grove/rules"),
	),
}

// NewRulesCmd creates the 'rules' command and its subcommands.
func NewRulesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "rules",
		Short: "Manage and switch between different context rule sets",
		Long:  `Provides commands to list, set, and save named context rule sets stored in the .cx/ directory.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// If no subcommand is given, run the interactive selector
			return runSelectTUI()
		},
	}

	cmd.AddCommand(newRulesListCmd())
	cmd.AddCommand(newRulesSetCmd())
	cmd.AddCommand(newRulesSaveCmd())
	cmd.AddCommand(newRulesSelectCmd())
	cmd.AddCommand(newRulesUnsetCmd())
	cmd.AddCommand(newRulesLoadCmd())

	return cmd
}

func newRulesSelectCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "select",
		Short: "Select the active rule set interactively",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSelectTUI()
		},
	}
}

func newRulesUnsetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "unset",
		Short: "Unset the active rule set and fall back to .grove/rules",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := state.Delete(context.StateSourceKey); err != nil {
				return fmt.Errorf("failed to update state: %w", err)
			}
			prettyLog.Success("Active rule set unset.")
			prettyLog.InfoPretty(fmt.Sprintf("Now using fallback file: %s (if it exists).", activeRules))
			return nil
		},
	}
}

func newRulesLoadCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "load <name>",
		Short: "Copy a named rule set to .grove/rules as a working copy",
		Long: `Copy a named rule set from .cx/ or .cx.work/ to .grove/rules.
This creates a working copy that you can edit freely without affecting the original.
The state is automatically unset so .grove/rules becomes active.

Examples:
  cx rules load default          # Copy .cx/default.rules to .grove/rules
  cx rules load dev-no-tests     # Copy from either .cx/ or .cx.work/`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]

			// Try to find the source file in .cx/ or .cx.work/
			var sourcePath string
			cxPath := filepath.Join(rulesDir, name+rulesExt)
			cxWorkPath := filepath.Join(rulesWorkDir, name+rulesExt)

			if _, err := os.Stat(cxPath); err == nil {
				sourcePath = cxPath
			} else if _, err := os.Stat(cxWorkPath); err == nil {
				sourcePath = cxWorkPath
			} else {
				return fmt.Errorf("rule set '%s' not found in %s/ or %s/", name, rulesDir, rulesWorkDir)
			}

			// Read the source file
			content, err := os.ReadFile(sourcePath)
			if err != nil {
				return fmt.Errorf("failed to read rule set: %w", err)
			}

			// Ensure .grove directory exists
			if err := os.MkdirAll(filepath.Dir(activeRules), 0755); err != nil {
				return fmt.Errorf("failed to create .grove directory: %w", err)
			}

			// Write to .grove/rules
			if err := os.WriteFile(activeRules, content, 0644); err != nil {
				return fmt.Errorf("failed to write to .grove/rules: %w", err)
			}

			// Unset any active rule set state so .grove/rules becomes active
			if err := state.Delete(context.StateSourceKey); err != nil {
				// Non-fatal, just warn
				prettyLog.WarnPretty(fmt.Sprintf("Warning: could not unset active rule set in state: %v", err))
			}

			prettyLog.Success(fmt.Sprintf("Loaded '%s' into .grove/rules as working copy", name))
			prettyLog.InfoPretty(fmt.Sprintf("Source: %s", sourcePath))
			prettyLog.InfoPretty("You can now edit .grove/rules freely without affecting the original.")
			return nil
		},
	}
}

func runSelectTUI() error {
	m := newRulesPickerModel()
	p := tea.NewProgram(m, tea.WithAltScreen())
	finalModel, err := p.Run()
	if err != nil {
		return fmt.Errorf("error running TUI: %w", err)
	}
	// Check for errors that occurred within the model
	if finalModel.(*rulesPickerModel).err != nil {
		return finalModel.(*rulesPickerModel).err
	}
	return nil
}

// listRulesForProject lists rule sets for a specific project alias.
func listRulesForProject(projectAlias string, jsonOutput bool) error {
	// Import the context package to use AliasResolver
	resolver := context.NewAliasResolver()
	projectPath, err := resolver.Resolve(projectAlias)
	if err != nil {
		return fmt.Errorf("failed to resolve project alias '%s': %w", projectAlias, err)
	}

	// Scan the .cx/ directory in the resolved project path
	cxDir := filepath.Join(projectPath, rulesDir)
	entries, err := os.ReadDir(cxDir)
	if err != nil {
		if os.IsNotExist(err) {
			if jsonOutput {
				fmt.Println("[]")
				return nil
			}
			return fmt.Errorf("no %s directory found in project '%s' at %s", rulesDir, projectAlias, projectPath)
		}
		return fmt.Errorf("error reading %s directory: %w", cxDir, err)
	}

	// Collect rule set names
	var ruleNames []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), rulesExt) {
			name := strings.TrimSuffix(entry.Name(), rulesExt)
			ruleNames = append(ruleNames, name)
		}
	}

	return outputJSON(ruleNames)
}

// outputJSON outputs a slice of strings as JSON.
func outputJSON(data []string) error {
	// Use encoding/json to marshal the data
	jsonBytes, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %w", err)
	}
	fmt.Println(string(jsonBytes))
	return nil
}

func newRulesListCmd() *cobra.Command {
	var forProject string
	var jsonOutput bool

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List available rule sets",
		RunE: func(cmd *cobra.Command, args []string) error {
			// If --for-project is set, list rules for that project
			if forProject != "" {
				return listRulesForProject(forProject, jsonOutput)
			}

			// Original behavior: list rules for current project
			activeSource, _ := state.GetString(context.StateSourceKey)
			if activeSource == "" {
				activeSource = "(default)"
			}

			// Helper to collect rules from a directory
			collectRules := func(dir string) ([]string, error) {
				entries, err := os.ReadDir(dir)
				if err != nil {
					if os.IsNotExist(err) {
						return nil, nil // Directory doesn't exist, that's ok
					}
					return nil, fmt.Errorf("error reading %s directory: %w", dir, err)
				}

				var names []string
				for _, entry := range entries {
					if !entry.IsDir() && strings.HasSuffix(entry.Name(), rulesExt) {
						name := strings.TrimSuffix(entry.Name(), rulesExt)
						names = append(names, name)
					}
				}
				return names, nil
			}

			// Collect from .cx/
			cxRules, err := collectRules(rulesDir)
			if err != nil {
				return err
			}

			// Collect from .cx.work/
			cxWorkRules, err := collectRules(rulesWorkDir)
			if err != nil {
				return err
			}

			// Combine all rules
			var ruleNames []string
			ruleNames = append(ruleNames, cxRules...)
			ruleNames = append(ruleNames, cxWorkRules...)

			if jsonOutput {
				return outputJSON(ruleNames)
			}

			// Human-readable output
			prettyLog.InfoPretty("Available Rule Sets:")
			if len(ruleNames) == 0 {
				prettyLog.InfoPretty("  No rule sets found.")
			} else {
				for _, name := range ruleNames {
					// Check both directories for the rule
					var path string
					if _, err := os.Stat(filepath.Join(rulesDir, name+rulesExt)); err == nil {
						path = filepath.Join(rulesDir, name+rulesExt)
					} else if _, err := os.Stat(filepath.Join(rulesWorkDir, name+rulesExt)); err == nil {
						path = filepath.Join(rulesWorkDir, name+rulesExt)
					}

					indicator := "  "
					if path == activeSource {
						indicator = "✓ "
					}
					prettyLog.InfoPretty(fmt.Sprintf("%s%s", indicator, name))
				}
			}

			prettyLog.Blank()
			prettyLog.InfoPretty(fmt.Sprintf("Active Source: %s", activeSource))
			return nil
		},
	}

	cmd.Flags().StringVar(&forProject, "for-project", "", "List rule sets for a specific project alias")
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output results in JSON format")

	return cmd
}

func newRulesSetCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "set <name>",
		Short: "Set the active rule set",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			sourcePath := filepath.Join(rulesDir, name+rulesExt)

			if _, err := os.Stat(sourcePath); os.IsNotExist(err) {
				return fmt.Errorf("rule set '%s' not found at %s", name, sourcePath)
			}

			if err := state.Set(context.StateSourceKey, sourcePath); err != nil {
				return fmt.Errorf("failed to update state: %w", err)
			}

			// Warn user if a .grove/rules file exists, as it will now be ignored.
			if _, err := os.Stat(activeRules); err == nil {
				prettyLog.WarnPretty(fmt.Sprintf("Warning: %s exists but will be ignored while '%s' is active.", activeRules, name))
			}

			prettyLog.Success(fmt.Sprintf("Active context rules set to '%s'", name))
			return nil
		},
	}
	return cmd
}

func newRulesSaveCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "save <name>",
		Short: "Save the current active rules to a new named rule set",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]

			mgr := context.NewManager("")
			content, _, err := mgr.LoadRulesContent()
			if err != nil {
				return fmt.Errorf("failed to load active rules to save: %w", err)
			}
			if content == nil {
				return fmt.Errorf("no active rules found to save")
			}

			if err := os.MkdirAll(rulesDir, 0755); err != nil {
				return fmt.Errorf("failed to create %s directory: %w", rulesDir, err)
			}

			destPath := filepath.Join(rulesDir, name+rulesExt)
			if err := os.WriteFile(destPath, content, 0644); err != nil {
				return fmt.Errorf("failed to save rule set: %w", err)
			}

			prettyLog.Success(fmt.Sprintf("Saved current rules as '%s'", name))
			return nil
		},
	}
	return cmd
}
