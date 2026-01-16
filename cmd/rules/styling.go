package rules

import (
	"strings"

	"github.com/grovetools/cx/pkg/context"
	core_theme "github.com/grovetools/core/tui/theme"
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

	case context.LineTypeGitURLRuleset:
		// Git URLs with ruleset have component-level styling
		var styledParts []string

		// URL part
		if val, ok := parsed.Parts["url"]; ok {
			styledParts = append(styledParts, theme.Info.Render(val))
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
