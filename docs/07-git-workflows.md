# Git-Based Workflows

`grove-context` can generate rules based on Git history. This is used to create temporary, task-specific contexts that focus on a set of changes within a repository.

## The `cx from-git` Command

The `cx from-git` command queries the Git history and overwrites the `.grove/rules` file with explicit paths to files that match the specified criteria.

Because this command overwrites existing rules, it is common to save the primary rule set as a snapshot before use and restore it afterward.

```bash
# Save the current pattern-based rules
cx save my-dev-rules

# Generate a temporary context from staged files
cx from-git --staged

# ... use the generated context ...

# Restore the original rules
cx load my-dev-rules
```

### Command Syntax and Options

**Usage:**
```bash
cx from-git [flags]
```

**Flags:**

| Flag        | Description                                                 | Example                        |
|-------------|-------------------------------------------------------------|--------------------------------|
| `--staged`  | Includes files that are currently staged for commit.        | `cx from-git --staged`         |
| `--branch`  | Includes files changed between two branches or commits.     | `cx from-git --branch main..HEAD` |
| `--commits` | Includes files changed in the last `N` commits.             | `cx from-git --commits 3`      |
| `--since`   | Includes files changed since a specific date or commit hash. | `cx from-git --since "2 days ago"` |

At least one flag must be specified.

## Use Cases

### 1. Preparing for a Commit

To generate a context containing only files in the staging area, which can be used for writing a commit message or performing a final review.

**Command:**
```bash
cx from-git --staged
```

**Mechanism:**
This command executes `git diff --cached --name-only` and writes the resulting file paths to `.grove/rules`.

### 2. Working on a Feature Branch

To generate a context that includes every file changed on a feature branch relative to a base branch.

**Scenario:** On a feature branch `feat/user-auth`, generate a context of all changes relative to `main`.

**Command:**
```bash
cx from-git --branch main..HEAD
```

**Mechanism:**
The command populates `.grove/rules` with the output of `git diff --name-only main..HEAD`, listing all files added, modified, or renamed on the current branch.

### 3. Summarizing Recent Work

To generate context from files modified in the last few commits.

**Scenario:** Generate a context of all files affected by the last five commits.

**Command:**
```bash
cx from-git --commits 5
```

**Mechanism:**
This command uses `git log` to find all unique files modified in the last five commits and writes their paths to `.grove/rules`.

### 4. Code Review Workflows

To generate the exact context of a pull request's changes for review.

**Scenario:** Reviewing a colleague's branch named `fix/api-bug`.

**Commands:**
```bash
# Check out the branch
git checkout fix/api-bug

# Generate context for the changes relative to the main branch
cx from-git --branch main..HEAD
```

**Outcome:**
The `.grove/rules` file is populated with the specific files changed in the pull request, creating an isolated context for the review.

### 5. Merge Conflict Resolution

To build a context of incoming changes when a merge conflict occurs.

**Scenario:** A `git pull` on the `main` branch results in a merge conflict with `origin/main`.

**Command:**
```bash
# Generate context of the remote changes being merged
cx from-git --branch HEAD..origin/main
```

**Outcome:**
This command provides the file paths of the remote changes. This context can be provided to an LLM, along with the conflicted file content, to assist in resolving the merge.