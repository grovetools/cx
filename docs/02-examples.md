# Practical Examples

This guide provides practical examples of how to use Grove Context (`cx`) in common development workflows, from simple projects to complex multi-repository setups.

## Example 1: Quick Start - Pattern-Based Context

This example covers the fundamental workflow for a new project: creating rules, generating context, and inspecting the results.

Let's assume you have a simple Go project with the following structure:

```
my-go-app/
├── main.go
├── utils.go
├── utils_test.go
└── README.md
```

#### 1. Create a Rules File

First, create a `.grove/rules` file to define your context. You can do this manually or with a simple command. This file will include all Go source files but exclude test files.

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

#### 2. Edit Rules Quickly

The `cx edit` command opens your rules file in your default editor (`$EDITOR`). This is the fastest way to iterate on your context rules.

**Pro Tip:** Bind this command to a keyboard shortcut in your shell for instant access. For example, in your `.zshrc` or `.bashrc`:

```bash
# Edit Grove Context rules with Ctrl+X+C
bindkey -s '^x^c' 'cx edit\n'
```

Pressing `Ctrl+X C` will now open `.grove/rules` for immediate editing.

#### 3. List the Context Files

Run `cx list` to see which files are included based on your rules. `cx` automatically excludes binary files and files ignored by Git.

```bash
cx list
```

**Expected Output:**

```
/path/to/my-go-app/main.go
/path/to/my-go-app/utils.go
```

Notice that `utils_test.go` is correctly excluded by the `!**/*_test.go` pattern.

#### 4. Analyze Context Statistics

Use `cx stats` to get a summary of your context, including file counts, token estimates, and language breakdown.

```bash
cx stats
```

**Expected Output:**

```
Hot Context Statistics:

╭─ Summary ────────────────────────────────────────╮
│ Total Files:    2                                  │
│ Total Tokens:   ~25                                │
│ Total Size:     100 bytes                          │
╰──────────────────────────────────────────────────╯

Language Distribution:
  Go           100.0%  (25 tokens, 2 files)

Largest Files (by tokens):
   1. main.go                                     15 tokens (60.0%)
   2. utils.go                                    10 tokens (40.0%)

...
```

This confirms you have a small, targeted context ready to be used with an LLM.

## Example 2: Managing Complex Projects

`cx` excels at managing context for complex monorepos or multi-project workspaces. This example demonstrates advanced features for such scenarios.

Imagine a workspace containing a web frontend, a backend API, and a shared library:

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

You might need different contexts for different tasks. You can maintain multiple rule files and switch between them using `cx set-rules`.

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

This copies the content of `api-dev.rules` into `.grove/rules`, making it the active configuration.

#### 2. Visually Browse and Modify Context with the TUI

The `cx view` command launches an interactive terminal UI, providing the best way to understand and modify complex contexts.

```bash
cx view
```

Inside the TUI, you can:
-   Navigate the file tree with `j`/`k` or arrow keys.
-   See real-time status indicators for each file (hot, cold, excluded, omitted).
-   Toggle a file or directory's inclusion in hot (`h`), cold (`c`), or excluded (`x`) contexts. Your `.grove/rules` file is updated automatically.
-   Press `Tab` to switch to the **Repository Management** view.

#### 3. Manage Repositories and Worktrees

In the `cx view` TUI, press `Tab` to access the repository manager. From here, you can:
-   See all local workspaces, Git worktrees, and cloned external repositories.
-   Add an entire repository to your hot or cold context with a single keypress.
-   Run a security audit on an external repository before including it.

#### 4. Generate Context from Git History

To generate context based only on your current work, use `cx from-git`. This is perfect for code reviews or summarizing changes.

```bash
# Include all files staged for commit
cx from-git --staged

# Include all files changed on your current branch against 'main'
cx from-git --branch main..HEAD
```

This command overwrites your `.grove/rules` file with explicit paths to the changed files, giving you a precise, task-specific context.

#### 5. Include External Repositories

You can include files from other repositories by adding their Git URL to your rules file. `cx` will clone the repository locally and manage it. For safety, it is best to audit the repository first.

```bash
# 1. Audit the repository to understand its size and content
cx repo audit https://github.com/charmbracelet/lipgloss

# 2. If the audit looks good, add the URL to your .grove/rules file.
#    You can pin to a specific tag or commit hash.
echo "https://github.com/charmbracelet/lipgloss@v0.13.0" >> .grove/rules
```

#### 6. Reset to Defaults

If your rules file becomes too complex or you want to start fresh, `cx reset` restores it to a default state. If `context.default_rules_path` is defined in your `grove.yml`, it will use that file; otherwise, it creates a basic boilerplate.

```bash
# Reset .grove/rules to the project's default configuration
cx reset
```