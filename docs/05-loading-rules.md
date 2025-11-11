# Managing Context with Rule Sets

The `grove-context` tool (`cx`) provides mechanisms for managing different context configurations through named rule sets. This allows users to switch between different views of a codebase depending on the task.

## Reusable Rule Sets

The primary mechanism for managing context is through rule set files stored in dedicated directories within a project.

-   **`.cx/`**: This directory is for version-controlled, shared rule sets. Files in this directory are intended to be committed to Git and shared across a team.
-   **`.cx.work/`**: This directory is for local, temporary, or experimental rule sets. It is conventionally added to `.gitignore`.

This separation allows for different context configurations for different tasks. For example, a project might have rule sets for:

-   Backend development (`.cx/backend.rules`)
-   Frontend development (`.cx/frontend.rules`)
-   Documentation generation (`.cx/docs.rules`)
-   Analyzing a specific feature (`.cx.work/feature-x.rules`)

### Example: Creating Specialized Rule Sets

```bash
# Create a rule set for backend development
cat > .cx/backend-only.rules << 'EOF'
# Backend API code
src/api/**/*.go
src/services/**/*.go
src/models/**/*.go

# Exclude tests
!**/*_test.go
EOF

# Create a rule set for frontend development
cat > .cx/frontend-only.rules << 'EOF'
# Frontend UI code
src/ui/**/*.tsx
src/components/**/*.tsx
src/styles/**/*.css
EOF

# Create a rule set for documentation
cat > .cx/docs-only.rules << 'EOF'
docs/**/*.md
README.md
*.md
EOF
```

## Managing Active Rule Sets

The context used by `cx` commands is determined by the active rule set. This can be switched to fit the current task.

### Switching the Active Rule Set (`cx rules set`)

The `cx rules set <name>` command sets the active context to a named rule set from `.cx/` or `.cx.work/`. This creates a read-only link to the specified file.

```bash
# Switch between the rule sets created above
cx rules set backend-only
cx rules set frontend-only
cx rules set docs-only
```

### Creating a Modifiable Working Copy (`cx rules load`)

The `cx rules load <name>` command copies a named rule set to `.grove/rules`. This creates a local, modifiable working copy. Any changes made to `.grove/rules` will not affect the original named rule set. This is useful for starting from a template and customizing it for a specific task.

### Viewing Rule Sets (`cx rules list`)

The `cx rules list` command (or simply `cx rules`) displays all available rule sets in `.cx/` and `.cx.work/` and indicates which one is currently active.

## Integration with Other Tools

Other Grove tools can be configured to use specific rule sets for their operations, allowing for task-specific context.

### Grove Flow Integration

In `grove-flow`, a job can specify its own context by using the `rules_file` field in its frontmatter. This directs the job executor to use that specific rule set instead of the globally active one.

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

## Importing Rule Sets Across Projects

Rules can be imported from other projects using an alias syntax. This allows for the creation of shared, centralized rule definitions.

The syntax is `@a:project-alias::ruleset-name`, where `project-alias` is the identifier of another Grove workspace and `ruleset-name` is the name of the rule set file (without the `.rules` extension) in that project's `.cx/` directory.

### Example: Importing Rules

```bash
# In api-server/.grove/rules

# Import a set of standard backend patterns from a template project
@a:project-template::backend-patterns

# Include this project's specific patterns
src/**/*.go
!vendor/**

# Import a shared list of common library dependencies
@a:shared-libs::dependencies
```

## Resetting to Project Defaults (`cx reset`)

The `cx reset` command overwrites the local `.grove/rules` file with a project-defined default. This is useful for returning to a known-good configuration.

-   The default rule set is specified by the `context.default_rules_path` field in the project's `grove.yml`.
-   If no default is configured, `cx reset` creates a boilerplate file that includes all non-gitignored files.
-   The command asks for confirmation before overwriting an existing `.grove/rules` file.

### Example: Team Workflow with Shared and Local Rules

1.  A team defines standard rule sets (e.g., `backend`, `frontend`) in the `.cx/` directory, which is committed to the repository.
2.  Developers can switch between these standard contexts using `cx rules set backend`.
3.  An individual developer can create a temporary, experimental rule set in `.cx.work/my-feature.rules`, which is ignored by Git.
4.  Projects can import team-wide standards from a central repository (e.g., `@a:standards-repo::backend-best`) to ensure consistency.

---

## See Also

- [Rules & Patterns](03-rules-and-patterns.md) - Syntax for creating and importing rule sets
- [Context Generation](04-context-generation.md) - How rule sets generate context
- [Context TUI](06-context-tui.md) - Visual interface for switching and managing rule sets
- [Examples](02-examples.md) - Practical examples of rule set workflows
- [Command Reference](10-command-reference.md) - Complete `cx rules` command documentation