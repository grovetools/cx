# Grove Context (cx)

Grove Context is an LLM context management tool that helps prepare and maintain contextual information about your codebase for use with Large Language Models.

## Installation

Install via the Grove meta-CLI:
```bash
grove install context
```

Or install directly:
```bash
go install github.com/yourorg/grove-context@latest
```

## Usage

### Update Context

Scan and update the context for the current directory:
```bash
cx update
```

With options:
```bash
cx update --exclude="*.test.go" --include="pkg/**/*.go" --force
```

### Generate Context Output

Generate a formatted context file for LLM consumption:
```bash
cx generate                      # Creates context.md
cx generate my-context.json --format=json
cx generate --include-tests --max-depth=3
```

### Show Context Information

Display information about the current context:
```bash
cx show
cx show --detailed
cx show --filter="*.go"
```

## Features

- **Smart Scanning**: Automatically identifies important files and patterns
- **Flexible Filtering**: Include/exclude patterns for precise control
- **Multiple Formats**: Output in Markdown, JSON, or YAML
- **Incremental Updates**: Only rescans changed files for efficiency
- **Standard Flags**: Supports `--verbose`, `--json`, and `--config` via grove-core

## Context Storage

Context information is stored in `.grove/context/` within your project directory.

## Configuration

Configure defaults in `grove.yml`:
```yaml
context:
  exclude:
    - "vendor/**"
    - "*.generated.go"
  include:
    - "**/*.go"
    - "**/*.md"
  maxDepth: 5
```