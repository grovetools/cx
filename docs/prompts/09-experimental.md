# Experimental Features Documentation

You are documenting experimental and advanced features of grove-context.

## IMPORTANT: Start with Prominent Warning

Begin the document with this warning block:

> ⚠️ **WARNING: EXPERIMENTAL FEATURES**
> Features documented in this section are experimental and may:
> - Change or be removed without notice in future versions
> - Have incomplete error handling or edge case coverage
> - Cause unexpected API costs (especially caching features)
> - Lack comprehensive testing in production environments
>
> **Use in production at your own risk.** Monitor costs, behavior, and API usage closely.

## Task
Create a guide on experimental features that are still being refined or have specialized use cases.

## Topics to Cover

1. **Hot/Cold Context Caching**
   - Add a second prominent warning specifically for caching:
   > ⚠️ **CACHING COST WARNING**
   > Improper cache configuration can result in **significant unexpected API costs**.
   > Cache regeneration can cost hundreds of dollars if misconfigured with short TTLs or frequent changes.
   > **Only use caching if you thoroughly understand LLM API pricing models and caching behavior.**

   - **WARNING**: This feature is experimental and can lead to high API costs if misconfigured
   - Using `---` separator to split hot (changing) and cold (stable) context
   - Cold context can be cached by grove-gemini to reduce token costs
   - Caching directives: `@enable-cache`, `@freeze-cache`, `@no-expire`, `@expire-time`
   - **RISKS**:
     - Improper TTL settings can lead to excessive cache regeneration costs
     - Stale cached context if files change unexpectedly
     - Difficult to debug when cache is out of sync
   - **Recommendation**: Only use if you thoroughly understand LLM API caching and costs
   - Requires careful monitoring of cache hit rates and costs
   - Consider this feature unstable and subject to significant changes

2. **MCP Integration for Automatic Context Management**
   - `grove-mcp` enables agents to manage context automatically
   - Agents can call `cx` commands through MCP protocol
   - Allows LLMs to dynamically adjust context during conversations
   - Experimental: agents autonomously managing their own context needs

