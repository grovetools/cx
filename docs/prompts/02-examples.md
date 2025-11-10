# Examples Documentation for Grove Context

You are documenting Grove Context (cx), a rule-based tool for managing file context for LLMs.

## Task
Create five practical examples that demonstrate Grove Context's capabilities, from basic usage through ecosystem integration, emphasizing reusable rule sets and context switching.

## Required Examples

### Example 1: Quick Start - Pattern-Based Context
- Setting up a `.grove/rules` file with basic patterns
- Using `cx edit` with keyboard shortcut for rapid iteration
- Running `cx list` to verify included files
- Checking statistics with `cx stats`
- Using `cx view` TUI for visual exploration (TREE, RULES, STATS, LIST tabs)
- Excluding binary files automatically
- Viewing real-time stats in Neovim with virtual text (brief mention)

### Example 2: Reusable Rule Sets for Context Switching
- Creating specialized rule sets in `.cx/` directory
- **Backend-only context**: `cx set-rules backend-only` for API work
- **Frontend-only context**: `cx set-rules frontend-only` for UI work
- **Docs context**: `cx set-rules docs-only` for documentation
- Importing rule sets from other projects: `@a:api-server::backend-only`
- Team workflow: shared rule sets in `.cx/`, personal in `.cx.work/`
- Viewing active rules and switching between contexts

### Example 3: Working with Aliases and Workspaces
- Using `cx workspace list` to see available projects
- Creating rules with aliases: `@a:grove-nvim`, `@a:grove-ecosystem:grove-core`
- Context-aware resolution (siblings in same ecosystem)
- Using `<leader>f?` in Neovim to preview alias matches
- Combining aliases with patterns: `@a:my-project/src/**/*.go`

### Example 4: Grove-Flow Integration
- Setting up context for a flow plan worktree
- Custom rules files per job (`rules_file: .cx/backend-only.rules`)
- How grove-flow regenerates context before each job
- Interactive context creation when rules are missing

### Example 5: Managing Complex Projects
- Loading different rule sets with `cx set-rules`
- Using `cx view` TUI to visually browse and modify context:
  - TREE tab: See directory hierarchy with token counts
  - STATS tab: Identify what's consuming context space
  - RULES tab: View active rules and press `e` to edit
  - LIST tab: Exclude specific files with `x`
  - Press `s` to switch rule sets interactively
- Including local repos with relative paths (../api/**, ../shared-lib/**)
- Git integration with `cx from-git` for specific commits
- Including external repositories with `cx repo audit` first
- Resetting to defaults with `cx reset`

## Output Format
- Each example should have clear headings (e.g., "Example 1: Basic Context Generation")
- Include both the commands and the context for why you'd use them
- Show expected outcomes and results
- Provide commentary on when to use each pattern
- Include practical, real-world scenarios that developers would actually encounter
