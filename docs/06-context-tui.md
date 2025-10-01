# Interactive Context TUI (`cx view`)

The `cx view` command starts a terminal user interface (TUI) for browsing a project's files and modifying context rules. It displays a file tree that shows the status of each file and directory relative to the patterns in the active `.grove/rules` file.

## Overview

The TUI provides the following functions:
-   Navigate a file tree that shows file status relative to the context.
-   Modify the `.grove/rules` file by pressing keys to include or exclude files and directories.
-   View token counts, a summary of context statistics, and the content of the rules file.
-   Switch to a repository list to manage context for local workspaces and cloned repositories.

## Interface Components

The `cx view` interface is split into two primary views that can be toggled using the `Tab` key.

### File Tree View

This is the default view, organized into two main panels:

1.  **File Tree Pane (Left)**: Displays an expandable tree of the project and any external directories included in the rules. Each entry is color-coded and prefixed with a status indicator.
2.  **Rules & Stats Panel (Right)**:
    *   The top section shows the content of the active `.grove/rules` file.
    *   The bottom section displays summary statistics for hot and cold contexts, including file and token counts.

### Repository Management View

Pressing `Tab` switches to this view, which provides a list of all discovered repositories:
-   **Workspace Repos**: Local repositories found within the Grove ecosystem workspace.
-   **Worktrees**: Git worktrees associated with workspace repositories.
-   **Cloned Repositories**: External Git repositories managed by `cx repo`.

From this view, a repository's version and audit status can be seen, and the entire repository can be added to or removed from the context.

## File Status Indicators

Each file and directory in the TUI is prefixed with an icon to indicate its context status:

| Indicator | Status           | Description                                                               |
| :-------- | :--------------- | :------------------------------------------------------------------------ |
| `✓`       | **Hot Context**  | The file is included in the main context (`.grove/context`).              |
| `❄️`      | **Cold Context** | The file is included in the cached context (`.grove/cached-context`).     |
| `🚫`      | **Excluded**     | The file was matched by an exclusion rule (`!pattern`).                   |
| `🙈`      | **Git Ignored**  | The file is ignored by `.gitignore` (and not in context).                 |
| `(none)`  | **Omitted**      | The file does not match any inclusion patterns and is not in the context. |
| `⚠️`      | **Risky Path**   | The path is outside the project and may be unsafe to add.               |

## Navigation and Modification

Interaction is keyboard-driven, with controls for navigation and rule modification.

-   **Navigation**: Use arrow keys or `j`/`k` to move the cursor. `g` and `G` jump to the top and bottom.
-   **Context Modification**: With the cursor on a file or directory, press a key to modify its inclusion status. Changes are saved directly to the `.grove/rules` file.
    -   `h`: Toggle inclusion in **hot context**.
    -   `c`: Toggle inclusion in **cold context**.
    -   `x`: Toggle **exclusion**.
-   **View Control**:
    -   `p`: Toggle pruning mode, which hides directories that do not contain any context files.
    -   `H` or `.`: Toggle visibility of git-ignored files.
    -   `/`: Search for files by name.

## Keyboard Shortcuts Reference

### File Tree View
| Key(s)            | Action                                     |
| :---------------- | :----------------------------------------- |
| `q`, `Ctrl+c`     | Quit the application                       |
| `?`               | Toggle help screen                         |
| `Tab`             | Switch to Repository Management View       |
| `↑`/`k`           | Move cursor up                             |
| `↓`/`j`           | Move cursor down                           |
| `Enter`/`Space`   | Toggle directory expand/collapse           |
| `g` / `G`         | Jump to top / bottom                       |
| `Ctrl+d`/`Ctrl+u` | Scroll down / up half a page               |
| `/`               | Start searching for a file                 |
| `n` / `N`         | Go to next / previous search result        |
| `h`               | Toggle item's inclusion in **hot context** |
| `c`               | Toggle item's inclusion in **cold context**|
| `x`               | Toggle item's **exclusion**                |
| `p`               | Toggle pruning mode                        |
| `H` / `.`         | Toggle visibility of git-ignored files     |
| `r`               | Refresh the view                           |
| `zo`/`zc`         | Open/close fold at cursor (Vim-style)      |
| `zR`/`zM`         | Open/close all folds (Vim-style)           |

### Repository Management View
| Key(s)            | Action                                         |
| :---------------- | :--------------------------------------------- |
| `q`, `Ctrl+c`     | Quit the application                           |
| `?`               | Toggle help screen                             |
| `Tab`, `Esc`      | Switch back to File Tree View                  |
| `↑`/`↓`/`j`/`k`   | Move cursor up/down                            |
| `/`               | Filter repository list                         |
| `h`               | Toggle repository's inclusion in **hot context**|
| `c`               | Toggle repository's inclusion in **cold context**|
| `x`               | Toggle repository's **exclusion**              |
| `a`               | Add/remove repository from tree view (`@view`) |
| `A`               | Run security audit on the selected repository  |
| `R`               | View audit report for the selected repository  |
| `r`               | Refresh repository list                        |

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
    -   With the cursor on the `docs` directory, press `c`.
    -   The directory and all its children will be marked with the `❄️` indicator.

5.  **Exclude a Specific File**
    -   Navigate down into the expanded `docs` directory to a file named `wip.md`.
    -   With the cursor on `wip.md`, press `x`.
    -   The file is now marked with the `🚫` indicator.

6.  **Quit and Inspect Changes**
    -   Press `q` to exit.
    -   The `.grove/rules` file is automatically updated with these changes. It will now contain an entry for `docs/**` in the cold section and an exclusion for `!docs/wip.md`.