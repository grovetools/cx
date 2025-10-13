package view

import (
	"fmt"
	"io"

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
}

func NewListPage(state *sharedState) Page {
	l := list.New([]list.Item{}, itemDelegate{}, 0, 0)
	l.Title = "Files in Hot Context"
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(true)
	return &listPage{
		sharedState: state,
		list:        l,
	}
}

func (p *listPage) Name() string { return "list" }

func (p *listPage) Keys() interface{} {
	return pagerKeys
}

func (p *listPage) Init() tea.Cmd { return nil }

func (p *listPage) Focus() tea.Cmd {
	var items []list.Item
	// Create a map of file paths to their stats for quick lookup
	fileStats := make(map[string]context.FileStats)
	if p.sharedState.hotStats != nil {
		for _, fs := range p.sharedState.hotStats.LargestFiles { // LargestFiles contains all files sorted
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
	p.list, cmd = p.list.Update(msg)
	return p, cmd
}

func (p *listPage) View() string {
	return lipgloss.NewStyle().
		Width(p.width).
		Height(p.height - 5).
		Render(p.list.View())
}
