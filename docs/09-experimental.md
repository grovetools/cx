# Experimental Features

This section describes features that are under active development or have specialized use cases. These features may change or be removed in future versions.

## Hot & Cold Context Separation (⚠️ WARNING: EXPERIMENTAL)

**CRITICAL WARNING:** This feature is experimental and can result in substantial costs when used with caching-enabled LLM APIs like Google's Gemini API. It is **not recommended for general use**. Misconfiguration or frequent changes to the cold context can consume a large number of cached tokens.

This feature separates context into two files based on a `---` separator in the `.grove/rules` file.

*   **Hot Context**: Patterns above the separator are resolved into `.grove/context`. This is intended for files that change frequently.
*   **Cold Context**: Patterns below the separator are resolved into `.grove/cached-context`. This is intended for stable files like libraries or dependencies.

The intended function is to send the cold context once to an LLM provider and have it cached. The caching behavior and cost implications are not fully evaluated.

## MCP Integration for Automatic Context Management

`grove-context` can be integrated with `grove-mcp` (Model Context Protocol server), which allows LLM-based agents to interact with local development tools. Through the MCP protocol, an agent can be granted permissions to execute `cx` commands.

This allows an agent to perform actions such as:
*   Analyzing the current context with `cx stats`.
*   Modifying the context rules with `cx edit` or `cx set-rules`.
*   Generating context from recent Git changes using `cx from-git`.

This integration enables an agent to manage its own file-based understanding of a codebase during a task.