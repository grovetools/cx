## v0.5.0 (2025-10-01)

This release introduces a complete documentation overhaul, replacing the previous content with a new, comprehensive set of guides covering everything from core concepts to advanced workflows (8be1720, 99a7e71). The documentation structure has been standardized with numbered filenames (57de7c0), and the `docgen` configuration has been updated to support automatic Table of Contents generation and syncing the main `README.md` from a template (a101d8f, 83db5c0). The content has also been refined to be more focused and succinct (3647741, da8ba98).

The interactive TUIs have been improved by unifying their styling with the `grove-core` theme package, resulting in a consistent visual experience (b1eccab, 15e7e57). Both the `cx view` and `cx dashboard` commands now use a standardized, multi-context help system, replacing custom help rendering with a full-screen, searchable overlay (7150fca, b50ba80). The `cx view` layout has been refined with descriptive subtitles and stability improvements to eliminate content shifting during scrolling (15e7e57).

A new `@view` directive has been added, allowing repositories to be mounted for browsing in the `cx view` TUI without being automatically included in the context, providing more fine-grained control over external dependencies (66d690f). Additionally, the CI workflow has been updated to use the `branches: [ none ]` syntax to prevent execution on pushes, and the README template format has been cleaned up (a4c67af, a17dab7).

### Features

*   Add a comprehensive new set of documentation guides (8be1720, 99a7e71)
*   Unify TUI styling with the `grove-core` theme package (b1eccab)
*   Improve context visualization UI with theme integration and layout fixes (15e7e57)
*   Migrate `cx view` TUI to a standardized, multi-context help system (7150fca)
*   Implement standardized help component in `cx dashboard` TUI (b50ba80)
*   Add `@view` directive for selectively browsing repositories in `cx view` (66d690f)
*   Add `README.md` generation from a template file (83db5c0)
*   Add Table of Contents generation and update docgen configuration (a101d8f)
*   Refine documentation content to be more succinct and focused (da8ba98, 3647741)

### Bug Fixes

*   Update CI workflow to use `branches: [ none ]` to disable triggers (a4c67af)
*   Clean up `README.md.tpl` template format (a17dab7)
*   Add logo to README (83db5c0)

### Documentation

*   Update docgen configuration and README template for TOC support (dd3629b)
*   Update docgen config and overview prompt (c0e9e5b)
*   Rename documentation sections from Introduction to Overview (85fd237)
*   Simplify installation instructions to point to main Grove guide (66d93a4)

### Chores

*   Standardize documentation filenames to `DD-name.md` convention (57de7c0)
*   Temporarily disable CI workflow (07732e3)

### File Changes

```
 .github/workflows/ci.yml                 |    5 +-
 Makefile                                 |    9 +-
 README.md                                |  186 +----
 cmd/dashboard.go                         |  138 ++--
 cmd/view.go                              |  679 ++++++++----------
 docs/01-overview.md                      |   41 ++
 docs/02-examples.md                      |  173 +++++
 docs/03-rules-and-patterns.md            |  110 +++
 docs/04-context-generation.md            |  147 ++++
 docs/05-loading-rules.md                 |   97 +++
 docs/06-context-tui.md                   |  128 ++++
 docs/07-git-workflows.md                 |  115 +++
 docs/08-external-repositories.md         |  125 ++++
 docs/09-experimental.md                  |   25 +
 docs/10-command-reference.md             |  403 +++++++++++
 docs/README.md.tpl                       |    6 +
 docs/advanced-topics.md                  |  259 -------
 docs/best-practices.md                   |  195 ------
 docs/command-reference.md                |  339 ---------
 docs/contributing.md                     |  154 -----
 docs/core-concepts.md                    |  161 -----
 docs/docgen.config.yml                   |   97 +--
 docs/docs.rules                          |    2 +-
 docs/getting-started.md                  |  155 -----
 docs/images/grove-context-readme.svg     | 1116 ++++++++++++++++++++++++++++++
 docs/installation.md                     |   30 -
 docs/interactive-tools.md                |  175 -----
 docs/overview.md                         |  108 ---
 docs/prompts/01-overview.md              |   45 ++
 docs/prompts/02-examples.md              |   31 +
 docs/prompts/03-rules-and-patterns.md    |   54 ++
 docs/prompts/04-context-generation.md    |   83 +++
 docs/prompts/05-loading-rules.md         |   36 +
 docs/prompts/06-context-tui.md           |   69 ++
 docs/prompts/07-git-workflows.md         |   22 +
 docs/prompts/08-external-repositories.md |   47 ++
 docs/prompts/09-experimental.md          |   20 +
 docs/prompts/10-command-reference.md     |   49 ++
 docs/prompts/advanced-topics.md          |   23 -
 docs/prompts/best-practices.md           |   16 -
 docs/prompts/command-reference.md        |   17 -
 docs/prompts/contributing.md             |   15 -
 docs/prompts/core-concepts.md            |   24 -
 docs/prompts/getting-started.md          |   12 -
 docs/prompts/installation.md             |   12 -
 docs/prompts/interactive-tools.md        |   22 -
 docs/prompts/overview.md                 |   21 -
 pkg/context/manager.go                   |  156 ++++-
 pkg/context/view.go                      |   19 +-
 pkg/docs/docs.json                       |  352 ++++++++--
 50 files changed, 3871 insertions(+), 2452 deletions(-)
```

## v0.4.0 (2025-09-26)

This release introduces more flexible rules management and standardizes command-line output. A new `cx set-rules <path>` command allows setting the active rules from an external file, and the core logic has been refactored for programmatic context generation (695b3b0). Projects can now define a default rules path in `grove.yml`, which can be used to reset `.grove/rules` with pre-defined rules. A corresponding `cx reset` command has been added to restore these project defaults (a4854db).

The command-line output has been refactored to align with `grove-core`'s logging standards (b93aabc, 352ff85). User-facing messages and progress indicators are now sent to stderr, ensuring that command output to stdout (e.g., from `cx list` or `cx show`) remains clean for scripting and piping (b93aabc). This also includes restoring pretty logging with visual feedback for successful operations (43663e1).

Several bugs have been fixed, including an issue with cold context precedence where hot files were not being correctly excluded (bdc09d0). The interactive TUI (`cx view`) has received layout and scrolling improvements, such as visual scrollbars and corrected panel width calculations to prevent content wrapping issues (c5b8131, 4fa8fc4). The safety validation for adding external paths has also been refined to reduce false positives (bdc09d0).

Finally, a draft set of documentation has been added, covering core concepts, commands, best practices, and contribution guidelines (a240a64, dfbf30b).

### Features

*   Add `cx set-rules` command for setting active rules from an external file (695b3b0)
*   Add `cx reset` command to restore rules to project defaults (a4854db)
*   Support project-default rules via `default_rules_path` in `grove.yml` (a4854db)

### Bug Fixes

*   Resolve cold context precedence to correctly filter hot files (bdc09d0)
*   Improve TUI layout with scrollbars and correct panel widths (c5b8131, 4fa8fc4)
*   Refine TUI safety validation to reduce false positives for external paths (bdc09d0)
*   Remove duplicate structured logging messages (d5dcd54)

### Code Refactoring

*   Integrate `grove-core` logging to separate user messages (stderr) from data output (stdout) (b93aabc)
*   Update logging to use parameterless `NewPrettyLogger` (352ff85)
*   Restore pretty logging with visual feedback for successful operations (43663e1)
*   Refactor core logic to support programmatic context generation (695b3b0)

### Documentation

*   Add draft documentation for commands, concepts, and contributing (a240a64, dfbf30b)

### Chores

*   Update `.gitignore` to track `CLAUDE.md` and ignore `go.work` files (c31d1f9)

### File Changes

```
 .gitignore                               |   7 +
 .grovectx                                |   2 -
 CLAUDE.md                                |  30 ++
 README.md                                |  27 ++
 cmd/diff.go                              |  84 ++++-
 cmd/edit.go                              |  48 ++-
 cmd/fix.go                               |   9 +-
 cmd/fromgit.go                           |  16 +-
 cmd/generate.go                          |  14 +-
 cmd/load.go                              |  15 +-
 cmd/loggers.go                           |  10 +
 cmd/repo.go                              |  57 +--
 cmd/reset.go                             |  93 +++++
 cmd/save.go                              |  15 +-
 cmd/setrules.go                          |  40 ++
 cmd/stats.go                             |   2 +-
 cmd/validate.go                          |   3 +-
 cmd/view.go                              | 244 ++++++++++---
 docs/advanced-topics.md                  | 259 +++++++++++++
 docs/best-practices.md                   | 195 ++++++++++
 docs/command-reference.md                | 339 +++++++++++++++++
 docs/contributing.md                     | 154 ++++++++
 docs/core-concepts.md                    | 161 +++++++++
 docs/docgen.config.yml                   |  59 +++
 docs/docs.rules                          |   1 +
 docs/getting-started.md                  | 155 ++++++++
 docs/installation.md                     |  30 ++
 docs/interactive-tools.md                | 175 +++++++++
 docs/overview.md                         | 108 ++++++
 docs/prompts/advanced-topics.md          |  23 ++
 docs/prompts/best-practices.md           |  16 +
 docs/prompts/command-reference.md        |  17 +
 docs/prompts/contributing.md             |  15 +
 docs/prompts/core-concepts.md            |  24 ++
 docs/prompts/getting-started.md          |  12 +
 docs/prompts/installation.md             |  12 +
 docs/prompts/interactive-tools.md        |  22 ++
 docs/prompts/overview.md                 |  21 ++
 main.go                                  |   2 +
 overview-gemini.md                       |  16 +
 pkg/context/diff.go                      |   8 +-
 pkg/context/manager.go                   | 603 +++++++++++++++++++++++++++----
 pkg/context/manager_test.go              | 234 ++++++++++++
 pkg/docs/docs.json                       |  66 ++++
 tests/e2e/main.go                        |   8 +
 tests/e2e/scenarios_default_directive.go | 379 +++++++++++++++++++
 tests/e2e/scenarios_tui.go               | 465 ++++++++++++++++++++++++
 tests/e2e/test_utils.go                  |  51 +++
 48 files changed, 4136 insertions(+), 210 deletions(-)
```
## v0.3.0 (2025-09-17)

This release introduces a context repository management TUI designed to streamline context management across multiple projects and reference repositories from Github (008b68b), available by running `cx view` and pressint `Tab`. Users can add/remove everything `grove repo list` and `grove ws list`, including worktrees, from the TUI. It also displays audit status and version information for cloned repositories. 

### File Changes

```
 CHANGELOG.md               |   41 ++
 cmd/test_discovery/main.go |   78 +++
 cmd/view.go                | 1240 +++++++++++++++++++++++++++++++++++++++++++-
 go.mod                     |    3 +
 go.sum                     |    6 +
 pkg/context/manager.go     |   86 ++-
 pkg/discovery/cloned.go    |   51 ++
 pkg/discovery/discover.go  |   37 ++
 pkg/discovery/repo.go      |   20 +
 pkg/discovery/workspace.go |  164 ++++++
 10 files changed, 1692 insertions(+), 34 deletions(-)
```

## v0.2.29 (2025-09-17)

The `cx view` command has been significantly enhanced with a new repository management interface, accessible via the `Tab` key (166e0fb). This new view provides a comprehensive list of discovered and cloned repositories, displaying their version and color-coded audit status (5fe09bc). Users can now directly manage a repository's inclusion in the hot or cold context using hotkeys, view audit reports, and filter the repository list (166e0fb, 04abe41).

UI and UX improvements include a new refresh capability in the repository view (ec1ab48) and wider side panels for rules and statistics to improve readability (06aa894). The help menus for both the tree and repository views have also been simplified for better clarity and maintainability (612ecef).

Several bugs have been addressed in the interactive view. The refresh functionality now correctly updates all components, including the rules and statistics panels (9e12bd5). Additionally, the repository filtering mechanism has been made more intuitive, and the quit behavior has been clarified (04abe41).

### Features

*   Enhance cx view with improved repository management and help system (166e0fb)
*   Enhance cx view with audit status and version display for cloned repos (5fe09bc)
*   Add refresh functionality to repository selection view (ec1ab48)
*   Increase width of rules and stats panels in repository view (06aa894)

### Bug Fixes

*   Improve refresh functionality to update all components (9e12bd5)
*   Improve repository filtering behavior and quit functionality (04abe41)

### Code Refactoring

*   Simplify help menu layouts to improve readability (612ecef)

### File Changes

```
 cmd/test_discovery/main.go |   78 +++
 cmd/view.go                | 1240 +++++++++++++++++++++++++++++++++++++++++++-
 go.mod                     |    3 +
 go.sum                     |    6 +
 go.work                    |    7 +
 go.work.sum                |    9 +
 pkg/context/manager.go     |   86 ++-
 pkg/discovery/cloned.go    |   51 ++
 pkg/discovery/discover.go  |   37 ++
 pkg/discovery/repo.go      |   20 +
 pkg/discovery/workspace.go |  164 ++++++
 11 files changed, 1667 insertions(+), 34 deletions(-)
```

## v0.2.29 (2025-09-17)

### Chores

* bump dependencies

### Bug Fixes

* deduplicate paths with different cases in cx view and cx list
* allow explicit inclusion of .grove-worktrees directories in rules

## v0.2.28 (2025-09-13)

### Chores

* update Grove dependencies to latest versions

## v0.2.27 (2025-09-12)

### Bug Fixes

* remove circular dependency by calling gemapi binary instead of importing library

## v0.2.26 (2025-09-12)

### Bug Fixes

* disable GitRepositoryCloneScenario, fails on ci

## v0.2.25 (2025-09-12)

### Chores

* **deps:** bump dependencies
* rm indirect deps

### Features

* **repo:** implement interactive LLM-powered security audit workflow
* **repo:** add Git repository cloning and management functionality

## v0.2.24 (2025-09-11)

### Bug Fixes

* **view:** use absolute paths for external files to prevent false safety warnings

## v0.2.23 (2025-09-06)

### Bug Fixes

* **context:** resolve absolute file paths correctly in rules

## v0.2.22 (2025-09-04)

### Bug Fixes

* **context:** fix XML format and improve test error messages
* **context:** handle absolute directory paths in rules correctly

### Documentation

* **changelog:** update CHANGELOG.md for v0.2.22
* **changelog:** update CHANGELOG.md for v0.2.22

### Chores

* **deps:** sync Grove dependencies to latest versions

## v0.2.22 (2025-09-04)

### Documentation

* **changelog:** update CHANGELOG.md for v0.2.22

### Chores

* **deps:** sync Grove dependencies to latest versions

### Bug Fixes

* **context:** fix XML format and improve test error messages
* **context:** handle absolute directory paths in rules correctly

## v0.2.22 (2025-09-04)

### Bug Fixes

* **context:** handle absolute directory paths in rules correctly

### Chores

* **deps:** sync Grove dependencies to latest versions

## v0.2.21 (2025-08-29)

### Chores

* **deps:** sync Grove dependencies to latest versions

### Bug Fixes

* **cx-view:** implement comprehensive safety measures for context rules

## v0.2.20 (2025-08-28)

### Bug Fixes

* properly show gitignored files and add H hotkey
* increase viewport padding in cx view to prevent content cutoff

### Features

* wrap hot context with XML structure matching cold context format
* add gitignored files toggle and fix worktree exclusion logic
* exclude .grove-worktrees directories from context
* enhance cx view with improved UI and functionality
* add pruning mode to cx view
* add token count display to cx view

### Chores

* **deps:** sync Grove dependencies to latest versions

## v0.2.19 (2025-08-26)

### Chores

* update readme (#2)

## v0.2.18 (2025-08-26)

### Bug Fixes

* improve auto-expansion to show project list at grove-ecosystem level
* exclude empty and git-ignored directories from cx view
* cx view dirs outside proj

### Features

* use changelog content for GitHub releases
* add directory tree inclusion/exclusion to cx view
* add toggle functionality and fix exclusion display in cx view
* add interactive rule modification to cx view command
* improve cx view tree structure and auto-expansion
* add synthetic root to cx view for clearer local/external separation
* consolidate file resolution logic and fix cx view for external directories
* add cx view command for interactive context visualization

## v0.2.17 (2025-08-25)

### Continuous Integration

* add Git LFS disable to release workflow

### Chores

* **deps:** sync Grove dependencies to latest versions

## v0.2.16 (2025-08-25)

### Continuous Integration

* disable linting in workflow

### Chores

* **deps:** bump dependencies

## v0.2.15 (2025-08-25)

### Chores

* **deps:** bump dependencies
* bump dependencies

## v0.2.14 (2025-08-25)

### Bug Fixes

* change cached-context to only include cold files
* set editor working directory to git root in cx edit command
* improve glob pattern matching for recursive ** patterns

### Continuous Integration

* add Git LFS configuration to disable LFS during CI runs

### Features

* add @disable-cache directive to completely disable caching
* add cache control directives @no-expire and @expire-time
* add list for cold cache
* add @freeze-cache directive and plain directory pattern support
* add cached-context generation and improve dashboard
* add live-updating CLI dashboard for context statistics
* show cold context in stats
* add hybrid hot/cold context with rules separator
* add gitignore-compatible exclusion patterns and improve tests

### Chores

* update go.mod tidy

## v0.2.13 (2025-08-15)

### Chores

* **deps:** bump dependencies
* bump deps

## v0.2.12 (2025-08-13)

### Code Refactoring

* standardize E2E binary naming and use grove.yml for binary discovery

### Continuous Integration

* optimize workflows to reduce redundancy and costs
* switch to Linux runners to reduce costs
* consolidate to single test job on macOS
* reduce test matrix to macOS with Go 1.24.4 only

### Chores

* bump tend

## v0.2.11 (2025-08-12)

### Chores

* **deps:** bump dependencies

### Bug Fixes

* handle missing rules files gracefully in the cx generate command and improved test code maintainability

## v0.2.10 (2025-08-08)

### Chores

* **deps:** bump dependencies

### Features

* add comprehensive E2E test scenarios for grove-context
* add grove-tend E2E testing framework integration

### Documentation

* **changelog:** update CHANGELOG.md for v0.2.10

### Bug Fixes

* resolve git branch comparison test failure in CI
* update tests to match new rules-based API and add CI workflows

## v0.2.10 (2025-08-08)

### Features

* add comprehensive E2E test scenarios for grove-context
* add grove-tend E2E testing framework integration

### Bug Fixes

* update tests to match new rules-based API and add CI workflows

### Chores

* **deps:** bump dependencies

## v0.2.9 (2025-08-08)

### Features

* enhance grove context pattern handling

### Chores

* **deps:** bump dependencies

