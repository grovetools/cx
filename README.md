# Grove Context (cx)

Grove Context is a comprehensive context management tool for LLM interactions, allowing you to manage which files are included in your context and how they're formatted. This tool was migrated from the monolithic `grove cx` command to a standalone binary.

## Installation

Install via the Grove meta-CLI:
```bash
grove install context
```

Or install directly:
```bash
go install github.com/yourorg/grove-context@latest
```

The binary will be installed as `cx` in your PATH.

## Overview

The context system uses a top-level rules file and a `.grove` directory for generated content:
- `.grovectx` - Rules file with glob patterns for selecting files (top level for easy access)
- `.grove/context` - The generated context file containing all concatenated files
- `.grove/context-files` - The generated list of files to include in the context
- `.grove/context-snapshots/` - Saved context snapshots

## File Structure

### .grovectx
Contains glob patterns to automatically select files. Supports recursive patterns with `**` and exclusions with `!`:
```
# Include all Go files recursively
**/*.go

# But exclude test files
!*_test.go

# Include all markdown files recursively
**/*.md

# Include specific directories
internal/**/*.go
cmd/**/*.go

# Include configuration
go.mod
go.sum

# Exclude vendor directory
!vendor/**/*.go
```

#### Pattern Examples:
- `*.go` - Go files in root directory only
- `**/*.go` - All Go files recursively
- `internal/**/*.go` - All Go files under internal/
- `!*_test.go` - Exclude test files
- `!vendor/**/*` - Exclude vendor directory

### .grove/context-files
Contains the actual list of files to include (one per line):
```
main.go
internal/cli/context.go
internal/cli/agent.go
README.md
docs/guide.md
```

### .grove/context
The generated context file with all files concatenated using XML-style delimiters:
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

### cx info

Display information about the current context:
```bash
cx info
```

Output:
```
Files in context: 12
Approximate token count: 45.7k
Context file size: 178.5 KB
```

List absolute paths of all files in context:
```bash
cx info --list-files
```

### cx list

List absolute paths of all files in the context:
```bash
cx list
```

This is a shorthand for `cx info --list-files`.

### cx show

Print the entire context file (useful for piping):
```bash
cx show | pbcopy  # Copy to clipboard on macOS
cx show > context.txt  # Save to file
```

### cx update

Update `.grove/context-files` based on patterns in `.grovectx`:
```bash
cx update
```

This reads the patterns from `.grovectx` and creates/updates `.grove/context-files` with all matching files.

### cx generate

Generate the `.grove/context` file from the files listed in `.grove/context-files`:
```bash
cx generate
```

Options:
- `--xml` (default: true) - Use XML-style delimiters
- `--xml=false` - Use classic delimiter style

### cx save

Save the current file list as a snapshot with optional description:
```bash
cx save my-snapshot --desc "Minimal bug fix context"
```

This saves `.grove/context-files` to `.grove/context-snapshots/my-snapshot`.

### cx load

Load a previously saved snapshot:
```bash
cx load my-feature-context
```

This replaces `.grove/context-files` with the saved snapshot.

### cx diff

Compare the current context with a saved snapshot to see what has changed:
```bash
cx diff feature-context
```

Output shows:
- Added files with token counts
- Removed files with token counts
- Summary of changes (files, tokens, size)

Compare with empty context to see everything in current context:
```bash
cx diff
```

### cx list-snapshots

View all saved context snapshots with metadata:
```bash
cx list-snapshots
```

Output:
```
Available snapshots:

NAME                 DATE         FILES  TOKENS   SIZE      DESCRIPTION
--------------------------------------------------------------------------------
bug-fix-minimal      2025-07-18   15     45.2k    180.8 KB  Minimal context for bug fixes
feature-full         2025-07-17   45     156.3k   625.2 KB  Full feature development
code-review          2025-07-16   28     89.7k    358.8 KB  Code review context
```

Sort snapshots by different criteria:
```bash
cx list-snapshots --sort=size      # Sort by total size
cx list-snapshots --sort=tokens    # Sort by token count
cx list-snapshots --sort=name      # Sort alphabetically
cx list-snapshots --sort=files     # Sort by file count
cx list-snapshots --sort=date      # Sort by date (default)
cx list-snapshots --desc=false     # Ascending order
```

### cx validate

Check the integrity of all files in your context:
```bash
cx validate
```

This command:
- Verifies all files exist
- Checks file permissions
- Detects duplicate entries
- Reports any issues

Example output:
```
Validating context files...

✗ Missing files (2):
  - internal/deleted-file.go (remove from context)
  - docs/moved-file.md (remove from context)

⚠ Duplicates found (1):
  - internal/api/handler.go appears 2 times

✓ Accessible files: 40/42
✗ Issues found: 3

Run 'cx update' to regenerate from rules.
```

### cx fix

Automatically fix validation issues by removing missing files and duplicates:
```bash
cx fix
```

Output:
```
Fixed context file list:
  Removed 2 invalid/missing files
  Removed 1 duplicate entries
  38 valid files remain
```

### cx stats

Get detailed statistics about your context composition:
```bash
cx stats
```

Output:
```
Context Statistics:

Language Distribution:
  Go            78.2%  (123.5k tokens, 28 files)
  Markdown      15.3%  (24.1k tokens, 8 files)
  YAML           4.2%  (6.6k tokens, 3 files)
  Other          2.3%  (3.6k tokens, 2 files)

Largest Files (by tokens):
   1. internal/cli/agent.go                              12.3k tokens (7.8%)
   2. internal/compose/service.go                         8.7k tokens (5.5%)
   3. docs/architecture.md                                6.2k tokens (3.9%)
   4. internal/mcp/server.go                              5.8k tokens (3.7%)
   5. internal/config/config.go                           4.9k tokens (3.1%)

Token Distribution:
  < 1k tokens:      12 files (29.3%) █████
  1k-5k tokens:     22 files (53.7%) ██████████
  5k-10k tokens:     5 files (12.2%) ██
  > 10k tokens:      2 files (4.9%)  █

Average tokens per file: 3.8k
Median tokens per file: 2.9k
```

Options:
- `--detailed` - Show additional statistics (total files, tokens, size)
- `--top N` - Number of largest files to show (default: 5)

### cx from-git

Generate context based on git history:
```bash
cx from-git [options]
```

This command creates a context from files that have been modified in your git repository based on various criteria.

Options:
- `--since` - Include files changed since a date or commit
- `--branch` - Include files changed in a branch comparison
- `--staged` - Include only files in the staging area
- `--commits` - Include files from the last N commits

Examples:
```bash
# Files changed in the last week
cx from-git --since="1 week ago"

# Files changed since a specific commit
cx from-git --since=abc123

# Files changed in current branch compared to main
cx from-git --branch=main..HEAD

# Files in staging area (ready to commit)
cx from-git --staged

# Files from last 5 commits
cx from-git --commits=5

# Generate context after getting files from git
cx from-git --staged
cx generate
```

This is particularly useful for:
- Creating minimal contexts for code reviews
- Focusing on recently modified code
- Working with specific features or bug fixes
- Preparing contexts for commit messages or PR descriptions

## Workflow Examples

### Initial Setup

1. Create `.grovectx` with your patterns:
```bash
cat > .grovectx << EOF
# Include all Go files recursively
**/*.go

# Exclude test files
!*_test.go

# Include documentation
**/*.md

# Include configuration
go.mod
go.sum
EOF
```

2. Generate the file list:
```bash
cx update
```

3. Generate the context:
```bash
cx generate
```

### Manual File Management

You can manually edit `.grove/context-files` to add or remove specific files:
```bash
# Remove a file
grep -v "internal/secret.go" .grove/context-files > tmp && mv tmp .grove/context-files

# Add a file
echo "internal/important.go" >> .grove/context-files

# Regenerate context
cx generate
```

### Working with Snapshots

Save different contexts for different purposes:
```bash
# Save current context for bug fixing
cx save bug-fix-context

# Switch to feature development files
cx update  # Update from rules
cx save feature-dev-context

# Later, switch back to bug fix context
cx load bug-fix-context
cx generate
```

### Integration with LLMs

Copy context to clipboard:
```bash
cx show | pbcopy  # macOS
cx show | xclip -selection clipboard  # Linux
```

Check token count before sending to LLM:
```bash
cx info
```

## Best Practices

1. **Use Rules for Common Patterns**: Define your common file patterns in `.grovectx`
2. **Use Exclusions Wisely**: Exclude test files, generated code, and large assets with `!` patterns
3. **Manual Adjustments**: Use `.grove/context-files` for fine-tuning what's included
4. **Save Snapshots**: Save different contexts for different tasks
5. **Monitor Size**: Use `cx info` to keep track of token counts
6. **Version Control**: 
   - Add `.grove/context` to `.gitignore` (it's generated)
   - Commit `.grovectx` (your rules file)
   - Optionally commit `.grove/context-files` and `.grove/context-snapshots/`
   
Example `.gitignore`:
```
# Grove generated files
.grove/context

# Optionally ignore the generated file list
# .grove/context-files
```

## Migration from grove cx

This tool was migrated from the monolithic `grove cx` command. The functionality remains the same, but the commands are now available through the standalone `cx` binary. If you were previously using `grove cx <command>`, you can now use `cx <command>` instead.

## Standard Flags

All commands support standard Grove flags via grove-core:
- `--verbose` - Enable verbose output
- `--json` - Output in JSON format (where applicable)
- `--config` - Specify a custom config file

## License

See the main Grove project for licensing information.