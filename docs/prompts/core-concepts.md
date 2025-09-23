# Documentation Task: Core Concepts

Explain the fundamental concepts of `grove-context` (`cx`). This is the most important section for understanding *how* the tool works.

## Task
Create a detailed explanation for each of the following concepts in its own subsection:

### The `.grove/rules` File
- Explain that this is the single source of truth for all context generation.
- Describe its location and purpose.

### Hot and Cold Contexts
- Explain the `---` separator.
- Define "Hot Context" (active, frequently changing files) and "Cold Context" (stable dependencies, libraries).
- Explain how this separation relates to the generated `.grove/context` and `.grove/cached-context` files.

### Rule Syntax and Precedence
- Describe the `.gitignore`-style syntax.
- Provide examples for basic patterns (`*.go`), recursive globs (`**/*.go`), and exclusions (`!vendor/**/*`).
- Explain the precedence rules: cold context patterns override hot context patterns for the same file.

## Context Files to Read
- `README.md`
- `pkg/context/manager.go`