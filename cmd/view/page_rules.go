package view

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
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
		styledLines[i] = styleLineByType(line, parsed)
	}

	return strings.Join(styledLines, "\n")
}

// styleLineByType applies appropriate styling based on line type
func styleLineByType(line string, parsed context.ParsedLine) string {
	theme := core_theme.DefaultTheme

	switch parsed.Type {
	case context.LineTypeComment:
		// Comments are muted
		return theme.Muted.Render(line)

	case context.LineTypeSeparator:
		// Separator (---) is bold/statement
		return theme.Bold.Render(line)

	case context.LineTypeExclude:
		// Exclusion patterns (!) are highlighted as error/red
		return theme.Error.Render(line)

	case context.LineTypeGitURL:
		// Git URLs are styled as info/cyan
		return theme.Info.Render(line)

	case context.LineTypeAliasPattern:
		// Aliases with component-level styling
		var styledParts []string

		// Directive (@a: or @alias:)
		if val, ok := parsed.Parts["directive"]; ok {
			styledParts = append(styledParts, theme.Accent.Render(val))
		}

		// Component 1 (ecosystem)
		if val, ok := parsed.Parts["component1"]; ok {
			styledParts = append(styledParts, theme.Accent.Render(val))
		}

		// Component 2 (repo)
		if val, ok := parsed.Parts["component2"]; ok {
			styledParts = append(styledParts, theme.Muted.Render(":"))
			styledParts = append(styledParts, theme.Info.Render(val))
		}

		// Component 3 (worktree)
		if val, ok := parsed.Parts["component3"]; ok {
			styledParts = append(styledParts, theme.Muted.Render(":"))
			styledParts = append(styledParts, theme.Warning.Render(val))
		}

		// Path
		if val, ok := parsed.Parts["path"]; ok {
			styledParts = append(styledParts, val)
		}

		return strings.Join(styledParts, "")

	case context.LineTypeRulesetImport:
		// Ruleset imports with component-level styling
		var styledParts []string

		// Directive (@a: or @alias:)
		if val, ok := parsed.Parts["directive"]; ok {
			styledParts = append(styledParts, theme.Accent.Render(val))
		}

		// Alias
		if val, ok := parsed.Parts["alias"]; ok {
			styledParts = append(styledParts, theme.Accent.Render(val))
		}

		// Delimiter (::)
		if val, ok := parsed.Parts["delimiter"]; ok {
			styledParts = append(styledParts, theme.Muted.Render(val))
		}

		// Ruleset name
		if val, ok := parsed.Parts["ruleset"]; ok {
			styledParts = append(styledParts, theme.Success.Render(val))
		}

		return strings.Join(styledParts, "")

	case context.LineTypeViewDirective, context.LineTypeCmdDirective,
		context.LineTypeFindDirective, context.LineTypeGrepDirective,
		context.LineTypeOtherDirective:
		// Directives are accented (violet)
		return theme.Accent.Render(line)

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
	// The pager passes us the available space after accounting for chrome and padding
	p.viewport.Width = width
	p.viewport.Height = height
}

func (p *rulesPage) Update(msg tea.Msg) (Page, tea.Cmd) {
	var cmd tea.Cmd
	p.viewport, cmd = p.viewport.Update(msg)
	return p, cmd
}

func (p *rulesPage) View() string {
	// The pager now handles all padding and layout. This page just needs to return
	// the rendered content of its viewport.
	return p.viewport.View()
}
