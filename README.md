# Grove Context (cx)
<img src="https://github.com/user-attachments/assets/f0f527de-ac59-41bf-b25f-a8bc138d7f1b" height=500 />

`grove-context` (`cx`) is a, rule-based tool for dynamically managing the file-based context provided to Large Language Models (LLMs).

It replaces manual copy-pasting with a repeatable, version-controlled workflow, ensuring your LLM has the precise information it needs for any task.

## Features
- **Dynamic Context Generation:** Define your context once in a `.grove/rules` file and generate it on demand.
- **Hot & Cold Contexts:** Separate frequently changing files ("hot") from stable dependencies ("cold") to optimize context size and relevance.
- **Interactive Tools:** Visualize your context with `cx view` and monitor it in real-time with `cx dashboard`.
- **Git Integration:** Automatically generate context from recent changes, branches, or staged files.
- **Snapshots:** Save and load different context configurations for different tasks (e.g., feature work, bug fixes).
- **External Directory Support:** Easily include files from other repositories or directories.

## How It Works

The core of `grove-context` is the `.grove/rules` file. This plain text file uses `.gitignore`-style patterns to define which files to include or exclude. Every `cx` command resolves files dynamically from these rules—there is no intermediate state.

A `---` separator divides the rules into a "hot" context (above) and a "cold" context (below).

- **Hot Context:** Files you are actively editing.
- **Cold Context:** Stable files, libraries, or dependencies that provide background information.

This separation allows you to send only the hot context in follow-up prompts, while the cold context can be sent once or managed by more advanced agents.

## Installation
Install via the Grove meta-CLI:
```bash
grove install context
```

## File Structure

### .grove/rules
Contains glob patterns to automatically select files. Supports recursive patterns with `**` and exclusions with `!`:
```
# Include all Go files recursively
src/**/*.go

# Exclude vendor directory
!vendor/**/*

# Include the project README
README.md

---

# Cold context: Stable dependencies from an external project
../grove-core/**/*.go

# Exclude tests from the external project
!../grove-core/**/*_test.go
```

### .grove/context & .grove/cached-context
The generated context files for hot and cold contexts, respectively. Files are concatenated using XML-style delimiters:
```xml
<file path="main.go">
package main

func main() {
    // code...
}
</file>

<file path="internal/cli/context.go">
package cli
// code...
</file>
```

## Commands

The `cx` binary provides a suite of commands for managing your context. Here are some of the most important ones. For a full list, run `cx --help`.

### Interactive Tools

- `cx view`: Launch an interactive tree view to see exactly which files are included, excluded, or ignored by your rules.
- `cx dashboard`: Display a live-updating terminal dashboard showing statistics for your hot and cold contexts.

### Core Workflow

- `cx edit`: Open `.grove/rules` in your default editor.
- `cx generate`: Build the `.grove/context` and `.grove/cached-context` files from your rules.
- `cx show`: Print the generated hot context to the console, ready to be piped to your clipboard or an LLM.

### Analysis & Inspection

- `cx stats`: Get a detailed breakdown of your context, including token counts, language distribution, and largest files.
- `cx diff [snapshot]`: Compare your current context to a saved snapshot or an empty context.
- `cx list`: List all files included in the hot context.
- `cx list-cache`: List all files included in the cold context.
- `cx validate`: Check for missing files or other issues in your resolved context.

### Snapshots

- `cx save <name>`: Save the current `.grove/rules` as a named snapshot.
- `cx load <name>`: Restore a snapshot to `.grove/rules`.
- `cx list-snapshots`: List all available snapshots.

### Git Integration

- `cx from-git`: Automatically generate rules based on git history (e.g., `--staged`, `--branch=main`, `--since="1 day ago"`).

## Example Workflow

1.  **Define your context:**
    ```bash
    # Open the rules file and add your patterns
    cx edit
    ```
2.  **Visualize and refine:**
    ```bash
    # Interactively see what's included and make adjustments
    cx view
    ```
3.  **Generate the context files:**
    ```bash
    cx generate
    ```
4.  **Copy to clipboard and send to your LLM:**
```bash
cx show | pbcopy  # macOS
```

## Best Practices

1. **Use Rules for Common Patterns**: Define your common file patterns in `.grove/rules`
2. **Use Exclusions Wisely**: Exclude test files, generated code, and large assets with `!` patterns
3. **Watch Large Files**: Use `cx stats` to identify files with high token counts (shown in red/yellow)
4. **Save Snapshots**: Save different rule sets for different tasks

## Version Control

It is recommended to add the `.grove` directory to your `.gitignore` file, as it contains locally generated files. You may choose to check in specific snapshots from `.grove/context-snapshots/` if they represent important, shared contexts.

```
# Grove directory (contains rules and generated files)
.grove/
```

## License

See the main Grove project for licensing information.
