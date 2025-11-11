# Context TUI (`cx view`)

The `cx view` command launches a tabbed terminal user interface (TUI) for inspecting and managing context. It provides several views to analyze which files are included based on the active rules, see token consumption, and interactively modify the context.

## Overview

The TUI is organized into four main tabs:

-   **TREE**: A hierarchical file browser showing the project structure.
-   **RULES**: A viewer for the active rules file.
-   **STATS**: An analytics view of context composition by file type and size.
-   **LIST**: A flat list of all files included in the context.

The interface provides real-time token counts and visual indicators for file status, and allows for interactive rule editing and context switching.

## Tab System

### TREE Tab

The TREE tab displays a hierarchical view of the project's file system.

-   **Navigation**: Directories can be expanded and collapsed using arrow keys.
-   **Visual Indicators**:
    -   `✓`: File or directory is included in the hot or cold context.
    -   `🚫`: File or directory is explicitly excluded by a rule.
    -   `(CWD)`: Marks the current working directory.
-   **Token Counts**: Estimated token counts are shown inline for each directory and included file, allowing for quick identification of high-token areas.

### RULES Tab

The RULES tab displays the content of the active rules file.

-   **Path Display**: Shows the path to the currently active rules file (e.g., `.grove/rules` or a named set from `.cx/`).
-   **Content Viewer**: Displays the full content of the rules file with syntax highlighting for comments and directives.
-   **Editing**: Pressing `e` opens the active rules file in your default editor (`$EDITOR`).

### STATS Tab

The STATS tab provides an analytical breakdown of the context composition. It is split into two panels:

-   **File Type Distribution**: Lists file types (e.g., `.go`, `.md`) sorted by their total token contribution, showing percentages, token counts, and file counts for each.
-   **Largest Files**: Lists the individual files that contribute the most tokens to the context.

This view helps identify which file types or specific files are consuming the most context space.

### LIST Tab

The LIST tab shows a flat, sortable list of every file included in the context.

-   **Detailed View**: Each entry displays the file's full path and individual token count.
-   **Sorting**: The list can be sorted alphabetically or by token count.
-   **Exclusion**: Individual files can be excluded from the context directly from this view by pressing `x`.

## Keyboard Shortcuts

### Global Navigation

-   `tab` / `shift+tab`: Switch between TREE, RULES, STATS, and LIST tabs.
-   `↑`/`↓` or `j`/`k`: Navigate up/down within the current view's list or tree.
-   `?`: Toggle the help display.
-   `q` / `ctrl+c`: Quit the TUI.

### TREE Tab

-   `→` or `l`: Expand the selected directory.
-   `←` or `h`: Collapse the selected directory.
-   `enter` or `space`: Toggle expand/collapse for the selected directory.
-   `za`: Toggle fold at cursor.
-   `zo`/`zc`: Open/close fold at cursor.
-   `zR`/`zM`: Open/close all folds.
-   `t`: Toggle sort order (alphabetical vs. token count).

### RULES Tab

-   `e`: Edit the active rules file in `$EDITOR`.

### STATS Tab

-   `s` or `←`/`→`: Switch focus between the "File Types" and "Largest Files" lists.

### LIST Tab

-   `x`: Exclude the selected file from the context.
-   `t`: Toggle sort order (alphabetical vs. token count).

### Context Management Actions

-   `e` (Most Tabs): Edit the active rules file in `$EDITOR`.
-   `s` (Most Tabs): Open a selector to switch to a different named rule set from `.cx/`.
-   `r` (Most Tabs): Refresh the context and reload all views.

## Workflow Examples

### Example 1: Understanding Your Context

1.  Run `cx view` to open the TUI.
2.  Start in the **TREE** tab to see a hierarchical overview of included directories (marked with `✓`).
3.  Press `tab` to navigate to the **STATS** tab. Review the "File Types" panel to see which languages or file types contribute most to the context size.
4.  Press `tab` again to go to the **LIST** tab for a complete, flat list of every included file.
5.  Finally, switch to the **RULES** tab to see the patterns in the active rules file that produced this context.

### Example 2: Optimizing Context Size

1.  Run `cx view` and navigate to the **STATS** tab.
2.  Identify a file type that is consuming a high percentage of tokens (e.g., `.go` files at 98.5%).
3.  Note the largest individual files in the "Largest Files" panel.
4.  Switch to the **LIST** tab, navigate to a large, unnecessary file, and press `x` to exclude it. The file will be immediately removed from the list.
5.  Alternatively, press `e` to open the rules file and add a broader exclusion pattern (e.g., `!**/*_test.go`).
6.  After saving the rules file, press `r` in the TUI to refresh and see the updated statistics.

### Example 3: Switching Contexts

1.  Run `cx view`.
2.  Press `s` to open the rule set selector, which lists all available sets from `.cx/` and `.cx.work/`.
3.  Use the arrow keys to select a different rule set (e.g., "backend-only") and press `enter`.
4.  The context regenerates, and all tabs update to reflect the new set of included files.
5.  Verify the change in the **TREE** tab by confirming that only backend-related directories are now included.

### Example 4: Visual Context Review

-   Use the **TREE** tab for a quick visual assessment. Included directories are marked with `✓` and show their total token contribution, such as `pkg ✓ (29.5k)`.
-   Immediately spot directories that are excluded by rules, marked with `🚫`, such as `tests 🚫`.
-   To understand why a directory is included or excluded, navigate to the **RULES** tab to review the patterns in effect.