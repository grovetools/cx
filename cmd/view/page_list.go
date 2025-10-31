package view

import (
	"fmt"
	"io"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/mattsolo1/grove-context/pkg/context"
)

type listItem struct {
	path   string
	tokens int
}

func (i listItem) Title() string       { return i.path }
func (i listItem) Description() string { return fmt.Sprintf("~%s tokens", context.FormatTokenCount(i.tokens)) }
func (i listItem) FilterValue() string { return i.path }

type itemDelegate struct{}

func (d itemDelegate) Height() int                               { return 1 }
func (d itemDelegate) Spacing() int                              { return 0 }
func (d itemDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil }
func (d itemDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	i, ok := item.(listItem)
	if !ok {
		return
	}

	str := i.Title()
	desc := i.Description()

	fn := lipgloss.NewStyle().Padding(0, 0, 0, 2)
	if index == m.Index() {
		str = "> " + str
	} else {
		str = "  " + str
	}

	fmt.Fprint(w, fn.Render(str)+" "+lipgloss.NewStyle().Faint(true).Render(desc))
}

type listPage struct {
	sharedState *sharedState
	list        list.Model
	width       int
	height      int
	keys        pagerKeyMap
	lastKey     string // Track last key for gg handling
}

func NewListPage(state *sharedState) Page {
	l := list.New([]list.Item{}, itemDelegate{}, 0, 0)
	l.Title = "Files in Hot Context"
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(true)
	return &listPage{
		sharedState: state,
		list:        l,
		keys:        pagerKeys,
	}
}

func (p *listPage) Name() string { return "list" }

func (p *listPage) Keys() interface{} {
	return p.keys
}

func (p *listPage) Init() tea.Cmd { return nil }

func (p *listPage) Focus() tea.Cmd {
	var items []list.Item
	// Create a map of file paths to their stats for quick lookup
	fileStats := make(map[string]context.FileStats)
	if p.sharedState.hotStats != nil {
		// AllFiles contains stats for every file
		for _, fs := range p.sharedState.hotStats.AllFiles {
			fileStats[fs.Path] = fs
		}
	}

	for _, path := range p.sharedState.hotFiles {
		tokens := 0
		if stat, ok := fileStats[path]; ok {
			tokens = stat.Tokens
		}
		items = append(items, listItem{path: path, tokens: tokens})
	}

	p.list.SetItems(items)
	return nil
}

func (p *listPage) Blur() {}

func (p *listPage) SetSize(width, height int) {
	p.width = width
	p.height = height
	p.list.SetWidth(width)
	p.list.SetHeight(height - 5)
}

func (p *listPage) Update(msg tea.Msg) (Page, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		// When filtering, pass keys to list for handling
		if p.list.FilterState() == list.Filtering {
			break // Let the default list update handle it
		}

		switch {
		case key.Matches(msg, p.keys.GotoTop):
			// Handle 'gg' - go to top
			if p.lastKey == "g" {
				p.list.Select(0)
				p.lastKey = ""
				return p, nil
			}
			p.lastKey = "g"
			return p, nil
		case key.Matches(msg, p.keys.GotoBottom):
			// Handle 'G' - go to bottom
			p.list.Select(len(p.list.Items()) - 1)
			p.lastKey = ""
			return p, nil
		case key.Matches(msg, p.keys.HalfPageUp):
			// Handle Ctrl-u - half page up
			current := p.list.Index()
			halfPage := p.list.Height() / 2
			newIndex := current - halfPage
			if newIndex < 0 {
				newIndex = 0
			}
			p.list.Select(newIndex)
			p.lastKey = ""
			return p, nil
		case key.Matches(msg, p.keys.HalfPageDown):
			// Handle Ctrl-d - half page down
			current := p.list.Index()
			halfPage := p.list.Height() / 2
			newIndex := current + halfPage
			maxIndex := len(p.list.Items()) - 1
			if newIndex > maxIndex {
				newIndex = maxIndex
			}
			p.list.Select(newIndex)
			p.lastKey = ""
			return p, nil
		default:
			// Reset lastKey on any other key
			p.lastKey = ""
		}
	}

	p.list, cmd = p.list.Update(msg)
	return p, cmd
}

func (p *listPage) View() string {
	return lipgloss.NewStyle().
		Width(p.width).
		Height(p.height - 5).
		Render(p.list.View())
}
