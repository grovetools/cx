package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/mattsolo1/grove-core/state"
	"github.com/mattsolo1/grove-core/tui/components/help"
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
	name string
	path string
}

func (i ruleItem) Title() string       { return i.name }
func (i ruleItem) Description() string { return i.path }
func (i ruleItem) FilterValue() string { return i.name }

type rulesPickerModel struct {
	list          list.Model
	keys          pickerKeyMap
	help          help.Model
	activeSource  string
	width, height int
	err           error
	quitting      bool
}

func newRulesPickerModel() *rulesPickerModel {
	m := &rulesPickerModel{
		keys: defaultPickerKeyMap,
		help: help.New(defaultPickerKeyMap),
	}

	delegate := list.NewDefaultDelegate()
	m.list = list.New([]list.Item{}, delegate, 0, 0)
	m.list.Title = "Select an Active Rule Set"
	m.list.SetShowStatusBar(false)
	m.list.SetFilteringEnabled(false)
	m.list.Styles.Title = core_theme.DefaultTheme.Header

	return m
}

func (m *rulesPickerModel) Init() tea.Cmd {
	return loadRulesCmd
}

func (m *rulesPickerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.list.SetSize(msg.Width, msg.Height-4)
		return m, nil

	case rulesLoadedMsg:
		if msg.err != nil {
			m.err = msg.err
			return m, tea.Quit
		}
		m.activeSource = msg.activeSource

		listItems := make([]list.Item, len(msg.items))
		selectedIndex := 0
		for i, item := range msg.items {
			listItems[i] = item
			if item.path == msg.activeSource {
				selectedIndex = i
			}
		}

		m.list.SetItems(listItems)
		m.list.Select(selectedIndex)
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
			if i, ok := m.list.SelectedItem().(ruleItem); ok {
				m.quitting = true
				return m, setRuleCmd(i)
			}
		case key.Matches(msg, m.keys.Help):
			m.help.ShowAll = !m.help.ShowAll
			return m, nil
		}
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m *rulesPickerModel) View() string {
	if m.quitting {
		return ""
	}
	if m.err != nil {
		return fmt.Sprintf("Error loading rule sets: %v\n", m.err)
	}

	mainContent := m.renderList()
	helpContent := m.help.View()

	return lipgloss.JoinVertical(lipgloss.Left, mainContent, helpContent)
}

func (m *rulesPickerModel) renderList() string {
	listView := m.list.View()

	if m.activeSource != "" {
		lines := strings.Split(listView, "\n")
		for i, line := range lines {
			if i == 0 || strings.TrimSpace(line) == "" {
				continue
			}

			for _, item := range m.list.Items() {
				if rItem, ok := item.(ruleItem); ok {
					if strings.Contains(line, rItem.name) && rItem.path == m.activeSource {
						lines[i] = core_theme.DefaultTheme.Success.Render("✓ ") + line
						break
					} else if strings.Contains(line, rItem.name) {
						lines[i] = "  " + line
						break
					}
				}
			}
		}
		listView = strings.Join(lines, "\n")
	}

	return listView
}

// --- TUI Commands ---

type rulesLoadedMsg struct {
	items        []ruleItem
	activeSource string
	err          error
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

	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), rulesExt) {
			items = append(items, ruleItem{
				name: strings.TrimSuffix(entry.Name(), rulesExt),
				path: filepath.Join(rulesDir, entry.Name()),
			})
		}
	}

	activeSource, _ := state.GetString(stateSourceKey)

	return rulesLoadedMsg{items: items, activeSource: activeSource, err: nil}
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
	return []key.Binding{k.Up, k.Down, k.Select, k.Help, k.Quit}
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

func newRulesListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List available rule sets",
		RunE: func(cmd *cobra.Command, args []string) error {
			activeSource, _ := state.GetString(stateSourceKey)
			if activeSource == "" {
				activeSource = "(default)"
			}

			prettyLog.InfoPretty("Available Rule Sets:")

			entries, err := os.ReadDir(rulesDir)
			if err != nil {
				if os.IsNotExist(err) {
					prettyLog.InfoPretty(fmt.Sprintf("  No rule sets found in %s/ directory.", rulesDir))
					return nil
				}
				return fmt.Errorf("error reading %s directory: %w", rulesDir, err)
			}

			found := false
			for _, entry := range entries {
				if !entry.IsDir() && strings.HasSuffix(entry.Name(), rulesExt) {
					found = true
					name := strings.TrimSuffix(entry.Name(), rulesExt)
					path := filepath.Join(rulesDir, entry.Name())

					indicator := "  "
					if path == activeSource {
						indicator = "✓ "
					}

					prettyLog.InfoPretty(fmt.Sprintf("%s%s", indicator, name))
				}
			}

			if !found {
				prettyLog.InfoPretty(fmt.Sprintf("  No rule sets found in %s/ directory.", rulesDir))
			}

			prettyLog.Blank()
			prettyLog.InfoPretty(fmt.Sprintf("Active Source: %s", activeSource))
			return nil
		},
	}
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
