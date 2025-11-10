# Rules & Patterns Documentation

You are documenting the rules file syntax and pattern matching system in grove-context.

## Task
Create comprehensive documentation focused on writing and understanding rules files and pattern syntax.

## Topics to Cover

1. **Rules File Basics**
   - Location: .grove/rules
   - Plain text format, one pattern per line
   - Comments with # prefix
   - Relationship to .gitignore syntax
   - Can include directives (GitHub URLs, @default) - see External Repositories section
   - Supports absolute or relative paths to pull in adjacent repos or specific files/directories
   - Supports workspace aliases for cross-project references

2. **Pattern Syntax Reference**
   - Basic wildcards: * (any characters)
   - Directory recursion: ** (any depth)
   - File extension patterns: *.go
   - Directory patterns: src/, /absolute/path
   - Workspace aliases: @a:project-name or @alias:project-name
   - Aliased patterns: @a:grove-nvim/lua/**/*.lua
   - Ecosystem references: @a:ecosystem:subproject
   - Worktree references: @a:project:branch-name

3. **Alias Resolution System**
   - How grove-context discovers workspaces using grove-core
   - Alias syntax: @a:name (short) or @alias:name (long)
   - Context-aware resolution: siblings prioritized in same ecosystem
   - Using `cx workspace list` to see available aliases
   - Combining aliases with glob patterns
   - Resolving to absolute paths during generation

4. **Include/Exclude Logic**
   - Default behavior (include matching patterns)
   - Exclusion with ! prefix
   - Order matters: last matching pattern wins
   - Combining positive and negative patterns
   - Things in .gitignore are excluded by default

5. **Pattern Writing Strategies**
   - Start broad, then exclude specifics
   - Common exclusion patterns (node_modules, .git, build/)
   - Documentation inclusion patterns
   - Using relative paths for sibling directories (../other-project/**/*.go)
   - Using aliases for cross-project references (@a:project-name/**/*.go)
   - Organizing multi-repo contexts with path patterns
   - Mixing aliases, relative paths, and absolute paths

6. **Reusable Rule Sets**
   - Creating named rule sets in `.cx/` directory
   - Switching between rule sets with `cx set-rules <name>`
   - **Why create multiple rule sets**:
     - Backend-only, frontend-only, docs-only contexts
     - Feature-specific contexts (auth, billing, admin)
     - Layer-specific contexts (API, UI, database)
     - Task-specific contexts (debugging, refactoring, documentation)
   - Using `.cx.work/` for personal/local rule sets (gitignored)
   - Importing rule sets from other projects: `@a:project::ruleset-name`
   - Sharing rule sets across teams via git

7. **Editing Rules with `cx edit`**
   - Quickly open rules file in your default editor
   - **Recommended**: Bind to a keyboard shortcut for editing
   - Real-time feedback in Neovim with virtual text

## Examples Required

Provide complete rules files for:
- Go project (exclude vendor, include .mod files)
- JavaScript/TypeScript project (exclude node_modules, dist)
- Mixed-language monorepo
- Multi-repo workspace using relative paths (../api/**, ../frontend/**)
- Multi-repo workspace using aliases (@a:grove-core/**/*.go, @a:grove-nvim/lua/**/*.lua)
- **Specialized rule sets**:
  - `.cx/backend-only.rules` - Backend API context
  - `.cx/frontend-only.rules` - Frontend UI context
  - `.cx/docs-only.rules` - Documentation context
  - `.cx/auth-module.rules` - Feature-specific context
- **Importing rule sets**: Rules file that imports from other projects using `@a:project::ruleset`
- Rules file with GitHub repo references (note: just show the syntax, details in External Repositories)

## Best Practices
- Keep patterns simple and readable
- Comment complex patterns
- Group related patterns together
- Version control your rules files
- **Create specialized rule sets for different workflows**:
  - Put shared/team rule sets in `.cx/` (committed to git)
  - Put personal rule sets in `.cx.work/` (gitignored)
  - Use descriptive names: `backend-only`, `frontend-only`, `docs-only`
- **Import rule sets across projects to maintain consistency**:
  - Create a "standards" project with common rule sets
  - Import with `@a:standards::backend-patterns`
  - Team gets consistent context definitions
- **Leverage rule set switching for task-specific context**:
  - Switch to backend-only when working on APIs
  - Switch to docs-only when writing documentation
  - Use feature-specific rules for focused work (auth, billing, etc.)
