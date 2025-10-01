# Loading and Managing Rules

This document describes three mechanisms for managing the `.grove/rules` file: setting rules from an external file, resetting to a default state, and using named snapshots.

## Setting Rules from an External File (`cx set-rules`)

The `cx set-rules <path>` command copies the content of a specified file into `.grove/rules`, making it the active configuration for all subsequent `cx` commands. This is used for maintaining separate, task-specific rule sets.

#### Use Case: Switching Between Rule Sets

A project may have different rule files for different tasks, such as one for API development and another for documentation generation.

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

To switch to the API development context:
```bash
cx set-rules api-dev.rules
```
This command overwrites `.grove/rules` with the content of `api-dev.rules`. To switch to the documentation context:
```bash
cx set-rules docs-gen.rules
```

## Resetting to Project Defaults (`cx reset`)

The `cx reset` command restores the `.grove/rules` file to a default state, overwriting any current rules. This is used to clear temporary changes (e.g., from `cx from-git`) or to revert to a known configuration.

The command functions in one of two ways:

1.  **Project-Defined Default**: If a `default_rules_path` is specified in `grove.yml`, `cx reset` copies the content of that file to `.grove/rules`.
2.  **Boilerplate Default**: If no default is configured, `cx reset` creates a basic `.grove/rules` file that includes all files (`*`) and contains commented-out examples.

#### Example: Configuring and Using a Project Default

A default rules file can be defined in `grove.yml`:
```yaml
# grove.yml
name: my-project
description: A sample project.
context:
  default_rules_path: .grove/default.rules
```
With this configuration, `cx reset` will restore `.grove/rules` from `.grove/default.rules`.

```bash
# Overwrite the current rules with the project's default configuration.
# This will prompt for confirmation.
cx reset

# Reset without a confirmation prompt.
cx reset --force
```

## Using Snapshots for Rule Configurations

Snapshots are named versions of a `.grove/rules` file that can be saved, listed, and loaded. They are stored in the `.grove/context-snapshots/` directory.

#### The Snapshot Workflow

1.  **Save a Configuration (`cx save`)**: Saves the current `.grove/rules` file as a named snapshot.
    ```bash
    # Save the current rules with a name and an optional description.
    cx save feature-x-api --desc "Context for developing the new user API"
    ```

2.  **List Available Snapshots (`cx list-snapshots`)**: Displays all saved snapshots.
    ```bash
    cx list-snapshots
    ```
    **Example Output:**
    ```
    Available snapshots:

    NAME                 DATE         FILES  TOKENS   SIZE      DESCRIPTION
    --------------------------------------------------------------------------------
    feature-x-api        2025-09-26   15     ~2.1k    8.4 KB    Context for developing the new user API
    ```

3.  **Load a Snapshot (`cx load`)**: Restores a previously saved configuration.
    ```bash
    # This overwrites .grove/rules with the content from the snapshot.
    cx load feature-x-api
    ```