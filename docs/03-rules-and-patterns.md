# Rules and Patterns

The `.grove/rules` file is the core of Grove Context's functionality. It defines which files and directories are included in or excluded from the context using a simple, powerful pattern-matching syntax.

## Rules File Basics

-   **Location**: The active rules file is located at `.grove/rules` within your project root. `cx` will also read a legacy `.grovectx` file if `.grove/rules` is not found.
-   **Format**: The file is plain text, with one pattern per line.
-   **Comments**: Lines beginning with a `#` are treated as comments and ignored.
-   **Directives**: Special commands starting with `@` (e.g., `@default:`) or Git URLs can be included to import rules or repositories. See the "External Repositories" documentation for details.
-   **Paths**: Patterns can be relative to the project root (e.g., `src/**/*.go`), relative to a parent directory (e.g., `../other-repo/**`), or absolute (e.g., `/path/to/shared/lib`).

## Pattern Syntax Reference

The syntax is designed to be familiar to anyone who has used `.gitignore` files.

| Pattern             | Description                                                                                                                                                                             | Example                       |
| ------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ----------------------------- |
| `*`                 | Matches any sequence of characters, but not directory separators (`/`).                                                                                                                   | `*.go`                        |
| `**`                | Matches directories recursively at any depth.                                                                                                                                           | `src/**/*.js`                 |
| `src/`              | Matches a directory named `src`. A trailing slash is optional but recommended for clarity.                                                                                                | `docs/` or `docs`             |
| `/path/to/file.txt` | An absolute path matches a specific file or directory on the filesystem.                                                                                                                | `/Users/dev/project/file.txt` |
| `!pattern`          | Negates a pattern, excluding any files that match.                                                                                                                                      | `!**/*_test.go`               |

**Note on Directory Patterns**: When used for inclusion, a plain directory name (e.g., `docs`) is automatically treated as a recursive pattern (`docs/**`) to include all its contents.

## Include/Exclude Logic

`cx` processes rules to determine the final set of files for the context.

1.  **Default Behavior**: All files are initially considered omitted. A file must match at least one inclusion pattern to be considered for the context.
2.  **Exclusion with `!`**: If a file matches an inclusion pattern, it can be subsequently excluded by a pattern prefixed with `!`.
3.  **Last Match Wins**: For any given file, the last pattern in the rules file that matches it determines its fate. If the last matching pattern is an inclusion rule, the file is included. If it's an exclusion rule, the file is excluded. This makes it practical to start with broad inclusion rules and follow them with specific exclusions.
4.  **.gitignore**: Files and directories listed in your project's `.gitignore` file are automatically excluded by default, unless they are explicitly included by a rule.

## Pattern Writing Strategies

-   **Start Broad, Then Exclude**: Begin with broad patterns like `**/*.go` and then add more specific exclusion patterns like `!vendor/**` or `!**/*_test.go`.
-   **Use Relative Paths for Portability**: When including files from sibling projects, use relative paths like `../other-project/**/*.go` to ensure the rules work for all team members.
-   **Group Related Patterns**: Keep patterns for a specific language or feature set together, often with comments, to improve readability.
-   **Be Specific with Exclusions**: To exclude a directory and its contents, use a pattern like `!dist/**`. A pattern without a slash, like `!tests`, will exclude any file or directory named `tests` at any level.

## Editing Rules with `cx edit`

The `cx edit` command provides a fast way to open the active `.grove/rules` file in your default editor (as defined by the `$EDITOR` environment variable).

**Recommended Usage**: Bind this command to a keyboard shortcut in your shell for rapid iteration on your context.

```bash
# Example for .zshrc or .bashrc
alias cxe='cx edit'

# Or bind to a hotkey
# bindkey -s '^x^c' 'cx edit\n' # Zsh example
```

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