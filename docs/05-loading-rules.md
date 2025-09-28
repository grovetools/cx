# Loading and Managing Rules

`grove-context` (`cx`) provides a flexible system for managing different rule configurations to suit various development tasks. You can switch between different sets of rules, reset to a project-wide default, or save and load named "snapshots" of your rules file. This allows you to tailor your context precisely for tasks like feature development, documentation generation, or debugging.

This guide covers the three primary mechanisms for managing your `.grove/rules` file:
1.  **Setting Rules**: Loading rules from any external file.
2.  **Resetting Rules**: Restoring the project's default rules.
3.  **Snapshots**: Saving and loading named versions of your rules.

## Setting Rules from an External File (`cx set-rules`)

The `cx set-rules <path>` command is the most direct way to switch between different context definitions. It copies the content of a specified file into `.grove/rules`, making it the active configuration for all subsequent `cx` commands.

This is useful for maintaining separate, task-specific rule sets. For example, you might have one set for API development and another for generating documentation.

#### Use Case: Switching Between Development and Documentation Contexts

Imagine a project with these rule files:
-   `api-dev.rules`: Includes Go source files for API development.
-   `docs-gen.rules`: Includes Markdown files and high-level source code for documentation generation.

**`api-dev.rules`:**
```gitignore
# Rules for API development
**/*.go
!**/*_test.go
!docs/**
```

**`docs-gen.rules`:**
```gitignore
# Rules for documentation generation
*.md
docs/**/*.md
cmd/**/*.go
```

You can switch to the API development context with:
```bash
cx set-rules api-dev.rules
```
This command overwrites `.grove/rules` with the content of `api-dev.rules`. Later, to generate documentation, you can switch contexts:
```bash
cx set-rules docs-gen.rules
```

This workflow ensures you always use the correct context for the task at hand. Other tools in the Grove ecosystem can also be configured to use specific rule files, providing consistent context across different workflows.

## Resetting to Project Defaults (`cx reset`)

The `cx reset` command restores your `.grove/rules` file to a known-good default state. This is useful for clearing out temporary changes (like those from `cx from-git`) or recovering from a misconfiguration.

The command behaves in one of two ways:

1.  **Project-Defined Default**: If your `grove.yml` specifies a `default_rules_path`, `cx reset` will copy the content of that file to `.grove/rules`.
2.  **Boilerplate Default**: If no default is configured, `cx reset` creates a basic boilerplate `.grove/rules` file that includes all files (`*`) and provides commented-out examples.

#### Example: Configuring and Using a Project Default

You can define a default rules file for your project in `grove.yml`:
```yaml
# grove.yml
name: my-project
description: A sample project.
context:
  default_rules_path: .grove/default.rules
```
With this configuration, running `cx reset` will always restore `.grove/rules` from `.grove/default.rules`.

```bash
# Overwrite the current rules with the project's default configuration
# It will prompt for confirmation unless --force is used.
cx reset

# Reset without a confirmation prompt
cx reset --force
```

## Using Snapshots for Versioning Rules

Snapshots allow you to save, list, and load named versions of your `.grove/rules` file. This is ideal for preserving complex configurations, sharing them with a team, or saving your context before starting a task that requires a different set of rules.

Snapshots are stored in the `.grove/context-snapshots/` directory.

#### The Snapshot Workflow

1.  **Save a Configuration**: Once your `.grove/rules` file is configured for a specific task, save it as a snapshot.
    ```bash
    # Save the current rules with a name and an optional description
    cx save feature-x-api --desc "Context for developing the new user API"
    ```

2.  **List Available Snapshots**: To see all saved snapshots, use `cx list-snapshots`.
    ```bash
    cx list-snapshots
    ```
    **Expected Output:**
    ```
    Available snapshots:

    NAME                 DATE         FILES  TOKENS   SIZE      DESCRIPTION
    --------------------------------------------------------------------------------
    feature-x-api        2025-09-26   15     ~2.1k    8.4 KB    Context for developing the new user API
    ```

3.  **Load a Snapshot**: To restore a previously saved configuration, use `cx load`.
    ```bash
    # This overwrites .grove/rules with the content from the snapshot
    cx load feature-x-api
    ```

This workflow makes it simple to switch between complex, version-controlled context configurations without having to manage multiple external rule files manually.