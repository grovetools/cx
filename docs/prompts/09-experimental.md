# Experimental Features Documentation

You are documenting experimental and advanced features of grove-context.

## Task
Create a guide on experimental features that are still being refined or have specialized use cases.

## Topics to Cover

1. **Hot & Cold Context Separation (⚠️ WARNING: EXPERIMENTAL)**
   - **CRITICAL WARNING**: This feature can lead to substantial charges when used with Gemini models
   - Not recommended for use
   - May consume excessive cached tokens if TTL misconfigured
   - Brief overview: hot (active) vs cold (reference) context
   - Current implementation is unstable and may change

2. **MCP Integration for Automatic Context Management**
   - `grove-mcp` enables agents to manage context automatically
   - Agents can call `cx` commands through MCP protocol

