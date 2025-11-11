# Experimental Features

> ⚠️ **WARNING: EXPERIMENTAL FEATURES**
> Features documented in this section are experimental and may:
> - Change or be removed without notice in future versions
> - Have incomplete error handling or edge case coverage
> - Cause unexpected API costs (especially caching features)
> - Lack comprehensive testing in production environments
>
> **Use in production at your own risk.** Monitor costs, behavior, and API usage closely.

---

## Core Mechanism

The `cx` tool operates on a plain text "rules file" (by default `.grove/rules`) that defines which files to include in a context. It resolves patterns in this file to a list of file paths.

- **Rules File**: A text file containing patterns (globs, file paths, aliases) to include or exclude files.
- **Context Generation**: The `cx generate` command reads the rules and concatenates the content of all matched files into a single output file (`.grove/context`).
- **Interactive View**: The `cx view` command provides a terminal interface to visualize which files are included, excluded, or ignored.

## Usage

| Command                 | Description                                                               |
| ----------------------- | ------------------------------------------------------------------------- |
| `cx generate`           | Generates the `.grove/context` file from the active rules.                |
| `cx view`               | Starts an interactive TUI to visualize the context tree.                  |
| `cx stats`              | Shows statistics about the current context (token counts, file types).    |
| `cx list`               | Prints the list of absolute file paths included in the context.           |
| `cx rules <subcommand>` | Manages named rule sets (list, set, save, load).                          |
| `cx from-git`           | Populates rules from files changed in git history.                        |
| `cx from-cmd`           | Populates rules from the stdout of a shell command.                       |
| `cx repo <subcommand>`  | Manages external git repositories used in the context.                    |
| `cx diff [ruleset]`     | Compares the current context with a named rule set.                       |
| `cx resolve [rule]`     | Prints the list of files a single rule resolves to.                       |

## The Rules File (`.grove/rules`)

The rules file is a line-by-line definition of the context.

### Basic Patterns

-   **Inclusion**: Standard glob patterns (e.g., `*.go`, `src/**/*.js`).
-   **Exclusion**: Patterns prefixed with `!` (e.g., `!*_test.go`).
-   **Comments**: Lines starting with `#` are ignored.
-   **Order**: The last matching pattern for a file determines its inclusion or exclusion.

### Hot and Cold Context

A `---` separator splits the rules file into two sections:
1.  **Hot Context (above `---`)**: Files that change frequently.
2.  **Cold Context (below `---`)**: Files that are stable (e.g., library code, dependencies).

This separation is used by other tools (like `grove-gemini`) to potentially cache the cold context.

### Workspace Aliases (`@a:`)

Workspace aliases refer to other projects discovered by Grove, allowing for cross-repository context. The alias format is based on the project's identifier.

-   **Standalone Project**: `@a:project-name/path/to/file`
-   **Ecosystem Sub-Project**: `@a:ecosystem-name:project-name/path/to/file`
-   **Project Worktree**: `@a:project-name:worktree-name/path/to/file`

**Example:**
```
# Include a directory from the grove-core project within the grove-ecosystem
@a:grove-ecosystem:grove-core/pkg/workspace/**
```

### Git Repositories

External git repositories can be included by adding their URL to the rules file. The `cx repo sync` command is used to clone or update these repositories to a local cache.

**Example:**
```
# Include a specific version of an external repository
https://github.com/some-org/some-repo@v1.2.3

# Include the main branch
https://github.com/another-org/another-repo
```

### Inheriting Rules (`@default:`)

The `@default:` directive inherits the default rule set from another project. It reads the `context.default_rules_path` from the target project's `grove.yml` and includes its rules.

**Example:**
```
# Inherit all rules from the project located at ../my-library
@default: ../my-library
```

### Dynamic Rules

-   `@cmd: <command>`: Executes a shell command and includes each line of its output as a file path.
-   `cx from-git`: A command to generate rules from files in recent commits or branches.
-   `cx from-cmd`: A command to generate rules from a command's output.

## Interactive View (`cx view`)

`cx view` launches a terminal-based file tree that visualizes the current context.

-   **Color Coding**: Files are colored based on their status (hot, cold, excluded).
-   **Interactivity**:
    -   `h`: Toggle inclusion in hot context.
    -   `c`: Toggle inclusion in cold context.
    -   `x`: Toggle exclusion.
-   **Live Updates**: Changes made in the TUI are written back to the `.grove/rules` file.

## Named Rule Sets (`.cx/`)

`grove-context` supports storing multiple named rule sets. This allows for switching between different context definitions for various tasks.

-   `.cx/`: Directory for version-controlled rule sets (e.g., `.cx/api.rules`).
-   `.cx.work/`: Directory for local, temporary rule sets (gitignored by default).

**Commands:**
-   `cx rules list`: List all available rule sets.
-   `cx rules set <name>`: Set a named rule set as the active, read-only source.
-   `cx rules load <name>`: Copy a named rule set to `.grove/rules` to use as a modifiable working copy.
-   `cx rules save <name>`: Save the current `.grove/rules` content to a new named rule set.

## Experimental Feature Details

### Hot/Cold Context Caching

> ⚠️ **CACHING COST WARNING**
> Improper cache configuration can result in **significant unexpected API costs**.
> Cache regeneration can cost hundreds of dollars if misconfigured with short TTLs or frequent changes.
> **Only use caching if you thoroughly understand LLM API pricing models and caching behavior.**

This feature is experimental and can lead to high API costs if misconfigured.
-   The cold context (files defined below `---`) can be cached by `grove-gemini` to reduce token costs on repeated requests.
-   **Caching Directives**:
    -   `@enable-cache`: Opt-in to enable caching for this rules file.
    -   `@freeze-cache`: Use the existing cache even if files have changed.
    -   `@no-expire`: Prevent the cache from expiring based on TTL.
    -   `@expire-time <duration>`: Set a custom TTL for the cache (e.g., `24h`).
-   **Risks**:
    -   Improper TTL settings can lead to excessive cache regeneration costs.
    -   Stale cached context can be used if files change without the cache being updated.
    -   Debugging can be difficult if the cache is out of sync.
-   **Recommendation**: Use this feature only if you have a thorough understanding of LLM API caching mechanisms and associated costs. Monitor API usage and billing closely.

### MCP Integration for Automatic Context Management

-   The `grove-mcp` tool enables agents to manage context by calling `cx` commands via a defined protocol.
-   This allows an LLM to dynamically adjust its own file-based context during a task.
-   This is an experimental feature for developing autonomous agents.

## Common Use Cases and Limitations

### Use Cases
-   Defining a consistent set of files to be included in an LLM prompt.
-   Switching between different "views" of a codebase (e.g., a "full-context" vs. a "docs-only" rule set).
-   Interactively building a context for a specific task using `cx view`.
-   Auditing which files are being sent to an LLM.

### Limitations
-   `cx` is a file preprocessor; it does not directly interact with any LLM APIs. Other tools like `grove-gemini` consume its output.
-   The tool relies on filesystem operations and `git` commands. Performance may vary on very large projects.
-   It is not a language-specific dependency management tool. It operates on file paths and patterns.