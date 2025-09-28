# Examples Documentation for Grove Context

You are documenting Grove Context (cx), a rule-based tool for managing file context for LLMs.

## Task
Create two practical examples that demonstrate Grove Context's capabilities with increasing complexity.

## Required Examples

### Example 1: Quick Start - Pattern-Based Context
- Setting up a `.grove/rules` file with basic patterns
- Using `cx edit` with keyboard shortcut for rapid iteration
- Running `cx list` to verify included files
- Checking statistics with `cx stats`
- Excluding binary files automatically

### Example 2: Managing Complex Projects
- Loading different rule sets with `cx set-rules`
- Using `cx view` TUI to visually browse and modify context
- Including local repos with relative paths (../api/**, ../shared-lib/**)
- Managing repos in `cx view` repos page (Tab to switch views)
- Git integration with `cx from-git` for specific commits
- Including external repositories with `cx repo audit` first
- Resetting to defaults with `cx reset`

## Output Format
- Each example should have clear headings (e.g., "Example 1: Basic Context Generation")
- Include both the commands and the context for why you'd use them
- Show expected outcomes and results
- Provide commentary on when to use each pattern
- Include practical, real-world scenarios that developers would actually encounter
