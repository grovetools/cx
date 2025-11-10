# Documentation Task: Project Overview

You are an expert technical writer. Write a clear, engaging single-page overview for the `grove-context` (`cx`) tool.

## Task
Based on the provided codebase context, create a complete overview

1. **High-level description**
   - What grove-context is: a tool for assembling comprehensive file-based context for LLMs
   - **The primary use case**: Creating detailed development plans by giving LLMs comprehensive context
     - Define the "universe" of relevant files, repos, modules, directories, and notes for a feature
     - Send this large context (100k+ tokens) to APIs with large context windows
     - Get back well-informed implementation plans that can be executed by agents with smaller, focused contexts
   - This is **planning-focused**, not retrieval-focused: embrace large contexts for planning, then break down work
   - Position within the Grove ecosystem as the context engine that enables this workflow

2. **High level ASCII diagram**
   - Visual representation of the context generation flow
   - Keep it simple and clear (5-10 lines)

3. **Animated GIF placeholder**: Include `<!-- placeholder for animated gif -->`

4. **Key features**
   - Identify the most important capabilities from the codebase
   - Focus on what makes grove-context valuable and unique
   - Include both technical capabilities and workflow benefits
   - Consider: pattern matching, workspace awareness, rule management, visual tools, ecosystem integration, security, multi-repo support

5. **How it works**
   - Technical description of the file resolution pipeline
   - Explain the core concepts: rules files, pattern matching, alias resolution, output format
   - Mention security boundaries and file handling

6. **Ecosystem Integration**
   - How grove-context integrates with other Grove tools
   - Should be substantial, showing grove-context's role as infrastructure
   - Cover grove-core, grove-nvim, grove-gemini, and grove-flow integrations

7. **Installation**
   - Dedicated H2 section at the bottom with standardized installation instructions

## Important Considerations

**Primary Use Case - Planning with Comprehensive Context:**
The most important framing is that grove-context enables a **planning → execution workflow**:
1. Assemble comprehensive context defining the "universe" of relevant content for a feature
2. Send to LLM with large context window (Gemini 2M, Claude 200k, etc.)
3. Get back detailed, well-informed implementation plans
4. Execute plans with focused contexts (grove-flow) or smaller agent contexts

This is fundamentally different from RAG/retrieval-based "chat with your codebase" tools. Grove-context embraces large contexts for planning rather than trying to work around context limits.

**Key Capabilities Supporting This:**
- **Declarative universe definition**: `.grove/rules` defines what's relevant, comprehensively
- **Workspace awareness**: Reference files across multiple repos to build complete picture
- **Rule set management**: Different features need different universes (backend-only, full-stack, etc.)
- **Token analytics**: Understand and optimize context size before sending
- **Interactive tools**: `cx view` TUI and grove-nvim for exploring and refining context
- **Ecosystem integration**: Feeds into grove-gemini (API calls) and grove-flow (plan execution)
- **Multi-repo composition**: Essential for modern architectures with 5+ related repositories
- **Security boundaries**: Workspace model prevents accidental inclusion

Let the codebase guide which features deserve prominence, but keep the planning workflow framing central.

## Ecosystem Integration Requirements
Include a dedicated section explaining how grove-context integrates with other Grove tools.

Cover these integrations, letting the codebase determine the emphasis:

- **grove-core**: Workspace discovery and alias resolution system
- **grove-nvim**: Editor integration with real-time feedback
- **grove-gemini**: Automatic context generation for API requests
- **grove-flow**: Per-job context management with worktree support

The section should show grove-context as foundational infrastructure that enables other tools, not just as a standalone utility.

## Installation Section Requirements
Include this condensed installation section at the bottom:

### Installation

Install via the Grove meta-CLI:
```bash
grove install context
```

Verify installation:
```bash
cx version
```

Requires the `grove` meta-CLI. See the [Grove Installation Guide](https://github.com/mattsolo1/grove-meta/blob/main/docs/02-installation.md) if you don't have it installed.

## Context Files to Read
- `README.md`
- `main.go`
- `pkg/context/alias.go` (for alias resolution)
- `pkg/context/manager.go` (for core manager functionality)

## Output Format
Create a well-structured Markdown document that serves as a complete introduction to grove-context.

- Use clear H2 headers for major sections
- Lead with value and use cases, not just technical features
- Include concrete examples where helpful
- Keep it accessible to developers new to Grove while being technically accurate
- Aim for comprehensive but concise (~800-1200 words)

**Critical Framing:**
Position grove-context as the tool that enables **comprehensive planning with large contexts**, not just file selection. The overview should convey:

1. **The planning → execution workflow**: Large context for planning, focused context for execution
2. **Embrace large contexts**: This is the opposite of RAG/retrieval approaches that try to work around context limits
3. **Declarative universe definition**: Define what's relevant, then generate comprehensive context
4. **Foundational infrastructure**: Enables grove-gemini (API calls) and grove-flow (plan execution)

**NOT** just: "a tool to select files for LLMs"
**YES**: "infrastructure for comprehensive planning workflows with large-context LLMs"
