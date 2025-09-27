# Getting Started with grove-context (cx)

This quick-start walks through the typical workflow for assembling and using a file-based context.

Prerequisites:
- Install via the Grove meta-CLI:
  ```bash
  grove install context
  ```
- Run the commands below from your project’s root (ideally a Git repository).

---

## 1) Initialize Rules

Create or open the rules file in your editor:

```bash
cx edit
```

- This opens .grove/rules (creating it if needed).
- If you’re in a Git repo, the editor launches with the working directory set to the repo root.

---

## 2) Define Rules

Add a simple set of patterns to include all Go files and exclude test files:

```
# Hot context: files you'll actively work with
**/*.go
!**/*_test.go
```

Notes:
- Patterns use .gitignore-style syntax.
- You can add a cold section with a separator (---) later if needed for stable reference files.

Example with a cold section:

```
# Hot
**/*.go
!**/*_test.go

---
# Cold (optional): stable or background files
docs/**/*.md
```

Save the file and exit your editor.

---

## 3) Visualize and Refine

Launch the interactive view to verify what your rules include and exclude:

```bash
cx view
```

What you’ll see:
- A file tree with inclusion status (hot, cold, excluded, git-ignored, omitted).
- Basic keys:
  - Up/Down or j/k to move
  - Enter/Space to expand/collapse folders
  - h to toggle hot
  - c to toggle cold
  - x to toggle exclude
  - r to refresh
  - ? for a help overlay
  - q to quit

Use this to confirm your rules behave as expected and make quick adjustments.

---

## 4) Generate Context

Build the concatenated context files:

```bash
cx generate
```

This produces:
- .grove/context — hot context
- .grove/cached-context — cold context (only if you defined a cold section)

The files are written with an XML-style wrapper. Example snippet:

```xml
<?xml version="1.0" encoding="UTF-8"?>
<context>
  <hot-context files="2" description="Files to be used for reference/background context to carry out the user's question/task to be provided later">
    <file path="main.go">
      package main
      // ...
    </file>
    <file path="internal/handler.go">
      package internal
      // ...
    </file>
  </hot-context>
</context>
```

---

## 5) Use the Context

Print the hot context to stdout:

```bash
cx show
```

Copy to clipboard:

- macOS:
  ```bash
  cx show | pbcopy
  ```
- Linux (xclip):
  ```bash
  cx show | xclip -selection clipboard
  ```
  or with xsel:
  ```bash
  cx show | xsel --clipboard --input
  ```
- Windows (PowerShell):
  ```powershell
  cx show | clip
  ```

You can now paste the full context into your LLM tool.

---

## Tips

- Iterate quickly: run cx view to confirm, then cx generate to produce the final files.
- If you see a warning about missing rules, create .grove/rules with patterns and rerun.
- To inspect what’s included without generating context, you can list resolved files:
  ```bash
  cx list
  ```
- If you define a cold section, you can list cold files:
  ```bash
  cx list-cache
  ```