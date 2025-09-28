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
   - Supports absolte or relative paths to pull in adjacent repos or specific files/directories

2. **Pattern Syntax Reference**
   - Basic wildcards: * (any characters)
   - Directory recursion: ** (any depth)
   - File extension patterns: *.go
   - Directory patterns: src/, /absolute/path

3. **Include/Exclude Logic**
   - Default behavior (include matching patterns)
   - Exclusion with ! prefix
   - Order matters: last matching pattern wins
   - Combining positive and negative patterns
   - Things in .gitignore are excluded by default

4. **Pattern Writing Strategies**
   - Start broad, then exclude specifics
   - Common exclusion patterns (node_modules, .git, build/)
   - Documentation inclusion patterns
   - Using relative paths for sibling directories (../other-project/**/*.go)
   - Organizing multi-repo contexts with path patterns

5. **Editing Rules with `cx edit`**
   - Quickly open rules file in your default editor
   - **Recommended**: Bind to a keyboard shortcut for editing

## Examples Required
Provide complete rules files for:
- Go project (exclude vendor, include .mod files)
- JavaScript/TypeScript project (exclude node_modules, dist)
- Mixed-language monorepo
- Multi-repo workspace using relative paths (../api/**, ../frontend/**)
- Rules file with GitHub repo references (note: just show the syntax, details in External Repositories)

## Best Practices
- Keep patterns simple and readable
- Comment complex patterns
- Group related patterns together
- Version control your rules files
