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

