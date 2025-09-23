# Documentation Task: Interactive Tools

Create documentation for the interactive TUI tools provided by `cx`. These are key features that differentiate the tool.

## Task
Create two subsections:

### Visualizing Context with `cx view`
- Describe the interactive file tree and the repository management view.
- Explain the meaning of the status symbols and colors for files (Hot, Cold, Excluded, Git-ignored, Omitted).
- List the primary keybindings for navigation (`j/k`, `g/G`, etc.) and for modifying the context (`h` for hot, `c` for cold, `x` for exclude).
- Mention the repository management view accessible via `Tab`.

### Monitoring with `cx dashboard`
- Describe the live-updating terminal dashboard.
- Explain the statistics it shows for both hot and cold contexts.
- Mention that it automatically updates when files in the project change.

## Context Files to Read
- `cmd/view.go`
- `cmd/dashboard.go`
- `README.md`