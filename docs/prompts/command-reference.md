# Documentation Task: Command Reference

Create a comprehensive command reference for `cx`. This should be a detailed, practical guide to every available command.

## Task
Generate a section for each command. Organize them into logical groups (e.g., Core Workflow, Interactive Tools, Analysis, Snapshots).

For each command, provide:
1.  **Name and Usage:** e.g., `cx save <name>`
2.  **Description:** A clear explanation of what the command does (use the `Short` and `Long` descriptions from the cobra command definition).
3.  **Arguments:** Describe any required arguments.
4.  **Flags:** List all available flags with their descriptions.
5.  **Example:** A practical example of how to use the command.

## Context Files to Read
- `main.go` (for the list of all commands)
- All files in the `cmd/` directory to get details for each command.