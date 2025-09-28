# Context Generation Pipeline

Grove Context (`cx`) transforms a set of rules into a structured, file-based context that can be provided to Large Language Models (LLMs). This document explains the pipeline, from defining rules to generating the final output, and the commands used to inspect and manage it.

## Generation Pipeline Overview

The core function of `cx` is to execute a clear, repeatable pipeline:

1.  **Rules Definition**: You define which files to include and exclude in a `.grove/rules` file using a `.gitignore`-style syntax.
2.  **File Resolution**: `cx` reads the rules, walks the specified file paths (including the current project and any external directories), and resolves a final list of files. It automatically filters out binary files and respects `.gitignore` patterns.
3.  **Context Generation**: The content of the resolved files is concatenated into a single, structured XML file located at `.grove/context`.

This process is typically triggered in two ways:
-   **Manually**: By running `cx generate`, which generates both the hot (`.grove/context`) and cold (`.grove/cached-context`) files.
-   **Automatically**: Higher-level tools in the Grove ecosystem, such as `grove-gemini`, automatically invoke the generation pipeline before making a request to an LLM, ensuring the context is always up-to-date.

## Inspecting the File List with `cx list`

Before generating the full context, you can verify which files will be included in the **hot context** using `cx list`. This command resolves the patterns in your rules file and prints a simple list of the resulting absolute file paths. It is an essential tool for debugging your rules to ensure you are including exactly what you intend.

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

The final output of the generation process is the `.grove/context` file. This file contains the concatenated contents of all files from the hot context, wrapped in a structured XML format that is easily parsable by LLMs and other tools.

**File Location**:
-   **Hot Context**: `.grove/context`
-   **Cold Context**: `.grove/cached-context`

**Format Specification**:

The generated context is wrapped in XML tags, with each file's content enclosed in a `<file>` tag that includes a `path` attribute.

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

To understand the composition of your context without reading the entire file, use the `cx stats` command. It provides a detailed analysis of both the hot and cold contexts, including:

-   Total file count, estimated token count, and total size.
-   A breakdown of languages by file count and token percentage.
-   A list of the largest files, which helps identify major token consumers.
-   Token distribution statistics across all included files.

This command is invaluable for managing context size and ensuring you are not unintentionally including overly large files.

## File Handling and Security

`cx` includes smart defaults and requires careful usage to maintain security.

### Automatic Binary File Exclusion

By default, `cx` attempts to exclude binary files to keep the context focused on text-based source code. The detection process works as follows:
1.  Checks for common binary file extensions (e.g., `.exe`, `.png`, `.so`, `.zip`).
2.  If the file has no extension, it inspects the file's first 512 bytes for binary signatures (magic bytes) of common executable formats (ELF, Mach-O, PE).
3.  As a fallback, it analyzes the file content for a high percentage of non-printable characters or null bytes.

### Filesystem Access and Security Best Practices

**⚠️ WARNING: FILESYSTEM ACCESS**

The patterns in `.grove/rules` can match **any readable file on your filesystem**, including sensitive files outside your project directory if broad or parent-relative patterns (`../`) are used. Always exercise caution.

-   **ALWAYS** review the output of `cx list` before using the generated context, especially after modifying rules. This is your primary safeguard against unintended file inclusion.
-   **NEVER** use overly broad, unqualified patterns like `/**/*` or `~/**`.
-   **AVOID** patterns that traverse excessively up the directory tree (e.g., `../../../../**`). `cx` has built-in safety checks to block the most dangerous patterns, but careful rule writing is essential.
-   **EXPLICITLY EXCLUDE** sensitive files or directories (e.g., `!config/secrets.yml`, `!~/.ssh/**`).
-   **ADD** `.grove/` to your project's `.gitignore` file to prevent committing large, generated context files.

## Integration Points

`grove-context` serves as the foundational context provider for other tools in the Grove ecosystem:
-   **`grove-gemini` / `grove-openai`**: The `grove llm request` command automatically uses `cx` to gather context. `grove-gemini` specifically leverages the hot/cold context separation to optimize token usage with Gemini's caching API.
-   **`grove-docgen`**: The documentation generator uses `cx` to build a comprehensive understanding of a codebase before generating documentation.

## Complete Workflow Example

This example demonstrates the end-to-end flow from rules to final output.

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