# Interactive Tools

The `cx` CLI includes two interactive terminal interfaces that make it easier to understand, curate, and monitor your LLM context:

- `cx view` — an interactive file-tree viewer with live rule editing and a repository management view
- `cx dashboard` — a live-updating dashboard that summarizes hot and cold context statistics

This guide explains what each tool shows, how to navigate them, and how to use them effectively.

## Visualizing Context with `cx view`

`cx view` renders an interactive file tree that shows exactly which files are in your hot and cold contexts based on `.grove/rules`. It also includes a repository management view to add or remove entire repositories from your rules.

Run:
```bash
cx view
```

### What you see

- File tree (left)
  - Each node shows a directory or file and its status (hot, cold, excluded, git-ignored, omitted)
  - Large-file hinting: token counts are shown next to files, with color highlighting by size
- Rules panel (right, top)
  - A live view of `.grove/rules` (first lines with line numbers)
- Statistics panel (right, bottom)
  - Counts and token totals for hot and cold contexts

You can switch between:
- Tree view (default)
- Repository selection view (press Tab), which lists both:
  - Workspace repositories (from your Grove ecosystem or local workspace)
  - Repositories cloned and tracked by `cx repo` (with version, commit, audit status, and report availability)

### Status indicators and colors

The UI uses a combination of symbols and colors to indicate file status.

- Hot context
  - Indicator: checkmark
  - Color: green tint
- Cold context
  - Indicator: snowflake
  - Color: light blue tint
- Excluded by rule
  - Indicator: circle-with-slash
  - Color: muted red tint
- Git-ignored
  - Indicator: “hidden” icon
  - Color: very dark gray
- Omitted (matches no include pattern and not explicitly excluded)
  - Indicator: none
  - Color: gray

Additional cues:
- Directories are bold
- Token counts next to files are color-weighted:
  - ≥ 100k tokens: red
  - ≥ 50k tokens: orange
  - ≥ 10k tokens: yellow
  - else: dim gray
- Potentially risky additions (e.g., patterns that traverse multiple parent directories) prompt for confirmation before rules are changed

### Tree view keybindings

Navigation
- Up/Down: arrow keys or j/k
- Half page: Ctrl+u / Ctrl+d
- Page: PgUp / PgDn
- Top/Bottom: gg / G
- Expand/collapse current: Enter or Space
- Vim-style folding:
  - za (toggle), zo (open), zc (close), zR (open all), zM (close all)
- Search:
  - / to enter query, Enter to apply
  - n / N to move to next/previous match
  - Esc to cancel search

Actions
- h: toggle item in hot context
- c: toggle item in cold context
- x: toggle exclusion for item
- p: toggle pruning (show only directories that contain context files)
- . or H: toggle visibility of git-ignored files
- r: refresh the analysis
- Tab: open the repository management view
- A: open the repository management view directly (same destination as Tab)
- ?: toggle help
- q or Ctrl+c: quit

Safety confirmations
- When an addition would introduce a broad or potentially unsafe rule (e.g., multiple ../ segments), the UI shows a warning and requires y to confirm or n/Esc to cancel.

Notes on rule changes
- Adding a directory rule adds it as a recursive pattern (e.g., path/**) for clarity and consistency
- Cold context takes precedence over hot: if the same file is matched by both sections, it is treated as cold

### Repository management view (Tab)

The repository view lists:
- Workspace repositories (main and worktree branches)
- Cloned repositories managed by `cx repo` (with URL, version, short commit, audit status, and report availability)

Status hints next to each repo indicate whether the repo path is represented in your rules:
- In hot, in cold, excluded, or none

Keybindings
- Navigation: Up/Down or j/k; half page Ctrl+u/d; PgUp/PgDn; g/G for top/bottom
- Filtering: / to filter by name/path/branch; Backspace to edit; Esc to clear filter
- Toggle context or exclusion for the selected repo:
  - h: add/remove repo path (as path/**) to hot rules
  - c: add/remove to cold rules
  - x: add/remove exclusion rule
- Refresh: r (refresh repo list, rules, and stats)
- View audit report (cloned repos only): R (uppercase)
- Start audit workflow (cloned repos only): A
  - Exits `cx view` and launches `cx repo audit <url>` which:
    - Ensures the repo is cloned and on the pinned version
    - Sets up default audit rules
    - Lets you refine context using `cx view` in the repo
    - Runs LLM analysis and saves a report (if configured)
    - Updates the manifest audit status on approval
- Switch back to tree view: Tab
- Help: ?
- Quit: q

What gets written
- Toggling a repo writes or removes rules using the repo path with a recursive pattern (path/**)
- Exclusions and removals update `.grove/rules` immediately

## Monitoring with `cx dashboard`

`cx dashboard` displays a live summary of your context. It watches the workspace and updates automatically when files or rules change.

Run:
```bash
cx dashboard
```

What it shows
- Two summary panels for Hot and Cold context:
  - Total files
  - Approximate total tokens
  - Total size
- Last update timestamp and an “updating” indicator during refresh

Live updates
- The dashboard watches the current directory tree and refreshes when relevant files change (rules, matched files, etc.). Common build and vendor directories are ignored to reduce noise.

Controls
- r: manual refresh
- q / Esc / Ctrl+c: quit

Layout options
- Horizontal (side-by-side panels):
  ```bash
  cx dashboard -H
  ```
- Plain-text (no TUI; suitable for logs or CI):
  ```bash
  cx dashboard -p
  ```

Notes
- The dashboard uses the same resolution logic as other commands:
  - Hot files are those matched by the top section of `.grove/rules`, excluding files that also appear in the cold section
  - Cold files come from the section below the `---` separator
- Token counts are estimates derived from file sizes

## Practical tips

- Use `cx view` first to refine your rules with clear feedback, then `cx generate` to create `.grove/context` and `.grove/cached-context`.
- Use pruning (p) and git-ignored visibility (./H) in `cx view` to focus the tree on relevant parts of the workspace.
- Manage entire repositories from the repository view when your context spans multiple projects or tracked dependencies.
- Keep `cx dashboard` running in another terminal while you work to monitor the impact of changes on context size and token counts.