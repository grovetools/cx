# External Repositories Documentation

You are documenting how to include context from external repositories and projects.

## Task
Create documentation covering external repository management, including the audit functionality.

## Topics to Cover

1. **Including External Repositories**
   - GitHub repository references in rules files
   - Using `cx repo add <url>` to add repositories
   - Local path references (../other-project)
   - Managing multiple external sources

2. **The @default Directive**
   - Importing rules from other local Grove projects
   - Path resolution with @default
   - Combining external rules with local patterns

3. **Managing External Context**
   - `cx repo list` to view added repositories
   - `cx repo remove` for cleanup
   - Updating external repositories
   - Explain where the external repos are cloned

4. **Cross-Project Workflows**
   - Monorepo patterns
   - Shared library inclusion
   - Dependency context management
   - Avoiding circular references

5. **Repository Audit (`cx repo audit`)**
   - Quick analysis before adding repositories
   - Understanding size and token estimates
   - Language breakdown and file counts
   - Making inclusion decisions based on audit

## Examples Required
- Adding a GitHub repository with audit first
- Using @default to inherit repo rules
- Setting up monorepo with relative paths
- Managing large external dependencies

## Best Practices
- Always audit before adding large/unknown repos
- Use specific paths to limit external context
