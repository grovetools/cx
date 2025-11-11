# Context Generation Pipeline

The `grove-context` tool, aliased as `cx`, provides a mechanism for generating a context file from a set of rules. This document describes the pipeline from rule definition to final output and its integration with other Grove tools.

## 1. Generation Pipeline Overview

The context generation process follows a clear transformation:

1.  **Rules File (`.grove/rules`)**: The process starts with a rules file containing patterns that specify which files to include or exclude. The active rules file can be `.grove/rules` or a named set from `.cx/` or `.cx.work/`.
2.  **File List Resolution**: The tool resolves these patterns into a definitive list of file paths. This step respects `.gitignore` files, handles aliases (`@a:`), and resolves rule set imports (`::`).
3.  **Context Output (`.grove/context`)**: The contents of the resolved files are concatenated into a single XML-formatted file, `.grove/context`, which is then used by LLM-based tools.

### Automatic and Manual Generation

-   **Automatic**: Tools like `grove-gemini` and `grove-flow` automatically trigger context generation before making an LLM request to ensure the context is up-to-date.
-   **Manual**: You can manually trigger this process using the `cx generate` command. This is useful for inspecting the final output or when using the context file with external tools.

## 2. The `cx list` Command

The `cx list` command displays the absolute paths of all files included in the context based on the current rules. Its primary purpose is to allow verification of the context before it is used.

**Example Usage:**

```bash
# List all files currently in the context
cx list
```

The command outputs a simple, newline-separated list of file paths, suitable for piping to other commands. For detailed metrics like token counts and file sizes, use the `cx stats` command.

## 3. The .grove/context Output File

The final output of the generation process is the `.grove/context` file. This file is not meant to be edited directly and should be added to `.gitignore`.

### Format Specification

The file uses a simple XML structure to wrap the content of each included file. This format provides clear delimiters and metadata for LLMs to parse.

```xml
<file path="src/main.go">
// file content here
</file>

<file path="pkg/config.yaml">
# yaml content here
</file>
```

-   Each file's content is enclosed in a `<file>` tag.
-   The `path` attribute contains the file path relative to the project root.

## 4. Context Statistics with `cx stats`

The `cx stats` command provides a detailed analysis of the context's composition, helping you monitor its size and complexity.

**Features:**

-   **Summary Metrics**: Total number of files, total token count, and total file size.
-   **Language Distribution**: A breakdown of the context by programming language, showing token and file counts for each.
-   **Largest Files**: A list of the largest files by token count, helping to identify major contributors to context size.

**Example Usage:**

```bash
# Show statistics for the current context
cx stats
```

## 5. File Handling and Security

`grove-context` includes mechanisms to handle different file types and enforce security boundaries to prevent accidental inclusion of unintended files.

-   **Binary File Exclusion**: Binary files (e.g., images, executables, archives) are detected and excluded by default to keep the context focused on text-based content.
-   **Security Boundaries**: The tool restricts file inclusion to specific, allowed root directories. By default, these are:
    1.  Discovered Grove workspaces (projects, ecosystems, and their worktrees).
    2.  The `~/.grove/` directory.
    3.  Notebook root directories defined in `grove.yml`.
    4.  Paths explicitly defined in `context.allowed_paths` in `grove.yml`.

    This boundary prevents rules like `../**/*` from including arbitrary files from the filesystem (e.g., `/etc/passwd`). Any attempt to include a file outside these boundaries is ignored, and a warning is printed.

### Best Practices for Security

-   Always review the output of `cx list` before using the generated context, especially before sharing it.
-   Add sensitive files and directories (e.g., `secrets/`, `.env*`) to your exclusion patterns in `.grove/rules`.
-   Keep the generated `.grove/context` file in your project's `.gitignore`.

## 6. Integration Points

The context generation pipeline is a core component used by several other Grove tools.

-   **grove-gemini**: When making a request with `gemapi request`, context is automatically generated. Hot context files are passed as dynamic files, while cold context is managed via Gemini's caching API.
-   **grove-flow**: Before executing `oneshot` or `chat` jobs, `grove-flow` regenerates the context to ensure it is current. Jobs can specify their own `rules_file` in their frontmatter for job-specific context. Context is scoped to the job's working directory or worktree.
-   **grove-nvim**: The Neovim plugin provides real-time feedback while editing `.grove/rules` files. It displays virtual text next to each rule showing the number of files and tokens it contributes, and allows interactive preview of the files matched by a rule.