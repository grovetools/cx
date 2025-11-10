# Context TUI Documentation

You are documenting the `cx view` terminal user interface for grove-context.

## Task
Create comprehensive documentation for the `cx view` command, the tabbed terminal user interface (TUI) for browsing and managing context.

## Topics to Cover

1. **Overview of cx view TUI**
   - Multi-tab terminal interface for comprehensive context management
   - Four main tabs: TREE, RULES, STATS, LIST
   - Real-time context inspection with token counts
   - Interactive rule editing and context switching

2. **Tab System**
   - **TREE Tab** - Hierarchical file browser
     - Expandable/collapsible directory tree
     - Visual indicators: ✓ (included), 🚫 (excluded)
     - Token counts shown inline at directory level
     - Navigate with arrow keys, expand/collapse folders
     - Shows (CWD) for current working directory

   - **RULES Tab** - Rules file viewer/editor
     - Displays active rules file path
     - Shows complete rules file content
     - Syntax highlighting for comments
     - Can see imported rules (commented out by default)
     - Press `e` to edit in your default editor

   - **STATS Tab** - Context analytics
     - File type distribution with percentages and token counts
     - "Largest Files" section showing top contributors
     - Visual breakdown of context composition
     - Helps identify what's consuming context space

   - **LIST Tab** - Flat file listing
     - Complete list of all included files
     - Shows full paths and individual token counts
     - Sortable view
     - Can exclude individual files with `x`

3. **Navigation Controls**
   - **Tab switching**: `tab` (next), `shift+tab` (previous)
   - **Vertical navigation**: arrow keys or j/k
   - **Tree expansion**: arrow keys or h/l to collapse/expand
   - **Sorting**: `t` to toggle sort order
   - **Help**: `?` to show/hide help
   - **Quit**: `q` or `ctrl+c`

4. **Context Management Actions**
   - **Edit rules** (`e`): Opens rules file in $EDITOR
   - **Select rule set** (`s`): Switch to different rule set from `.cx/`
   - **Exclude files** (`x`): Exclude specific files (in LIST view)
   - **Refresh** (`r`): Regenerate context and reload view

5. **Visual Indicators**
   - ✓ - Directory or file is included in context
   - 🚫 - Directory or file is excluded
   - Token counts: `(82.7k)` shown inline for directories and files
   - Percentages: `98.5%` in STATS tab showing proportion of total
   - (CWD): Marks current working directory in tree

## Key Features to Document

- **Tab-based interface** with four specialized views
- **TREE tab**: Visual file hierarchy with token counts
- **RULES tab**: View and edit active rules file
- **STATS tab**: Analytics on file types and largest contributors
- **LIST tab**: Detailed file listing with exclusion capability
- **Rule set switching** (`s` key) for quick context changes
- **Real-time token counts** at every level
- Color-coded status indicators (✓ included, 🚫 excluded)
- Keyboard-driven navigation throughout

## Keyboard Shortcuts Reference

**Global Navigation**
- `tab` / `shift+tab`: Switch between TREE, RULES, STATS, LIST tabs
- `↑/↓` or `j/k`: Navigate up/down in current view
- `?`: Toggle help display
- `q` / `ctrl+c`: Quit

**TREE Tab**
- `→` or `l`: Expand directory
- `←` or `h`: Collapse directory
- `t`: Toggle sort order (alphabetical vs token count)

**RULES Tab**
- `e`: Edit rules file in $EDITOR
- View-only display of active rules

**STATS Tab**
- `s` or `←/→`: Switch between file types and largest files views
- View-only display of analytics

**LIST Tab**
- `x`: Exclude selected file
- `t`: Toggle sort order
- `s` or `←/→`: Switch sort criteria

**Context Management (Available in Most Tabs)**
- `e`: Edit rules file
- `s`: Select/switch rule set
- `r`: Refresh and regenerate context

## Workflow Examples Required

**Example 1: Understanding Your Context**
1. Open `cx view`
2. Start in TREE tab to see directory structure
3. Navigate to STATS tab to see what's consuming tokens
4. Check LIST tab to see all included files
5. Review RULES tab to understand the patterns

**Example 2: Optimizing Context Size**
1. Open `cx view` and go to STATS tab
2. Identify file types consuming most tokens (.go files = 98.5%)
3. Note largest files in bottom panel
4. Switch to LIST tab and exclude specific large files with `x`
5. Or press `e` to edit rules and add exclusion patterns
6. Press `r` to refresh and see updated stats

**Example 3: Switching Contexts**
1. Open `cx view`
2. Press `s` to see available rule sets
3. Select "backend-only" for API work
4. Context regenerates automatically
5. Verify in TREE tab that only backend files are included

**Example 4: Visual Context Review**
- Use TREE tab to quickly see which directories are included (✓ marker)
- Token counts at directory level show contribution: `pkg ✓ (29.5k)`
- Spot excluded directories: `tests 🚫`
- Navigate to RULES tab to understand why patterns match

## Context
Focus on `cx view` as the comprehensive TUI (Terminal User Interface) for context management. Emphasize:
- The multi-tab design separates concerns (browsing, rules, analytics, details)
- Terminal-based interface with keyboard-driven interaction
- Real-time feedback with token counts throughout
- Integration with rule set switching for workflow flexibility
- Visual indicators make included/excluded status immediately clear

This replaces older single-view designs and provides a complete context management experience in the terminal.