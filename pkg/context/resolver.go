package context

import (
	"io/fs"
	"path/filepath"
	"strings"
)

// Resolve emits FileAttributions for files matching this node. The Match
// wedge method is preserved for the legacy resolver.
func (n *GlobNode) Resolve(ctx ResolutionContext) []FileAttribution {
	pattern := n.Pattern
	if !strings.ContainsAny(pattern, "*?[") {
		probe := pattern
		if !filepath.IsAbs(probe) {
			probe = filepath.Join(ctx.BaseDir(), probe)
		}
		if info, err := ctx.Stat(probe); err == nil && info.IsDir() {
			pattern = strings.TrimSuffix(pattern, "/") + "/**"
		}
	}
	return walkAndEmit(ctx, pattern, n.LineNum, n.Excluded)
}

func (n *LiteralNode) Resolve(ctx ResolutionContext) []FileAttribution {
	pattern := n.ExpectedPath
	if info, err := ctx.Stat(absUnderBase(pattern, ctx.BaseDir())); err == nil && info.IsDir() {
		pattern = strings.TrimSuffix(pattern, "/") + "/**"
		return walkAndEmit(ctx, pattern, n.LineNum, n.Excluded)
	}
	return walkAndEmit(ctx, pattern, n.LineNum, n.Excluded)
}

func (n *ImportNode) Resolve(ctx ResolutionContext) []FileAttribution {
	// Ruleset imports (@a:proj::ruleset) require nested rules-file expansion;
	// Phase 5B owns that. Until then, defer to the legacy walker.
	if n.Ruleset != "" {
		return nil
	}
	rebuilt := "@a:" + n.Target
	resolved, err := ctx.ResolveAliasLine(rebuilt)
	if err != nil || resolved == "" {
		return nil
	}
	pattern := resolved
	if !hasGlobMeta(pattern) {
		if info, err := ctx.Stat(pattern); err == nil && info.IsDir() {
			pattern = strings.TrimSuffix(pattern, "/") + "/**"
		}
	}
	return walkAndEmit(ctx, pattern, n.LineNum, n.Excluded)
}

func (n *CommandNode) Resolve(ctx ResolutionContext) []FileAttribution {
	files, err := ctx.ExecCommand(n.Command)
	if err != nil {
		return nil
	}
	var attrs []FileAttribution
	for _, f := range files {
		attrs = append(attrs, walkAndEmit(ctx, f, n.LineNum, n.Excluded)...)
	}
	return attrs
}

func (n *FilterNode) Resolve(ctx ResolutionContext) []FileAttribution {
	if n.Child == nil {
		return nil
	}
	raw := n.Child.Resolve(ctx)
	out := make([]FileAttribution, 0, len(raw))
	for _, attr := range raw {
		if attr.IsExclude {
			out = append(out, attr)
			continue
		}
		ok := true
		for _, d := range n.Directives {
			if !ctx.MatchDirective(attr.Path, d.Name, d.Query) {
				ok = false
				break
			}
		}
		if ok {
			attr.EffectiveLineNum = n.LineNum
			out = append(out, attr)
		}
	}
	return out
}

// walkAndEmit iterates the context's walk root and emits a FileAttribution
// for any file whose path matches `pattern` per ctx.MatchPattern semantics.
// Uses the same float-vs-anchored matching the legacy classify path uses.
func walkAndEmit(ctx ResolutionContext, pattern string, line int, excluded bool) []FileAttribution {
	floating := !filepath.IsAbs(pattern) && !strings.Contains(pattern, "/") && !IsRelativeExternalPath(pattern)
	var attrs []FileAttribution
	_ = ctx.WalkDir(ctx.BaseDir(), func(path string, d fs.DirEntry, err error) error {
		if err != nil || d == nil || d.IsDir() {
			return nil
		}
		matchPath := relForMatch(path, ctx.BaseDir())
		if filepath.IsAbs(pattern) {
			matchPath = filepath.ToSlash(path)
		} else if IsRelativeExternalPath(pattern) {
			if rel, err2 := filepath.Rel(ctx.BaseDir(), path); err2 == nil {
				matchPath = filepath.ToSlash(rel)
			}
		} else if floating {
			isExternal := strings.HasPrefix(matchPath, "..")
			if isExternal && !excluded {
				return nil
			}
		}
		if !ctx.MatchPattern(pattern, matchPath) {
			return nil
		}
		attrs = append(attrs, FileAttribution{
			Path:             path,
			EffectiveLineNum: line,
			IsExclude:        excluded,
		})
		return nil
	})
	return attrs
}

// ResolveAST evaluates the AST in a single pass. The reducer applies
// last-write-wins to inclusion vs. exclusion decisions and tracks superseded
// inclusion rules in FilteredResult.
func ResolveAST(nodes []RuleNode, ctx ResolutionContext) (AttributionResult, ExclusionResult, FilteredResult) {
	perFile := map[string][]FileAttribution{}
	for _, n := range nodes {
		for _, a := range n.Resolve(ctx) {
			perFile[a.Path] = append(perFile[a.Path], a)
		}
	}

	attrR := make(AttributionResult)
	exclR := make(ExclusionResult)
	filtR := make(FilteredResult)

	for path, ms := range perFile {
		last := ms[len(ms)-1]
		if last.IsExclude {
			exclR[last.EffectiveLineNum] = append(exclR[last.EffectiveLineNum], path)
			continue
		}
		winner := last.EffectiveLineNum
		attrR[winner] = append(attrR[winner], path)
		seen := map[int]bool{}
		for i := 0; i < len(ms)-1; i++ {
			if ms[i].IsExclude {
				continue
			}
			line := ms[i].EffectiveLineNum
			if line == winner || seen[line] {
				continue
			}
			filtR[line] = append(filtR[line], FilteredFileInfo{
				File:           path,
				WinningLineNum: winner,
			})
			seen[line] = true
		}
	}

	return attrR, exclR, filtR
}

// ruleInfosToNodes synthesizes AST nodes from expanded RuleInfo records so
// the single-pass attribution path can consume the output of expandAllRules
// without changing that function's signature.
func ruleInfosToNodes(rules []RuleInfo) []RuleNode {
	out := make([]RuleNode, 0, len(rules))
	for _, r := range rules {
		var inner RuleNode
		if hasGlobMeta(r.Pattern) {
			inner = &GlobNode{
				Pattern:  r.Pattern,
				LineNum:  r.EffectiveLineNum,
				RawText:  r.Pattern,
				Excluded: r.IsExclude,
			}
		} else {
			inner = &LiteralNode{
				ExpectedPath: r.Pattern,
				LineNum:      r.EffectiveLineNum,
				RawText:      r.Pattern,
				Excluded:     r.IsExclude,
			}
		}
		if len(r.Directives) > 0 {
			out = append(out, &FilterNode{
				Child:      inner,
				Directives: r.Directives,
				LineNum:    r.EffectiveLineNum,
				RawText:    r.Pattern,
				Excluded:   r.IsExclude,
			})
			continue
		}
		out = append(out, inner)
	}
	return out
}

func absUnderBase(p, base string) string {
	if filepath.IsAbs(p) {
		return p
	}
	return filepath.Join(base, p)
}

func relForMatch(absPath, base string) string {
	rel, err := filepath.Rel(base, absPath)
	if err != nil {
		return filepath.ToSlash(absPath)
	}
	return filepath.ToSlash(rel)
}
