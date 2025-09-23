# Documentation Task: Advanced Topics

Document the advanced features of `cx` that allow for more complex workflows and project structures.

## Task
Create subsections for each of the following advanced topics:

### Reusing Rules with `@default`
- Explain the purpose and syntax of the `@default: <path>` directive.
- Detail how it imports rules from another Grove project's `grove.yml`.
- Clarify how rules are imported depending on whether `@default` is in the hot or cold section.
- Mention that circular dependencies are handled automatically.

### Managing External Repositories
- Explain that Git repository URLs can be added directly to the `.grove/rules` file.
- Describe how `cx` clones and manages these repositories locally (mention `~/.grove/cx/repos`).
- Detail the `cx repo` subcommands: `list`, `sync`, and the interactive `audit` workflow.

### Using Snapshots for Different Tasks
- Explain the concept of snapshots as named versions of your `.grove/rules` file.
- Describe the workflow: `cx save`, `cx load`, `cx list-snapshots`.
- Explain how `cx diff [snapshot]` can be used to compare the current context against a saved snapshot.

