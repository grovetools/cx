package rules

import (
	"strings"

	"github.com/mattsolo1/grove-context/pkg/context"
	core_theme "github.com/mattsolo1/grove-core/tui/theme"
)

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
