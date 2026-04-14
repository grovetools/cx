package view

import (
	gocontext "context"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/grovetools/core/pkg/daemon"
	"github.com/grovetools/core/pkg/models"
	"github.com/grovetools/core/tui/theme"
	"github.com/grovetools/cx/pkg/context"
)

// suggestionsPage queries the daemon's memory store for files semantically
// related to the current rules, then filters out files already included in
// context. The remainder are rendered as add-able suggestions.
type suggestionsPage struct {
	sharedState *sharedState
	width       int
	height      int

	suggestions []models.MemorySearchResult
	cursor      int
	loading     bool
	err         error
	statusMsg   string
}

// suggestionsRefreshedMsg carries the filtered suggestions back from the async command.
type suggestionsRefreshedMsg struct {
	suggestions []models.MemorySearchResult
	err         error
}

func NewSuggestionsPage(state *sharedState) Page {
	return &suggestionsPage{
		sharedState: state,
		loading:     true,
	}
}

func (p *suggestionsPage) Name() string  { return "suggestions" }
func (p *suggestionsPage) TabID() string { return "suggestions" }

func (p *suggestionsPage) Init() tea.Cmd {
	return p.refreshSuggestionsCmd()
}

func (p *suggestionsPage) Focus() tea.Cmd {
	return p.refreshSuggestionsCmd()
}

func (p *suggestionsPage) Blur() {}

func (p *suggestionsPage) SetSize(width, height int) {
	p.width = width
	p.height = height
}

func (p *suggestionsPage) Ready() (bool, string) {
	if p.loading {
		return false, "Searching memory…"
	}
	return true, ""
}

func (p *suggestionsPage) refreshSuggestionsCmd() tea.Cmd {
	workDir := p.sharedState.workDir
	rulesContent := p.sharedState.rulesContent
	return func() tea.Msg {
		client := daemon.NewWithAutoStart()

		// Use the rules content as the search query so results are
		// semantically related to the current context definition.
		query := rulesContent
		if query == "" {
			query = workDir
		}
		// Truncate very long queries to a reasonable size for the search API.
		if len(query) > 500 {
			query = query[:500]
		}

		results, err := client.SearchMemory(gocontext.Background(), models.MemorySearchRequest{
			Query:     query,
			Limit:     50,
			UseFTS:    true,
			UseVector: true,
		})
		if err != nil {
			return suggestionsRefreshedMsg{err: err}
		}

		// Get included files map to filter out already-included results.
		mgr := context.NewManager(workDir)
		classified, err := mgr.ClassifyAllProjectFiles(false)
		if err != nil {
			return suggestionsRefreshedMsg{err: err}
		}

		var filtered []models.MemorySearchResult
		for _, r := range results {
			if r.Path == "" {
				continue
			}
			status, found := classified[r.Path]
			if found && (status == context.StatusIncludedHot || status == context.StatusIncludedCold) {
				continue
			}
			filtered = append(filtered, r)
		}

		return suggestionsRefreshedMsg{suggestions: filtered}
	}
}

func (p *suggestionsPage) Update(msg tea.Msg) (Page, tea.Cmd) {
	switch msg := msg.(type) {
	case suggestionsRefreshedMsg:
		p.loading = false
		p.err = msg.err
		p.suggestions = msg.suggestions
		p.cursor = 0
		p.statusMsg = ""
		return p, nil

	case tea.KeyMsg:
		if p.loading {
			return p, nil
		}

		switch {
		case key.Matches(msg, key.NewBinding(key.WithKeys("j", "down"))):
			if p.cursor < len(p.suggestions)-1 {
				p.cursor++
			}
		case key.Matches(msg, key.NewBinding(key.WithKeys("k", "up"))):
			if p.cursor > 0 {
				p.cursor--
			}
		case key.Matches(msg, key.NewBinding(key.WithKeys("G"))):
			if len(p.suggestions) > 0 {
				p.cursor = len(p.suggestions) - 1
			}
		case key.Matches(msg, key.NewBinding(key.WithKeys("g"))):
			p.cursor = 0
		case key.Matches(msg, key.NewBinding(key.WithKeys("ctrl+d"))):
			half := p.height / 2
			p.cursor += half
			if p.cursor >= len(p.suggestions) {
				p.cursor = len(p.suggestions) - 1
			}
			if p.cursor < 0 {
				p.cursor = 0
			}
		case key.Matches(msg, key.NewBinding(key.WithKeys("ctrl+u"))):
			half := p.height / 2
			p.cursor -= half
			if p.cursor < 0 {
				p.cursor = 0
			}
		case key.Matches(msg, key.NewBinding(key.WithKeys("enter", "a", "alt+a"))):
			return p, p.addSuggestionCmd()
		case key.Matches(msg, key.NewBinding(key.WithKeys("ctrl+r"))):
			p.loading = true
			return p, p.refreshSuggestionsCmd()
		}
	}

	return p, nil
}

func (p *suggestionsPage) addSuggestionCmd() tea.Cmd {
	if len(p.suggestions) == 0 || p.cursor >= len(p.suggestions) {
		return nil
	}
	selected := p.suggestions[p.cursor]
	workDir := p.sharedState.workDir
	return func() tea.Msg {
		mgr := context.NewManager(workDir)
		if err := mgr.AppendRule(selected.Path, "hot"); err != nil {
			return suggestionsRefreshedMsg{err: fmt.Errorf("failed to add %s: %w", selected.Path, err)}
		}
		return refreshStateMsg{}
	}
}

func (p *suggestionsPage) View() string {
	if p.err != nil {
		return fmt.Sprintf("Error: %v", p.err)
	}
	if len(p.suggestions) == 0 {
		return "No suggestions found. All memory results are already in context."
	}

	themeStyle := theme.DefaultTheme

	var b strings.Builder

	// Compute visible window based on height
	visibleHeight := p.height - 2 // reserve space for footer
	if visibleHeight < 1 {
		visibleHeight = 1
	}

	// Calculate scroll offset to keep cursor visible
	scrollOffset := 0
	if p.cursor >= visibleHeight {
		scrollOffset = p.cursor - visibleHeight + 1
	}

	end := scrollOffset + visibleHeight
	if end > len(p.suggestions) {
		end = len(p.suggestions)
	}

	for i := scrollOffset; i < end; i++ {
		s := p.suggestions[i]

		// Build display line
		pathInfo := p.sharedState.getDisplayPathInfo(s.Path)
		var pathStr string
		if pathInfo.ecosystem != "" {
			pathStr += themeStyle.Accent.Render(pathInfo.ecosystem)
		}
		if pathInfo.repo != "" {
			pathStr += themeStyle.Highlight.Render(pathInfo.repo)
		}
		pathStr += pathInfo.path

		score := lipgloss.NewStyle().Faint(true).Render(fmt.Sprintf("%.2f", s.Score))

		// Snippet: first 60 chars of content, single line
		snippet := strings.ReplaceAll(s.Content, "\n", " ")
		if len(snippet) > 60 {
			snippet = snippet[:60] + "…"
		}
		snippet = lipgloss.NewStyle().Faint(true).Render(snippet)

		if i == p.cursor {
			fmt.Fprintf(&b, "  %s %s %s %s\n", theme.IconArrowRightBold, pathStr, score, snippet)
		} else {
			fmt.Fprintf(&b, "    %s %s %s\n", pathStr, score, snippet)
		}
	}

	// Footer
	footer := lipgloss.NewStyle().Faint(true).Render(
		fmt.Sprintf("  %d/%d  enter/a: add to context  ctrl+r: refresh", p.cursor+1, len(p.suggestions)),
	)
	if p.statusMsg != "" {
		footer = "  " + p.statusMsg
	}
	b.WriteString(footer)

	return b.String()
}
