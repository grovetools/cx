# Documentation Task: Project Overview

You are an expert technical writer. Write a clear, engaging overview for the `grove-context` (`cx`) tool.

## Task
Based on the provided codebase context, create an overview that:
- Explains that `cx` is a rule-based tool for managing file context for LLMs.
- Describes the problem it solves: concatenating many files into a single file to be submitted to LLM tools.
- Highlights key features using a bulleted list:
  - Dynamic context generation from a `.grove/rules` file. Like a gitignore, but for things on your filesystem that should be included in context.
  - Hot & Cold context separation -- for use with services that can cache tokens.
  - Interactive tools for understanding/managing context: `cx view`.
  - Git integration for context generation (`cx from-git`).
  - Snapshots for saving/loading different context configurations.
  - External repository management and auditing (`cx repo`).
- Briefly explains how it works at a high level (the `.grove/rules` file is the core).
- Identifies the target audience (developers working with LLMs who need to provide file-based context).

## Context Files to Read
- `README.md`
- `main.go`
