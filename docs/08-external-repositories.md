# External Repositories

`grove-context` (`cx`) can include files from sources outside the current project's directory. This is managed through path patterns, Git URL references in the rules file, and a dedicated set of `repo` commands.

## 1. Including External Sources

There are three methods for including files from external sources in your context.

### Git Repository References

A Git repository URL can be added directly to the `.grove/rules` file. `cx` will clone or update the repository locally and apply patterns to its files. A specific version (tag, branch, or commit hash) can be pinned using the `@` symbol.

Repositories are cloned to a central cache directory (`~/.grove/cx/repos`).

**Example `.grove/rules`:**
```gitignore
# Include the entire lipgloss repository at a specific tag
https://github.com/charmbracelet/lipgloss@v0.13.0

# Exclude specific parts of the cloned repository
!**/examples/**
!**/*_test.go
```

### Local Path References

Relative or absolute filesystem paths can be used to include files from other local projects, such as in a monorepo workspace.

**Example `.grove/rules`:**
```gitignore
# Include all Go files from a sibling 'shared-lib' project
../shared-lib/**/*.go

# Include the API definition from a sibling 'api' project
../api/schema.openapi.yaml
```

### The `@default` Directive

The `@default` directive imports rules from another local Grove project. When `cx` encounters `@default: <path>`, it performs the following steps:

1.  Locates the `grove.yml` file in the specified `<path>`.
2.  Reads the `context.default_rules_path` value from that `grove.yml`.
3.  Loads the rules from that file and prepends `<path>` to all relative patterns to ensure they resolve from the external project's location.

**Example:**
Assume a `grove-core` project with its own default rules.

**`../grove-core/.grove/default.rules`:**
```gitignore
*.go
!*_test.go
```

**Your project's `.grove/rules`:**
```gitignore
# Include local files
src/**/*.js

# Include all default files from grove-core
@default: ../grove-core
```
This configuration includes all `.js` files from the local `src` directory and all non-test `.go` files from the `grove-core` project.

## 2. Managing External Repositories

The `cx repo` command suite manages the Git repositories that `cx` tracks and clones from rules files.

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

This command fetches the latest changes for all tracked repositories and checks out their pinned versions to ensure local copies are up-to-date.

```bash
cx repo sync
```

## 3. Repository Audit (`cx repo audit`)

Before including an unknown third-party repository, the `cx repo audit` command can be used to analyze its contents. It provides an interactive workflow assisted by an LLM.

The workflow is as follows:
1.  **Clone**: `cx` clones the repository to a temporary location.
2.  **Context Refinement**: The command launches `cx view`, allowing interactive selection of files to be included in the audit context.
3.  **LLM Analysis**: `cx` generates a context from the selection and sends it to an LLM with a prompt to analyze it for security vulnerabilities.
4.  **Review and Approve**: The analysis is saved to a report file and opened in your editor. You are then prompted to approve or reject the audit.
5.  **Manifest Update**: The result ("passed" or "failed") is recorded in a central manifest, which is visible in `cx repo list`.

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

-   Run `cx repo audit` on external repositories before adding them to rules to prevent including malicious or unexpectedly large codebases.
-   Pin Git repositories to a specific tag or commit hash (e.g., `url@v1.2.3`) for reproducible context.
-   When including an external repository, add specific exclusion patterns to limit the context to necessary files.
-   For shared context between local Grove projects, use the `@default` directive to avoid duplicating rule definitions.