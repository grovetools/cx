# Context Generation Documentation

You are documenting how grove-context generates context from rules files, creating both file lists and the .grove/context output.

## Task
Create a comprehensive guide explaining the complete context generation pipeline from rules to output.

## Topics to Cover

1. **Generation Pipeline Overview**
   - The transformation: rules → file list → context output
   - Automatic generation triggers
   - Manual generation with `cx generate`
   - Caching and incremental updates

2. **The `cx list` Command**
   - Viewing which files made it into context
   - Output columns: path, size, tokens
   - Sorting options (size, path, date)
   - Filtering results
   - Exporting lists for analysis

3. **The .grove/context Output File**
   - Location and naming conventions
   - XML-wrapped format structure
   - File delimiters and metadata
   - How LLMs parse this format
   - Size limits and splitting

4. **Context Statistics with `cx stats`**
   - Quick summary metrics
   - Token usage breakdown
   - Language distribution
   - Largest files identification
   - Directory-level analysis

5. **File Handling and Security**
   - Binary files excluded by default (images, executables, archives)
   - Text file detection and processing
   - **Security boundaries**: grove-context only allows file inclusion from:
     - Discovered Grove workspaces (projects found in configured search paths)
     - Files within `~/.grove/` directory
     - This prevents accidental inclusion of arbitrary system files
   - Always review `cx list` before sharing generated context
   - Best practices for sensitive file protection

6. **Integration Points**
   - **grove-gemini Integration**: Automatic context generation
     - Seamless context inclusion in API requests
     - No manual file management required
   - **grove-flow Integration**: Per-job context management
     - Automatic regeneration before oneshot/chat jobs
     - Custom rules files per job
     - Worktree-scoped context
   - **grove-nvim Integration**: Real-time editing feedback
     - Virtual text showing token counts
     - Interactive rule preview
   - Using with other LLM tools via generated files
   - API access to generated context
   - Streaming context for large outputs

## Examples Required
Show the complete flow:
- Rules file → `cx list` output → .grove/context content
- Using `cx stats` to analyze generated context
- Integration example: grove-flow job using custom rules file
- Integration example: grove-gemini automatic context generation
- Debugging generation issues
- Optimizing rules based on output

## Format Specifications
Document the exact .grove/context format:
```
<file path="src/main.go">
// file content here
</file>

<file path="pkg/config.yaml">
# yaml content here
</file>
```

## Security Best Practices
- Review `cx list` output before sharing generated context
- grove-context restricts file access to discovered workspaces and ~/.grove
- Add sensitive directories to exclusion patterns (e.g., secrets/, .env files)
- Keep .grove/context in .gitignore
- Use workspace discovery to manage which projects can be included

## General Best Practices
- Validating generation with `cx list` before use
- Monitoring context size with `cx stats`
- Understanding binary file exclusion
- Checking for unintended file inclusions
