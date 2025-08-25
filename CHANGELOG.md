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

