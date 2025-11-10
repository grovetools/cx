Generate a comprehensive Command Reference for grove-context.

## Requirements
Create detailed documentation for every `cx` command, organized into logical groups.

## Command Groups to Document

### Core Commands
- `cx generate` - Generate context from rules
- `cx list` - List files in context
- `cx show` - Show context content
- `cx edit` - Edit rules file

### Git Integration
- `cx from-git` - Generate context from git changes
- `cx diff` - Show context differences

### Snapshots
- `cx save` - Save current context configuration
- `cx load` - Load saved context configuration  
- `cx listsnapshots` - List saved snapshots

### Interactive Tools
- `cx view` - Interactive context browser
- `cx stats` - Context statistics

### Repository Management
- `cx repo` - Manage external repositories

### Validation & Maintenance
- `cx validate` - Validate rules file
- `cx fix` - Fix common issues
- `cx reset` - Reset to default state
- `cx setrules` - Set rules programmatically

### Cache Management
- `cx listcache` - List cached contexts

### Utility
- `cx version` - Show version information

## Format for Each Command
1. **Command and Usage**: Full command syntax
2. **Description**: What the command does
3. **Arguments**: Required and optional arguments
4. **Flags**: All available flags with descriptions
5. **Examples**: Practical usage examples
6. **Related Commands**: Links to related functionality