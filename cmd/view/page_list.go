package view

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/mattsolo1/grove-context/pkg/context"
	"github.com/mattsolo1/grove-core/tui/theme"
)

type listItem struct {
	realPath  string
	ecosystem string // Ecosystem context (e.g., "grove-ecosystem/")
	repo      string // Repository name (e.g., "grove-core/")
	path      string // File path relative to repo
	tokens    int
	isLocal   bool // True if file is in CWD, false if external/aliased
	isDir     bool // True if this represents a directory
	fileCount int  // Number of files in directory (only for isDir=true)
}

func (i listItem) Title() string { return i.ecosystem + i.repo + i.path }
func (i listItem) Description() string {
	if i.isDir {
		return fmt.Sprintf("(%d files, ~%s tokens)", i.fileCount, context.FormatTokenCount(i.tokens))
	}
	return fmt.Sprintf("~%s tokens", context.FormatTokenCount(i.tokens))
}
func (i listItem) FilterValue() string { return i.realPath }

// Sort constants
const (
	SortAlphanumeric = iota
	SortByTokens
)

type itemDelegate struct{}

func (d itemDelegate) Height() int                               { return 1 }
func (d itemDelegate) Spacing() int                              { return 0 }
func (d itemDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil }

// highlight returns a styled string with occurrences of substr highlighted.
func highlight(s, substr string, baseStyle, highlightStyle lipgloss.Style) string {
	if substr == "" || !strings.Contains(strings.ToLower(s), strings.ToLower(substr)) {
		return baseStyle.Render(s)
	}

	// Case-insensitive search
	lowerS := strings.ToLower(s)
	lowerSubstr := strings.ToLower(substr)

	var result strings.Builder
	lastIndex := 0

	for {
		index := strings.Index(lowerS[lastIndex:], lowerSubstr)
		if index == -1 {
			result.WriteString(baseStyle.Render(s[lastIndex:]))
			break
		}

		actualIndex := lastIndex + index
		result.WriteString(baseStyle.Render(s[lastIndex:actualIndex]))
		result.WriteString(highlightStyle.Render(s[actualIndex : actualIndex+len(substr)]))
		lastIndex = actualIndex + len(substr)
	}

	return result.String()
}

func (d itemDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	i, ok := item.(listItem)
	if !ok {
		return
	}

	theme := theme.DefaultTheme

	// Add folder icon for directories
	var prefix string
	if i.isDir {
		prefix = "📁 "
	}

	// Build the path string with colored components
	var pathParts []string
	isFiltering := m.FilterState() == list.Filtering
	filter := m.FilterValue()

	if isFiltering && filter != "" {
		highlightStyle := lipgloss.NewStyle().Reverse(true)
		if i.ecosystem != "" {
			pathParts = append(pathParts, highlight(i.ecosystem, filter, theme.Accent, theme.Accent.Copy().Inherit(highlightStyle)))
		}
		if i.repo != "" {
			pathParts = append(pathParts, highlight(i.repo, filter, theme.Highlight, theme.Highlight.Copy().Inherit(highlightStyle)))
		}
		pathParts = append(pathParts, highlight(i.path, filter, lipgloss.NewStyle(), highlightStyle))
	} else {
		if i.ecosystem != "" {
			pathParts = append(pathParts, theme.Accent.Render(i.ecosystem))
		}
		if i.repo != "" {
			pathParts = append(pathParts, theme.Highlight.Render(i.repo))
		}
		// Bold directories
		if i.isDir {
			pathParts = append(pathParts, lipgloss.NewStyle().Bold(true).Render(i.path))
		} else {
			pathParts = append(pathParts, i.path)
		}
	}
	pathStr := prefix + lipgloss.JoinHorizontal(lipgloss.Top, pathParts...)

	desc := i.Description()

	fn := lipgloss.NewStyle().Padding(0, 0, 0, 2)
	if index == m.Index() {
		pathStr = "> " + pathStr
	} else {
		pathStr = "  " + pathStr
	}

	fmt.Fprint(w, fn.Render(pathStr)+" "+lipgloss.NewStyle().Faint(true).Render(desc))
}

type listPage struct {
	sharedState *sharedState
	list        list.Model
	width       int
	height      int
	keys        pagerKeyMap
	lastKey     string            // Track last key for gg handling
	sortMode    int
	foldedDirs  map[string]bool   // Track which directories are folded
}

func NewListPage(state *sharedState) Page {
	l := list.New([]list.Item{}, itemDelegate{}, 0, 0)
	l.Title = ""
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(true)

	// Hide the title completely
	l.Styles.Title = lipgloss.NewStyle()

	return &listPage{
		sharedState: state,
		list:        l,
		keys:        pagerKeys,
		sortMode:    SortAlphanumeric,
		foldedDirs:  make(map[string]bool),
	}
}

func (p *listPage) Name() string { return "list" }

func (p *listPage) Keys() interface{} {
	return p.keys
}

func (p *listPage) Init() tea.Cmd { return nil }

func (p *listPage) Focus() tea.Cmd {
	p.updateAndSortItems()
	return nil
}

func (p *listPage) updateAndSortItems() {
	// Create a map of file paths to their stats for quick lookup
	fileStats := make(map[string]context.FileStats)
	if p.sharedState.hotStats != nil {
		// AllFiles contains stats for every file
		for _, fs := range p.sharedState.hotStats.AllFiles {
			fileStats[fs.Path] = fs
		}
	}

	// Group files by directory
	type dirGroup struct {
		files     []listItem
		tokens    int
		isLocal   bool
		ecosystem string
		repo      string
	}
	dirMap := make(map[string]*dirGroup)

	for _, path := range p.sharedState.hotFiles {
		tokens := 0
		if stat, ok := fileStats[path]; ok {
			tokens = stat.Tokens
		}
		pathInfo := p.sharedState.getDisplayPathInfo(path)
		isLocal := pathInfo.ecosystem == ""

		// Get directory key
		fullDirKey := pathInfo.ecosystem + pathInfo.repo + filepath.Dir(pathInfo.path)

		if dirMap[fullDirKey] == nil {
			dirMap[fullDirKey] = &dirGroup{
				files:     []listItem{},
				tokens:    0,
				isLocal:   isLocal,
				ecosystem: pathInfo.ecosystem,
				repo:      pathInfo.repo,
			}
		}

		file := listItem{
			realPath:  path,
			ecosystem: pathInfo.ecosystem,
			repo:      pathInfo.repo,
			path:      pathInfo.path,
			tokens:    tokens,
			isLocal:   isLocal,
			isDir:     false,
		}

		dirMap[fullDirKey].files = append(dirMap[fullDirKey].files, file)
		dirMap[fullDirKey].tokens += tokens
	}

	// Build items list based on fold state
	var items []list.Item

	// Sort directory keys
	dirKeys := make([]string, 0, len(dirMap))
	for k := range dirMap {
		dirKeys = append(dirKeys, k)
	}
	sort.Strings(dirKeys)

	for _, dirKey := range dirKeys {
		group := dirMap[dirKey]

		// Check if this directory is folded
		if p.foldedDirs[dirKey] {
			// Create a directory item
			// Get the dir path from the first file
			firstFile := group.files[0]
			dirPath := filepath.Dir(firstFile.path)
			if dirPath == "." {
				dirPath = ""
			} else {
				dirPath += "/"
			}

			dirItem := listItem{
				realPath:  filepath.Dir(firstFile.realPath),
				ecosystem: group.ecosystem,
				repo:      group.repo,
				path:      dirPath,
				tokens:    group.tokens,
				isLocal:   group.isLocal,
				isDir:     true,
				fileCount: len(group.files),
			}
			items = append(items, dirItem)
		} else {
			// Add individual files
			for _, file := range group.files {
				items = append(items, file)
			}
		}
	}

	// Sort items based on current mode
	switch p.sortMode {
	case SortByTokens:
		sort.Slice(items, func(i, j int) bool {
			return items[i].(listItem).tokens > items[j].(listItem).tokens
		})
	case SortAlphanumeric:
		// Default sorting is by title, which is already alphanumeric
		sort.Slice(items, func(i, j int) bool {
			return items[i].(listItem).Title() < items[j].(listItem).Title()
		})
	}

	p.list.SetItems(items)
}

func (p *listPage) Blur() {}

func (p *listPage) SetSize(width, height int) {
	p.width = width
	p.height = height
	// The pager passes us the available space after accounting for chrome and padding
	p.list.SetSize(width, height)
}

func (p *listPage) Update(msg tea.Msg) (Page, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		// When filtering, pass keys to list for handling
		if p.list.FilterState() == list.Filtering {
			break // Let the default list update handle it
		}

		// Handle fold commands when lastKey is 'z'
		if p.lastKey == "z" {
			switch msg.String() {
			case "a":
				// za - toggle fold on current item
				p.toggleFold()
				p.lastKey = ""
				return p, nil
			case "o":
				// zo - open fold on current item
				p.openFold()
				p.lastKey = ""
				return p, nil
			case "c":
				// zc - close fold on current item
				p.closeFold()
				p.lastKey = ""
				return p, nil
			case "R":
				// zR - open all folds
				p.openAllFolds()
				p.lastKey = ""
				return p, nil
			case "M":
				// zM - close all folds
				p.closeAllFolds()
				p.lastKey = ""
				return p, nil
			default:
				p.lastKey = ""
				return p, nil
			}
		}

		switch {
		case key.Matches(msg, p.keys.Exclude):
			p.lastKey = ""
			return p, p.excludeFileCmd()
		case key.Matches(msg, p.keys.ExcludeDir):
			p.lastKey = ""
			return p, p.excludeDirCmd()
		case key.Matches(msg, p.keys.Refresh):
			p.lastKey = ""
			return p, p.refreshCmd()
		case key.Matches(msg, p.keys.ToggleSort):
			p.lastKey = ""
			p.sortMode = (p.sortMode + 1) % 2 // Cycle between 0 and 1
			p.updateAndSortItems()
			return p, nil
		case key.Matches(msg, p.keys.FoldPrefix):
			// Set lastKey to 'z' and wait for next key
			p.lastKey = "z"
			return p, nil
		case key.Matches(msg, p.keys.GotoTop):
			// Handle 'gg' - go to top
			if p.lastKey == "g" {
				if len(p.list.Items()) > 0 {
					p.list.Select(0)
				}
				p.lastKey = ""
				return p, nil
			}
			p.lastKey = "g"
			return p, nil
		case key.Matches(msg, p.keys.GotoBottom):
			// Handle 'G' - go to bottom
			if len(p.list.Items()) > 0 {
				p.list.Select(len(p.list.Items()) - 1)
			}
			p.lastKey = ""
			return p, nil
		case key.Matches(msg, p.keys.HalfPageUp):
			// Handle Ctrl-u - half page up
			if len(p.list.Items()) > 0 {
				current := p.list.Index()
				halfPage := p.list.Height() / 2
				newIndex := current - halfPage
				if newIndex < 0 {
					newIndex = 0
				}
				p.list.Select(newIndex)
			}
			p.lastKey = ""
			return p, nil
		case key.Matches(msg, p.keys.HalfPageDown):
			// Handle Ctrl-d - half page down
			if len(p.list.Items()) > 0 {
				current := p.list.Index()
				halfPage := p.list.Height() / 2
				newIndex := current + halfPage
				maxIndex := len(p.list.Items()) - 1
				if newIndex > maxIndex {
					newIndex = maxIndex
				}
				if newIndex < 0 {
					newIndex = 0
				}
				p.list.Select(newIndex)
			}
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
	// The pager handles padding; this page just returns the rendered list.
	return p.list.View()
}

// excludeFileCmd creates a command to exclude the selected file from context
func (p *listPage) excludeFileCmd() tea.Cmd {
	if item, ok := p.list.SelectedItem().(listItem); ok {
		return func() tea.Msg {
			mgr := context.NewManager("")

			var rule string
			if item.ecosystem != "" {
				// Aliased path - use @a:ecosystem:repo/path format
				ecosystem := strings.TrimSuffix(item.ecosystem, "/")
				repo := strings.TrimSuffix(item.repo, "/")
				path := filepath.ToSlash(strings.TrimSuffix(item.path, "/"))

				if path == "" {
					rule = "@a:" + ecosystem + ":" + repo
				} else {
					rule = "@a:" + ecosystem + ":" + repo + "/" + path
				}

				if item.isDir {
					rule = rule + "/**"
				}
			} else {
				// Local path - use real path
				if item.isDir {
					rule = filepath.ToSlash(strings.TrimSuffix(item.realPath, "/")) + "/**"
				} else {
					rule = item.realPath
				}
			}

			// AppendRule adds the '!' prefix internally for "exclude" type
			if err := mgr.AppendRule(rule, "exclude"); err != nil {
				fmt.Fprintf(os.Stderr, "Error excluding %s: %v\n", rule, err)
				return nil
			}
			// Signal the main model to refresh all data
			return refreshStateMsg{}
		}
	}
	return nil
}

// excludeDirCmd creates a command to exclude the selected file's directory from context
func (p *listPage) excludeDirCmd() tea.Cmd {
	if item, ok := p.list.SelectedItem().(listItem); ok {
		return func() tea.Msg {
			mgr := context.NewManager("")

			var rule string
			if item.ecosystem != "" {
				// Aliased path - use @a:ecosystem:repo/path format
				dirPath := filepath.ToSlash(filepath.Dir(item.path))
				if dirPath == "." {
					dirPath = ""
				}

				ecosystem := strings.TrimSuffix(item.ecosystem, "/")
				repo := strings.TrimSuffix(item.repo, "/")

				if dirPath == "" {
					rule = "@a:" + ecosystem + ":" + repo + "/**"
				} else {
					rule = "@a:" + ecosystem + ":" + repo + "/" + dirPath + "/**"
				}
			} else {
				// Local path - use real path
				dir := filepath.ToSlash(filepath.Dir(item.realPath))
				rule = dir + "/**"
			}

			// AppendRule adds the '!' prefix internally for "exclude" type
			if err := mgr.AppendRule(rule, "exclude"); err != nil {
				fmt.Fprintf(os.Stderr, "Error excluding %s: %v\n", rule, err)
				return nil
			}
			// Signal the main model to refresh all data
			return refreshStateMsg{}
		}
	}
	return nil
}

// refreshCmd creates a command to refresh the context data
func (p *listPage) refreshCmd() tea.Cmd {
	return func() tea.Msg {
		return refreshStateMsg{}
	}
}

// getDirKeyForItem returns the directory key for a given item
func (p *listPage) getDirKeyForItem(item listItem) string {
	var dirPath string
	if item.isDir {
		// Remove trailing slash if present
		dirPath = strings.TrimSuffix(item.path, "/")
	} else {
		dirPath = filepath.Dir(item.path)
	}
	return item.ecosystem + item.repo + dirPath
}

// toggleFold toggles the fold state of the current directory
func (p *listPage) toggleFold() {
	if item, ok := p.list.SelectedItem().(listItem); ok {
		dirKey := p.getDirKeyForItem(item)
		p.foldedDirs[dirKey] = !p.foldedDirs[dirKey]
		p.updateAndSortItems()
	}
}

// openFold opens (unfolds) the current directory
func (p *listPage) openFold() {
	if item, ok := p.list.SelectedItem().(listItem); ok {
		dirKey := p.getDirKeyForItem(item)
		p.foldedDirs[dirKey] = false
		p.updateAndSortItems()
	}
}

// closeFold closes (folds) the current directory
func (p *listPage) closeFold() {
	if item, ok := p.list.SelectedItem().(listItem); ok {
		dirKey := p.getDirKeyForItem(item)
		p.foldedDirs[dirKey] = true
		p.updateAndSortItems()
	}
}

// openAllFolds opens all directory folds
func (p *listPage) openAllFolds() {
	p.foldedDirs = make(map[string]bool)
	p.updateAndSortItems()
}

// closeAllFolds closes all directory folds
func (p *listPage) closeAllFolds() {
	// Mark all directories as folded
	for _, path := range p.sharedState.hotFiles {
		pathInfo := p.sharedState.getDisplayPathInfo(path)
		dirKey := pathInfo.ecosystem + pathInfo.repo + filepath.Dir(pathInfo.path)
		p.foldedDirs[dirKey] = true
	}
	p.updateAndSortItems()
}
