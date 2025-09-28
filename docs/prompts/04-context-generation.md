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
   - **⚠️ FILESYSTEM ACCESS WARNING**: Patterns can match ANY file on your filesystem
   - Dangerous patterns to avoid (/**/*, ../../../**, ~/**)
   - Always review `cx list` before sharing generated context
   - Sensitive file protection best practices

6. **Integration Points**
   - Feeding context to LLM tools
   - Using with grove-gemini/grove-openai
   - API access to generated context
   - Streaming context for large outputs

## Examples Required
Show the complete flow:
- Rules file → `cx list` output → .grove/context content
- Using `cx stats` to analyze generated context
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
- **ALWAYS** review `cx list` output before using context
- **NEVER** use overly broad patterns without exclusions
- **AVOID** patterns that traverse up directories (../)
- Add sensitive directories to exclusion patterns
- Keep .grove/context in .gitignore
- Be aware that any readable file can be included
- Consider using snapshots for safe, tested configurations

## General Best Practices
- Validating generation with `cx list` before use
- Monitoring context size with `cx stats`
- Understanding binary file exclusion
- Checking for unintended file inclusions
