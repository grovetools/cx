# Context TUI Documentation

You are documenting the `cx view` terminal user interface for grove-context.

## Task
Create comprehensive documentation for the `cx view` command, the terminal user interface (TUI) for browsing context.

## Topics to Cover

1. **Overview of cx view TUI**
   - Terminal User Interface for context browsing
   - Visual exploration of context in the terminal
   - Real-time context inspection
   - File preview capabilities

2. **Interface Components**
   - File tree navigation pane
   - File preview pane
   - Status bar and indicators
   - Repository management tab

3. **Navigation Controls**
   - Movement keys (j/k for up/down, h/l for collapse/expand)
   - Jump commands (g for top, G for bottom)
   - Search with / key
   - Tab to switch between views

4. **Context Modification**
   - Hot/cold markers (if experimental features enabled)
   - Exclude files with x
   - Refresh view with r
   - Save changes

5. **File Status Indicators**
   - Included vs excluded files
   - Git status integration
   - Size indicators
   - Pattern match highlighting

## Key Features to Document

- File tree with expandable directories
- Color-coded status indicators
- Vim-style navigation keys
- Real-time preview of selected files
- Quick exclude/include toggles
- Search functionality with highlighting
- Repository management view (Tab key)

## Keyboard Shortcuts Reference
Document all keybindings:
- j/k: Move up/down
- h/l: Collapse/expand directories
- g/G: Jump to top/bottom
- /: Search
- x: Toggle exclude
- r: Refresh
- q/Esc: Quit
- Tab: Switch views
- Enter: Open file preview

## Examples Required
- Basic navigation workflow
- Searching for specific files
- Excluding files interactively
- Using preview pane effectively

## Context
Focus on cx view as the primary TUI (Terminal User Interface) tool for understanding and modifying context. Emphasize the terminal-based interface and keyboard-driven interaction. Note that cx stats and cx dashboard are covered in other sections (Context Generation for stats, Experimental for dashboard).