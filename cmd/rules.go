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
	rulesDir       = ".cx"
	rulesExt       = ".rules"
	activeRules    = ".grove/rules"
	stateSourceKey = "context.active_rules_source"
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
	entries, err := os.ReadDir(rulesDir)
	if err != nil {
		if os.IsNotExist(err) {
			return rulesLoadedMsg{items: []ruleItem{}, err: nil}
		}
		return rulesLoadedMsg{err: fmt.Errorf("reading %s directory: %w", rulesDir, err)}
	}

	activeSource, _ := state.GetString(stateSourceKey)

	for _, entry := range entries {
		if !entry.IsDir() && filepath.Ext(entry.Name()) == rulesExt {
			path := filepath.Join(rulesDir, entry.Name())
			items = append(items, ruleItem{
				name:   entry.Name()[:len(entry.Name())-len(rulesExt)],
				path:   path,
				active: path == activeSource,
			})
		}
	}

	return rulesLoadedMsg{items: items, err: nil}
}

func setRuleCmd(item ruleItem) tea.Cmd {
	return func() tea.Msg {
		sourcePath := item.path
		content, err := os.ReadFile(sourcePath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error reading rule set: %v\n", err)
			return tea.Quit()
		}
		if err := os.MkdirAll(filepath.Dir(activeRules), 0755); err != nil {
			fmt.Fprintf(os.Stderr, "Error creating directory: %v\n", err)
			return tea.Quit()
		}
		if err := os.WriteFile(activeRules, content, 0644); err != nil {
			fmt.Fprintf(os.Stderr, "Error writing active rules: %v\n", err)
			return tea.Quit()
		}
		if err := state.Set(stateSourceKey, sourcePath); err != nil {
			fmt.Fprintf(os.Stderr, "Error updating state: %v\n", err)
			return tea.Quit()
		}
		fmt.Println(core_theme.DefaultTheme.Success.Render(fmt.Sprintf("✓ Active context rules set to '%s'", item.name)))
		return tea.Quit()
	}
}

// --- TUI Keymap ---

type pickerKeyMap struct {
	keymap.Base
	Select key.Binding
}

func (k pickerKeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Select, k.Quit}
}

func (k pickerKeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Up, k.Down, k.Select},
		{k.Help, k.Quit},
	}
}

var defaultPickerKeyMap = pickerKeyMap{
	Base: keymap.NewBase(),
	Select: key.NewBinding(
		key.WithKeys("enter"),
		key.WithHelp("enter", "select"),
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
			activeSource, _ := state.GetString(stateSourceKey)
			if activeSource == "" {
				activeSource = "(default)"
			}

			entries, err := os.ReadDir(rulesDir)
			if err != nil {
				if os.IsNotExist(err) {
					if jsonOutput {
						fmt.Println("[]")
						return nil
					}
					prettyLog.InfoPretty(fmt.Sprintf("  No rule sets found in %s/ directory.", rulesDir))
					return nil
				}
				return fmt.Errorf("error reading %s directory: %w", rulesDir, err)
			}

			var ruleNames []string
			for _, entry := range entries {
				if !entry.IsDir() && strings.HasSuffix(entry.Name(), rulesExt) {
					name := strings.TrimSuffix(entry.Name(), rulesExt)
					ruleNames = append(ruleNames, name)
				}
			}

			if jsonOutput {
				return outputJSON(ruleNames)
			}

			// Human-readable output
			prettyLog.InfoPretty("Available Rule Sets:")
			if len(ruleNames) == 0 {
				prettyLog.InfoPretty(fmt.Sprintf("  No rule sets found in %s/ directory.", rulesDir))
			} else {
				for _, name := range ruleNames {
					path := filepath.Join(rulesDir, name+rulesExt)
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

			content, err := os.ReadFile(sourcePath)
			if err != nil {
				return fmt.Errorf("rule set '%s' not found at %s", name, sourcePath)
			}

			// Ensure .grove dir exists
			if err := os.MkdirAll(filepath.Dir(activeRules), 0755); err != nil {
				return fmt.Errorf("failed to create .grove directory: %w", err)
			}

			if err := os.WriteFile(activeRules, content, 0644); err != nil {
				return fmt.Errorf("failed to write active rules file: %w", err)
			}

			if err := state.Set(stateSourceKey, sourcePath); err != nil {
				return fmt.Errorf("failed to update state: %w", err)
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

			content, err := os.ReadFile(activeRules)
			if err != nil {
				if os.IsNotExist(err) {
					return fmt.Errorf("no active rules file at %s to save", activeRules)
				}
				return fmt.Errorf("failed to read active rules: %w", err)
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
