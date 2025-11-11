This document describes the syntax and usage of `grove-context` rules files for defining the context provided to LLMs.

### Rules File Basics

The context for a project is defined by a rules file. This file lists patterns that determine which files are included or excluded.

-   **Location**: The primary rules file is located at `.grove/rules` in the project root.
-   **Format**: A plain text file with one pattern per line.
-   **Comments**: Lines beginning with `#` are ignored.
-   **Syntax**: The syntax is based on `.gitignore` patterns.
-   **Hot vs. Cold Context**: A `---` separator divides the file into two sections. Patterns *before* the separator define "hot context" (included in every request). Patterns *after* define "cold context" (used for caching with supported models like Gemini).

### Pattern Syntax Reference

Patterns specify which files to include or exclude. The system uses "last match wins" logic.

| Pattern                  | Description                                                                                              |
| ------------------------ | -------------------------------------------------------------------------------------------------------- |
| `*.go`                   | Matches files with a `.go` extension in the current directory.                                           |
| `src/**/*.ts`            | Recursively matches all files with a `.ts` extension in the `src` directory and its subdirectories.      |
| `!node_modules/`         | Excludes the `node_modules` directory.                                                                   |
| `src/`                   | Matches all files within the `src` directory.                                                            |
| `/absolute/path/to/file` | Matches an exact absolute file path.                                                                     |
| `../sibling-project/**`  | Matches all files in an adjacent directory.                                                              |
| `@a:project-name`        | An **alias** referencing a discovered workspace. Expands to include all files in that project.           |
| `@a:project/src/**.go`   | Combines an alias with a glob pattern to include specific files from another project.                    |
| `@a:eco:repo`            | An alias referencing a sub-project within an ecosystem.                                                  |
| `@a:repo:worktree`       | An alias referencing a specific worktree of a project.                                                   |
| `@a:project::ruleset`    | Imports a named rule set from another project.                                                           |

### Alias Resolution System

Aliases provide a stable way to reference other projects without using relative or absolute paths.

-   **Syntax**: `@a:name` (short form) or `@alias:name` (long form).
-   **Discovery**: `grove-context` uses `grove-core` to discover all workspaces (projects, ecosystems, worktrees) defined in your configuration. Each discovered entity is given a unique identifier that can be used as an alias.
-   **Resolution**:
    -   **1-part (`@a:project-name`):** Resolves to a project. If multiple projects have the same name, it prioritizes siblings within the same ecosystem or top-level projects.
    -   **2-part (`@a:ecosystem:repo` or `@a:repo:worktree`):** Resolves a sub-project within an ecosystem, or a worktree of a specific repository.
    -   **3-part (`@a:ecosystem:repo:worktree`):** Resolves a specific worktree of a sub-project within an ecosystem.
-   **Viewing Aliases**: Use `cx workspace list` to see all available workspaces and their identifiers, which can be used as aliases.

### Include/Exclude Logic

The context is built by applying rules in order:

1.  **`.gitignore` First**: All files ignored by `.gitignore` are excluded by default. They cannot be re-included.
2.  **Last Match Wins**: For files not ignored by Git, the last pattern in the rules file that matches the file path determines its inclusion or exclusion.
    -   If the last matching rule starts with `!`, the file is **excluded**.
    -   If the last matching rule does not start with `!`, the file is **included**.

### Pattern Writing Strategies

-   **Start Broad, Exclude Specifics**: A common approach is to start with a broad inclusion pattern like `*` or `**/*`, and then add `!` patterns to exclude unwanted files and directories.
-   **Organize Multi-Repo Contexts**:
    -   **Relative Paths**: Use relative paths for projects in a known directory structure (e.g., `../api-service/**`).
    -   **Aliases**: Use aliases for a more robust way to reference projects regardless of their location on disk (e.g., `@a:api-service/**`). This is the recommended approach for team consistency.
-   **Comment Your Rules**: Use `#` to add comments explaining why certain files are included or excluded, especially for complex patterns.

### Reusable Rule Sets

For different tasks, you often need different contexts. `grove-context` supports creating and switching between named rule sets.

-   **Location**:
    -   `.cx/`: For shared, version-controlled rule sets (e.g., `.cx/backend.rules`).
    -   `.cx.work/`: For personal, temporary, or experimental rule sets (this directory is gitignored).
-   **Why Create Rule Sets?**
    -   **Role-based**: `backend-only.rules`, `frontend-only.rules`, `docs-only.rules`.
    -   **Feature-based**: `auth-module.rules`, `billing-api.rules`.
    -   **Task-based**: `debugging.rules` (include logs), `refactoring.rules` (include tests).
-   **Commands**:
    -   `cx rules`: Interactively select the active rule set.
    -   `cx rules set <name>`: Set the active rule set.
    -   `cx rules save <name>`: Save the current `.grove/rules` to a named set in `.cx/`. Use `--work` to save to `.cx.work/`.
    -   `cx rules load <name>`: Copy a named rule set to `.grove/rules` to use as a modifiable working copy.
-   **Importing Rule Sets**: You can import rules from another project's named rule set using the `::` syntax. This promotes consistency across a large ecosystem.
    -   `@a:shared-patterns::go-backend` imports the `go-backend.rules` set from the `shared-patterns` project.

### Editing Rules

-   **`cx edit`**: Run `cx edit` in your terminal to open the active rules file in your default `$EDITOR`.
-   **Neovim Integration**: The `grove-nvim` plugin provides real-time feedback directly in the editor:
    -   Virtual text shows the token and file count for each rule.
    -   Syntax highlighting for rules and directives.
    -   `gf` keymap to jump to the file or directory referenced by a rule.

---

### Examples

#### Go Project

```sh
# .grove/rules

# Include all Go source files, module files, and Makefiles
*.go
go.mod
go.sum
Makefile

# Exclude test files and vendor directories
!*_test.go
!vendor/
```

#### JavaScript/TypeScript Project

```sh
# .grove/rules

# Include all source files from the src/ directory
src/**/*.ts
src/**/*.tsx

# Exclude test files, node_modules, and build artifacts
!*.spec.ts
!*.test.ts
!node_modules/
!dist/
!build/
```

#### Multi-Repo Workspace (Relative Paths)

```sh
# .grove/rules

# Include the entire api and frontend sibling directories
../api/**
../frontend/**

# Exclude test files from both
!**/*_test.go
!**/*.spec.ts
```

#### Multi-Repo Workspace (Aliases)

```sh
# .grove/rules

# Use aliases to reference other discovered workspaces
@a:grove-core/**/*.go
@a:grove-nvim/lua/**/*.lua

# Exclude test files from both projects
!**/*_test.go
```

#### Specialized Rule Set (`.cx/backend-only.rules`)

```sh
# .cx/backend-only.rules

# This rule set is for backend API development.
# It includes only Go source files and excludes everything else.

*.go
go.mod
go.sum

!*_test.go
!frontend/
!docs/
```

#### Importing a Rule Set

```sh
# .grove/rules for a new service

# Import the standard Go backend patterns from a shared project.
# This ensures consistency across microservices.
@a:shared-infra::go-backend

# Add service-specific files
src/main.go
config.yml
```

### Best Practices

-   **Keep Patterns Simple**: Prefer simple, readable patterns over complex ones.
-   **Version Control Rules**: Store shared rule sets in the `.cx/` directory and commit them to Git.
-   **Use `.cx.work/` for Personal Rules**: Use the gitignored `.cx.work/` directory for local experiments or personal workflows.
-   **Use Descriptive Names**: Name your rule sets clearly (e.g., `api-only`, `refactor-auth-service`).
-   **Import for Consistency**: Create a central project with standard rule sets (`go-defaults`, `ts-react-app`) and import them using `@a:project::ruleset` to enforce consistency.
-   **Switch Contexts Frequently**: Use `cx rules` to switch between rule sets based on your current task. This keeps the context small and relevant, improving LLM accuracy and reducing cost.