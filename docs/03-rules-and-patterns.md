# Rules and Patterns

The `.grove/rules` file defines which files and directories are included in or excluded from the context using a pattern-matching syntax.

## Rules File Basics

-   **Location**: The active rules file is `.grove/rules`. The legacy `.grovectx` file is read as a fallback if `.grove/rules` is not found.
-   **Format**: The file is plain text, with one pattern per line.
-   **Comments**: Lines beginning with `#` are ignored.
-   **Directives**: Special commands like `@default:` or Git URLs can be included. See the "External Repositories" documentation for details.
-   **Paths**: Patterns can be relative to the project root (`src/**/*.go`), relative to a parent directory (`../other-repo/**`), or absolute (`/path/to/shared/lib`).

## Pattern Syntax Reference

The syntax is based on `.gitignore` conventions.

| Pattern             | Description                                                                     | Example                       |
| ------------------- | ------------------------------------------------------------------------------- | ----------------------------- |
| `*`                 | Matches any sequence of characters except directory separators (`/`).             | `*.go`                        |
| `**`                | Matches directories recursively.                                                | `src/**/*.js`                 |
| `src/`              | Matches a directory named `src`. A trailing slash is optional.                  | `docs/` or `docs`             |
| `/path/to/file.txt` | An absolute path matches a specific file or directory on the filesystem.        | `/Users/dev/project/file.txt` |
| `!pattern`          | Excludes files that match the pattern.                                          | `!**/*_test.go`               |

**Note**: An inclusion pattern for a plain directory name (e.g., `docs`) is treated as a recursive pattern (`docs/**`) to include all its contents.

## Include/Exclude Logic

`cx` processes rules to determine the final set of files.

1.  **Default Behavior**: A file is omitted unless it matches at least one inclusion pattern.
2.  **Exclusion**: If a file matches an inclusion pattern, it can be subsequently excluded by a pattern prefixed with `!`.
3.  **Precedence**: The last pattern in the rules file that matches a file determines its inclusion or exclusion.
4.  **.gitignore**: Files listed in `.gitignore` are excluded by default unless explicitly included by a rule.

## Pattern Writing Strategies

-   Begin with broad inclusion patterns (`**/*.go`) and follow them with specific exclusions (`!vendor/**`).
-   Use relative paths (`../other-project/**/*.go`) to include files from sibling projects.
-   Group related patterns with comments for readability.
-   To exclude a directory and its contents, use `!dist/**`. A pattern without a slash, like `!tests`, excludes any file or directory named `tests` at any level.

## Editing Rules with `cx edit`

The `cx edit` command opens the `.grove/rules` file in the editor specified by the `$EDITOR` environment variable. This is intended for rapid iteration and can be bound to a shell keyboard shortcut.

## Examples

### Go Project
```gitignore
# Include all Go files, plus mod and sum files
**/*.go
go.mod
go.sum

# Exclude test files and the vendor directory
!**/*_test.go
!vendor/**
```

### JavaScript/TypeScript Project
```gitignore
# Include all source files from the src directory
src/**/*.ts
src/**/*.js

# Exclude build outputs and dependencies
!node_modules/**
!dist/**
!build/**
```

### Mixed-Language Monorepo
```gitignore
# Python backend
api/**/*.py
!api/tests/**

# TypeScript frontend
web/src/**/*.ts
!web/node_modules/**

# Shared Protobuf definitions
proto/**/*.proto
```

### Multi-Repo Workspace
```gitignore
# Include the current project's source
src/**/*.go

# Include the source from a sibling API project
../api/**/*.go

# Include shared libraries, but exclude their tests
../shared-lib/**/*.go
!../shared-lib/**/tests/**
```

### Rules with a GitHub Repository
```gitignore
# Include the current project's source
**/*.go

# Also include a specific version of an external library
https://github.com/charmbracelet/lipgloss@v0.13.0

# Exclude examples from the cloned library
!**/examples/**
```