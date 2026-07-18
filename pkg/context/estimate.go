package context

import (
	"path/filepath"
	"strings"
)

// Token-estimate divisors, calibrated against real API tokenization from
// request ledgers: code-heavy content tokenizes near bytes/2 (observed
// actual/estimate ratios of 1.9–2.2x under the old flat bytes/4), prose
// stays near bytes/4. Divisor-based on purpose — a real tokenizer would
// add a dependency for accuracy cx doesn't need (~15% is the bar).
const (
	codeTokenDivisor    = 2
	proseTokenDivisor   = 4
	unknownTokenDivisor = 3
)

// codeExtensions covers source code and structured data formats, which
// tokenize much denser than prose.
var codeExtensions = map[string]bool{
	".go": true, ".ts": true, ".tsx": true, ".js": true, ".jsx": true,
	".mjs": true, ".cjs": true, ".py": true, ".rs": true, ".c": true,
	".h": true, ".hpp": true, ".cc": true, ".cpp": true, ".java": true,
	".kt": true, ".kts": true, ".swift": true, ".rb": true, ".php": true,
	".sh": true, ".bash": true, ".zsh": true, ".fish": true, ".sql": true,
	".json": true, ".jsonl": true, ".yaml": true, ".yml": true, ".toml": true,
	".xml": true, ".html": true, ".css": true, ".scss": true, ".less": true,
	".lua": true, ".pl": true, ".proto": true, ".vue": true, ".svelte": true,
	".zig": true, ".cs": true, ".m": true, ".mm": true, ".tf": true,
	".gradle": true, ".cmake": true, ".mk": true, ".ini": true, ".cfg": true,
	".conf": true, ".ex": true, ".exs": true, ".erl": true, ".hs": true,
	".scala": true, ".clj": true, ".dart": true, ".r": true, ".ps1": true,
}

// proseExtensions covers natural-language documents, which tokenize near
// the classic bytes/4.
var proseExtensions = map[string]bool{
	".md": true, ".txt": true, ".rst": true, ".adoc": true, ".org": true,
}

// EstimateTokens estimates the token count for a file from its size,
// keyed by extension content class: code/structured data ≈ bytes/2,
// prose (and extensionless files like Makefile, README) ≈ bytes/4,
// unknown extensions ≈ bytes/3 as a middle ground.
func EstimateTokens(path string, size int64) int {
	ext := strings.ToLower(filepath.Ext(path))
	switch {
	case codeExtensions[ext]:
		return int(size / codeTokenDivisor)
	case ext == "" || proseExtensions[ext]:
		return int(size / proseTokenDivisor)
	default:
		return int(size / unknownTokenDivisor)
	}
}
