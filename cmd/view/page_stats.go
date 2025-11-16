package view

import (
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/mattsolo1/grove-context/pkg/context"
	core_theme "github.com/mattsolo1/grove-core/tui/theme"
)

// focus constants
const (
	langListIndex = iota
	fileListIndex
)

// --- list items ---

type languageItem struct {
	context.LanguageStats
	contextType      string // "hot", "cold", "both"
	showContextType bool   // only show indicator if cold context exists
}

func (i languageItem) Title() string       { return i.Name }
func (i languageItem) Description() string { return "language" }
func (i languageItem) FilterValue() string { return i.Name }

type fileItem struct {
	context.FileStats
	contextType     string // "hot", "cold", "both"
	showContextType bool   // only show indicator if cold context exists
}

func (i fileItem) Title() string       { return i.Path }
func (i fileItem) Description() string { return "file" }
func (i fileItem) FilterValue() string { return i.Path }

// --- list delegates ---

type languageDelegate struct{}

func (d languageDelegate) Height() int                               { return 1 }
func (d languageDelegate) Spacing() int                              { return 0 }
func (d languageDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil }
func (d languageDelegate) Render(w io.Writer, m list.Model, index int, listItem list.Item) {
	i, ok := listItem.(languageItem)
	if !ok {
		return
	}
	theme := core_theme.DefaultTheme

	var indicator string
	if i.showContextType {
		switch i.contextType {
		case "hot":
			indicator = "(H) "
		case "cold":
			indicator = "(C) "
		case "both":
			indicator = "(B) "
		}
	}

	var parts []string
	name := fmt.Sprintf("%-12s", i.Name)
	percentage := fmt.Sprintf("%5.1f%%", i.Percentage)
	details := fmt.Sprintf("  (~%s tokens, %d files)",
		context.FormatTokenCount(i.TotalTokens),
		i.FileCount,
	)

	if index == m.Index() {
		if indicator != "" {
			parts = append(parts, theme.Bold.Render("> "+name), theme.Success.Render(indicator), theme.Highlight.Render(percentage), details)
		} else {
			parts = append(parts, theme.Bold.Render("> "+name), theme.Highlight.Render(percentage), details)
		}
	} else {
		if indicator != "" {
			parts = append(parts, "  "+name, theme.Success.Render(indicator), theme.Highlight.Render(percentage), details)
		} else {
			parts = append(parts, "  "+name, theme.Highlight.Render(percentage), details)
		}
	}

	fmt.Fprint(w, lipgloss.JoinHorizontal(lipgloss.Bottom, parts...))
}

type fileDelegate struct{}

func (d fileDelegate) Height() int                               { return 1 }
func (d fileDelegate) Spacing() int                              { return 0 }
func (d fileDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil }
func (d fileDelegate) Render(w io.Writer, m list.Model, index int, listItem list.Item) {
	i, ok := listItem.(fileItem)
	if !ok {
		return
	}
	theme := core_theme.DefaultTheme

	var indicator string
	if i.showContextType {
		switch i.contextType {
		case "hot":
			indicator = "(H) "
		case "cold":
			indicator = "(C) "
		case "both":
			indicator = "(B) "
		}
	}

	displayPath := i.Path
	pathWidth := 45
	if i.showContextType {
		pathWidth = 41 // Make room for indicator
	}
	if len(displayPath) > pathWidth {
		displayPath = "..." + displayPath[len(displayPath)-(pathWidth-3):]
	}

	var tokenStyle lipgloss.Style
	if i.Tokens > 10000 {
		tokenStyle = theme.Error
	} else if i.Tokens > 5000 {
		tokenStyle = theme.Warning
	} else {
		tokenStyle = theme.Info
	}

	details := fmt.Sprintf("%s tokens (%4.1f%%)",
		context.FormatTokenCount(i.Tokens),
		i.Percentage,
	)

	var line string
	if index == m.Index() {
		if indicator != "" {
			line = fmt.Sprintf("> %s%-*s %s", theme.Success.Render(indicator), pathWidth, displayPath, tokenStyle.Render(details))
		} else {
			line = fmt.Sprintf("> %-*s %s", pathWidth, displayPath, tokenStyle.Render(details))
		}
	} else {
		if indicator != "" {
			line = fmt.Sprintf("  %s%-*s %s", theme.Success.Render(indicator), pathWidth, displayPath, tokenStyle.Render(details))
		} else {
			line = fmt.Sprintf("  %-*s %s", pathWidth, displayPath, tokenStyle.Render(details))
		}
	}
	fmt.Fprint(w, line)
}

// --- page model ---

type statsPage struct {
	sharedState *sharedState
	langList    list.Model
	fileList    list.Model
	focusIndex  int
	width       int
	height      int
	keys        statsKeyMap
	lastKey     string // Track last key for gg handling
}

func NewStatsPage(state *sharedState) Page {
	langList := list.New([]list.Item{}, languageDelegate{}, 0, 0)
	langList.SetShowHelp(false)
	langList.SetShowStatusBar(false)
	langList.SetFilteringEnabled(false)
	langList.Title = "File Types"

	fileList := list.New([]list.Item{}, fileDelegate{}, 0, 0)
	fileList.SetShowHelp(false)
	fileList.SetShowStatusBar(false)
	fileList.SetFilteringEnabled(false)
	fileList.Title = "Largest Files (by tokens)"

	return &statsPage{
		sharedState: state,
		langList:    langList,
		fileList:    fileList,
		focusIndex:  langListIndex,
		keys:        statsKeys,
	}
}

func (p *statsPage) Name() string { return "stats" }

func (p *statsPage) Keys() interface{} {
	return p.keys
}

func (p *statsPage) Init() tea.Cmd { return nil }

func (p *statsPage) Focus() tea.Cmd {
	hot := p.sharedState.hotStats
	cold := p.sharedState.coldStats

	if hot == nil && cold == nil {
		p.langList.SetItems(nil)
		p.fileList.SetItems(nil)
		return nil
	}

	// Determine if we should show context indicators (only if cold context exists)
	hasColdContext := cold != nil && cold.TotalFiles > 0

	// --- Aggregate Stats ---
	aggStats := &context.ContextStats{
		Languages: make(map[string]*context.LanguageStats),
	}
	langContext := make(map[string]map[string]bool) // lang -> {"hot": true, "cold": true}
	fileContext := make(map[string]map[string]bool) // file -> {"hot": true, "cold": true}

	// Process hot stats
	if hot != nil {
		aggStats.TotalFiles += hot.TotalFiles
		aggStats.TotalTokens += hot.TotalTokens
		aggStats.TotalSize += hot.TotalSize
		for name, lang := range hot.Languages {
			if _, ok := aggStats.Languages[name]; !ok {
				aggStats.Languages[name] = &context.LanguageStats{Name: name}
				langContext[name] = make(map[string]bool)
			}
			aggStats.Languages[name].FileCount += lang.FileCount
			aggStats.Languages[name].TotalTokens += lang.TotalTokens
			langContext[name]["hot"] = true
		}
		for _, file := range hot.LargestFiles {
			if _, ok := fileContext[file.Path]; !ok {
				fileContext[file.Path] = make(map[string]bool)
			}
			fileContext[file.Path]["hot"] = true
			aggStats.LargestFiles = append(aggStats.LargestFiles, file)
		}
	}

	// Process cold stats
	if cold != nil {
		aggStats.TotalFiles += cold.TotalFiles
		aggStats.TotalTokens += cold.TotalTokens
		aggStats.TotalSize += cold.TotalSize
		for name, lang := range cold.Languages {
			if _, ok := aggStats.Languages[name]; !ok {
				aggStats.Languages[name] = &context.LanguageStats{Name: name}
				langContext[name] = make(map[string]bool)
			}
			aggStats.Languages[name].FileCount += lang.FileCount
			aggStats.Languages[name].TotalTokens += lang.TotalTokens
			langContext[name]["cold"] = true
		}
		for _, file := range cold.LargestFiles {
			if _, ok := fileContext[file.Path]; !ok {
				fileContext[file.Path] = make(map[string]bool)
			}
			fileContext[file.Path]["cold"] = true
			aggStats.LargestFiles = append(aggStats.LargestFiles, file)
		}
	}

	// --- Populate Lists ---
	var langItems []list.Item
	for name, lang := range aggStats.Languages {
		if aggStats.TotalTokens > 0 {
			lang.Percentage = float64(lang.TotalTokens) * 100 / float64(aggStats.TotalTokens)
		}
		var patterns []string
		for _, ext := range context.GetExtsFromLanguage(name) {
			// If ext starts with ".", it's an extension pattern (e.g., ".go" -> "**/*.go")
			// Otherwise, it's a basename pattern (e.g., "Makefile" -> "**/Makefile")
			if strings.HasPrefix(ext, ".") {
				patterns = append(patterns, "**/*"+ext)
			} else {
				patterns = append(patterns, "**/"+ext)
			}
		}
		lang.Patterns = patterns

		contextType := ""
		if langContext[name]["hot"] && langContext[name]["cold"] {
			contextType = "both"
		} else if langContext[name]["hot"] {
			contextType = "hot"
		} else {
			contextType = "cold"
		}
		langItems = append(langItems, languageItem{
			LanguageStats:   *lang,
			contextType:     contextType,
			showContextType: hasColdContext,
		})
	}
	sort.Slice(langItems, func(i, j int) bool {
		return langItems[i].(languageItem).TotalTokens > langItems[j].(languageItem).TotalTokens
	})
	p.langList.SetItems(langItems)

	// De-duplicate, re-sort, and truncate largest files
	uniqueFiles := make(map[string]context.FileStats)
	for _, file := range aggStats.LargestFiles {
		if existing, ok := uniqueFiles[file.Path]; ok {
			// If file is in both, sum tokens/size (shouldn't happen with hot/cold separation, but good practice)
			existing.Tokens += file.Tokens
			existing.Size += file.Size
			uniqueFiles[file.Path] = existing
		} else {
			uniqueFiles[file.Path] = file
		}
	}
	var allFiles []context.FileStats
	for _, file := range uniqueFiles {
		allFiles = append(allFiles, file)
	}
	sort.Slice(allFiles, func(i, j int) bool {
		return allFiles[i].Tokens > allFiles[j].Tokens
	})
	if len(allFiles) > 10 {
		allFiles = allFiles[:10]
	}

	var fileItems []list.Item
	for _, file := range allFiles {
		if aggStats.TotalTokens > 0 {
			file.Percentage = float64(file.Tokens) * 100 / float64(aggStats.TotalTokens)
		}
		contextType := ""
		if fileContext[file.Path]["hot"] && fileContext[file.Path]["cold"] {
			contextType = "both" // Should not happen, but for completeness
		} else if fileContext[file.Path]["hot"] {
			contextType = "hot"
		} else {
			contextType = "cold"
		}
		fileItems = append(fileItems, fileItem{
			FileStats:       file,
			contextType:     contextType,
			showContextType: hasColdContext,
		})
	}
	p.fileList.SetItems(fileItems)

	p.langList.Title = fmt.Sprintf("File Types (~%s tokens, %d files)", context.FormatTokenCount(aggStats.TotalTokens), aggStats.TotalFiles)
	p.fileList.Title = "Largest Files"

	return nil
}

func (p *statsPage) Blur() {
	p.focusIndex = langListIndex
	p.lastKey = ""
}

func (p *statsPage) SetSize(width, height int) {
	p.width = width
	p.height = height
	// The pager passes us the available space. We need to account for:
	// - The focused list's border (2 chars width)
	// - This page's internal help footer (1 line height)
	listWidth := width - 2
	listHeight := (height - 1) / 2
	p.langList.SetSize(listWidth, listHeight)
	p.fileList.SetSize(listWidth, listHeight)
}

func (p *statsPage) Update(msg tea.Msg) (Page, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		// When a list is filtering, pass keys to it for handling
		if p.langList.FilterState() == list.Filtering || p.fileList.FilterState() == list.Filtering {
			break // Let the default list update handle it
		}

		// Get the active list
		activeList := &p.langList
		if p.focusIndex == fileListIndex {
			activeList = &p.fileList
		}

		switch {
		case key.Matches(msg, p.keys.SwitchFocus):
			p.focusIndex = (p.focusIndex + 1) % 2
			p.lastKey = ""
			return p, nil
		case key.Matches(msg, p.keys.Exclude):
			p.lastKey = ""
			return p, p.excludeItemCmd()
		case key.Matches(msg, p.keys.Refresh):
			p.lastKey = ""
			return p, func() tea.Msg { return refreshStateMsg{} }
		case key.Matches(msg, p.keys.GotoTop):
			// Handle 'gg' - go to top
			if p.lastKey == "g" {
				activeList.Select(0)
				p.lastKey = ""
				return p, nil
			}
			p.lastKey = "g"
			return p, nil
		case key.Matches(msg, p.keys.GotoBottom):
			// Handle 'G' - go to bottom
			activeList.Select(len(activeList.Items()) - 1)
			p.lastKey = ""
			return p, nil
		case key.Matches(msg, p.keys.HalfPageUp):
			// Handle Ctrl-u - half page up
			current := activeList.Index()
			halfPage := activeList.Height() / 2
			newIndex := current - halfPage
			if newIndex < 0 {
				newIndex = 0
			}
			activeList.Select(newIndex)
			p.lastKey = ""
			return p, nil
		case key.Matches(msg, p.keys.HalfPageDown):
			// Handle Ctrl-d - half page down
			current := activeList.Index()
			halfPage := activeList.Height() / 2
			newIndex := current + halfPage
			maxIndex := len(activeList.Items()) - 1
			if newIndex > maxIndex {
				newIndex = maxIndex
			}
			activeList.Select(newIndex)
			p.lastKey = ""
			return p, nil
		default:
			// Reset lastKey on any other key
			p.lastKey = ""
		}
	}

	switch p.focusIndex {
	case langListIndex:
		p.langList, cmd = p.langList.Update(msg)
	case fileListIndex:
		p.fileList, cmd = p.fileList.Update(msg)
	}
	cmds = append(cmds, cmd)

	return p, tea.Batch(cmds...)
}

func (p *statsPage) View() string {
	if p.sharedState.hotStats == nil && p.sharedState.coldStats == nil {
		return "No context files found to generate statistics."
	}
	theme := core_theme.DefaultTheme

	// Style for focused vs unfocused list
	listStyle := lipgloss.NewStyle().
		Border(lipgloss.HiddenBorder())

	focusedStyle := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(theme.Colors.Green)

	// Set width and height dynamically before rendering
	listHeight := (p.height - 10) / 2
	p.langList.SetSize(p.width-2, listHeight) // -2 for border
	p.fileList.SetSize(p.width-2, listHeight) // -2 for border

	langView := p.langList.View()
	fileView := p.fileList.View()

	if p.focusIndex == langListIndex {
		langView = focusedStyle.Render(langView)
		fileView = listStyle.Render(fileView)
	} else {
		langView = listStyle.Render(langView)
		fileView = focusedStyle.Render(fileView)
	}

	helpView := p.keys.ShortHelp()
	helpStr := ""
	for i, h := range helpView {
		helpStr += h.Help().Key + " " + h.Help().Desc
		if i < len(helpView)-1 {
			helpStr += " • "
		}
	}

	return lipgloss.JoinVertical(lipgloss.Left, langView, fileView, theme.Muted.Render(helpStr))
}

// excludeItemCmd creates a command to exclude the selected item from context
func (p *statsPage) excludeItemCmd() tea.Cmd {
	var patterns []string

	if p.focusIndex == langListIndex {
		if item, ok := p.langList.SelectedItem().(languageItem); ok {
			patterns = item.Patterns
		}
	} else {
		if item, ok := p.fileList.SelectedItem().(fileItem); ok {
			patterns = []string{item.Path}
		}
	}

	if len(patterns) == 0 {
		return nil
	}

	return func() tea.Msg {
		mgr := context.NewManager("")
		for _, pattern := range patterns {
			// AppendRule adds the '!' prefix internally for "exclude" type
			if err := mgr.AppendRule(pattern, "exclude"); err != nil {
				fmt.Fprintf(os.Stderr, "Error excluding %s: %v\n", pattern, err)
				return nil
			}
		}
		// Signal the main model to refresh all data
		return refreshStateMsg{}
	}
}
