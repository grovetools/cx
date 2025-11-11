This document describes how `grove-context` (`cx`) manages context from external Git repositories and other local projects.

### Including External Sources

`grove-context` can include files from sources outside the current project's working directory. This is useful for monorepos, shared libraries, or including third-party code for analysis. There are two primary methods for including external sources: direct path references and managed Git repositories.

#### 1. Local Path References

You can include files from other local projects using relative paths in your `.grove/rules` file. This is common in monorepo setups where projects are in sibling directories.

**Example:**
```
# in my-app/.grove/rules
../shared-ui/src/**/*.tsx
```

All paths are sandboxed. By default, `cx` will only resolve patterns within discovered Grove workspaces. You can add specific directories to this sandbox using the `context.allowed_paths` key in your `grove.yml`.

#### 2. Managed Git Repositories

You can reference remote Git repositories directly in your rules file. `cx` will automatically clone and manage these repositories in a centralized location (`~/.grove/repos/`).

**Example:**
```
# Include all files from the main branch
https://github.com/mattsolo1/grove-core

# Pin to a specific tag or branch
https://github.com/mattsolo1/grove-core@v0.4.0

# Include only a specific subdirectory
https://github.com/mattsolo1/grove-core@v0.4.0/pkg/workspace/**
```

When a rule containing a Git URL is processed, `cx` performs the following actions:
1.  Parses the URL to identify the repository and an optional version (tag, branch, or commit hash).
2.  Adds the repository to a central manifest located at `~/.grove/repos.json`.
3.  Clones the repository into `~/.grove/repos/<domain>/<owner>/<repo>` if it doesn't already exist.
4.  Fetches updates and checks out the specified version. If no version is specified, it uses the repository's default branch.
5.  Replaces the URL in the rule with the local path to the cloned repository and resolves the file patterns.

### Cross-Project References with Aliases

For more complex monorepos or ecosystems, `grove-context` uses an alias system for robust, location-independent references between projects.

#### The `@default` Directive

The `@default` directive imports the default rule set from another Grove project. The target project must have a `context.default_rules_path` defined in its `grove.yml`.

**Example:**
Suppose you have a `shared-lib` project with default rules defined in its `grove.yml`:

```yaml
# in ../shared-lib/grove.yml
context:
  default_rules_path: .cx/api.rules
```

You can import these rules into `my-app` as follows:

```
# in my-app/.grove/rules
# Import default rules from the sibling project
@default: ../shared-lib

# Local rules are also applied
src/main.go
```

#### The `@alias` (`@a:`) Directive

The `@alias` directive (short form `@a:`) provides a way to reference any discovered Grove workspace using a unique identifier. This is the preferred method for cross-project references as it does not rely on fragile relative paths.

The alias format is based on the workspace's identifier, which often includes its parent ecosystem.

**Alias Formats:**
-   `@a:<project-name>`: References a standalone project or the most relevant project with that name.
-   `@a:<ecosystem-name>:<project-name>`: References a project within an ecosystem.
-   `@a:<project-name>:<worktree-name>`: References a specific worktree of a project.
-   `@a:<project-name>::<ruleset-name>`: Imports a named rule set from another project.

**Example:**
```
# Include the workspace package from the grove-core project
# within the grove-ecosystem.
@a:grove-ecosystem:grove-core/pkg/workspace/**

# Import the 'api' ruleset from the 'shared-lib' project.
@a:shared-lib::api
```

### Managing Repositories with `cx repo`

The `cx repo` command suite provides tools to inspect and manage the repositories that `cx` clones.

-   **`cx repo list`**: Displays all repositories tracked in the manifest, including their source URL, pinned version, resolved commit, last sync time, and audit status.

-   **`cx repo sync`**: Fetches the latest updates for all tracked repositories and ensures they are checked out to their pinned versions. This is useful for keeping local copies up-to-date.

### Repository Security Audits

Before including an unfamiliar external repository, it is recommended to perform a security audit. The `cx repo audit` command facilitates an LLM-based analysis to identify potential risks like prompt injection.

**Workflow for `cx repo audit <url>`:**

1.  **Clone**: The repository is cloned to the local cache.
2.  **Context Refinement**: An interactive TUI (`cx view`) is launched, allowing you to review the repository's files and define the context for the audit by including or excluding files and directories.
3.  **LLM Analysis**: Once you exit the TUI, `cx` generates the context and sends it to an LLM with a prompt to analyze the code for security vulnerabilities and prompt injection vectors.
4.  **Review Report**: The LLM's analysis is saved to a markdown file in the cloned repo's `.grove/audits/` directory and opened in your default editor (`$EDITOR`).
5.  **Approve/Reject**: After reviewing the report, you are prompted to approve or reject the audit.
6.  **Update Manifest**: Your decision (`passed` or `failed`) is recorded in the `~/.grove/repos.json` manifest. The `cx repo list` command will reflect this status.

**Best Practice**: Always run `cx repo audit` on new third-party repositories before adding rules that reference them. This ensures you understand the content and potential security implications of the code you are including in your context.