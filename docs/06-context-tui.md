# Interactive Context TUI (`cx view`)

The `cx view` command launches a terminal user interface (TUI) for interactively visualizing and managing your project's context. It provides a real-time, color-coded file tree that shows exactly which files are included, excluded, or ignored based on your current rules, making it the primary tool for understanding and refining your context.

## Overview

The TUI allows you to:
-   **Explore Visually**: Navigate your project's file structure and see the status of each file and directory.
-   **Modify Interactively**: Add files to hot or cold context, or exclude them, with single keypresses. Changes are saved directly to your `.grove/rules` file.
-   **Inspect in Real-Time**: See token counts for files and directories, view the current rules file, and monitor summary statistics without leaving the interface.
-   **Manage Repositories**: Switch to a dedicated view to manage context inclusion for local workspaces, worktrees, and cloned external repositories.

## Interface Components

The `cx view` interface is split into two main views: the File Tree and Repository Management.

### File Tree View

This is the default view, organized into two main panels:

1.  **File Tree Pane (Left)**: Displays an expandable tree of your project and any external directories included in your rules. Each entry is color-coded and prefixed with a status indicator.
2.  **Rules & Stats Panel (Right)**:
    *   The top section shows a snippet of your active `.grove/rules` file.
    *   The bottom section displays summary statistics for your hot and cold contexts, including file and token counts.

### Repository Management View

Pressing `Tab` switches to this view, which provides a list of all discovered repositories:
-   **Workspace Repos**: Local repositories found within your Grove ecosystem workspace.
-   **Worktrees**: Git worktrees associated with your workspace repositories.
-   **Cloned Repositories**: External Git repositories managed by `cx repo`.

From this view, you can see a repository's version, audit status, and add or remove the entire repository from your context.

## Understanding File Status

Each file and directory in the TUI is prefixed with an icon to indicate its context status:

| Indicator | Status           | Description                                                               |
| :-------- | :--------------- | :------------------------------------------------------------------------ |
| `✓`       | **Hot Context**  | The file is included in the main context (`.grove/context`).              |
| `❄️`      | **Cold Context** | The file is included in the cached context (`.grove/cached-context`).     |
| `🚫`      | **Excluded**     | The file was matched by an exclusion rule (`!pattern`).                   |
| `🙈`      | **Git Ignored**  | The file is ignored by `.gitignore` (and not in context).                 |
| `(none)`  | **Omitted**      | The file does not match any inclusion patterns and is not in the context. |
| `⚠️`      | **Risky Path**   | The path is outside the project and could be unsafe to add.               |

## Navigation and Modification

Interaction is keyboard-driven, with many controls inspired by Vim.

### File Tree View
-   **Navigate**: Use arrow keys or `j`/`k` to move the cursor up and down. Use `g` and `G` to jump to the top and bottom.
-   **Expand/Collapse**: Press `Enter` or `Space` to toggle a directory's expanded state. Vim-style folding commands (`zo`, `zc`, `zR`, `zM`) are also available.
-   **Modify Context**:
    -   `h`: Toggle inclusion in **hot context**.
    -   `c`: Toggle inclusion in **cold context**.
    -   `x`: Toggle **exclusion**.
-   **Control View**:
    -   `p`: Toggle pruning mode (hides directories that don't contain any context files).
    -   `H` or `.`: Toggle visibility of git-ignored files.
    -   `r`: Refresh the entire view.
    -   `/`: Search for files by name.

### Repository Management View
-   **Navigate**: Use arrow keys or `j`/`k` to select a repository.
-   **Modify Context**: Use `h`, `c`, and `x` to add or remove the entire selected repository from the respective context.
-   **Repository Actions**:
    -   `A`: Run the interactive security audit workflow for the selected repository.
    -   `R`: View the audit report for a repository that has been audited.

## Keyboard Shortcuts Reference

### File Tree View
| Key(s)         | Action                                     |
| :------------- | :----------------------------------------- |
| `q`, `Ctrl+c`  | Quit the application                       |
| `?`            | Toggle help screen                         |
| `Tab`          | Switch to Repository Management View       |
| `↑`/`k`        | Move cursor up                             |
| `↓`/`j`        | Move cursor down                           |
| `Enter`/`Space`| Toggle directory expand/collapse           |
| `g` / `G`      | Jump to top / bottom                       |
| `Ctrl+d`/`Ctrl+u`| Scroll down / up half a page             |
| `/`            | Start searching for a file                 |
| `n` / `N`      | Go to next / previous search result        |
| `h`            | Toggle item's inclusion in **hot context** |
| `c`            | Toggle item's inclusion in **cold context**|
| `x`            | Toggle item's **exclusion**                |
| `p`            | Toggle pruning mode                        |
| `H` / `.`      | Toggle visibility of git-ignored files     |
| `r`            | Refresh the view                           |
| `zo`/`zc`      | Open/close fold at cursor (Vim-style)      |
| `zR`/`zM`      | Open/close all folds (Vim-style)           |

### Repository Management View
| Key(s)         | Action                                         |
| :------------- | :--------------------------------------------- |
| `q`, `Ctrl+c`  | Quit the application                           |
| `?`            | Toggle help screen                             |
| `Tab`, `Esc`   | Switch back to File Tree View                  |
| `↑`/`↓`/`j`/`k`| Move cursor up/down                            |
| `/`            | Filter repository list                         |
| `h`            | Toggle repository's inclusion in **hot context**|
| `c`            | Toggle repository's inclusion in **cold context**|
| `x`            | Toggle repository's **exclusion**              |
| `A`            | Run security audit on the selected repository  |
| `R`            | View audit report for the selected repository  |
| `r`            | Refresh repository list                        |

## Practical Example: Refining Context Interactively

This workflow shows how to use `cx view` to add a documentation directory to your cold context and exclude a specific file.

1.  **Launch the TUI**
    ```bash
    cx view
    ```

2.  **Navigate to the `docs` Directory**
    -   Use the `j` key or down arrow to move the cursor down to the `docs` directory entry.

3.  **Expand the Directory**
    -   Press `Enter`. The contents of the `docs` directory are now visible.

4.  **Add the Directory to Cold Context**
    -   With the cursor still on the `docs` directory, press `c`.
    -   The directory and all its children will now be marked with the `❄️` indicator, signifying they are in the cold context.

5.  **Exclude a Specific File**
    -   Navigate down into the expanded `docs` directory to a file named `wip.md`.
    -   With the cursor on `wip.md`, press `x`.
    -   The file is now marked with the `🚫` indicator and will be excluded.

6.  **Quit and Inspect Changes**
    -   Press `q` to exit.
    -   Your `.grove/rules` file has been automatically updated with the changes you made. It will now contain an entry for `docs/**` in the cold section and an exclusion for `!docs/wip.md`.