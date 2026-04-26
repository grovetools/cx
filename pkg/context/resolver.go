package context

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
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

	// Leading / that doesn't resolve as a real absolute path is a
	// root-anchor (gitignore convention): /go.mod → workspace-root go.mod.
	if filepath.IsAbs(pattern) {
		if _, err := ctx.Stat(pattern); err != nil {
			rel := pattern[1:]
			target := filepath.Join(ctx.BaseDir(), rel)
			if info, err := ctx.Stat(target); err == nil {
				if info.IsDir() {
					return walkAndEmit(ctx, rel+"/**", n.LineNum, n.Excluded)
				}
				return []FileAttribution{{Path: target, EffectiveLineNum: n.LineNum, IsExclude: n.Excluded}}
			}
			return nil
		}
	}

	floating := !filepath.IsAbs(pattern) && !strings.Contains(pattern, "/")
	// Floating exclusions use gitignore-style component matching via
	// MatchPattern; don't expand to dir/** which anchors to top-level only.
	if !(floating && n.Excluded) {
		if info, err := ctx.Stat(absUnderBase(pattern, ctx.BaseDir())); err == nil && info.IsDir() {
			pattern = strings.TrimSuffix(pattern, "/") + "/**"
			return walkAndEmit(ctx, pattern, n.LineNum, n.Excluded)
		}
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

	root := ctx.BaseDir()
	if filepath.IsAbs(pattern) || IsRelativeExternalPath(pattern) {
		root = walkRootForPattern(pattern, ctx.BaseDir())
	}

	var attrs []FileAttribution
	_ = ctx.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
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

// walkRootForPattern computes the filesystem walk root for an absolute or
// external relative pattern by stripping glob meta from the path tail.
func walkRootForPattern(pattern, base string) string {
	resolved := pattern
	if !filepath.IsAbs(resolved) {
		resolved = filepath.Join(base, resolved)
	}
	resolved = filepath.Clean(resolved)
	parts := strings.Split(filepath.ToSlash(resolved), "/")
	var baseParts []string
	for _, part := range parts {
		if strings.ContainsAny(part, "*?[") {
			break
		}
		baseParts = append(baseParts, part)
	}
	candidate := filepath.FromSlash(strings.Join(baseParts, "/"))
	if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
		return filepath.Dir(candidate)
	}
	return candidate
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

// resolveFilesViaAST converts expanded RuleInfo records to AST nodes and
// resolves files through the single-pass ResolveAST engine. Replaces the
// legacy "extract patterns → resolveFilesFromPatterns" chain.
//
// Two-phase resolution: inclusions walk the real filesystem (potentially
// multiple roots for external patterns), then all rules are re-evaluated
// against the discovered file set so exclusions see the full cross-root set.
func (m *Manager) resolveFilesViaAST(rules []RuleInfo) ([]string, error) {
	if len(rules) == 0 {
		return []string{}, nil
	}

	// Validate: reject unauthorized external paths, check directive regexes.
	var validated []RuleInfo
	for _, r := range rules {
		if !r.IsExclude && (filepath.IsAbs(r.Pattern) || IsRelativeExternalPath(r.Pattern)) {
			base := r.Pattern
			if strings.ContainsAny(base, "*?[") {
				parts := strings.Split(filepath.ToSlash(base), "/")
				var keep []string
				for _, p := range parts {
					if strings.ContainsAny(p, "*?[") {
						break
					}
					keep = append(keep, p)
				}
				base = strings.Join(keep, "/")
			}
			if !filepath.IsAbs(base) {
				base = filepath.Join(m.rulesBaseDir, base)
			}
			if ok, reason := m.IsPathAllowed(base); !ok {
				pattern := r.Pattern
				if r.IsExclude {
					pattern = "!" + pattern
				}
				fmt.Fprintf(os.Stderr, "Warning: skipping rule '%s': %s\n", pattern, reason)
				m.addSkippedRule(0, pattern, reason)
				continue
			}
		}
		for _, d := range r.Directives {
			switch d.Name {
			case "grep", "grep!", "grep-i":
				q := d.Query
				if d.Name == "grep-i" {
					q = "(?i)" + q
				}
				if _, err := regexp.Compile(q); err != nil {
					return nil, fmt.Errorf("invalid regex %q in @%s directive: %w", d.Query, d.Name, err)
				}
			}
		}
		validated = append(validated, r)
	}
	rules = validated

	hasExclusion := false
	for _, r := range rules {
		if r.IsExclude {
			hasExclusion = true
			break
		}
	}

	nodes := ruleInfosToNodes(rules)
	ctx := newProdResolutionContext(m)

	if !hasExclusion {
		attr, _, _ := ResolveAST(nodes, ctx)
		return m.flattenAttrResult(attr), nil
	}

	// Phase 1: resolve inclusion-only nodes to discover the full file set.
	var inclRules []RuleInfo
	for _, r := range rules {
		if !r.IsExclude {
			inclRules = append(inclRules, r)
		}
	}
	inclAttr, _, _ := ResolveAST(ruleInfosToNodes(inclRules), ctx)
	var discovered []string
	for _, paths := range inclAttr {
		discovered = append(discovered, paths...)
	}

	// Phase 2: re-evaluate all rules against the discovered set so
	// exclusions see files from every walk root.
	primedCtx := newProdResolutionContext(m).withFileSet(discovered)
	attr, _, _ := ResolveAST(nodes, primedCtx)
	return m.flattenAttrResult(attr), nil
}

func (m *Manager) flattenAttrResult(attr AttributionResult) []string {
	seen := make(map[string]bool)
	var files []string
	for _, paths := range attr {
		for _, p := range paths {
			if rel, err := filepath.Rel(m.rulesBaseDir, p); err == nil && !strings.HasPrefix(rel, "..") {
				p = rel
			}
			if !seen[p] {
				seen[p] = true
				files = append(files, p)
			}
		}
	}
	sort.Strings(files)
	return files
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
