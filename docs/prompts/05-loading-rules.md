# Loading Rules Documentation

You are documenting various ways to load and manage rules in grove-context, including snapshots, defaults, and reset functionality.

## Task
Create documentation covering all methods of loading and managing rules configurations.

## Topics to Cover

1. **Rule Loading Methods**
   - Loading from custom paths with `cx set-rules <path>`
   - Built-in rule templates and defaults

2. **The `cx reset` Command**
   - Resetting to defaults with `cx reset`
   - Default rules discovery determined by grove.yml field
   - When to use reset

3. **Setting Custom Rules (`cx set-rules`)**
   - Loading rules from any file path
   - Switching contexts for different tasks (e.g. use with docgen, or releases)
   - Discuss grove flow frontmatter integration can specify rules file

4. **Snapshots for Rule Management**
   - Saving current configuration with `cx snapshot save`
   - Loading saved configurations with `cx snapshot load`
   - Listing available snapshots with `cx snapshot list`
   - Snapshot storage location and format
   - Sharing snapshots across teams

## Examples Required
- Using `cx reset` to recover from bad configuration
- Setting project-specific rules with `cx set-rules`
- Creating and loading snapshots for different workflows
- Template selection for new projects

