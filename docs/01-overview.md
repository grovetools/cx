# Overview

`grove-context` (`cx`) is a rule-based CLI tool for assembling file-based context for Large Language Models (LLMs). It file selection by writing a `.grove/rules` file. This is a foundational tool in the Grove ecosystem, designed to solve a critical problem: making coding agents more effective by providing them with curated, relevant, and consistent context.

The central idea is that the quality of an agent's output, particularly in complex planning stages, is directly proportional to the quality of the context it receives. `cx` provides a rational, predictable process for generating that context. `cx` is editor-independent and easily integrated into other workflows. 

## Key features

- Dynamic context generation from a `.grove/rules` file  
  A gitignore-like syntax defines which files to include or exclude from your filesystem. `cx` resolves patterns on demand.

- Hot & Cold context separation  
  Use a `---` separator in the rules to split context:
  - Hot: frequently changing files you’ll reference repeatedly
  - Cold: stable dependencies or background material suitable for token caching  
  Outputs are written to:
  - `.grove/context` (hot)
  - `.grove/cached-context` (cold)

- Interactive tools for understanding/managing context: `cx view`  
  An interactive TUI that shows included, excluded, and git-ignored files, with quick toggles to adjust rules.

- Git integration for context generation (`cx from-git`)  
  Build rules from staged files, a commit range, a branch diff, or a time window.

- Snapshots for saving/loading different context configurations  
  Save the current rules with `cx save`, restore with `cx load`, and compare with `cx diff`.

- External repository management and auditing (`cx repo`)  
  Include Git URLs directly in rules. `cx` will clone, track, and optionally audit these repositories. Manage them with `cx repo list`, `cx repo sync`, and `cx repo audit`.

## How it works

The core is the `.grove/rules` file in your project. It uses a familiar, `.gitignore`-style syntax to declare inclusion and exclusion patterns. A `---` separator divides the file into hot (above) and cold (below) sections. When you run `cx generate`, `cx` resolves these patterns and writes the concatenated outputs to `.grove/context` (hot) and `.grove/cached-context` (cold). Cold-over-hot precedence ensures files listed in the cold section do not also appear in the hot output.

Example rules:

```txt
# Hot context (active work)
**/*.go
!**/*_test.go
README.md

---
# Cold context (stable references)
../shared-lib/**/*.go
!../shared-lib/**/internal/**
```

Typical workflow:

```bash
# Define or edit rules
cx edit

# Visualize and refine
cx view

# Generate concatenated context files
cx generate

# Use the hot context (example: copy to clipboard on macOS)
cx show | pbcopy

# Optional: manage variants
cx save feature-foo
cx diff feature-foo
cx load feature-foo
```

Git-driven generation:

```bash
# Only staged files
cx from-git --staged

# Files from last 3 commits
cx from-git --commits 3

# Compare branch range
cx from-git --branch main..HEAD

# Since a time
cx from-git --since "2 days ago"
```

External repositories:

- Add a URL directly to `.grove/rules`, optionally with a version (e.g., `https://github.com/org/repo@v1.2.3`)
- Manage with:
  - `cx repo list` — view tracked repositories and their status
  - `cx repo sync` — fetch and check out pinned versions
  - `cx repo audit <url>` — run an interactive audit workflow

## Who should use `cx`?

- Developers working with LLMs who need to supply curated, file-based context from code repositories
- Teams that want a consistent, version-controlled process for assembling inputs to LLM tools
- Projects that benefit from separating frequently edited files (hot) from stable background materials (cold), especially when using services that cache tokens

By centering on a single rules file and providing both interactive and automated workflows, `cx` makes context assembly predictable, inspectable, and easy to adapt to different tasks.
