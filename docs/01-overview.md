`cx` is a command-line tool for defining and managing file-based context for Large Language Models (LLMs). It uses `.gitignore`-style rules to assemble a precise set of files from local and remotely-cloned sources, which can then be analyzed, visualized, or provided to an LLM.

## Use Cases

Many agents have a "Plan Mode" for designing changes. This process involves the agent searching for files, understanding the codebase, assembling its own context, and then formulating a plan. This can be time-consuming and may require iterative prompting to guide the agent to the correct files.

The `cx` tool facilitates a different approach. It enables a developer to curate a large, precise set of files from across a codebase and provide it directly to an LLM API along with a prompt. This often produces higher-quality plans in a fraction of the time compared to an agent's self-directed planning mode. The developer can have a rapid, back-and-forth chat with the model, with the exact required context present at every turn.

After a high-quality plan is developed, it can be passed to a coding agent for implementation. The agent's role shifts from discovery and planning to the execution of a detailed, pre-vetted plan.

## Key Features

*   **Rules-Based Context**: Defines context using `.gitignore`-style glob patterns in simple text files.
*   **Interactive Viewer**: An integrated Terminal UI (`cx view`) for visualizing, analyzing, and navigating the file tree and its context status.
*   **Cross-Repository Context**: Includes files from other local projects via aliases or from remote Git repositories by URL, tag, branch, or commit.
*   **Context Analysis**: Provides detailed statistics (`cx stats`), comparisons between rule sets (`cx diff`), and validation (`cx validate`).
*   **Reusable Rulesets**: Saves and manages named rule sets (`cx rules`) that can be shared, version-controlled, and imported across projects.
*   **Editor Integration**: Supports editor plugins like `grove.nvim` with features for alias completion and per-rule token analysis.

