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

