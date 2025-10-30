package rules

import (
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
)

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
