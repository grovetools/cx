# cx Command Reference

This reference documents all commands provided by the `cx` CLI. It is organized by common workflows and includes usage, description, arguments, flags, and practical examples drawn from the implementation.

Notes
- Unless noted, commands operate relative to the current working directory.
- The active rules file is .grove/rules unless you use a command that reads rules from another path.
- When no rules file exists, some commands (e.g., generate) create outputs but warn accordingly.

---

## Core Workflow

### cx generate
- Usage: cx generate [--xml=<bool>]
- Description: Generate .grove/context from the resolved hot context and .grove/cached-context from the cold context. When no rules exist, generates an empty context and prints a warning to stderr. Also writes .grove/cached-context-files with the list of cold files.
- Arguments: None
- Flags:
  - --xml (default: true) Use XML-style file wrappers and headers for the generated context
- Example:
  ```
  cx generate
  # Generates:
  #   .grove/context
  #   .grove/cached-context
  #   .grove/cached-context-files
  ```

### cx show
- Usage: cx show
- Description: Print the contents of .grove/context (the hot context) to stdout. Useful for piping into other tools.
- Arguments: None
- Flags: None
- Example:
  ```
  cx show | pbcopy   # macOS: copy hot context to clipboard
  ```

### cx list
- Usage: cx list
- Description: List absolute paths of files resolved into the hot context (after hot/cold precedence is applied).
- Arguments: None
- Flags: None
- Example:
  ```
  cx list
  ```

### cx list-cache
- Usage: cx list-cache
- Description: List absolute paths of files resolved into the cold (cached) context.
- Arguments: None
- Flags: None
- Example:
  ```
  cx list-cache
  ```

---

## Interactive Tools

### cx view
- Usage: cx view
- Description: Launch an interactive tree view (TUI) to visualize which files are included (hot or cold), excluded, or omitted by your rules and gitignore. You can toggle inclusion/exclusion and switch to a repository selection view.
- Arguments: None
- Flags: None
- Example:
  ```
  cx view
  ```
  Notes:
  - Tree view supports navigation (j/k, gg/G, PgUp/PgDn, Ctrl-U/D), search (/ n/N), and actions (h=hot, c=cold, x=exclude, r=refresh, p=pruning, H=toggle gitignored).
  - Press Tab to open repository selection; press ? for help overlays.

### cx dashboard
- Usage: cx dashboard [--horizontal|-H] [--plain|-p]
- Description: Display a live-updating TUI that shows hot and cold context statistics (files, tokens, size). It watches the working directory and refreshes automatically; press r to force refresh and q to quit.
- Arguments: None
- Flags:
  - --horizontal, -H Display stats side-by-side
  - --plain, -p Output a plain-text, non-TUI summary (useful for logs/CI)
- Examples:
  ```
  cx dashboard
  cx dashboard -H
  cx dashboard -p
  ```

---

## Rules Management

### cx edit
- Usage: cx edit
- Description: Open .grove/rules in your editor ($EDITOR or a sensible default). If no rules exist, creates .grove/rules with either project defaults (from grove.yml) or a small boilerplate.
- Arguments: None
- Flags: None
- Example:
  ```
  EDITOR=code cx edit
  ```

### cx reset
- Usage: cx reset [--force|-f]
- Description: Reset .grove/rules to the project’s default rules as defined in grove.yml (context.default_rules_path). If no default exists, writes a basic boilerplate. Prompts before overwriting unless --force is set.
- Arguments: None
- Flags:
  - --force, -f Overwrite without confirmation
- Example:
  ```
  cx reset
  cx reset --force
  ```

### cx set-rules
- Usage: cx set-rules <path-to-rules-file>
- Description: Copy the specified rules file into .grove/rules, making it active.
- Arguments:
  - path-to-rules-file (required): Source rules file to use
- Flags: None
- Example:
  ```
  cx set-rules ../templates/default.rules
  ```

---

## Analysis and Validation

### cx stats
- Usage: cx stats [--top=<n>]
- Description: Analyze the current hot and cold contexts and print language breakdown, largest files by estimated tokens, token distribution, and summary statistics. Outputs two sections: Hot Context Statistics and Cold (Cached) Context Statistics.
- Arguments: None
- Flags:
  - --top (default: 5) Number of largest files to display
  - JSON output: if the root command provides a --json flag (via grove-core), cx stats will emit JSON instead of text
- Examples:
  ```
  cx stats
  cx stats --top 10
  cx --json stats    # when supported by your setup, returns JSON array with hot and cold stats
  ```

### cx validate
- Usage: cx validate
- Description: Validate resolved context files for existence, duplicates (by absolute path), and permission issues; then print a report and summary.
- Arguments: None
- Flags: None
- Example:
  ```
  cx validate
  ```

### cx fix
- Usage: cx fix
- Description: Deprecated. Prints a note explaining that context is resolved dynamically from rules; edit .grove/rules to address issues rather than fixing a static list.
- Arguments: None
- Flags: None
- Example:
  ```
  cx fix
  ```

---

## Snapshots

### cx save
- Usage: cx save <name> [--desc=<text>]
- Description: Save the current .grove/rules as a named snapshot under .grove/context-snapshots/<name>.rules. Stores an optional description.
- Arguments:
  - name (required): Snapshot name
- Flags:
  - --desc Free-form description saved as <name>.rules.desc
- Example:
  ```
  cx save feature-foo --desc "Rules for the feature-foo branch"
  ```

### cx load
- Usage: cx load <name>
- Description: Load a snapshot into .grove/rules. Looks for .grove/context-snapshots/<name>.rules first; also supports legacy non-suffixed files.
- Arguments:
  - name (required): Snapshot name
- Flags: None
- Example:
  ```
  cx load feature-foo
  ```

### cx list-snapshots
- Usage: cx list-snapshots [--sort=<field>] [--desc=<bool>]
- Description: List snapshots with date, file count, token estimate, and size. Supports sorting.
- Arguments: None
- Flags:
  - --sort (default: date) One of: date, name, size, tokens, files
  - --desc (default: true) Sort descending
- Example:
  ```
  cx list-snapshots --sort size --desc=false
  ```

### cx diff
- Usage: cx diff [snapshot|current]
- Description: Compare the current context (resolved from rules) to a snapshot or to current. Without an argument, compares to an empty context. Reports added/removed files and token/size deltas.
- Arguments:
  - snapshot (optional): Name of a saved snapshot, or use current to compare the current context to itself
- Flags: None
- Examples:
  ```
  cx diff feature-foo
  cx diff current      # no-op comparison
  cx diff              # compare against an empty context
  ```

---

## Git Integration

### cx from-git
- Usage: cx from-git [--since=<date|commit>] [--branch=<a..b>] [--staged] [--commits=<n>]
- Description: Update .grove/rules with explicit file paths derived from Git activity (staged files, recent commits, a commit range, or since a given date/commit). At least one option is required. This overwrites .grove/rules with explicit paths.
- Arguments: None
- Flags:
  - --since Include files changed since a date or commit (e.g., "2 days ago" or a commit hash)
  - --branch Include files in a commit range (e.g., main..HEAD)
  - --staged Include staged files only
  - --commits Include files from the last N commits
- Examples:
  ```
  cx from-git --staged
  cx from-git --commits 3
  cx from-git --branch main..HEAD
  cx from-git --since "1 day ago"
  ```

---

## Repository Management

These commands manage Git repositories referenced in rules (e.g., rules that contain a Git URL). Repositories are cloned under ~/.grove/cx/repos and tracked in ~/.grove/cx/manifest.json.

### cx repo list
- Usage: cx repo list
- Description: List tracked repositories with URL, pinned version, resolved commit, audit status, report indicator, and last synced time.
- Arguments: None
- Flags: None
- Example:
  ```
  cx repo list
  ```

### cx repo sync
- Usage: cx repo sync
- Description: Fetch and optionally checkout pinned versions for all tracked repositories; updates the manifest with the current commit and sync time.
- Arguments: None
- Flags: None
- Example:
  ```
  cx repo sync
  ```

### cx repo audit
- Usage: cx repo audit <url> [--status=<value>]
- Description: Start an interactive audit workflow for a repository:
  - Ensures the repo is cloned and on the correct commit
  - Creates a default .grove/rules if needed
  - Launches cx view for context refinement
  - Runs an LLM analysis via the external gemapi binary and saves a report under .grove/audits
  - Prompts for approval and updates the manifest with status and report
  Alternatively, use --status to set the audit status directly without running the full workflow.
- Arguments:
  - url (required): Repository URL to audit (must already be tracked or clonable)
- Flags:
  - --status Update audit status only (e.g., passed, failed, audited, not_audited) without running the full flow
- Examples:
  ```
  cx repo audit https://github.com/example/repo
  cx repo audit https://github.com/example/repo --status=passed
  ```

---

## Version and Misc

### cx version
- Usage: cx version [--json]
- Description: Print version information for the binary (version, commit, branch, build date).
- Arguments: None
- Flags:
  - --json Output the version info as JSON
- Examples:
  ```
  cx version
  cx version --json
  ```

---

## Practical Usage Patterns

- Initialize and refine rules:
  ```
  cx edit
  cx view
  ```
- Generate and inspect context:
  ```
  cx generate
  cx show | pbcopy      # macOS
  ```
- Analyze and validate:
  ```
  cx stats
  cx validate
  ```
- Manage snapshots:
  ```
  cx save sprint-1 --desc "Initial sprint rules"
  cx list-snapshots --sort name
  cx diff sprint-1
  cx load sprint-1
  ```
- Build rules from Git activity:
  ```
  cx from-git --staged
  cx from-git --commits 5
  ```
- Include external repositories and audit them:
  - In .grove/rules, include a Git URL (optionally with @version):  
    https://github.com/org/repo@v1.2.3
  ```
  cx generate
  cx repo list
  cx repo audit https://github.com/org/repo
  ```

This guide reflects the command behavior as implemented in the codebase and avoids hidden state: `cx` resolves files directly from rules at invocation time. Adjust .grove/rules to change outcomes.