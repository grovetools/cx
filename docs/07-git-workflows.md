# Grove Context Git Workflows

grove-context can generate context rules based on Git history. This mechanism uses `git` commands to identify relevant files and populates the active `.grove/rules` file with their paths, allowing for context generation that is scoped to specific changes.

## The `from-git` Command

The `cx from-git` command populates the active `.grove/rules` file with a list of files identified by various `git` commands. It overwrites the existing rules file with the generated file list.

### Options

-   `--staged`: Includes files currently in the Git staging area. This corresponds to the output of `git diff --cached --name-only`.
-   `--branch <range>`: Includes files that differ between two branches or commits. For example, `main..HEAD`. This uses `git diff --name-only <range>`.
-   `--since <ref>`: Includes files modified since a specific date, tag, or commit hash. This uses `git log --since=<ref>`.
-   `--commits <n>`: Includes files modified in the last `<n>` commits. This uses `git log -<n>`.

## Workflows and Use Cases

The `from-git` command is designed for several common development scenarios.

### Branch Context (Feature Development)

When working on a feature branch, you can generate context containing only the files that have changed relative to the main branch.

**Command:**
```bash
cx from-git --branch main..HEAD
```

**Mechanism:**
1.  `git diff --name-only main..HEAD` is executed to list the files that have changed.
2.  The active `.grove/rules` file is overwritten with this list of files.
3.  Subsequent `cx generate` or `gemapi request` commands will use this file list as the context.

### Commit Context (Bug Fixes & Analysis)

To analyze changes from recent commits, context can be generated from the last `n` commits.

**Command:**
```bash
# Generate context from the last 2 commits
cx from-git --commits 2
```

**Mechanism:**
1.  `git log -2 --name-only --pretty=format:` is executed to list files modified in the last two commits.
2.  The file list is deduplicated and written to `.grove/rules`.

### Staged Files (Pre-Commit Review)

Before committing, you can build a context consisting only of the files currently in the Git staging area. This is useful for reviewing changes or writing a commit message.

**Command:**
```bash
# Stage some files
git add path/to/file1.go path/to/file2.go

# Generate context from staged files
cx from-git --staged
```

**Mechanism:**
1.  `git diff --cached --name-only` lists the staged files.
2.  The output is written to `.grove/rules`.

### Code Review Workflows

A reviewer can generate context for a branch they are about to review.

**Command:**
```bash
# After checking out the feature branch 'feature/new-api'
cx from-git --branch main
```

**Mechanism:**
1.  This compares the current branch (`feature/new-api`) against `main`.
2.  The resulting file list populates `.grove/rules`, providing the reviewer with the exact context of the changes.

### Merge Conflict Resolution

When a merge conflict occurs, you can generate a context of the files that have changed on your current branch to help understand the scope of your changes relative to the merge base.

**Command:**
```bash
# After a failed merge from 'main'
cx from-git --branch main..HEAD
```

**Mechanism:**
1.  This lists all files changed on your current branch that are not in `main`.
2.  This file list can be used to provide an LLM with the relevant files for assistance in resolving the conflict.