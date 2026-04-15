package rules

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/grovetools/core/tui/embed"
	"github.com/grovetools/cx/pkg/context"
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
	return m.loadRulesCmd
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
			m.statusMessage = fmt.Sprintf("Loaded '%s' to %s as working copy", m.items[m.loadingFromIdx].name, m.manager.ResolveRulesWritePath())
		}

		// Reload the rules to reflect the new state and notify host.
		return m, tea.Batch(clearLoadCmd(), m.loadRulesCmd, func() tea.Msg { return RulesWrittenMsg{} })

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

		// Reload the rules to reflect the new state and notify host.
		return m, tea.Batch(clearSetCmd(), m.loadRulesCmd, func() tea.Msg { return RulesWrittenMsg{} })

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

		// Reload the rules to show the new file and notify host.
		return m, tea.Batch(clearSaveCmd(), m.loadRulesCmd, func() tea.Msg { return RulesWrittenMsg{} })

	case clearSaveMsg:
		m.statusMessage = ""
		return m, nil

	case deleteCompleteMsg:
		if msg.err != nil {
			m.err = msg.err
			m.deletingActive = false
			m.deleteConfirmNeeded = false
			m.statusMessage = ""
			return m, nil
		}
		// Successfully deleted
		m.deletingActive = false
		m.deletingComplete = true
		m.deleteConfirmNeeded = false

		if m.deletingIdx >= 0 && m.deletingIdx < len(m.items) {
			m.statusMessage = fmt.Sprintf("Deleted '%s'", m.items[m.deletingIdx].name)
		}

		// Reload the rules to reflect deletion and notify host.
		return m, tea.Batch(clearDeleteCmd(), m.loadRulesCmd, func() tea.Msg { return RulesWrittenMsg{} })

	case clearDeleteMsg:
		m.deletingComplete = false
		m.statusMessage = ""
		return m, nil

	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.updatePreviewSize()
		return m, nil

	case embed.EditFinishedMsg:
		// Editor closed; reload the rules so the preview reflects any edits.
		if msg.Err != nil {
			m.err = msg.Err
		}
		return m, m.loadRulesCmd

	case embed.SetWorkspaceMsg:
		// Host switched workspace context; create a new Manager for
		// the new workspace and reload the rules list.
		if msg.Node != nil {
			m.manager = context.NewManager(msg.Node.Path)
		}
		return m, m.loadRulesCmd

	case rulesLoadedMsg:
		if msg.err != nil {
			m.err = msg.err
			return m, func() tea.Msg { return embed.DoneMsg{Err: msg.err} }
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
					return m, m.performSaveCmd(name, m.saveToWork)
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
			return m, func() tea.Msg { return embed.CloseRequestMsg{} }
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
				return m, m.performLoadCmd(m.items[m.selectedIndex])
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

				// Check if this is a version-controlled file
				isVersionControlled := filepath.Dir(m.items[m.selectedIndex].path) == context.RulesDir

				// If confirmation is needed and this is the same file, proceed with delete
				if m.deleteConfirmNeeded && m.deleteConfirmIdx == m.selectedIndex {
					m.deleteConfirmNeeded = false
					m.deletingIdx = m.selectedIndex
					m.deletingActive = true
					return m, performDeleteCmd(m.items[m.selectedIndex], true) // force=true
				}

				// If version-controlled, require confirmation
				if isVersionControlled {
					m.deleteConfirmNeeded = true
					m.deleteConfirmIdx = m.selectedIndex
					m.statusMessage = fmt.Sprintf("'%s' is version-controlled. Press 'd' again to confirm deletion", m.items[m.selectedIndex].name)
					return m, clearSaveCmd()
				}

				// Not version-controlled, delete immediately
				m.deletingIdx = m.selectedIndex
				m.deletingActive = true
				return m, performDeleteCmd(m.items[m.selectedIndex], false)
			}
		case key.Matches(msg, m.keys.Up):
			if m.selectedIndex > 0 {
				m.selectedIndex--
			}
			// Clear delete confirmation when navigating away
			m.deleteConfirmNeeded = false
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
			// Clear delete confirmation when navigating away
			m.deleteConfirmNeeded = false
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
