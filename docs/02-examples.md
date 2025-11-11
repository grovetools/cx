Here are five practical examples for using Grove Context (`cx`), following the provided documentation style guide.

### Example 1: Quick Start - Pattern-Based Context

Grove Context (`cx`) generates a context file (`.grove/context`) by including files that match patterns defined in a rules file. By default, it reads rules from `.grove/rules`.

1.  **Create a rules file**: Start by creating a `.grove/rules` file. You can do this manually or by running `cx edit`. In Neovim, the keybinding `<leader>fe` opens the active rules file.

    ```
    # .grove/rules
    
    # Include all Go files
    *.go
    
    # Exclude test files
    !*_test.go
    
    # Include all Markdown files in the docs/ directory
    docs/**/*.md
    ```

2.  **Verify the context**: Run `cx list` to see a list of absolute file paths included in the context.

    ```bash
    $ cx list
    /path/to/project/main.go
    /path/to/project/internal/server.go
    /path/to/project/docs/guide.md
    ```

3.  **Analyze context statistics**: Use `cx stats` to see a breakdown of the context by file type, token count, and size. This helps identify which files contribute most to the context size. In Neovim, statistics for each rule are displayed as virtual text next to the rule itself, updating in real-time as you edit.

4.  **Explore visually with the TUI**: Run `cx view` to launch an interactive terminal interface.
    *   **TREE**: A file explorer view showing which files are included (hot/cold), excluded, or omitted.
    *   **RULES**: Displays the content of the active rules file. Press `e` to edit it.
    *   **STATS**: A detailed breakdown of token and file counts by language.
    *   **LIST**: A flat list of all included files, sortable by name or token count.

The tool automatically excludes binary files, so there is no need to add patterns like `!*.bin` or `!*.png`.

### Example 2: Reusable Rule Sets for Context Switching

For projects with multiple components (e.g., backend, frontend, docs), you can create named rule sets in the `.cx/` directory to switch contexts quickly.

1.  **Create named rule sets**: Define different contexts in separate files within the `.cx/` directory.

    ```
    # .cx/backend.rules
    # Context for backend development
    pkg/api/**/*.go
    !pkg/api/**/*_test.go
    cmd/server/*.go
    ```

    ```
    # .cx/frontend.rules
    # Context for frontend development
    ui/src/**/*.ts
    ui/src/**/*.tsx
    ```

2.  **Switch between contexts**: Use `cx rules set <name>` to activate a rule set. The active set is stored in project state (`.grove/state`) and used by all subsequent `cx` commands.

    ```bash
    # Work on the backend
    $ cx rules set backend
    Active context rules set to 'backend'
    
    # Work on the frontend
    $ cx rules set frontend
    Active context rules set to 'frontend'
    ```

3.  **Import rule sets**: You can import rule sets from other projects using the `@a:project::ruleset` syntax. This allows for composition of context from multiple sources.

    ```
    # .cx/full-stack.rules
    
    # Import the backend rules from the 'api-server' project
    @a:api-server::backend
    
    # Also include local frontend files
    ui/src/**/*.ts
    ```

For temporary or personal rule sets that should not be committed to version control, use the `.cx.work/` directory instead of `.cx/`.

### Example 3: Working with Aliases and Workspaces

Aliases are shortcuts that resolve to the absolute path of a Grove workspace (a project or ecosystem). This is useful in multi-repository setups.

1.  **List available workspaces**: Run `cx workspace list` (or `grove ws list`) to see all discovered projects and their unique identifiers, which function as aliases.

2.  **Use aliases in rules**: Reference projects directly without needing relative paths. Grove Context provides context-aware resolution, prioritizing sibling projects within the same ecosystem.

    ```
    # .grove/rules
    
    # Include the main package from the grove-nvim project
    @a:grove-nvim/main.go
    
    # Include all Go files from the grove-core project within the grove-ecosystem
    @a:grove-ecosystem:grove-core/**/*.go
    
    # Include files from a specific worktree of a project
    @a:grove-flow:my-feature-branch/pkg/orchestration/*.go
    ```

3.  **Preview alias matches**: In Neovim, place your cursor over a rule containing an alias and press `<leader>f?` to open a file picker showing all files matched by that rule. This allows for quick verification of complex patterns.

### Example 4: Grove-Flow Integration

Grove Flow, the job orchestrator, uses `grove-context` to automatically prepare the context for each job it runs.

1.  **Per-job context**: You can specify a custom rules file for a particular job in its frontmatter. This is useful for tasks that require a very specific subset of the codebase.

    ```yaml
    # 02-refactor-api.md
    ---
    id: refactor-api-job
    title: "Refactor API"
    type: agent
    rules_file: .cx/backend.rules # This job will use the backend-only context
    ---
    Refactor the API endpoints in `pkg/api/` to use the new service layer.
    ```

2.  **Automatic context generation**: Before executing a job, `grove-flow` regenerates the context based on the active or job-specific rules. This ensures the LLM always has the most up-to-date view of the relevant files.

3.  **Interactive context creation**: If `grove-flow` runs a job in a worktree where no `.grove/rules` file exists, it will prompt you interactively to create one, edit it, or proceed without context. This prevents jobs from running with an empty context by mistake.

### Example 5: Managing Complex Projects

`cx` provides tools for understanding and refining context in large or unfamiliar codebases.

1.  **Load a base rule set**: Start by loading a shared rule set as your working copy with `cx rules load <name>`. This copies the file to `.grove/rules` so your changes don't affect the original.

2.  **Use the TUI for refinement**: Run `cx view` to analyze the context.
    *   Navigate the **TREE** tab to see the file structure and identify directories with high token counts.
    *   Switch to the **STATS** tab to see a breakdown by language and identify the largest files.
    *   Open the **RULES** tab to see the active rules and press `e` to edit them directly.
    *   Use the **LIST** tab to find a specific file and press `x` to add an exclusion rule for it.
    *   Press `s` to open an interactive selector to switch to a different named rule set.

3.  **Include local or external repositories**:
    *   **Local**: Add relative paths to your rules file (e.g., `../shared-library/**`).
    *   **External**: For third-party repositories, first run `cx repo audit <git-url>` to perform a security review. Once audited, you can add the Git URL directly to your rules file (e.g., `git@github.com:user/repo.git`), and `cx` will use the locally cloned, audited version.

4.  **Use Git for context**: Quickly generate context for a code review by using Git history.
    *   `cx from-git --staged`: Includes all currently staged files.
    *   `cx from-git --commits 1`: Includes all files changed in the last commit.

5.  **Reset to defaults**: If your rules file becomes too complex, run `cx reset` to revert it to the default specified in your project's `grove.yml` or to a basic boilerplate.