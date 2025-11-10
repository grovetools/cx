# Editor Integration Documentation

You are documenting editor integrations for grove-context, primarily focusing on Neovim support.

## Task
Create comprehensive documentation for how grove-context integrates with code editors, with deep focus on the Neovim plugin.

## Topics to Cover

1. **Overview of Editor Integration**
   - Benefits of editor integration for context management
   - Real-time feedback while editing rules
   - Seamless workflow between editing and context generation

2. **Neovim Plugin (grove-nvim)**
   - Installation and setup
   - Custom filetype detection for `.grove/rules`
   - Comment support and syntax highlighting

3. **Real-time Virtual Text**
   - Token counts displayed inline while editing
   - File counts for each pattern
   - Warning indicators for patterns with no matches
   - Exclusion info (`-3 files, -500 tokens`)
   - Git repository status for external repos
   - How to enable/disable virtual text

4. **Interactive Rule Preview**
   - `<leader>f?` keybinding to preview matched files
   - Shows resolved files in a picker/selector
   - Works with aliases and patterns
   - Support for multi-file selection to add as explicit rules
   - Instant feedback on what a pattern will match

5. **Smart Navigation**
   - `gf` (go-to-file) with full alias resolution
   - Resolves `@a:ecosystem:repo/path` syntax
   - Falls back to picker if pattern matches multiple files
   - Navigate across workspace boundaries

6. **Interactive Commands**
   - `:GroveContextView` - Opens `cx view` TUI in floating terminal
   - `:GroveRules` - Opens `cx rules` interactive picker
   - Integration with flow plan context commands

7. **Alias Resolution in Editor**
   - How grove-nvim calls `cx workspace list --json`
   - Parsing workspace aliases for navigation
   - Context-aware resolution within editor
   - Making relative paths when adding files to rules

8. **Workflow Examples**
   - Editing `.grove/rules` with live feedback
   - Using `<leader>f?` to verify patterns before saving
   - Navigating to aliased files with `gf`
   - Opening the full TUI with `:GroveContextView`

9. **Configuration**
   - Setting up keybindings
   - Customizing virtual text appearance
   - Configuring update frequency
   - Integration with other Neovim plugins

10. **VS Code and Other Editors**
    - Current state of support for other editors
    - Planned integrations
    - How to use `cx edit` as a basic integration

## Examples Required
- Complete workflow: Open `.grove/rules` → see virtual text → preview pattern → navigate to file → add to rules
- Screenshot placeholders showing virtual text in action
- Keybinding configuration examples
- Integration with Neovim file pickers (Telescope, fzf, etc.)

## Notes
- Focus on practical workflows that improve developer experience
- Emphasize the tight integration loop: edit → preview → verify → save
- Show how editor integration reduces context switching
- Demonstrate the value of real-time feedback
