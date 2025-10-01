# Practical Examples

This document provides examples of using Grove Context (`cx`) in common development workflows.

## Example 1: Quick Start - Pattern-Based Context

This example covers the basic workflow for a new project: creating rules, generating context, and inspecting the results.

Assume a Go project with the following structure:

```
my-go-app/
├── main.go
├── utils.go
├── utils_test.go
└── README.md
```

#### 1. Create a Rules File

Create a `.grove/rules` file to define the context. This file will include all Go source files but exclude test files.

```bash
# Create the directory
mkdir -p .grove

# Create the rules file
echo "**/*.go" > .grove/rules
echo "!**/*_test.go" >> .grove/rules
```

Your `.grove/rules` file now contains:

```gitignore
**/*.go
!**/*_test.go
```

#### 2. Edit Rules

The `cx edit` command opens the rules file in the editor defined by the `$EDITOR` environment variable. This can be bound to a keyboard shortcut for frequent editing.

```bash
# Example for .zshrc or .bashrc
bindkey -s '^x^c' 'cx edit\n'
```

#### 3. List the Context Files

Run `cx list` to see which files are included based on the rules. `cx` filters out binary files and files ignored by Git.

```bash
cx list
```

**Expected Output:**

```
/path/to/my-go-app/main.go
/path/to/my-go-app/utils.go
```

The file `utils_test.go` is excluded by the `!**/*_test.go` pattern.

#### 4. Analyze Context Statistics

Use `cx stats` to get a summary of the context, including file counts, token estimates, and language breakdown.

```bash
cx stats
```

The command outputs a summary of total files and tokens, a distribution of languages, and a list of the largest files by token count.

## Example 2: Managing Projects with Multiple Components

`cx` can manage context for workspaces containing multiple projects. This example demonstrates features for such scenarios.

Assume a workspace with a web frontend, a backend API, and a shared library:

```
my-workspace/
├── api/
│   ├── main.go
│   └── handlers/
│       └── user.go
├── frontend/
│   ├── src/
│   │   └── App.tsx
│   └── package.json
└── shared-lib/
    ├── utils.go
    └── README.md
```

#### 1. Switch Between Different Rule Sets

Different tasks may require different contexts. You can maintain multiple rule files and switch between them using `cx set-rules`.

-   **`docs.rules`:** For generating documentation.
-   **`api-dev.rules`:** For working on the API, including the shared library.

```gitignore
# docs.rules
README.md
shared-lib/README.md
```

```gitignore
# api-dev.rules
api/**/*.go
shared-lib/**/*.go
!**/*_test.go
```

Switch to the API development context:

```bash
cx set-rules api-dev.rules
```

This command copies the content of `api-dev.rules` into `.grove/rules`, making it the active configuration.

#### 2. Visually Browse and Modify Context

The `cx view` command starts an interactive terminal interface to inspect and modify the context.

```bash
cx view
```

In the TUI, you can:
-   Navigate the file tree.
-   View status indicators for each file (hot, cold, excluded, omitted).
-   Modify the `.grove/rules` file by toggling a file's inclusion in hot (`h`), cold (`c`), or excluded (`x`) contexts.
-   Press `Tab` to switch to the repository management view.

#### 3. Manage Repositories and Worktrees

In the `cx view` TUI, pressing `Tab` shows a list of local workspaces, Git worktrees, and cloned external repositories. From this view, you can add an entire repository to your hot or cold context or run a security audit on an external repository.

#### 4. Generate Context from Git History

To generate context from recent changes, use `cx from-git`. This command overwrites `.grove/rules` with explicit paths to the changed files.

```bash
# Include all files staged for commit
cx from-git --staged

# Include all files changed on the current branch against 'main'
cx from-git --branch main..HEAD
```

#### 5. Include External Repositories

You can include files from other repositories by adding their Git URL to the rules file. `cx` will clone the repository locally. It is recommended to audit the repository first.

```bash
# 1. Audit the repository to check its contents.
cx repo audit https://github.com/charmbracelet/lipgloss

# 2. Add the URL to .grove/rules, optionally pinning to a version.
echo "https://github.com/charmbracelet/lipgloss@v0.13.0" >> .grove/rules
```

#### 6. Reset to Defaults

The `cx reset` command restores `.grove/rules` to a default state. If `context.default_rules_path` is defined in `grove.yml`, it will use that file; otherwise, it creates a boilerplate file.

```bash
# Reset .grove/rules to the project's default configuration.
cx reset
```