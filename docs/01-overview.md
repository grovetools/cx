# grove-context (cx)

## Introduction

`grove-context` (`cx`) is a CLI tool for assembling multi-repository context for LLMs. It supports a planning → execution workflow: define what files, repos, and content are relevant to a feature, generate an implementation plan with a large-context LLM (200k-2M+ tokens), then execute that plan with smaller, focused contexts.

The typical approach today is letting agents discover context on their own - either in "plan mode" or directly during implementation, where the agent greps and inspects the codebase from scratch each time. Many editors and CLI-based agents allow referencing context, but this is ad-hoc: context references scattered across chat history, not reproducible across runs, not shareable with team members. Agent-driven discovery is inefficient (wastes tokens searching), incomplete (misses context in other repos), and lacks architectural understanding that developers have.

Grove-context inverts this model. The developer curates exactly what context the feature needs upfront. Think of it as defining a funnel: all code and repos are available, but you're specifying the precise scope where the LLM should focus. The key is including slightly more context than an agent would discover on its own - adjacent modules, related components, relevant documentation. This extra context improves plan quality measurably. Plans can include specific code snippets, architectural patterns, and implementation guidance. The approach is also faster: plans are generated quickly because the LLM has everything upfront (no iterative discovery), and implementation is faster because the plan is more complete (less back-and-forth to figure out what was missed). Plans generated from developer-curated context improve agent implementation success rates. When an agent receives a plan with the full picture (relevant files, architectural context, dependencies, tasks), it executes with fewer mistakes. Agents working without comprehensive plans miss details, leading to incomplete implementations and post-hoc cleanup. The cost per planning request is higher (you're routinely sending 100k+ tokens in single API requests), but total development cost is lower through better plans and fewer implementation iterations. Once you have the plan, execution happens with smaller, focused contexts (via `grove-flow`).

## Workflow

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
│   LLM Planning      │  Large context LLM gets full picture
│  (Comprehensive)    │  Returns detailed, informed implementation plan
└──────────┬──────────┘
           │
           ▼
┌─────────────────────┐
│   Agent Execution   │  Agent-based tool (Claude Code, Codex, etc.)
│                     │  carries out the plan
└─────────────────────┘
```

Context is defined in a `.grove/rules` file using gitignore-style patterns. These rules files support an architectural pattern of keeping repositories small and focused (which helps agent and LLM performance per repo) while composing comprehensive context across repos for planning. You can cross-reference repositories: a feature in your API server can include frontend code (`@a:web-app/src/**/*.tsx`), shared libraries (`@a:common/types/**`), and documentation. Pre-defined rule sets let you assemble contexts for different features (backend-only, full-stack, etc.). Repository locations are abstracted through workspace aliases - `@a:api-server` works for your team regardless of where developers cloned repos, eliminating hardcoded paths and enabling team-scale context management across microservices architectures. 

Example rules file demonstrating context selection syntax:

```groverules
# Relevant code for current repository
*.go                                                                      ~164.0k tokens (69 files) 
Makefile                                                                  ~1.6k tokens
go.mod                                                                    ~869 tokens
grove.yml                                                                 ~136 tokens

# Exclude test files
!tests                                                                    -43 files, -105.6k tokens
!*_test.go                                                                -21 files, -38.3k tokens

# Pull in ruleset from other repo
@a:grove-ecosystem:grove-context::dev-no-tests                            ~97.1k tokens (54 files)

# Directory in another repository
@a:grove-ecosystem:grove-gemini/docs                                      ~25.4k tokens (11 files) 

# Match all files across ecosystem of repos with "config" in filepath
@a:grove-ecosystem @find: "config"                                        ~353.6k tokens (225 files) 

# Match all files in `grove-core` repository it w/ "tui" in filepath
@a:grove-ecosystem:grove-core @grep: "tui"                                ~21.7k tokens (15 files)

# View allows selecting specific dirs/files in TUI/neovim
@view:@a:grove-ecosystem:grove-core::dev-no-tests
@a:grove-ecosystem:grove-core/config/types.go                             ~2.0k tokens
@a:grove-ecosystem:grove-core/errors/types.go                             ~837 tokens
@a:grove-ecosystem:grove-core/pkg/tmux/types.go                           ~63 tokens
@a:grove-ecosystem:grove-core/pkg/workspace/types.go                      ~3.6k tokens
@a:grove-ecosystem:grove-core/config/config.go                            ~3.9k tokens

# Add a github repo to context
@a:git:charmbracelet/bubbletea/**/*.go                                    [not_audited | ffa0502] 
                                                                          ~65.3k tokens (76 files)
```

A common pattern is multi-turn planning: start with core context to scope foundational aspects, then add more context in subsequent turns. For example, begin with the API layer to establish data flow, add frontend components once API contracts are clear, then pull in documentation for edge cases. This iterative refinement within a single conversation produces more refined plans. The context definitions are persistent (defined in `.grove/rules` files), so this process is reproducible and shareable.

Grove-context acts as the foundational context engine for this workflow across the Grove ecosystem.


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
