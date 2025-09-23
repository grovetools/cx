# Advanced Topics

This section covers advanced features that support multi-project workflows, reusable rule sets, external repositories, and repeatable context configurations.

## Reusing Rules with `@default`

The `@default: <path>` directive lets you import another Grove project’s default rules into the current project. This is useful when multiple projects share a common baseline (e.g., a core library) and you want to avoid duplicating rule sets.

### Syntax and resolution

- Place the directive on its own line in `.grove/rules`:
  - `@default: <path>` where `<path>` is the root of another Grove project (relative or absolute).
- The target project must define its default rules path in `grove.yml`:
  - context.default_rules_path: path to its default rules file (relative to that project’s root).

Example:

```txt
# Project A (.grove/rules)

# Hot section
*.go
@default: ../project-b

---
# Cold section
docs/**/*.md
```

And in Project B’s `grove.yml`:

```yaml
version: 1.0
context:
  default_rules_path: .grove/default.rules
```

### How rules are imported

The effect depends on where `@default` appears:

- In the hot section (above the `---` separator):
  - All patterns from the referenced project’s default rules (both its hot and cold sections) are imported into the current project’s hot context.
- In the cold section (below the `---` separator):
  - All patterns from the referenced project’s default rules are imported into the current project’s cold context.

Notes:

- Imported patterns are prefixed with the referenced project’s filesystem path so they resolve against that project, not the current one.
- Normal precedence applies within the current project: files matched by cold patterns take precedence over hot patterns (i.e., a file matched by both ends up in the cold context).
- Paths can be absolute or relative. Relative paths are resolved from the file that contains the `@default` directive.

### Circular dependencies

`cx` detects and short-circuits circular imports (e.g., A imports B, B imports A) to prevent infinite recursion. Imported patterns are processed once per file; further cycles are ignored.

---

## Managing External Repositories

You can reference Git repositories directly in `.grove/rules`. `cx` will clone them locally and treat their contents as part of your context, allowing you to pin versions and keep a manifest of what was used.

### Adding Git URLs to rules

- Use a Git URL on its own line, optionally with a version:
  - Supported forms include:
    - https://github.com/org/repo
    - https://github.com/org/repo@v1.2.3
    - git@github.com:org/repo.git (normalized to HTTPS)
    - github.com/org/repo (normalized to HTTPS)
- You can still use exclusion patterns that apply within the cloned repository.

Example:

```txt
# Local rules
*.go
!**/*_test.go

# External repository (pin to a tag)
https://github.com/charmbracelet/lipgloss@v0.13.0

# Exclude tests and examples from the external repo
!**/*_test.go
!**/examples/**
!**/testdata/**
```

### Where repositories are stored

- Repositories are cloned to: `~/.grove/cx/repos/`
- A manifest is kept at: `~/.grove/cx/manifest.json`
  - Tracks each URL, local path, pinned version (if any), resolved commit, audit status, and optional audit report path.

Internally, when a Git URL is encountered in `.grove/rules`, `cx` resolves or clones the repo, then rewrites the rule to the repository’s local path (e.g., `/home/user/.grove/cx/repos/.../**`), so your other patterns and exclusions apply normally.

### Repository management commands

- List tracked repositories:

  ```bash
  cx repo list
  ```

  Columns include URL, VERSION (pinned or “default”), COMMIT (short SHA), STATUS, REPORT indicator, and last sync time.

- Sync all repositories:

  ```bash
  cx repo sync
  ```

  Fetches updates for all tracked repositories and checks out the pinned version when set.

- Audit a repository:

  ```bash
  cx repo audit <url>
  ```

  This starts an interactive workflow:
  - Ensures the repo is cloned and checked out at the current commit.
  - Sets up default rules in the repo, then launches `cx view` to refine the context interactively.
  - Runs an LLM analysis (via an external tool) and saves a Markdown report under the repo at `.grove/audits/<commit>.md`.
  - Prompts for approval to mark the audit as passed or failed and updates the manifest accordingly.
  - You can view reports in `cx view` (repository view) or open the file in an editor.

  To update status without a full audit:

  ```bash
  cx repo audit <url> --status=passed    # or failed, audited, etc.
  ```

Considerations:

- Pinning a version (`@tag`, branch, or commit) makes builds reproducible. Without a pinned version, `cx` uses the repository’s current default branch.
- Network access is required for cloning and syncing.
- Cloned repositories are shared across projects via the global `~/.grove/cx/` store.

---

## Using Snapshots for Different Tasks

Snapshots capture the current `.grove/rules` content under a name. This is useful for switching between rule sets (e.g., a feature branch context vs. a release-support context), reviewing changes over time, and collaborating on curated contexts.

### Creating and loading snapshots

- Save the current rules:

  ```bash
  cx save feature-foo --desc "Rules tuned for feature Foo"
  ```

  This writes `.grove/context-snapshots/feature-foo.rules` and an optional description file.

- Load a snapshot into `.grove/rules`:

  ```bash
  cx load feature-foo
  ```

- List available snapshots:

  ```bash
  cx list-snapshots
  ```

  Sorting options:

  - `--sort {date|name|size|tokens|files}` (default: date)
  - `--desc` (default: true) to sort descending; use `--desc=false` for ascending

Snapshots store rules, not a static list of resolved files. When you load a snapshot, `cx` resolves files again against the current filesystem and repository state.

### Comparing contexts with `cx diff`

Use `cx diff` to compare your current context to a saved snapshot (or to an empty baseline).

- Compare current to a named snapshot:

  ```bash
  cx diff feature-foo
  ```

  Output includes:
  - Added/removed files (sorted by token estimate)
  - Summary deltas for file count, total tokens, and total size

- Compare current to an empty context:

  ```bash
  cx diff
  ```

  This is equivalent to `cx diff empty` and shows what would be added from a blank slate.

Notes:

- Token counts in diff and stats are rough estimates (based on file size).
- Because snapshots record rules, not file lists, differences can reflect both rule changes and changes in the underlying files or repositories referenced by those rules.

---

## Practical Examples

### Import project defaults into hot context

```txt
# .grove/rules in Project A
*.go
@default: ../project-b
---
# keep cold section separate if desired
docs/**/*.md
```

Effect: Project B’s hot and cold default rules contribute files to Project A’s hot context (unless Project A’s cold section also matches them, in which case cold takes precedence).

### Include and refine an external repository

```txt
# Local project files
**/*.go
!**/*_test.go

# External repo pinned to a tag
https://github.com/org/lib@v1.2.3

# Trim the external repo
!**/examples/**
!**/vendor/**
```

Then:

```bash
cx generate
cx repo list
cx repo audit https://github.com/org/lib
```

### Create, compare, and switch snapshots

```bash
# Save current rules
cx save sprint-12 --desc "Sprint 12 rules"

# Evolve rules...
cx edit
cx save sprint-12b

# Compare
cx diff sprint-12

# Switch back
cx load sprint-12
```

These features enable modular, repeatable context setups across projects, with clear provenance for external dependencies and a simple way to move between task-specific configurations.