# grove-context (cx)

`grove-context` (`cx`) is a command-line tool for assembling comprehensive, multi-repository context for Large Language Models (LLMs). It enables a **planning → execution** workflow where you define the complete "universe" of relevant files, repos, and content for a feature, then use large-context LLMs (200k-2M+ tokens) to generate detailed implementation plans.

**Why large contexts for planning?** The typical approach today is to let agents discover context on their own - either using them in "plan mode" or proceeding directly to implementation, where the agent must grep and inspect the codebase from scratch every time. Many editors and CLI-based agents allow referencing context, but this process is ad-hoc, not systematic: context references are scattered across chat history, not reproducible across runs, and impossible to share with team members. This agent-driven discovery is inefficient and incomplete: agents spend tokens searching, miss relevant context in other repositories, and lack the architectural understanding a developer has. Grove-context inverts this: the developer surgically defines exactly what context the feature needs - illuminating the precise scope where the LLM needs to think. Think of it as a funnel: all code and repos are available, but you're curating the exact universe this specific feature touches. Crucially, adding just a bit more context than the agent would discover on its own - adjacent modules, related components, relevant documentation - gives the LLM that extra understanding that leads to dramatically higher quality plans. This approach is also significantly faster: high-quality plans are generated quickly at the planning stage because the LLM has everything it needs upfront (no iterative context discovery), and because the plan is comprehensive and accurate, agent implementation is often fast as well - no back-and-forth to figure out what was missed. With large context windows (Gemini 2M, Claude 200k+), these plans can even include specific code snippets, architectural patterns, and implementation guidance that agents can directly incorporate.

An effective pattern is multi-turn planning conversations: start with core context to scope out foundational aspects of the feature, then progressively add additional context in subsequent turns as the plan develops. For example, begin with just the API layer to establish the data flow, then add frontend components once the API contracts are clear, then pull in documentation for edge cases. This iterative refinement - all within a single conversation with persistent context - produces highly polished plans without starting from scratch each time. This surgical, developer-curated approach produces plans that just work.

**The agent success factor:** Plans generated from developer-curated, comprehensive context dramatically improve the success rate of agents implementing those plans. When an agent receives a plan that includes the big picture (relevant files, architectural context, dependencies, todo lists), it can execute with confidence. In contrast, agents attempting to figure out features without these context-rich plans tend to miss crucial details, resulting in significant churn, incomplete implementations, and post-hoc cleanup work. Creating multi-step development workflows - where planning happens with large context windows followed by focused execution - results in high-fidelity implementations and dramatically reduced need for downstream user intervention.

**Small repos, large contexts:** Grove-context enables an optimal architecture pattern: keep individual repositories small and focused (which improves agent and LLM performance within each repo), but easily compose comprehensive context across multiple repos for planning. Rules files make it trivial to cross-reference repositories - a feature in your API server can pull in relevant frontend code (`@a:web-app/src/**/*.tsx`), shared libraries (`@a:common/types/**`), and documentation. Pre-defined rule sets let you quickly assemble contexts for different features (backend-heavy, full-stack, etc.). Crucially, repository locations are abstracted through workspace aliases - `@a:api-server` works for your entire team regardless of where each developer has cloned the repo. This eliminates brittle hardcoded paths and enables true team-scale context management across a microservices architecture.

The cost is higher per planning request, but you're paying for comprehensive understanding that reduces total development cost through better plans and fewer iterations. Once you have the plan, execution happens with smaller, focused contexts (via grove-flow).

Grove-context acts as the foundational context engine for this workflow across the Grove ecosystem.

```
┌─────────────────────┐
│   Define Universe   │  .grove/rules with patterns, aliases, imports
│                     │  (backend code, frontend, docs, related repos)
└──────────┬──────────┘
           │
           ▼
┌─────────────────────┐
│  Generate Context   │  grove-context assembles 100k+ token context
│  (@a:alias → paths) │  spanning multiple repos and worktrees
└──────────┬──────────┘
           │
           ▼
┌─────────────────────┐
│   LLM Planning      │  Gemini 2M / Claude 200k gets full picture
│  (Comprehensive)    │  Returns detailed, informed implementation plan
└──────────┬──────────┘
           │
           ▼
┌─────────────────────┐
│ Focused Execution   │  grove-flow breaks down into steps with
│   (grove-flow)      │  smaller, targeted contexts per task
└─────────────────────┘
```

<!-- placeholder for animated gif -->

## Key Features

-   **Declarative Universe Definition:** Define the complete "universe" of relevant content with `.grove/rules` using gitignore-style patterns, workspace aliases, and imports. For a feature spanning multiple repos, declare everything relevant: backend code (`src/api/**/*.go`), frontend (`@a:web-app/src/**/*.tsx`), shared libraries (`@a:common/types/**`), and documentation.

-   **Workspace-Aware Aliasing:** Reference files across projects with `@a:project-name/path/**/*.ext` - works for your whole team, across machines, in any worktree. Powered by `grove-core`'s workspace discovery, aliases resolve differently based on context (siblings in same ecosystem prioritized). No hardcoded paths.

-   **Reusable Rule Sets & Context Switching:** Create specialized rule sets in `.cx/` for different workflows (`backend-only.rules`, `frontend-only.rules`, `docs-only.rules`). Switch instantly with `cx set-rules backend-only`. Import rule sets across projects with `@a:standards-repo::backend-patterns` to share context definitions across teams.

-   **Multi-Tab Interactive TUI:** Explore and manage context with `cx view`. **TREE tab** shows hierarchical file browser with token counts at every level. **RULES tab** displays and edits active rules. **STATS tab** shows file type distribution and largest contributors. **LIST tab** provides detailed file listing with exclusion capability. Press `s` to switch rule sets, `e` to edit, `r` to refresh.

-   **Token & Cost Analytics:** See exactly what you're sending before API costs hit. `cx stats` shows token counts by file type, language distribution, and largest files. `cx view` displays per-directory token counts. Optimize your 100k+ token contexts before sending to paid APIs.

-   **Security Boundaries:** File inclusion restricted to discovered Grove workspaces and `~/.grove/` directory. Workspace discovery acts as a security boundary, preventing accidental inclusion of system files. Review with `cx list` before sharing context.

-   **Multi-Repo Composition:** Combine context from local repos (`../api/**`), workspace aliases (`@a:backend/**/*.go`), and external Git repos (`cx repo add`). Build comprehensive context spanning 5+ related repositories - essential for modern microservices architectures.

## How It Works

`grove-context` follows a deterministic pipeline to resolve a final list of files:

1.  **Load Rules:** Reads the active rules file - either `.grove/rules` or a named set from `.cx/` (managed via `cx set-rules`).

2.  **Expand Directives:** Recursively expands import directives:
    - `@default` imports project defaults from `grove.yml`
    - `@a:project::ruleset` imports named rule sets from other projects
    - Enables composable, shareable context definitions

3.  **Resolve Aliases:** Transforms workspace aliases to absolute paths using `grove-core`'s discovery:
    - `@a:api-server/src/**/*.go` → `/absolute/path/to/api-server/src/**/*.go`
    - Context-aware: siblings in same ecosystem/worktree prioritized
    - Works consistently across team members and environments

4.  **Filter by Gitignore:** Walks file trees, automatically excluding files matched by `.gitignore` (unless explicitly tracked by git).

5.  **Apply Patterns:** Processes inclusion/exclusion patterns with "last match wins" logic, just like `.gitignore`. Binary files excluded automatically.

6.  **Generate Context:** Concatenates final file list into `.grove/context` with XML structure, ready for LLM consumption. Includes token counts and metadata.

## Ecosystem Integration

`grove-context` serves as foundational infrastructure that enables the planning → execution workflow across Grove tools.

**Planning Phase (Large Contexts):**
-   **`grove-gemini`**: The `gemapi request` command automatically calls `grove-context` to generate comprehensive context from `.grove/rules`. No manual file management - just define your universe and make API calls. Supports large-context planning with Gemini 2M tokens.
-   **`grove-nvim`**: Real-time editor feedback while crafting rule sets. Shows inline token counts as virtual text, interactive rule preview with `<leader>f?` to see matched files, and smart `gf` navigation across workspace aliases. Edit rules → see token changes instantly.

**Execution Phase (Focused Contexts):**
-   **`grove-flow`**: Orchestrates implementation with per-job contexts. Each job can specify `rules_file: .cx/backend-only.rules` in frontmatter - `grove-context` regenerates focused context before execution. Enables breakdown of large plans into targeted tasks, each with precisely the context they need.

**Foundation:**
-   **`grove-core`**: Provides the workspace discovery engine that powers `grove-context`'s alias resolution. Discovers projects, ecosystems, and worktrees across your filesystem, enabling `@a:project` syntax to work consistently for teams.

## Installation

Install via the Grove meta-CLI:
```bash
grove install context
```

Verify installation:
```bash
cx version
```

Requires the `grove` meta-CLI. See the [Grove Installation Guide](https://github.com/mattsolo1/grove-meta/blob/main/docs/02-installation.md) if you don't have it installed.
