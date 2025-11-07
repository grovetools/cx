package rules

import (
	"fmt"
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/mattsolo1/grove-context/pkg/context"
)

type loadCompleteMsg struct {
	err error
}

type clearLoadMsg struct{}

func clearLoadCmd() tea.Cmd {
	return tea.Tick(time.Second*2, func(t time.Time) tea.Msg {
		return clearLoadMsg{}
	})
}

type setCompleteMsg struct {
	err error
}

type clearSetMsg struct{}

func clearSetCmd() tea.Cmd {
	return tea.Tick(time.Second*2, func(t time.Time) tea.Msg {
		return clearSetMsg{}
	})
}

type saveCompleteMsg struct {
	err error
}

type clearSaveMsg struct{}

func clearSaveCmd() tea.Cmd {
	return tea.Tick(time.Second*2, func(t time.Time) tea.Msg {
		return clearSaveMsg{}
	})
}

type deleteCompleteMsg struct {
	err error
}

type clearDeleteMsg struct{}

func clearDeleteCmd() tea.Cmd {
	return tea.Tick(time.Second*2, func(t time.Time) tea.Msg {
		return clearDeleteMsg{}
	})
}

func (m *rulesPickerModel) Init() tea.Cmd {
	return loadRulesCmd
}

func (m *rulesPickerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case loadCompleteMsg:
		if msg.err != nil {
			m.err = msg.err
			m.loadingActive = false
			m.statusMessage = ""
			return m, nil
		}
		// Successfully loaded - update state and show completion
		m.loadingActive = false
		m.loadingComplete = true

		// Set status message
		if m.loadingFromIdx >= 0 && m.loadingFromIdx < len(m.items) {
			m.statusMessage = fmt.Sprintf("Loaded '%s' to .grove/rules as working copy", m.items[m.loadingFromIdx].name)
		}

		// Reload the rules to reflect the new state
		return m, tea.Batch(clearLoadCmd(), loadRulesCmd)

	case clearLoadMsg:
		m.loadingComplete = false
		m.statusMessage = ""
		return m, nil

	case setCompleteMsg:
		if msg.err != nil {
			m.err = msg.err
			m.settingActive = false
			m.statusMessage = ""
			return m, nil
		}
		// Successfully set - update state and show completion
		m.settingActive = false
		m.settingComplete = true

		// Set status message
		if m.settingIdx >= 0 && m.settingIdx < len(m.items) {
			itemName := m.items[m.settingIdx].name
			if m.items[m.settingIdx].path == context.ActiveRulesFile {
				m.statusMessage = "Using .grove/rules (default)"
			} else {
				m.statusMessage = fmt.Sprintf("Set '%s' as active (read-only)", itemName)
			}
		}

		// Reload the rules to reflect the new state
		return m, tea.Batch(clearSetCmd(), loadRulesCmd)

	case clearSetMsg:
		m.settingComplete = false
		m.statusMessage = ""
		return m, nil

	case saveCompleteMsg:
		if msg.err != nil {
			m.err = msg.err
			m.statusMessage = ""
			return m, nil
		}
		// Successfully saved
		m.statusMessage = fmt.Sprintf("Saved to %s", m.saveInput.Value()+".rules")

		// Reload the rules to show the new file
		return m, tea.Batch(clearSaveCmd(), loadRulesCmd)

	case clearSaveMsg:
		m.statusMessage = ""
		return m, nil

	case deleteCompleteMsg:
		if msg.err != nil {
			m.err = msg.err
			m.deletingActive = false
			m.statusMessage = ""
			return m, nil
		}
		// Successfully deleted
		m.deletingActive = false
		m.deletingComplete = true

		if m.deletingIdx >= 0 && m.deletingIdx < len(m.items) {
			m.statusMessage = fmt.Sprintf("Deleted '%s'", m.items[m.deletingIdx].name)
		}

		// Reload the rules to reflect deletion
		return m, tea.Batch(clearDeleteCmd(), loadRulesCmd)

	case clearDeleteMsg:
		m.deletingComplete = false
		m.statusMessage = ""
		return m, nil

	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.updatePreviewSize()
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
		// Set initial preview content
		if len(m.items) > 0 {
			selectedItem := m.items[m.selectedIndex]
			styledContent := styleRulesContent(selectedItem.content)
			m.preview.SetContent(styledContent)
		}
		return m, nil

	case tea.KeyMsg:
		// Handle save mode separately
		if m.saveMode {
			switch {
			case key.Matches(msg, m.keys.Quit):
				// Exit save mode
				m.saveMode = false
				m.saveInput.Reset()
				return m, nil
			case msg.Type == tea.KeyEnter:
				// Perform save
				name := m.saveInput.Value()
				if name != "" {
					m.saveMode = false
					return m, performSaveCmd(name, m.saveToWork)
				}
				return m, nil
			default:
				// Update text input
				var cmd tea.Cmd
				m.saveInput, cmd = m.saveInput.Update(msg)
				return m, cmd
			}
		}

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
				m.settingIdx = m.selectedIndex
				m.settingActive = true
				return m, performSetCmd(m.items[m.selectedIndex])
			}
		case key.Matches(msg, m.keys.Load):
			if len(m.items) > 0 && m.selectedIndex < len(m.items) {
				// Can't load .grove/rules into itself
				if m.items[m.selectedIndex].path == context.ActiveRulesFile {
					return m, nil
				}

				// Find .grove/rules index
				groveRulesIdx := -1
				for i, item := range m.items {
					if item.path == context.ActiveRulesFile {
						groveRulesIdx = i
						break
					}
				}

				m.loadingFromIdx = m.selectedIndex
				m.loadingToIdx = groveRulesIdx
				m.loadingActive = true
				return m, performLoadCmd(m.items[m.selectedIndex])
			}
		case key.Matches(msg, m.keys.Edit):
			if len(m.items) > 0 && m.selectedIndex < len(m.items) {
				return m, editRuleCmd(m.items[m.selectedIndex])
			}
		case key.Matches(msg, m.keys.Save):
			// Enter save mode
			m.saveMode = true
			m.saveToWork = false // TODO: add option to toggle this
			m.saveInput.Reset()
			m.saveInput.Focus()
			return m, nil
		case key.Matches(msg, m.keys.Delete):
			if len(m.items) > 0 && m.selectedIndex < len(m.items) {
				// Can't delete .grove/rules
				if m.items[m.selectedIndex].path == context.ActiveRulesFile {
					m.statusMessage = "Cannot delete .grove/rules"
					return m, clearSaveCmd()
				}

				m.deletingIdx = m.selectedIndex
				m.deletingActive = true
				return m, performDeleteCmd(m.items[m.selectedIndex])
			}
		case key.Matches(msg, m.keys.Up):
			if m.selectedIndex > 0 {
				m.selectedIndex--
			}
			if len(m.items) > 0 {
				selectedItem := m.items[m.selectedIndex]
				styledContent := styleRulesContent(selectedItem.content)
				m.preview.SetContent(styledContent)
				m.preview.GotoTop()
				m.updatePreviewSize()
			}
		case key.Matches(msg, m.keys.Down):
			if m.selectedIndex < len(m.items)-1 {
				m.selectedIndex++
			}
			if len(m.items) > 0 {
				selectedItem := m.items[m.selectedIndex]
				styledContent := styleRulesContent(selectedItem.content)
				m.preview.SetContent(styledContent)
				m.preview.GotoTop()
				m.updatePreviewSize()
			}
		case key.Matches(msg, m.keys.Help):
			m.help.ShowAll = !m.help.ShowAll
			return m, nil
		}
	}

	// Update preview viewport
	var cmd tea.Cmd
	m.preview, cmd = m.preview.Update(msg)
	return m, cmd
}
