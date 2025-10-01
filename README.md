<!-- DOCGEN:OVERVIEW:START -->

<img src="docs/images/grove-context-readme.svg" width="60%" />

`grove-context` (`cx`) is a command-line tool that generates file-based context for Large Language Models (LLMs) from a set of user-defined patterns. It reads files matching these patterns, concatenates their content into a structured format, and excludes binary files by default.

<!-- placeholder for animated gif -->

### Key Features

*   **Pattern-Based File Selection**: Reads include/exclude patterns from a `.grove/rules` file using a `.gitignore`-style syntax.
*   **Context Generation**: Generates a concatenated context file with XML delimiters.
*   **Rules Editing**: Opens the `.grove/rules` file in the default editor via the `cx edit` command.
*   **Context Inspection**: Lists included files (`cx list`) and displays token/file counts and language distribution (`cx stats`).
*   **Interactive TUI**: Provides a terminal interface (`cx view`) to browse the file tree, view file statuses, and modify rules.
*   **Rule Management**: Switches the active rules by copying an external file (`cx set-rules`), restores a project-defined default (`cx reset`), or saves/loads named rule configurations (`cx save`/`load`).
*   **Git Integration**: Generates a temporary rules file from Git history (`cx from-git`), such as files changed on a branch or staged for commit.
*   **External Repository Management**: Includes files from external Git repositories specified by URL in the rules file and manages local clones via the `cx repo` command, which includes an audit workflow.

## Ecosystem Integration

`grove-context` is a foundational tool within the Grove ecosystem that provides context to other LLM-powered tools.

*   **`grove-gemini` and `grove-openai`**: The `grove llm request` command calls the `cx` binary to generate context before sending a request to an LLM provider. The hot/cold context separation allows `grove-gemini` to send a stable "cold" context for caching, which is useful for large contexts.
*   **`grove-docgen`**: The documentation generator uses `cx` to gather a codebase's files before generating documentation.

This allows different tools in the ecosystem to operate on a consistent and reproducible set of files defined by a single `.grove/rules` file.

## Installation

Install via the Grove meta-CLI:
```bash
grove install context
```

Verify installation:
```bash
cx version
```

Requires the `grove` meta-CLI. See the [Grove Installation Guide](https://github.com/mattsolo1/grove-meta/blob/main/docs/02-installation.md) if you don't have it installed.

<!-- DOCGEN:OVERVIEW:END -->

<!-- DOCGEN:TOC:START -->

See the [documentation](docs/) for detailed usage instructions:
- [Overview](docs/01-overview.md)
- [Examples](docs/02-examples.md)
- [Rules & Patterns](docs/03-rules-and-patterns.md)
- [Context Generation](docs/04-context-generation.md)
- [Loading Rules](docs/05-loading-rules.md)
- [Context TUI](docs/06-context-tui.md)
- [Git Workflows](docs/07-git-workflows.md)
- [External Repositories](docs/08-external-repositories.md)
- [Experimental Features](docs/09-experimental.md)
- [Command Reference](docs/10-command-reference.md)

<!-- DOCGEN:TOC:END -->
