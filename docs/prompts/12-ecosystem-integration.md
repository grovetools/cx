# Ecosystem Integration Documentation

You are documenting how grove-context integrates with other Grove ecosystem tools.

## Task
Create comprehensive documentation explaining grove-context's role as the foundational context engine for the Grove ecosystem.

## Topics to Cover

1. **Architecture Overview**
   - grove-context as the central context engine
   - How other tools consume context
   - The workspace data model from grove-core
   - Shared configuration and state management

2. **grove-core Integration: Workspace Discovery**
   - How grove-context uses `grove-core/pkg/workspace` for discovery
   - The WorkspaceNode data model
   - Workspace types: StandaloneProject, EcosystemRoot, EcosystemSubProject, Worktrees
   - Discovery process and search paths
   - Provider pattern for fast lookups

3. **grove-core Integration: Alias Resolution**
   - The AliasResolver system
   - How aliases map to absolute paths
   - Context-aware resolution (siblings prioritized)
   - Resolution priority: direct children → siblings → top-level → shallower nodes
   - Examples of alias patterns:
     - `@a:project-name` - simple project reference
     - `@a:ecosystem:subproject` - ecosystem navigation
     - `@a:project:worktree` - worktree references
   - Using `cx workspace list` to explore available aliases

4. **grove-gemini Integration: Automatic Context Generation**
   - How `gemapi request` calls grove-context
   - Automatic context generation workflow:
     1. Check for `.grove/rules`
     2. Call `grovecontext.NewManager(workDir)`
     3. `UpdateFromRules()` and `GenerateContext()`
     4. Output `.grove/context` for inclusion in API requests
   - Seamless integration - no manual context management
   - Token usage tracking and reporting

5. **grove-flow Integration: Job-Level Context Management**
   - Automatic context regeneration before oneshot and chat jobs
   - The regeneration workflow:
     1. Prepare worktree (if specified)
     2. Create `grovecontext.NewManager(worktreePath)`
     3. Check for job-specific rules file
     4. Generate context with stats display
   - Custom rules files per job:
     - Frontmatter: `rules_file: .cx/backend-only.rules`
     - Resolution order: plan dir → cwd → git root → absolute
   - Worktree-scoped context (isolation)
   - Interactive context setup when `.grove/rules` missing
   - Integration with chat jobs
   - Context in conversational workflows

6. **grove-nvim Integration: Editor Experience**
   - Real-time virtual text feedback
   - Interactive rule preview (`<leader>f?`)
   - Smart navigation with `gf` and alias resolution
   - Commands: `:GroveContextView`, `:GroveRules`
   - Alias resolution using `cx workspace list --json`

7. **Integration Examples**

   **Example 1: grove-gemini automatic context**
   ```bash
   # .grove/rules
   echo "src/**/*.go" > .grove/rules
   echo "docs/**/*.md" >> .grove/rules
   echo "README.md" >> .grove/rules

   # Context automatically included in request
   gemapi request -p "Explain the architecture"
   # → Automatically generates .grove/context
   # → Includes in Gemini API request
   # → No manual context management needed
   ```

   **Example 2: grove-flow with custom context**
   ```markdown
   ---
   id: job-auth
   title: Implement authentication
   type: oneshot
   worktree: auth-feature
   rules_file: .cx/auth-only.rules
   ---

   Implement authentication module.
   ```

   Flow execution:
   - Creates worktree at `.grove-worktrees/auth-feature/`
   - Loads custom rules from `.cx/auth-only.rules`
   - Generates context in worktree
   - Passes to LLM with worktree as working directory

   **Example 3: Multi-project context with aliases**
   ```
   # .grove/rules
   # Frontend context
   @a:web-app/src/**/*.tsx
   @a:web-app/src/**/*.css

   # Backend API
   @a:api-server/src/**/*.go
   !@a:api-server/src/**/*_test.go

   # Shared types
   @a:shared-types/types/**/*.ts
   ```

   **Example 4: Rule set imports for team consistency**
   ```bash
   # Create a standards repo with shared rule sets
   # standards-repo/.cx/backend-best.rules
   src/api/**/*.go
   src/services/**/*.go
   src/models/**/*.go
   !**/*_test.go

   # Other projects import these standards
   # api-server/.grove/rules
   @a:standards-repo::backend-best

   # Add project-specific patterns
   internal/auth/**/*.go

   # Team gets consistent backend context across all projects
   ```

8. **Best Practices for Ecosystem Integration**
   - **Rule set organization for teams**:
     - Create shared rule sets in `.cx/` (backend-only, frontend-only, docs-only)
     - Import common patterns: `@a:standards-repo::backend-patterns`
     - Personal variants in `.cx.work/` for individual preferences
   - **Context switching workflows**:
     - Use `cx set-rules backend-only` for API work
     - Use `cx set-rules docs-only` for documentation generation (docgen)
     - Use feature-specific rules for focused development
   - Managing context across worktrees (each worktree can have different active rules)
   - Using aliases for cross-project context
   - Workspace organization strategies

9. **Data Flow Diagrams**
   - Context generation pipeline
   - Workspace discovery and alias resolution flow
   - grove-gemini automatic context workflow
   - grove-flow job execution with context

## Examples Required
- Complete workflows for each major integration
- Show command output and file contents at each step
- Show worktree isolation with grove-flow
- Display workspace discovery and alias resolution

## Output Format
Create a comprehensive guide that shows grove-context as the glue connecting different Grove tools through a unified context model. Include concrete examples, command outputs, and explain the value proposition of each integration.
