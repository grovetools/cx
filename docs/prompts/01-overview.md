# Documentation Task: Project Overview

You are an expert technical writer. Write a clear, engaging single-page overview for the `grove-context` (`cx`) tool.

## Task
Based on the provided codebase context, create a complete overview that includes the following sections in order:

1. **High-level description**: Explains that `cx` is a pattern-based tool for dynamically generating and managing file context for LLMs, with smart defaults that exclude binary files and include text-based source code
2. **Animated GIF placeholder**: Include `<!-- placeholder for animated gif -->`
3. **Key features**: Using a bulleted list including:
   - Pattern-based file selection with `.grove/rules` files
   - Automatic context generation with binary file exclusion
   - Quick rules editing with `cx edit` (bind to keyboard shortcut)
   - Context inspection with `cx list` and `cx stats`
   - Terminal UI for visual browsing (`cx view`)
   - Flexible rule loading (`cx set-rules`, `cx reset`, `cx save/load`)
   - Git integration for version-specific context (`cx from-git`)
   - External repository inclusion with audit (`cx repo`)
4. **Ecosystem Integration**: A dedicated H2 section explaining how grove-context fits into the Grove ecosystem as a foundational tool used by grove-gemini, grove-openai, and grove-docgen. Mention that grove-gemini is particularly useful for large contexts
5. **How it works**: Provide a more technical description and exactly what happens under the hood
6. **Installation**: A dedicated H2 section at the bottom with standardized installation instructions

## Installation Section Requirements
Include this condensed installation section at the bottom:

### Installation

Install via the Grove meta-CLI:
```bash
grove install context
```

Verify installation:
```bash
cx version
```

Requires the `grove` meta-CLI. See the [Grove Installation Guide](https://github.com/mattsolo1/grove-meta/blob/main/docs/02-installation.md) if you don't have it installed.

## Context Files to Read
- `README.md`
- `main.go`

## Output Format
Create a well-structured Markdown document that serves as a complete introduction to grove-context, combining description, ecosystem context, and installation in a single page.
