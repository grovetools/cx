# Command Reference: grove-context (`cx`)

This document provides a reference for all commands available in the `grove-context` (`cx`) tool.

## Core Commands

Commands for generating, inspecting, and editing the primary context files.

---

### `cx generate`

**Usage**: `cx generate [--xml]`

**Description**:
Reads file patterns from the active rules file (`.grove/rules` by default), resolves them to a list of files, and concatenates their contents into the `.grove/context` (hot context) and `.grove/cached-context` (cold context) files.

**Arguments**: None.

**Flags**:
- `--xml` (bool, default: `true`): Use XML-style delimiters for context files.

**Examples**:
```bash
# Generate context using active rules
cx generate
```

**Related Commands**: `cx update`, `cx show`, `cx edit`

---

### `cx list`

**Usage**: `cx list`

**Description**:
Prints the absolute paths of all files included in the hot context, based on the active rules file. The list is deduplicated and sorted.

**Arguments**: None.

**Flags**: None.

**Examples**:
```bash
# List all files in the current hot context
cx list

# Pipe the list to another command
cx list | wc -l
```

**Related Commands**: `cx show`, `cx listcache`

---

### `cx show`

**Usage**: `cx show`

**Description**:
Outputs the entire content of the generated `.grove/context` file to standard output. This is useful for piping the context directly to other tools or LLMs.

**Arguments**: None.

**Flags**: None.

**Examples**:
```bash
# Print the full context to the terminal
cx show

# Pipe the context to an LLM
cx show | llm -m gemini-2.5-pro "Analyze this code"
```

**Related Commands**: `cx list`, `cx generate`

---

### `cx edit`

**Usage**: `cx edit`

**Description**:
Opens the active `.grove/rules` file in the default editor specified by the `$EDITOR` environment variable. If the file does not exist, it will be created with boilerplate content.

**Arguments**: None.

**Flags**: None.

**Examples**:
```bash
# Open the rules file for editing
cx edit
```

**Related Commands**: `cx rules`, `cx reset`

## Git Integration

Commands for creating context based on Git repository history.

---

### `cx from-git`

**Usage**: `cx from-git [flags]`

**Description**:
Generates context rules by including files that have changed in the Git repository based on specified criteria. It overwrites the active `.grove/rules` file with the list of changed files.

**Arguments**: None.

**Flags**:
- `--since <date|commit>`: Include files changed since a specific date, time, or commit hash.
- `--branch <range>`: Include files changed in a branch or commit range (e.g., `main..HEAD`).
- `--staged`: Include only files that are currently staged for commit.
- `--commits <n>`: Include files from the last `n` commits.

**Examples**:
```bash
# Include files changed in the last 24 hours
cx from-git --since "24 hours ago"

# Include files changed on the current branch compared to 'main'
cx from-git --branch main

# Include all staged files
cx from-git --staged
```

**Related Commands**: `cx diff`

---

### `cx diff`

**Usage**: `cx diff [ruleset-name]`

**Description**:
Compares the files included by the currently active rules against a named rule set (snapshot). It shows which files were added or removed, and the change in total token count and file size.

**Arguments**:
- `ruleset-name` (optional): The name of the rule set to compare against. If omitted, compares against an empty context.

**Flags**: None.

**Examples**:
```bash
# Compare current context to the 'dev-no-tests' rule set
cx diff dev-no-tests

# See what the current context adds compared to an empty context
cx diff
```

**Related Commands**: `cx rules`, `cx from-git`

## Snapshots (Rule Sets)

Commands for managing named context configurations, referred to as rule sets or snapshots. These are stored in `.cx/` (version-controlled) and `.cx.work/` (local-only) directories.

---

### `cx rules`

**Usage**: `cx rules [subcommand]`

**Description**:
Interactive TUI for managing named rule sets. When run without arguments, opens an interactive selector to choose from available rule sets in `.cx/` and `.cx.work/`.

**Subcommands**:
- `list`: List all available rule sets
- `set <name>`: Set active rule set (read-only reference)
- `load <name>`: Copy rule set to working file (editable)
- `save <name>`: Save current rules as named set

**Arguments**: None (when run without subcommand).

**Flags**: See individual subcommands.

**Examples**:
```bash
# Open interactive selector
cx rules

# Use specific subcommand
cx rules set backend
```

**Related Commands**: `cx edit`, `cx view`

---

### `cx rules list`

**Usage**: `cx rules list [--for-project <alias>] [--json]`

**Description**:
Lists all available rule sets found in the `.cx/` and `.cx.work/` directories. Indicates the currently active rule set.

**Arguments**: None.

**Flags**:
- `--for-project <alias>`: List rule sets for a different project specified by its alias.
- `--json`: Output the list of rule sets in JSON format.

**Examples**:
```bash
# List all rule sets for the current project
cx rules list
```

**Related Commands**: `cx rules set`, `cx rules save`

---

### `cx rules save`

**Usage**: `cx rules save <name> [--work]`

**Description**:
Saves the currently active rules (from `.grove/rules` or another set) to a new named file in the `.cx/` directory.

**Arguments**:
- `name` (required): The name to give the new rule set.

**Flags**:
- `-w`, `--work`: Save the rule set to the `.cx.work/` directory for temporary, untracked sets.

**Examples**:
```bash
# Save the current rules as a version-controlled set named 'feature-x-api'
cx rules save feature-x-api

# Save the current rules as a temporary, local-only set
cx rules save my-temp-rules --work
```

**Related Commands**: `cx rules load`, `cx rules set`

---

### `cx rules set`

**Usage**: `cx rules set <name> [--work]`

**Description**:
Sets a named rule set from `.cx/` or `.cx.work/` as the active context source. This creates a read-only reference to the named set, meaning the original set remains unchanged. To create an editable copy, use `cx rules load` instead.

**Arguments**:
- `name` (required): The name of the rule set to activate.

**Flags**:
- `--work`: Look for the rule set in `.cx.work/` instead of `.cx/`.

**Examples**:
```bash
# Set backend rules as active
cx rules set backend

# Set a personal rule set from .cx.work/
cx rules set my-feature --work

# Switch between contexts for different tasks
cx rules set frontend  # Work on UI
cx rules set backend   # Work on API
```

**Related Commands**: `cx rules load`, `cx rules list`, `cx rules save`

---

### `cx rules load`

**Usage**: `cx rules load <name>`

**Description**:
Copies a named rule set from `.cx/` or `.cx.work/` to `.grove/rules`, creating a modifiable working copy. This makes `.grove/rules` the active context source.

**Arguments**:
- `name` (required): The name of the rule set to load.

**Flags**: None.

**Examples**:
```bash
# Create a local, editable copy of the 'default' rule set
cx rules load default
```

**Related Commands**: `cx rules save`, `cx rules set`

## Interactive Tools

Commands for interactive context management and analysis.

---

### `cx view`

**Usage**: `cx view [--page <page>]`

**Description**:
Launches a multi-tab interactive terminal UI for visualizing and managing context composition.

**Tabs**:
- **TREE**: A file tree showing which files are included (hot/cold), excluded, or ignored.
- **RULES**: A view of the active rules file, allowing for quick edits.
- **STATS**: A breakdown of context by file type and a list of the largest files by token count.
- **LIST**: A detailed, flat list of all files considered, with options for exclusion.

**Arguments**: None.

**Flags**:
- `-p`, `--page <page>` (string, default: `tree`): The page to open on startup (tree, rules, stats, list).

**Examples**:
```bash
# Launch the interactive view on the default TREE page
cx view

# Start directly on the STATS page
cx view --page stats
```

**Related Commands**: `cx stats`, `cx rules`

---

### `cx stats`

**Usage**: `cx stats [rules-file] [--top <n>] [--per-line]`

**Description**:
Provides a detailed command-line analysis of the context composition, including language breakdown, largest files, and token distribution.

**Arguments**:
- `rules-file` (optional): Path to a specific rules file to analyze. If omitted, uses the active rules file.

**Flags**:
- `--top <n>` (int, default: `5`): The number of largest files to display.
- `--per-line`: Provide token and file count statistics for each individual line in the rules file (output as JSON).

**Examples**:
```bash
# Get stats for the currently active context
cx stats

# Get stats for a different rule set and show the top 10 largest files
cx stats .cx/docs.rules --top 10
```

**Related Commands**: `cx view`

## Repository Management

Commands for managing external Git repositories used as context sources.

---

### `cx repo`

**Usage**: `cx repo <subcommand>`

**Description**:
Provides subcommands for managing external Git repositories that are cloned and used as sources for context rules (e.g., via `git@github.com:owner/repo` patterns).

**Subcommands**:
- `list`: List all tracked repositories and their status.
- `sync`: Fetch updates for all tracked repositories.
- `audit`: Perform an interactive security audit on a repository.

**Examples**:
```bash
# List all repositories defined in the rules files
cx repo list

# Fetch updates for all repositories
cx repo sync
```

**Related Commands**: `cx rules`

## Workspace Management

Commands for interacting with the Grove workspace model.

---

### `cx workspace list`

**Usage**: `cx workspace list [--json]`

**Description**:
Lists all discovered workspaces, including projects, ecosystems, and their worktrees. For each entry, it provides a unique, resolvable alias (identifier).

**Arguments**: None.

**Flags**:
- `--json`: Output the workspace list in JSON format.

**Examples**:
```bash
# List all workspaces and their aliases
cx workspace list
```

**Related Commands**: `cx resolve`

---

### `cx resolve`

**Usage**: `cx resolve <rule>`

**Description**:
Resolves a single rule pattern, such as an alias (`@a:my-repo/path`), to its corresponding list of absolute file paths.

**Arguments**:
- `rule` (required): The rule pattern to resolve.

**Flags**:
- `--rules-file <path>`: Path to a rules file for context-aware resolution.
- `--line-number <n>`: Line number within the rules file for context-aware resolution.

**Examples**:
```bash
# Resolve a simple alias
cx resolve "@a:grove-core"

# Resolve an alias with a path pattern
cx resolve "@a:grove-ecosystem:grove-flow/cmd"
```

**Related Commands**: `cx workspace list`, `cx edit`

## Validation & Maintenance

Commands for maintaining the integrity and state of context rules.

---

### `cx validate`

**Usage**: `cx validate`

**Description**:
Checks all files in the current context for integrity. It verifies that all files exist, are accessible, and reports any duplicates.

**Arguments**: None.

**Flags**: None.

**Examples**:
```bash
# Validate the files in the current context
cx validate
```

**Related Commands**: `cx fix`

---

### `cx fix`

**Usage**: `cx fix`

**Description**:
This command is deprecated. Context is now resolved dynamically from rules, so there is no file list to fix. To fix issues, edit the rules file directly.

**Arguments**: None.

**Flags**: None.

**Related Commands**: `cx edit`, `cx validate`

---

### `cx reset`

**Usage**: `cx reset [--force]`

**Description**:
Resets the active `.grove/rules` file to the project's default, as defined by `context.default_rules_path` in `grove.yml`. If no default is configured, a boilerplate rules file is created.

**Arguments**: None.

**Flags**:
- `-f`, `--force`: Reset the rules file without a confirmation prompt.

**Examples**:
```bash
# Reset the rules file to its default state
cx reset
```

**Related Commands**: `cx edit`, `cx rules load`

---

### `cx setrules`

**Usage**: `cx setrules <path-to-rules-file>`

**Description**:
Copies an external rules file to `.grove/rules`, making it the active set of rules for the project.

**Arguments**:
- `path-to-rules-file` (required): The path to the rules file to use.

**Flags**: None.

**Examples**:
```bash
# Set the active rules from a file in a shared directory
cx setrules ../../shared/rules/go-api.rules
```

**Related Commands**: `cx rules load`, `cx rules set`

## Cache Management

Commands for inspecting cached context.

---

### `cx listcache`

**Usage**: `cx listcache`

**Description**:
Lists the absolute paths of all files included in the cold (cached) context, based on the patterns found after the `---` separator in the active rules file.

**Arguments**: None.

**Flags**: None.

**Examples**:
```bash
# List all files that are part of the cached context
cx listcache
```

**Related Commands**: `cx list`

## Utility

Miscellaneous utility commands.

---

### `cx version`

**Usage**: `cx version [--json]`

**Description**:
Prints the version, commit, and build date for the `cx` binary.

**Arguments**: None.

**Flags**:
- `--json`: Output version information in JSON format.

**Examples**:
```bash
# Show version information
cx version
```