# External Repositories

`grove-context` can include files from sources outside the current project's directory, enabling context generation for monorepos, shared libraries, and third-party dependencies. This is managed through path patterns, Git URL references, and a dedicated set of `repo` commands.

## 1. Including External Repositories

There are three primary methods for including external files in your context.

### Git Repository References

You can add a Git repository URL directly to your `.grove/rules` file. `cx` will automatically clone or update the repository locally and include its files based on your patterns. This is the recommended method for third-party dependencies.

You can pin the repository to a specific version (tag, branch, or commit hash) using the `@` symbol.

**Example `.grove/rules`:**
```gitignore
# Include the entire lipgloss repository at a specific tag
https://github.com/charmbracelet/lipgloss@v0.13.0

# Exclude specific parts of the cloned repository
!**/examples/**
!**/*_test.go
```

When `cx` processes these rules, it clones the repository to a central cache directory (`~/.grove/cx/repos`) and treats its local path as the base for the provided patterns.

### Local Path References

For monorepos or workspaces where projects are located in sibling directories, you can use relative paths to include files from other local projects.

**Example `.grove/rules`:**
```gitignore
# Include all Go files from a sibling 'shared-lib' project
../shared-lib/**/*.go

# Include the API definition from a sibling 'api' project
../api/schema.openapi.yaml
```

Absolute paths are also supported for including files from any location on your filesystem.

### The `@default` Directive

The `@default` directive provides a structured way to import rules from another local Grove project. It's particularly useful for building composite contexts in a monorepo.

When `cx` encounters `@default: <path>`, it:
1.  Looks for a `grove.yml` file in the specified `<path>`.
2.  Reads the `context.default_rules_path` value from that `grove.yml`.
3.  Loads and processes the rules from that file, prepending the `<path>` to all relative patterns to ensure they resolve correctly from the external project's location.

**Example:**
Assume you have a `grove-core` project with its own default rules.

**`../grove-core/.grove/default.rules`:**
```gitignore
*.go
!*_test.go
```

**Your project's `.grove/rules`:**
```gitignore
# Include my local files
src/**/*.js

# Also include all default files from grove-core
@default: ../grove-core
```
This will include all `.js` files from your local `src` directory and all non-test `.go` files from the `grove-core` project.

## 2. Managing External Repositories

The `cx repo` command suite helps you manage the Git repositories that `cx` tracks and clones based on your rules files.

### Listing Tracked Repositories (`cx repo list`)

This command displays a table of all Git repositories that have been cloned by `cx`, including their pinned version, resolved commit, and audit status.

```bash
cx repo list
```

**Example Output:**
```
URL                                     VERSION  COMMIT   STATUS       REPORT  LAST SYNCED
---                                     -------  ------   ------       ------  -----------
https://github.com/charmbracelet/lipgloss v0.13.0  a1a2b3c  passed       ✓       2 hours ago
https://github.com/user/another-repo    default  b4c5d6e  not_audited          1 day ago
```

### Syncing Repositories (`cx repo sync`)

This command fetches the latest changes for all tracked repositories and checks out their pinned versions. This is useful for ensuring your local copies are up-to-date.

```bash
cx repo sync
```

## 3. Repository Audit (`cx repo audit`)

Before including an unknown third-party repository, it is best practice to perform an audit to understand its contents, size, and potential security risks. The `cx repo audit` command provides an interactive, LLM-assisted workflow for this purpose.

The workflow is as follows:
1.  **Clone**: `cx` clones the repository to a temporary location.
2.  **Context Refinement**: It launches `cx view`, allowing you to interactively select which files from the repository should be included in the audit context.
3.  **LLM Analysis**: `cx` generates a context from your selection and sends it to an LLM with a prompt to analyze it for security vulnerabilities or prompt injection risks.
4.  **Review and Approve**: The LLM's analysis is saved to a report file and opened in your editor. You are then prompted to approve or reject the audit.
5.  **Manifest Update**: The result ("passed" or "failed") is recorded in the central manifest, visible in `cx repo list`.

**Example Audit Workflow:**
```bash
# 1. Start the interactive audit for a repository
cx repo audit https://github.com/charmbracelet/lipgloss

# --> This launches `cx view` to select files for the audit.
# --> After quitting `cx view`, it runs the LLM analysis.
# --> After the report is generated and reviewed:

# 2. Approve the audit
Approve this audit and mark repository as 'passed'? (y/n): y

# 3. Add the audited repository to your rules file
echo "https://github.com/charmbracelet/lipgloss" >> .grove/rules
```

## 4. Best Practices

-   **Always Audit First**: Run `cx repo audit` on external repositories before adding them to your rules to prevent including malicious or unexpectedly large codebases.
-   **Pin Versions**: For reproducibility, always pin Git repositories to a specific tag or commit hash (e.g., `url@v1.2.3`).
-   **Use Specific Patterns**: When including external repos, add specific exclusion patterns to limit the context to only what you need, reducing token count and noise.
-   **Prefer `@default` for Monorepos**: Use the `@default` directive for managing shared context between local Grove projects to keep configurations DRY (Don't Repeat Yourself).