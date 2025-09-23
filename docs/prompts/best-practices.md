# Documentation Task: Best Practices

Document recommended best practices for using `grove-context` effectively.

## Task
Based on the project's design, provide a list of best practices and tips. Include:
- **Rule Organization:** Start with broad inclusion patterns (`**/*`) at the top, followed by more specific exclusion patterns (`!`).
- **Context Size Management:** Advise users to regularly run `cx stats` to monitor token counts and identify large files.
- **Hot vs. Cold Context:** Give guidance on what kinds of files belong in each context (e.g., active code in hot, stable dependencies in cold).
- **Snapshots:** Encourage using snapshots to save and version control important context configurations for different tasks.
- **Version Control:** Recommend adding `.grove/` to `.gitignore`, but consider checking in specific, important snapshots from `.grove/context-snapshots/`.

## Context Files to Read
- `README.md`
- `cmd/stats.go`
- `.gitignore`