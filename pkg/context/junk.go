package context

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// junkDirNames is the data-driven set of directory names excluded from
// *implicit* directory-glob expansion. These are dependency trees, build
// caches, and provider/state blobs that routinely balloon a single directory
// glob (e.g. "terraform/**") into hundreds of megabytes of context.
//
// The exclusion is implicit-only: a rule that walks into one of these
// directories via a parent-directory glob skips it, but a rule that names the
// directory explicitly in its own pattern still resolves normally — see
// patternReferencesDir. This lets an operator re-include a junk directory
// deliberately (e.g. "@a:web/frontend/node_modules/critical-dep/**" or a
// re-include glob "**/node_modules/**") while never ingesting one by accident.
//
// .git/.grove/.grove-worktrees are handled unconditionally in
// prodResolutionContext.WalkDir and intentionally omitted here: they are never
// legitimate context and cannot be re-included.
var junkDirNames = map[string]bool{
	".terraform":   true, // terraform provider binaries + state (200MB+)
	"node_modules": true, // npm/yarn/pnpm dependency tree
	".zig-cache":   true, // zig build cache
	"zig-out":      true, // zig build output
	".venv":        true, // python virtualenv
	"__pycache__":  true, // python bytecode cache
	"target":       true, // rust/cargo build output
	"dist":         true, // generic build/dist output
}

// isJunkDir reports whether name is a default-excluded junk directory.
func isJunkDir(name string) bool { return junkDirNames[name] }

// patternReferencesDir reports whether the (resolved) pattern names dirName as
// a literal path component. Such a directory is an explicit target and is
// exempt from implicit junk filtering.
func patternReferencesDir(pattern, dirName string) bool {
	for _, part := range strings.Split(filepath.ToSlash(pattern), "/") {
		if part == dirName {
			return true
		}
	}
	return false
}

const (
	// oversizeFileThreshold is the per-rule file count above which cx warns
	// loudly that a single rules line expanded suspiciously large.
	oversizeFileThreshold = 500
	// oversizeByteThreshold is the per-rule total byte size above which cx
	// warns. 10 MiB from a single line almost always means an accidental
	// junk-dir ingest.
	oversizeByteThreshold = 10 * 1024 * 1024
)

// warnOversizedRules emits a loud stderr warning for any inclusion rule line
// whose expansion exceeds oversizeFileThreshold files or oversizeByteThreshold
// bytes, naming the line and the count so silent 200MB context bombs become
// impossible. Byte size is computed by stat'ing the attributed files, which is
// cheap relative to the read+tokenize the files will otherwise incur.
func warnOversizedRules(rules []RuleInfo, attr AttributionResult) {
	type lineInfo struct {
		pattern string
		lineNum int
	}
	// Map EffectiveLineNum -> human-readable rule info (first occurrence wins),
	// mirroring warnZeroMatchRules so imported/re-rooted lines attribute right.
	lines := map[int]lineInfo{}
	for _, r := range rules {
		if r.IsExclude {
			continue
		}
		if _, seen := lines[r.EffectiveLineNum]; !seen {
			lines[r.EffectiveLineNum] = lineInfo{pattern: r.Pattern, lineNum: r.LineNum}
		}
	}

	for eln, paths := range attr {
		info, ok := lines[eln]
		if !ok {
			continue
		}
		count := len(paths)
		var totalBytes int64
		for _, p := range paths {
			if fi, err := os.Stat(p); err == nil {
				totalBytes += fi.Size()
			}
		}
		if count <= oversizeFileThreshold && totalBytes <= oversizeByteThreshold {
			continue
		}
		fmt.Fprintf(os.Stderr,
			"Warning: rule '%s' (line %d) expanded to %d files (%s) — exceeds the %d-file / %s guard; narrow the glob or add an exclusion\n",
			info.pattern, info.lineNum, count, FormatBytes(int(totalBytes)),
			oversizeFileThreshold, FormatBytes(oversizeByteThreshold))
	}
}
