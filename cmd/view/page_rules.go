package view

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/mattsolo1/grove-context/pkg/context"
	core_theme "github.com/mattsolo1/grove-core/tui/theme"
)

type rulesPage struct {
	sharedState *sharedState
	viewport    viewport.Model
	width       int
	height      int
}

func NewRulesPage(state *sharedState) Page {
	vp := viewport.New(0, 0)
	return &rulesPage{
		sharedState: state,
		viewport:    vp,
	}
}

func (p *rulesPage) Name() string { return "rules" }

func (p *rulesPage) Keys() interface{} {
	return pagerKeys
}

func (p *rulesPage) Init() tea.Cmd { return nil }

func (p *rulesPage) Focus() tea.Cmd {
	// Display the actual active rules file path relative to current working directory
	rulesPath := p.sharedState.rulesPath
	if rulesPath == "" {
		rulesPath = ".grove/rules" // Fallback if path is not set
	} else {
		// Convert to relative path
		cwd, err := os.Getwd()
		if err == nil {
			relPath, err := filepath.Rel(cwd, rulesPath)
			if err == nil {
				rulesPath = relPath
			}
		}
	}

	// Create a styled header with label and highlighted path using theme styles
	label := core_theme.DefaultTheme.Muted.Render("Rules File: ")
	path := core_theme.DefaultTheme.Accent.Render(rulesPath)
	header := label + path

	// Apply styling to rules content - make comments muted
	styledContent := styleRulesContent(p.sharedState.rulesContent)
	content := header + "\n\n" + styledContent
	p.viewport.SetContent(content)
	return nil
}

func (p *rulesPage) Blur() {}

// styleRulesContent applies syntax-aware styling to rules content using the parser
func styleRulesContent(content string) string {
	if content == "" {
		return content
	}

	lines := strings.Split(content, "\n")
	styledLines := make([]string, len(lines))

	for i, line := range lines {
		parsed := context.ParseRulesLine(line)
		styledLines[i] = styleLineByType(line, parsed.Type)
	}

	return strings.Join(styledLines, "\n")
}

// styleLineByType applies appropriate styling based on line type
func styleLineByType(line string, lineType context.LineType) string {
	switch lineType {
	case context.LineTypeComment:
		// Comments are muted
		return core_theme.DefaultTheme.Muted.Render(line)

	case context.LineTypeSeparator:
		// Separator (---) is bold/statement
		return core_theme.DefaultTheme.Bold.Render(line)

	case context.LineTypeExclude:
		// Exclusion patterns (!) are highlighted as error/red
		return core_theme.DefaultTheme.Error.Render(line)

	case context.LineTypeGitURL:
		// Git URLs are styled as info/cyan
		return core_theme.DefaultTheme.Info.Render(line)

	case context.LineTypeRulesetImport, context.LineTypeAliasPattern:
		// Ruleset imports and aliases are highlighted (orange/yellow)
		return core_theme.DefaultTheme.Highlight.Render(line)

	case context.LineTypeViewDirective, context.LineTypeCmdDirective,
		context.LineTypeFindDirective, context.LineTypeGrepDirective,
		context.LineTypeOtherDirective:
		// Directives are accented (violet)
		return core_theme.DefaultTheme.Accent.Render(line)

	case context.LineTypePattern:
		// Regular patterns - normal style
		return line

	case context.LineTypeEmpty:
		// Empty lines remain empty
		return line

	default:
		return line
	}
}

func (p *rulesPage) SetSize(width, height int) {
	p.width = width
	p.height = height
	// Subtract header/footer height, left padding, and top padding from viewport
	p.viewport.Width = width - 2  // Account for left padding
	p.viewport.Height = height - 6 // Account for header/footer and top padding
}

func (p *rulesPage) Update(msg tea.Msg) (Page, tea.Cmd) {
	var cmd tea.Cmd
	p.viewport, cmd = p.viewport.Update(msg)
	return p, cmd
}

func (p *rulesPage) View() string {
	return lipgloss.NewStyle().
		Width(p.width).
		Height(p.height - 5). // Reserve space for pager header and footer
		PaddingLeft(2).
		PaddingTop(1).
		Render(p.viewport.View())
}
