# Command Reference

This document provides a comprehensive reference for all `cx` commands, organized by functional category.

## Core Commands

These commands are for the fundamental workflow of creating, inspecting, and editing context.

### cx generate

Generates the primary context file from the active rules.

-   **Usage**: `cx generate`
-   **Description**: Reads the patterns from the `.grove/rules` file, resolves all matching file paths, and concatenates their contents into the `.grove/context` file. It also generates the `.grove/cached-context` file for any rules in the "cold" section (below a `---` separator).
-   **Arguments**: None
-   **Flags**:
    -   `--xml` (boolean, default: true): Use XML-style delimiters in the output file.
-   **Examples**:
    ```bash
    # Generate the context and cached-context files from .grove/rules
    cx generate
    ```
-   **Related Commands**: `cx list`, `cx show`, `cx edit`

---

### cx list

Lists all files included in the hot context.

-   **Usage**: `cx list`
-   **Description**: Resolves the active rules and prints the absolute paths of all files included in the "hot" context (rules above the `---` separator). This command is useful for verifying which files will be included in the final context before generation.
-   **Arguments**: None
-   **Flags**: None
-   **Examples**:
    ```bash
    # List all files currently in the hot context
    cx list

    # Pipe the file list to another command
    cx list | xargs wc -l
    ```
-   **Related Commands**: `cx generate`, `cx list-cache`, `cx stats`

---

### cx show

Prints the entire generated hot context file to standard output.

-   **Usage**: `cx show`
-   **Description**: Outputs the complete contents of the `.grove/context` file. This is useful for piping the generated context directly to other tools or language models. If the file doesn't exist, it will return an error prompting you to run `cx generate`.
-   **Arguments**: None
-   **Flags**: None
-   **Examples**:
    ```bash
    # Display the full context in the terminal
    cx show

    # Pipe the context to a language model CLI
    cx show | ollama run llama3
    ```
-   **Related Commands**: `cx generate`

---

### cx edit

Opens the active rules file in your default editor.

-   **Usage**: `cx edit`
-   **Description**: Opens the `.grove/rules` file in the editor specified by the `$EDITOR` environment variable. If no rules file exists, it creates one with boilerplate content. This command is designed for rapid iteration on context rules.
-   **Arguments**: None
-   **Flags**: None
-   **Examples**:
    ```bash
    # Open .grove/rules in vim, nano, or VS Code (based on $EDITOR)
    cx edit
    ```
-   **Related Commands**: `cx reset`, `cx set-rules`

---

## Git Integration

Commands for creating context based on Git repository history.

### cx from-git

Generates a new rules file based on Git history.

-   **Usage**: `cx from-git`
-   **Description**: Overwrites the `.grove/rules` file with explicit paths to files that have changed according to the specified Git criteria. This is useful for creating a context focused on recent work or specific changes.
-   **Arguments**: None
-   **Flags**:
    -   `--staged`: Include only files that are staged for the next commit.
    -   `--branch <range>`: Include files changed in a branch range (e.g., `main..HEAD`).
    -   `--commits <n>`: Include files from the last `n` commits.
    -   `--since <date|commit>`: Include files changed since a specific date (e.g., `"2 days ago"`) or commit hash.
-   **Examples**:
    ```bash
    # Create context from all files staged for commit
    cx from-git --staged

    # Create context from changes on the current branch compared to main
    cx from-git --branch main..HEAD

    # Create context from changes in the last 3 commits
    cx from-git --commits 3
    ```
-   **Related Commands**: `cx diff`

---

### cx diff

Compares the current context with a saved snapshot.

-   **Usage**: `cx diff [snapshot|current]`
-   **Description**: Shows the differences between the files resolved from the current `.grove/rules` and the files resolved from a saved snapshot. It lists added and removed files, and provides a summary of changes in file count, token count, and total size.
-   **Arguments**:
    -   `[snapshot|current]` (optional): The name of the snapshot to compare against. Defaults to comparing against an empty context if omitted.
-   **Flags**: None
-   **Examples**:
    ```bash
    # See what has changed since the 'initial-setup' snapshot was saved
    cx diff initial-setup

    # See what the current context includes compared to an empty state
    cx diff
    ```
-   **Related Commands**: `cx save`, `cx list-snapshots`

---

## Snapshots

Commands for saving, loading, and managing different rule configurations.

### cx save

Saves the current rules file as a named snapshot.

-   **Usage**: `cx save <name>`
-   **Description**: Creates a copy of the current `.grove/rules` file and stores it in `.grove/context-snapshots/` with the given name.
-   **Arguments**:
    -   `<name>` (required): The name to give the snapshot.
-   **Flags**:
    -   `--desc <description>`: An optional description for the snapshot.
-   **Examples**:
    ```bash
    # Save the current rules as 'feature-x-setup'
    cx save feature-x-setup --desc "Rules for developing feature X"
    ```
-   **Related Commands**: `cx load`, `cx list-snapshots`

---

### cx load

Loads a saved snapshot into the active rules file.

-   **Usage**: `cx load <name>`
-   **Description**: Overwrites the current `.grove/rules` file with the content of a previously saved snapshot.
-   **Arguments**:
    -   `<name>` (required): The name of the snapshot to load.
-   **Flags**: None
--   **Examples**:
    ```bash
    # Restore the rules from the 'feature-x-setup' snapshot
    cx load feature-x-setup
    ```
-   **Related Commands**: `cx save`, `cx list-snapshots`

---

### cx list-snapshots

Lists all saved snapshots.

-   **Usage**: `cx list-snapshots`
-   **Description**: Displays a table of all available snapshots with metadata, including name, creation date, file count, token count, size, and description.
-   **Arguments**: None
-   **Flags**:
    -   `--sort <column>` (string, default: "date"): Sort by column: `date`, `name`, `size`, `tokens`, `files`.
    -   `--desc` (boolean, default: true): Sort in descending order.
-   **Examples**:
    ```bash
    # List all snapshots, sorted by name in ascending order
    cx list-snapshots --sort name --desc=false
    ```
-   **Related Commands**: `cx save`, `cx load`

---

## Interactive Tools

Commands that provide a terminal-based user interface for inspecting context.

### cx view

Launches an interactive TUI to visualize context composition.

-   **Usage**: `cx view`
-   **Description**: Starts a terminal user interface that displays a file tree of your project. It visually indicates whether each file is included in the hot context, cold context, excluded by a rule, or omitted. You can navigate the tree and interactively modify the rules file by pressing keys (`h` for hot, `c` for cold, `x` for exclude).
-   **Arguments**: None
-   **Flags**: None
-   **Examples**:
    ```bash
    # Start the interactive context viewer
    cx view
    ```
-   **Related Commands**: `cx stats`, `cx dashboard`

---

### cx dashboard

Displays a live-updating dashboard of context statistics.

-   **Usage**: `cx dashboard`
-   **Description**: Launches a terminal UI that displays real-time statistics for both hot and cold contexts. The dashboard automatically updates when files in the project are changed, created, or deleted.
-   **Arguments**: None
-   **Flags**:
    -   `-H, --horizontal`: Display statistics side by side.
    -   `-p, --plain`: Output plain text statistics without launching the TUI.
-   **Examples**:
    ```bash
    # Launch the live dashboard TUI
    cx dashboard

    # Print a one-time stats summary to the console
    cx dashboard --plain
    ```
-   **Related Commands**: `cx stats`, `cx view`

---

### cx stats

Provides a detailed analysis of the context composition.

-   **Usage**: `cx stats`
-   **Description**: Generates and displays a detailed report on the composition of both the hot and cold contexts. The report includes a summary of file counts, token counts, and total size, as well as breakdowns by programming language and a list of the largest files.
-   **Arguments**: None
-   **Flags**:
    -   `--top <n>` (int, default: 5): The number of largest files to show in the report.
-   **Examples**:
    ```bash
    # Show standard statistics
    cx stats

    # Show the top 10 largest files in the context
    cx stats --top 10
    ```
-   **Related Commands**: `cx dashboard`, `cx list`

---

## Repository Management

Commands for managing external Git repositories used in the context.

### cx repo

Manages external Git repositories included in the context.

-   **Usage**: `cx repo [command]`
-   **Description**: A parent command for managing Git repositories that are cloned and tracked by `cx`. Repositories are added to the context by including their URL in the `.grove/rules` file.
-   **Subcommands**:
    -   `cx repo list`: Lists all tracked repositories, their pinned versions, and audit status.
    -   `cx repo sync`: Fetches the latest changes for all tracked repositories.
    -   `cx repo audit <url>`: Initiates an interactive, LLM-based security audit for a repository.
-   **Examples**:
    ```bash
    # List all external repositories used in the context
    cx repo list

    # Fetch updates for all external repositories
    cx repo sync

    # Run a security audit on a new repository before adding it to rules
    cx repo audit https://github.com/some/dependency
    ```
-   **Related Commands**: None

---

## Validation & Maintenance

Commands for maintaining and validating your context configuration.

### cx validate

Verifies the integrity and accessibility of files in the context.

-   **Usage**: `cx validate`
-   **Description**: Checks all files resolved from the active rules to ensure they exist and are readable. It reports any missing files, duplicates, or permission issues.
-   **Arguments**: None
-   **Flags**: None
-   **Examples**:
    ```bash
    # Check the current context for any issues
    cx validate
    ```
-   **Related Commands**: `cx fix`

---

### cx fix

(Deprecated) Automatically fix context validation issues.

-   **Usage**: `cx fix`
-   **Description**: This command is deprecated because context is now resolved dynamically from rules. It no longer performs any action but prints a message advising the user to edit their rules file directly to fix issues.
-   **Arguments**: None
-   **Flags**: None
-   **Related Commands**: `cx validate`, `cx edit`

---

### cx reset

Resets the rules file to project defaults.

-   **Usage**: `cx reset`
-   **Description**: Replaces the content of `.grove/rules` with a default configuration. If a `context.default_rules_path` is defined in `grove.yml`, it uses that file. Otherwise, it creates a boilerplate rules file.
-   **Arguments**: None
-   **Flags**:
    -   `-f, --force`: Reset without asking for confirmation.
-   **Examples**:
    ```bash
    # Reset the rules file, which will prompt for confirmation
    cx reset

    # Reset without a prompt
    cx reset --force
    ```
-   **Related Commands**: `cx edit`, `cx set-rules`

---

### cx set-rules

Sets the active rules from an external file.

-   **Usage**: `cx set-rules <path-to-rules-file>`
-   **Description**: Copies the content of a specified file into `.grove/rules`, making it the active rule set for the project. This is useful for switching between different context configurations for different tasks.
-   **Arguments**:
    -   `<path-to-rules-file>` (required): The path to the file containing the desired rules.
-   **Flags**: None
-   **Examples**:
    ```bash
    # Switch to a rule set designed for documentation generation
    cx set-rules .grove/docs.rules
    ```
-   **Related Commands**: `cx edit`, `cx reset`

---

## Cache Management

Commands related to the cold (cached) context.

### cx list-cache

Lists all files included in the cold (cached) context.

-   **Usage**: `cx list-cache`
-   **Description**: Resolves the active rules and prints the absolute paths of all files included in the "cold" context (rules below the `---` separator).
-   **Arguments**: None
-   **Flags**: None
-   **Examples**:
    ```bash
    # List all files currently in the cold context
    cx list-cache
    ```
-   **Related Commands**: `cx list`

---

## Utility

General-purpose utility commands.

### cx version

Prints the version information for the binary.

-   **Usage**: `cx version`
-   **Description**: Displays the version, commit hash, branch, and build date for the `cx` binary.
-   **Arguments**: None
-   **Flags**:
    -   `--json`: Output version information in JSON format.
-   **Examples**:
    ```bash
    # Show version information
    cx version

    # Get version info as JSON for scripting
    cx version --json
    ```
-   **Related Commands**: None