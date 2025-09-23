# Core Concepts

This section explains how grove-context (cx) selects files, separates them into “hot” and “cold” sets, and generates the output artifacts that other tools consume. Understanding these concepts will help you write effective rules and predict how cx behaves.

## The `.grove/rules` file

The `.grove/rules` file is the single source of truth for context generation. Every command that needs a list of files (e.g., `cx generate`, `cx list`, `cx stats`) resolves files dynamically from this file. There is no persistent registry of files; outputs are regenerated from rules each time.

- Location: the file lives in your project at `.grove/rules`. It is intended to be kept under version control alongside your code.
- Purpose: define, using patterns, which files belong in your LLM “context.” The rules file supports:
  - A “hot” section for actively changing files
  - An optional “cold” section for stable references
  - Exclusions using `!`-prefixed patterns
  - Comments with `#` and empty lines for readability

Notes:
- For backward compatibility, cx will read legacy `.grovectx` if `.grove/rules` does not exist.
- Projects can provide a default rules file via `grove.yml` (context.default_rules_path). If present and no local rules exist, cx will use that content as the starting point.

Example skeleton:
```txt
# Hot context rules (active work)
**/*.go
!**/*_test.go
README.md

---
# Cold context rules (stable references)
../shared-lib/**/*.go
!../shared-lib/**/internal/**
```

## Hot and Cold Contexts

cx separates files into two sets, defined by a `---` separator in `.grove/rules`:

- Hot Context (above `---`):
  - Files you’re currently working with or refer to interactively
  - Intended to be sent frequently to an LLM
- Cold Context (below `---`):
  - Stable files (libraries, configuration, external references)
  - Intended for long-lived or cached consumption by tools

Outputs:
- `.grove/context`: contains the concatenated Hot Context (by default, with an XML envelope). Each file is wrapped with a path attribute and its content.
- `.grove/cached-context`: contains the concatenated Cold Context (also XML). cx also writes `.grove/cached-context-files` listing the cold files, one per line.

Operational behavior:
- `cx list` prints hot files (absolute paths)
- `cx list-cache` prints cold files
- `cx stats` reports statistics for both hot and cold sets
- `cx generate` writes both `.grove/context` (hot) and `.grove/cached-context` (cold)

Precedence between hot and cold:
- If a file matches patterns in both sections, it is treated as cold. cx resolves both sets and removes any overlaps from hot before writing outputs.

## Rule Syntax and Precedence

### Pattern language

The rules file uses a `.gitignore`-style, line-oriented syntax:

- Comments and whitespace:
  - Lines starting with `#` are comments
  - Empty lines are ignored

- Inclusion patterns:
  - Basic glob: `*.go`
  - Recursive glob: `**/*.go` (matches at any depth)
  - Plain directory on an inclusion line is treated as recursive:
    - Example: `docs` is interpreted as `docs/**` (for inclusion only)

- Exclusions:
  - Prefix with `!` to exclude
  - Example: `!vendor/**/*` excludes all files under `vendor/`
  - Example: `!*_test.go` excludes Go test files

- Relative and absolute paths:
  - Relative patterns are resolved from the project root (the directory where `.grove/rules` lives)
  - Parent-relative patterns are allowed (e.g., `../sibling/**/*.go`)
  - Absolute paths are allowed and match directly on the filesystem

- Directory name matches (gitignore-like behavior):
  - A pattern without a slash can match a directory name anywhere in the path
  - Example: `!tests` excludes any directory named “tests” and its contents

- Git-ignored files:
  - By default, files ignored by Git are not included in the resolved context
  - `cx view` can optionally display git-ignored files for inspection

Examples:
```txt
# Include all Go files recursively
**/*.go

# Include top-level Markdown files
*.md

# Exclude vendor and any tests directory tree
!vendor/**/*
!**/tests/**

# Exclude file pattern across the tree
!*_test.go

# Include a local subdirectory (treated as docs/**)
docs

# Include files from a sibling project
../sibling-lib/**/*.go

# Include a specific absolute directory or file
/Users/alex/shared/configs
/Users/alex/shared/configs/app.yaml
```

### Pattern resolution and “last match wins”

For a given path, the last matching rule on that path determines inclusion or exclusion. This is consistent with common ignore-file semantics and is applied while walking the filesystem. Practically, put broad includes first and follow with specific exclusions.

Example ordering:
```txt
**/*.go         # include all Go
!**/*_test.go   # then exclude test files
!**/internal/** # and exclude internal
```

### Hot-over-cold precedence (and the cold override)

cx resolves hot patterns and cold patterns separately, then enforces cold-over-hot precedence:

- If a path appears in both hot and cold results, it is removed from hot and remains in cold
- This ensures a file belongs to exactly one set in the final outputs

Concrete example:
```txt
# Hot
**/*.go
README.md
---
# Cold
src/utils.go
```
Results:
- Hot: all Go files except `src/utils.go`, plus `README.md`
- Cold: `src/utils.go` only
- `.grove/context` will not contain `src/utils.go`; it will appear in `.grove/cached-context` instead

### Practical tips for writing rules

- Start broad, then narrow:
  - Include wide patterns (`**/*.go`, `*.md`)
  - Add exclusions (`!*_test.go`, `!vendor/**/*`, `!**/tests/**`)
- Use the cold section for stable or large files that don’t change often
- Prefer relative paths for portability; use absolute paths when necessary
- When including directories by name, remember a bare directory on an inclusion line is treated as recursive (e.g., `docs` → `docs/**`)
- Keep rules readable with comments and spacing

--- 

By structuring `.grove/rules` carefully and understanding how cx resolves patterns into hot and cold sets, you can produce predictable, maintainable contexts for downstream tools.