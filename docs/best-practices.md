# Grove Context (cx) — Best Practices

This guide summarizes practical recommendations for defining, maintaining, and using context with grove-context (`cx`). The goal is to keep your rules simple, your context predictable, and token usage manageable.

## 1) Organize Rules for Clarity and Predictability

- Start broad, then refine:
  - Put broad inclusion patterns first, followed by specific exclusions.
  - Keep hot rules above the `---` separator and cold rules below it.
- Prefer directory-scoped patterns over many single-file lines:
  - Use recursive globs (`**`) to include entire directories where appropriate.
  - Avoid over-broad upward traversals (`../../**`); they can unintentionally include large trees.
- Make exclusions explicit and close to related includes:
  - Common exclusions: tests, build outputs, vendored or large data.
- Keep external references intentional:
  - When including files outside your project root, prefer short relative paths (a small number of `../`) or absolute paths you control.
  - Reuse defaults from related projects with `@default: <path>` to avoid duplication and keep consistency.
- Remember precedence: cold rules override hot rules for the same path.

Example structure:
```txt
# Hot context: broad includes first
**/*.go
*.md
!**/*_test.go
!**/vendor/**
!**/dist/**

---
# Cold (cached) context: stable or infrequently changing files
docs/**/*.md
schema/**/*.json
../shared-lib/**/*.go
!../shared-lib/**/internal/**
```

Using project defaults to avoid duplication:
```txt
# Hot context plus rules from grove-core
@default: ../grove-core

---
# Cold context plus rules from grove-flow
@default: ../grove-flow
```

## 2) Manage Context Size Proactively

- Inspect regularly:
  - `cx stats` gives a breakdown of file counts, estimated tokens, largest files, and language distribution.
  - Use `--top N` to see heavy hitters quickly, e.g. `cx stats --top 10`.
  - Note: token counts are estimates based on file size; useful for relative comparisons.
- Monitor in real time during active work:
  - `cx dashboard` shows hot and cold summary stats and auto-refreshes when files change.
  - For non-interactive environments, `cx dashboard --plain` prints a compact text view.
- Keep large assets and generated data out of hot context:
  - Exclude logs, media, build artifacts, vendor directories, and large JSON/data files unless necessary.
- Use cold context for stable background material:
  - The cold file list is available via `cx list-cache`.
- When a file is too large but necessary, look for smaller subsets (e.g., a focused subdirectory, or an interface file) to reduce tokens.

## 3) Use Hot vs. Cold Context Intentionally

- Hot (above `---`):
  - Files you actively read or modify during the task at hand.
  - Entry points, key modules, API surfaces, task-specific docs.
- Cold (below `---`):
  - Stable dependencies, schemas, reference documentation, or libraries that provide background.
  - Materials you want available once but not repeatedly included in every request.
- Precedence rule:
  - If a file matches both hot and cold, it is treated as cold. Place rules accordingly.
- Helpful commands:
  - `cx list` shows the effective hot context.
  - `cx list-cache` lists cold files.
  - `cx view` helps visualize and adjust hot/cold/excluded items interactively.

## 4) Iterate with the Interactive View

- Use `cx view` to:
  - See exactly what’s included, excluded, or omitted.
  - Toggle hot (`h`), cold (`c`), or exclude (`x`) for selected paths.
  - Review statistics and rule snippets without leaving the TUI.
  - Switch to the repository view (Tab) to manage cloned or workspace repositories.

This is the fastest way to catch unintended matches and refine patterns.

## 5) Snapshot Important Configurations

- Treat rules as code and snapshot when you reach a good configuration:
  - Save: `cx save <name> --desc "short purpose or scope"`
  - List: `cx list-snapshots --sort date --desc`
  - Diff: `cx diff <name>` to see what changed since a snapshot
  - Restore: `cx load <name>`
- Practical uses:
  - Maintain separate snapshots for feature branches, bug investigations, or release prep.
  - Keep short descriptions (`--desc`) to make future decisions faster.

## 6) Keep Git-Focused Workflows Tight

- When context is task-specific, start from Git activity:
  - `cx from-git --staged` to include staged files only.
  - `cx from-git --commits N` for recent changes.
  - `cx from-git --branch main..HEAD` for branch diffs.
  - `cx from-git --since "2 days ago"` for temporal ranges.
- These commands produce explicit paths in `.grove/rules`, which is a good starting point to refine with patterns later.

## 7) Version Control Recommendations

- Ignore locally generated files:
  - Add `.grove/` to your `.gitignore` to avoid committing generated context files and transient state.
- Consider committing important snapshots:
  - If you want certain snapshots shared, allowlist that subdirectory while ignoring the rest of `.grove/`.
  - Example `.gitignore` configuration:
    ```gitignore
    # Ignore all of .grove by default
    .grove/

    # Allowlist only snapshots (optional)
    !.grove/context-snapshots/
    !.grove/context-snapshots/*.rules
    !.grove/context-snapshots/*.rules.desc
    ```
  - This keeps your working artifacts out of the repo while preserving named rule sets your team relies on.

## 8) Practical Patterns to Consider

- Typical Go project:
  ```txt
  **/*.go
  !**/*_test.go
  !**/vendor/**
  README.md

  ---
  docs/**/*.md
  schema/**/*.json
  ```
- Mixed code and docs with external dependencies:
  ```txt
  # Hot
  src/**/*.go
  pkg/**/*.go
  README.md
  CHANGELOG.md
  !**/*_test.go

  ---
  # Cold
  ../shared-lib/**/*.go
  docs/**/*.md
  api/**/openapi*.yaml
  ```
- Excluding binaries and builds:
  ```txt
  **/*
  !**/*.exe
  !**/*.dll
  !**/*.so
  !**/dist/**
  !**/build/**
  !**/node_modules/**
  ```

## 9) External Repositories (Optional)

- You can reference Git URLs directly in rules, optionally pinning versions:
  ```txt
  # Include a specific release of a dependency
  https://github.com/org/repo@v1.2.3

  # Exclude test data within that repo
  !**/testdata/**
  !**/*_test.go
  ```
- Manage them with:
  - `cx repo list` to review tracked repos, versions, and audit status.
  - `cx repo sync` to refresh.
  - `cx repo audit <url>` for an interactive audit workflow.
- Prefer pinned versions for reproducibility.

## 10) Troubleshooting and Guardrails

- If rules seem to include too much:
  - Inspect with `cx view`, then refine patterns and exclusions.
  - Check cold overrides: a file appearing in cold may have been matched in both sections.
- If token usage climbs:
  - Run `cx stats --top 10` to identify the largest files by tokens.
  - Move stable, heavy files to cold or exclude them if not required.
- If external paths behave unexpectedly:
  - Prefer explicit absolute paths or a small number of `../`.
  - Validate resolution from the project root with `cx view` and `cx list`.

---

Adopting these practices keeps your contexts targeted, reproducible, and efficient. Start with broad, clear rules, validate with `cx view`, monitor with `cx stats` (and `cx dashboard` when useful), and snapshot meaningful configurations as you refine them.