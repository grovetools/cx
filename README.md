<!-- DOCGEN:OVERVIEW:START -->

<img src="docs/images/grove-context-readme.svg" width="60%" />

`grove-context` (`cx`) is a pattern-based command-line tool for dynamically generating and managing file-based context for Large Language Models (LLMs). It automates the often manual and error-prone process of collecting and concatenating project files into a single, structured format, providing a repeatable and version-controlled workflow. With smart defaults that automatically exclude binary files, it focuses on including text-based source code relevant to your task.

<!-- placeholder for animated gif -->

### Key Features

*   **Pattern-Based File Selection**: Define context using a `.gitignore`-style syntax in a `.grove/rules` file to precisely include or exclude files and directories.
*   **Automatic Context Generation**: Dynamically generate a structured context file from your rules, with intelligent defaults that filter out binary files and other non-text assets.
*   **Quick Rules Editing**: Open the active rules file in your default editor instantly with `cx edit`, perfect for binding to a keyboard shortcut for rapid iteration.
*   **Context Inspection**: Easily verify your context with `cx list` to see included files and `cx stats` for a detailed breakdown of token counts, file sizes, and language distribution.
*   **Interactive TUI**: Launch a terminal user interface with `cx view` to visually browse your project, see which files are included or excluded in real-time, and modify rules interactively.
*   **Flexible Rule Management**: Load different rule sets for various tasks using `cx set-rules`, reset to project defaults with `cx reset`, and manage configurations with `cx save`/`load`.
*   **Git Integration**: Generate context based on your Git history, such as including all files changed since the last commit, on a specific branch, or only staged files using `cx from-git`.
*   **External Repository Management**: Include files from external Git repositories directly in your rules, and manage them with the `cx repo` command, which includes a security audit workflow.

## Ecosystem Integration

`grove-context` is a foundational tool within the Grove ecosystem, serving as the primary context provider for other LLM-powered tools. It is used by:

*   **`grove-gemini` and `grove-openai`**: The `grove llm request` facade uses `cx` to automatically gather context before making a request to an LLM provider. `grove-gemini` in particular leverages the hot/cold context separation feature to optimize token usage with Gemini's caching capabilities, which is especially useful for large contexts.
*   **`grove-docgen`**: The documentation generator uses `cx` to build a comprehensive understanding of a codebase before generating documentation.

By centralizing context management, `cx` ensures that all tools in the ecosystem operate with a consistent and reproducible understanding of the project.

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

## Documentation

See the [documentation](docs/) for detailed usage instructions:
- [Overview](docs/01-overview.md) - Introduction and core concepts
- [Examples](docs/02-examples.md) - Common usage patterns
- [Rules & Patterns](docs/03-rules-and-patterns.md) - Writing effective rules files
- [Context Generation](docs/04-context-generation.md) - How context is generated
- [Loading Rules](docs/05-loading-rules.md) - Rule file loading and inheritance
- [Context TUI](docs/06-context-tui.md) - Interactive context viewer
- [Git Workflows](docs/07-git-workflows.md) - Git integration features
- [External Repositories](docs/08-external-repositories.md) - Including external files
- [Experimental Features](docs/09-experimental.md) - Beta features
- [Command Reference](docs/10-command-reference.md) - Complete CLI reference


<!-- DOCGEN:TOC:START -->

See the [documentation](docs/) for detailed usage instructions:
- [Overview](docs/01-overview.md) - <img src="./images/grove-context-readme.svg" width="60%" />
- [Examples](docs/02-examples.md) - This guide provides practical examples of how to use Grove Context (`cx`) in ...
- [Rules & Patterns](docs/03-rules-and-patterns.md) - The `.grove/rules` file is the core of Grove Context's functionality. It defi...
- [Context Generation](docs/04-context-generation.md) - Grove Context (`cx`) transforms a set of rules into a structured, file-based ...
- [Loading Rules](docs/05-loading-rules.md) - `grove-context` (`cx`) provides a flexible system for managing different rule...
- [Context TUI](docs/06-context-tui.md) - The `cx view` command launches a terminal user interface (TUI) for interactiv...
- [Git Workflows](docs/07-git-workflows.md) - `grove-context` integrates with Git to create dynamic, task-specific contexts...
- [External Repositories](docs/08-external-repositories.md) - `grove-context` can include files from sources outside the current project's ...
- [Experimental Features](docs/09-experimental.md) - This section covers features in `grove-context` that are either under active ...
- [Command Reference](docs/10-command-reference.md) - This document provides a comprehensive reference for all `cx` commands, organ...

<!-- DOCGEN:TOC:END -->
