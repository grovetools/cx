# grove-context (cx)

`grove-context` (`cx`) is a command-line tool for assembling file-based context for Large Language Models (LLMs). It is designed to support a **planning → execution** workflow, where a comprehensive set of files defining a feature's "universe" is used to generate detailed implementation plans from large-context LLMs.

This approach embraces large context windows (200k-2M+ tokens) for high-level planning, rather than attempting to work around context limits with retrieval-based methods. It acts as the foundational context engine for the Grove ecosystem.

<!-- placeholder for animated gif -->

## Key Features

-   **Declarative Context Definition:** Define the context "universe" using a `.grove/rules` file with gitignore-style patterns and directives. The tool handles the file resolution and assembly.

-   **Workspace-Aware Aliasing:** Reference files across different projects, ecosystems, and worktrees using a consistent alias system (e.g., `@a:ecosystem:repo/path`). This is powered by `grove-core`'s workspace discovery.

-   **Rule Set Management:** Create, manage, and switch between named rule sets for different tasks (e.g., `frontend-only`, `full-stack`, `audit`) using the `cx rules` command.

-   **Interactive Visualization:** Explore context composition with an interactive TUI (`cx view`) that provides a tree view of your project, showing the status of each file (included, excluded, gitignored).

-   **Token & Cost Analytics:** Analyze token count, file size, and language distribution *before* sending a request to a paid API using the `cx stats` command.

-   **Security Boundaries:** Restricts file access to discovered workspaces and paths explicitly allowed in your `grove.yml` configuration, preventing accidental inclusion of sensitive system files.

-   **Git Integration:** Generate context dynamically from git history, including staged files (`cx from-git --staged`), recent commits, or branch diffs.

## How It Works

`grove-context` follows a deterministic pipeline to resolve a final list of files:

1.  **Load Rules:** It reads the active rules file (either `.grove/rules` or a named set from `.cx/` specified in state).
2.  **Expand Directives:** It recursively expands import directives (`@default` for project defaults, `::` for ruleset imports).
3.  **Resolve Aliases:** It resolves all workspace aliases (`@a:`) to their absolute file paths using `grove-core`'s discovery mechanism.
4.  **Filter by Gitignore:** It walks the specified file trees, filtering out files and directories matched by `.gitignore` files.
5.  **Apply Patterns:** It applies all inclusion and exclusion patterns using a "last match wins" logic, similar to `.gitignore`.
6.  **Generate Context:** The final list of files is used to generate a single, concatenated context file (`.grove/context`), separating files into hot (dynamic) and cold (cacheable) sections.

## Ecosystem Integration

`grove-context` serves as a foundational context engine that enables other tools in the Grove ecosystem.

-   **`grove-core`**: Provides the workspace discovery and identification engine that powers `cx`'s multi-repository alias resolution system.
-   **`grove-flow`**: Manages per-job context. A job in a `grove-flow` plan can specify a `rules_file` in its frontmatter, which `cx` uses to generate a focused context for that specific task.
-   **`grove-gemini`**: The `gemapi request` command automatically uses `cx` to generate context from the active `.grove/rules` file, handling the separation of hot and cold context for efficient caching with the Gemini API.
-   **`grove-nvim`**: Offers editor integration for `.grove/rules` files, providing real-time token counts as virtual text, file previews for rules, and commands to manage context directly from Neovim.

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
