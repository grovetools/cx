# Loading Rules Documentation

You are documenting various ways to load and manage rules in grove-context, with emphasis on reusable rule sets and context switching.

## Task
Create documentation covering all methods of loading and managing rules configurations, emphasizing the power of creating specialized rule sets for different workflows.

## Topics to Cover

1. **Reusable Rule Sets - The Core Workflow**
   - Creating named rule sets in `.cx/` directory (e.g., `.cx/backend-only.rules`, `.cx/docs-only.rules`)
   - Using `.cx.work/` for local/personal rule sets (gitignored by convention)
   - **Why this matters**: Different tasks need different context
     - Backend-only context for API work
     - Frontend-only for UI development
     - Docs-only for documentation generation
     - Dependencies-only for understanding third-party code
     - Full-stack for architectural work
   - Organizing rule sets by feature area, layer, or concern

2. **Switching Rule Sets (`cx set-rules`)**
   - Loading rules from `.cx/` with `cx set-rules backend-only`
   - Loading from absolute paths with `cx set-rules /path/to/custom.rules`
   - Quick context switching for different tasks
   - Integration with grove-flow: specify `rules_file:` in job frontmatter
   - Integration with grove-docgen: specify `rules_file:` in docgen config
   - Viewing active rules with `cx rules` or checking state

3. **Importing Rule Sets Across Projects**
   - **Powerful feature**: Reference rule sets from other projects
   - Syntax: `@a:project-name::ruleset-name` in your `.grove/rules`
   - Use cases:
     - Pull in shared context definitions across a team
     - Reference common patterns from a template project
     - Include frequently-used external repos defined once
   - Example: `@a:api-server::backend-only` imports that project's backend rules
   - Combining imports with local patterns

4. **The `cx reset` Command**
   - Resetting to project defaults with `cx reset`
   - Default rules discovery determined by grove.yml field
   - When to use reset (starting fresh, recovering from bad config)
   - Interactive confirmation to prevent accidental resets

5. **Snapshots for Rule Management**
   - Saving current configuration with `cx snapshot save`
   - Loading saved configurations with `cx snapshot load`
   - Listing available snapshots with `cx snapshot list`
   - Snapshot storage location and format
   - Sharing snapshots across teams

## Examples Required

**Example 1: Creating Specialized Rule Sets**
```bash
# Create backend-only context
cat > .cx/backend-only.rules << 'EOF'
# Backend API code
src/api/**/*.go
src/services/**/*.go
src/models/**/*.go

# Exclude tests
!**/*_test.go
EOF

# Create frontend-only context
cat > .cx/frontend-only.rules << 'EOF'
# Frontend UI code
src/ui/**/*.tsx
src/components/**/*.tsx
src/styles/**/*.css
EOF

# Create docs context
cat > .cx/docs-only.rules << 'EOF'
docs/**/*.md
README.md
*.md
EOF

# Switch between them as needed
cx set-rules backend-only
cx set-rules frontend-only
cx set-rules docs-only
```

**Example 2: Importing Rule Sets from Other Projects**
```bash
# In api-server/.grove/rules
# Pull in shared backend patterns from template project
@a:project-template::backend-patterns

# Plus our own specific patterns
src/**/*.go
!vendor/**

# Pull in commonly-used external dependencies
@a:shared-libs::dependencies
```

**Example 3: Grove-flow Integration with Custom Rules**
```markdown
---
id: job-backend-refactor
title: Refactor authentication
type: oneshot
rules_file: .cx/auth-only.rules
worktree: auth-refactor
---

Refactor the authentication module...
```

**Example 4: Team Workflow with Shared Rule Sets**
- Team creates shared rule sets in `.cx/` (committed to git)
- Individual developers create personal variants in `.cx.work/` (gitignored)
- Projects import team-standard rules: `@a:standards-repo::backend-best`
- Everyone gets consistent context definitions

**Example 5: Snapshots for Complex Contexts**
- Using `cx reset` to recover from bad configuration
- Setting project-specific rules with `cx set-rules`
- Creating and loading snapshots for different workflows
- Template selection for new projects

