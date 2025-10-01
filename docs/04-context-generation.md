# Context Generation Pipeline

Grove Context (`cx`) transforms a set of rules into a structured, file-based context that can be provided to Large Language Models (LLMs). This document explains the pipeline from rules definition to the final output file.

## Generation Pipeline Overview

The core function of `cx` is to execute a repeatable pipeline:

1.  **Rules Definition**: File inclusion and exclusion is defined in a `.grove/rules` file using a `.gitignore`-style syntax.
2.  **File Resolution**: `cx` reads the rules, walks the specified file paths, and resolves a final list of files. It filters out binary files and respects `.gitignore` patterns by default.
3.  **Context Generation**: The content of the resolved files is concatenated into a single, structured XML file located at `.grove/context`.

This process is triggered either manually by running `cx generate` or automatically by other Grove tools like `grove-gemini` before making a request to an LLM.

## Inspecting the File List with `cx list`

Before generating the full context, `cx list` can be used to verify which files will be included in the **hot context**. This command resolves the patterns in the active rules file and prints a list of the resulting absolute file paths. It is used for debugging rules to ensure the correct files are being included.

**Example:**
```bash
# Given .grove/rules:
# **/*.go
# !**/*_test.go

cx list
```

**Expected Output:**
```
/path/to/project/main.go
/path/to/project/pkg/server/server.go
```

## The `.grove/context` Output File

The output of the generation process is the `.grove/context` file. This file contains the concatenated contents of all files from the hot context, wrapped in a structured XML format.

**File Location**:
-   **Hot Context**: `.grove/context`
-   **Cold Context**: `.grove/cached-context`

**Format Specification**:

The generated context uses XML tags. Each file's content is enclosed in a `<file>` tag that includes a `path` attribute.

```xml
<?xml version="1.0" encoding="UTF-8"?>
<context>
  <hot-context files="2" description="...">
    <file path="src/main.go">
    // file content here
    </file>
    <file path="pkg/config.yaml">
    # yaml content here
    </file>
  </hot-context>
</context>
```

The cold context file (`.grove/cached-context`) follows the same structure but uses a `<cold-context>` root tag.

## Analyzing Composition with `cx stats`

The `cx stats` command provides a detailed analysis of both the hot and cold contexts. The output includes:

-   Total file count, estimated token count, and total size.
-   A breakdown of languages by file count and token percentage.
-   A list of the largest files by token count.
-   Token distribution statistics across all included files.

This command is used for managing context size and identifying which files contribute most to the token count.

## File Handling and Security

### Automatic Binary File Exclusion

By default, `cx` excludes binary files. The detection process is as follows:
1.  Checks for common binary file extensions (e.g., `.exe`, `.png`, `.so`, `.zip`).
2.  If the file has no extension, it inspects the file's first 512 bytes for binary signatures of common executable formats (ELF, Mach-O, PE).
3.  As a fallback, it analyzes the file content for a high percentage of non-printable characters or null bytes.

### Filesystem Access and Security Best Practices

**⚠️ WARNING: FILESYSTEM ACCESS**

The patterns in `.grove/rules` can match any readable file on your filesystem, including sensitive files outside your project directory if broad or parent-relative patterns (`../`) are used.

-   **ALWAYS** review the output of `cx list` before using the generated context, especially after modifying rules.
-   **NEVER** use overly broad, unqualified patterns like `/**/*` or `~/**`.
-   **AVOID** patterns that traverse excessively up the directory tree (e.g., `../../../../**`).
-   **EXPLICITLY EXCLUDE** sensitive files or directories (e.g., `!config/secrets.yml`, `!~/.ssh/**`).
-   **ADD** `.grove/` to your project's `.gitignore` file to prevent committing generated context files.

## Integration Points

`grove-context` is used by other tools in the Grove ecosystem:
-   **`grove-gemini` / `grove-openai`**: The `grove llm request` command uses `cx` to gather context. `grove-gemini` uses the hot/cold context separation to interact with Gemini's caching API.
-   **`grove-docgen`**: The documentation generator uses `cx` to build an understanding of a codebase before generating documentation.

## Complete Workflow Example

This example demonstrates the flow from rules to final output.

**1. Project Structure and Rules:**

-   `main.go`
-   `README.md`
-   `.grove/rules`:
    ```gitignore
    *.go
    *.md
    ```

**2. Verify with `cx list`:**

```bash
cx list
```

```
/path/to/project/main.go
/path/to/project/README.md
```

**3. Generate the context:**

```bash
cx generate
```

**4. Inspect the output (`.grove/context`):**

```xml
<?xml version="1.0" encoding="UTF-8"?>
<context>
  <hot-context files="2" description="...">
    <file path="main.go">
    package main
    // ...
    </file>
    <file path="README.md">
    # My Project
    // ...
    </file>
  </hot-context>
</context>
```