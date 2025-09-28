# Git-Based Workflows

`grove-context` integrates with Git to create dynamic, task-specific contexts based on your repository's history. This allows you to focus the context on recent changes, work in progress, or differences between branches, which is particularly useful for code reviews, summarizing work, and resolving conflicts.

## The `cx from-git` Command

The primary tool for this integration is the `cx from-git` command. Instead of using patterns, this command queries your Git history and generates a new `.grove/rules` file containing explicit paths to the files that match your criteria. This provides a precise, temporary context tailored to a specific task.

Because `cx from-git` overwrites your existing `.grove/rules` file, it's often useful to save your primary rule set as a snapshot before using it:

```bash
# Save your current pattern-based rules
cx save my-dev-rules --desc "Default rules for Go development"

# Generate context from staged files for a commit message
cx from-git --staged

# ... use the generated context ...

# Restore your original rules when done
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
| `--staged`  | Includes only files that are currently staged for commit.   | `cx from-git --staged`         |
| `--branch`  | Includes files changed between two branches or commits.     | `cx from-git --branch main..HEAD` |
| `--commits` | Includes files changed in the last `N` commits.             | `cx from-git --commits 3`      |
| `--since`   | Includes files changed since a specific date or commit hash. | `cx from-git --since "2 days ago"` |

At least one flag must be specified.

## Practical Workflows and Scenarios

Here are common development scenarios where `cx from-git` can streamline your workflow.

### 1. Preparing for a Commit (Staged Files)

When you're ready to commit your work, you can generate a context containing only the files you've staged. This is ideal for writing a detailed commit message or running a final review with an LLM.

**Scenario:** You have modified `handler.go` and created `new_service.go`, and both are staged.

**Command:**
```bash
cx from-git --staged
```

**Outcome:**
Your `.grove/rules` file is overwritten with the following content:
```
src/api/handler.go
src/services/new_service.go
```
The context now precisely represents the changes you are about to commit.

### 2. Working on a Feature Branch (Branch Context)

To get a complete picture of the work done on a feature branch, you can generate a context that includes every file changed relative to your main branch.

**Scenario:** You are on a feature branch `feat/user-auth` and want to summarize all changes made since you branched off `main`.

**Command:**
```bash
cx from-git --branch main..HEAD
```

**Outcome:**
The `.grove/rules` file will be populated with a list of all files added, modified, or renamed on the `feat/user-auth` branch. This gives an LLM the full scope of your feature for documentation, review, or refactoring suggestions.

### 3. Summarizing Recent Work (Commit Context)

If you want to review the last few commits or generate a summary of recent activity, you can target a specific number of commits.

**Scenario:** You want to generate release notes based on the changes in the last five commits.

**Command:**
```bash
cx from-git --commits 5
```

**Outcome:**
`cx` generates a rules file containing every file that was modified in the last five commits on your current branch.

### 4. Code Review Workflows

`cx from-git` is an excellent tool for code reviewers. By checking out a pull request branch, a reviewer can instantly generate the exact context of the proposed changes.

**Scenario:** You are reviewing a pull request from a colleague's branch named `fix/api-bug`.

**Commands:**
```bash
# Check out the branch
git checkout fix/api-bug

# Generate context for the changes relative to the main branch
cx from-git --branch main..HEAD
```

**Outcome:**
You now have the precise context needed to ask an LLM to "review this code for potential issues" or "suggest improvements to this implementation," knowing it has all the relevant files.

### 5. Merge Conflict Resolution

When a merge conflict occurs, understanding the context of the incoming changes can be critical. You can use `cx from-git` to build a context of the branch you are trying to merge.

**Scenario:** You are on the `main` branch and a `git pull` results in a merge conflict with changes from the `origin/main` branch.

**Command:**
```bash
# Generate context of the changes you are pulling down
cx from-git --branch HEAD..origin/main
```

**Outcome:**
This command provides the context of the remote changes, which can help you understand the intent behind the conflicting code. You can then provide this context to an LLM along with the conflicted file content and ask for help in resolving the merge.