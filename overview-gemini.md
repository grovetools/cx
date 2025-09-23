`grove-context` (`cx`) is a rule-based command-line tool designed for managing file-based context for Large Language Models (LLMs). It addresses the common task of collecting and concatenating files from a project into a single, structured format suitable for submission to an LLM. This automates a manual and error-prone process, providing a repeatable and version-controlled workflow.

The tool is intended for developers who work with LLMs and need to provide substantial, file-based context for tasks such as code generation, analysis, or documentation.

### Key Features

-   **Dynamic Context Generation:** Define which files to include or exclude in a `.grove/rules` file using a syntax similar to `.gitignore`. The context is generated dynamically based on these rules.
-   **Hot & Cold Context Separation:** Divide the context into a "hot" section for actively changing files and a "cold" section for stable dependencies. This separation can optimize interactions with services that support token caching.
-   **Interactive Tools:** The `cx view` command provides an interactive terminal interface to visualize the file tree, showing which files are included, excluded, or ignored based on the current rules.
-   **Git Integration:** Automatically generate context rules from Git history using the `cx from-git` command, with options to include staged files, files from recent commits, or changes between branches.
-   **Snapshots:** Save and load different context configurations using `cx save` and `cx load`. This allows for switching between different rule sets for various tasks, such as feature development or bug fixing.
-   **External Repository Management:** Include files from external Git repositories directly in the rules file. The `cx repo` subcommand provides tools to clone, manage, and perform security audits on these dependencies.

### How It Works

The core of `grove-context` is the `.grove/rules` file, which serves as the single source of truth for context generation. This text file contains patterns that specify which files and directories to include or exclude. Commands like `cx generate` and `cx show` read these rules, resolve the corresponding files on the filesystem, and produce the final context output. This approach ensures that the context is always up-to-date with the project's state and the defined rules.