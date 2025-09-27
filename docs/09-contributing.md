# Contributing to grove-context

This guide explains how to set up a development environment, run the build and test suite, and contribute changes to the grove-context project.

grove-context is a Go command-line tool with unit tests and end-to-end (E2E) tests. Builds and tests are driven by the Makefile; please use the provided make targets.

## Prerequisites

- Go 1.24.4 (matches go.mod and CI)
- Git
- tmux (recommended for running TUI E2E tests via the harness)
- Optional: golangci-lint for local linting

## Building and Testing

Refer to the Makefile and CLAUDE.md for the canonical workflow. Key points:

- Binaries are created in ./bin
- Do not copy binaries elsewhere in your PATH; binaries are managed by the Grove meta-tool
- Use grove list to view active binaries across the ecosystem

Common commands:

```bash
# Build the cx binary into ./bin
make build

# Run unit tests
make test

# Format and static analysis
make fmt
make vet
make lint    # requires golangci-lint installed locally

# E2E tests (builds custom test runner and executes scenarios)
make test-e2e-build
make test-e2e

# Development build with race detector
make dev

# Clean artifacts
make clean

# Show all available targets
make help
```

Notes:

- The E2E test runner builds a binary named tend in ./bin from tests/e2e and executes scenarios. You can pass arguments via ARGS, for example:
  - make test-e2e ARGS="run -i cx-basic-generation"
- The Grove meta-tool automatically discovers binaries in ./bin. See CLAUDE.md for more details.

## Linting

The project uses golangci-lint. Local linting requires installing the tool:

```bash
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
make lint
```

Linter configuration is in .golangci.yml. Enabled linters include (non-exhaustive): govet, errcheck, staticcheck, gofmt, goimports, misspell, gosec, and others. Some rules are tailored for this repo (e.g., exclusions for tests, interface{} tolerances, local import prefixes).

Formatting and vetting:

```bash
make fmt
make vet
```

## Running the CLI during development

```bash
# Build then run with arguments
make run ARGS="version"
make run ARGS="view"
```

The Makefile injects version info (version, commit, branch, build date) at build time via LDFLAGS.

## End-to-End (E2E) Tests

The E2E suite validates interactive flows (including TUIs) and command behavior:

- The custom test runner is built from tests/e2e into ./bin/tend
- The harness may use tmux for interactive TUI testing
- Typical workflow:

```bash
make test-e2e-build
make test-e2e
```

Tips:

- Use tend list (once installed in your environment) to see available scenarios
- The harness manages sessions and recordings; failures will show captured output for diagnostics

## Continuous Integration (CI)

CI runs on GitHub Actions for pushes and pull requests to main. It:

- Uses Go 1.24.4 on ubuntu-latest
- Caches Go modules
- Builds and runs unit and E2E tests
- Disables Git LFS during the workflow to avoid dependency issues

Linting is available via make lint but is currently commented out in the CI workflow; run it locally before submitting a PR.

## Contribution Workflow

1. Fork the repository and clone your fork.
2. Create a feature branch:
   ```bash
   git checkout -b feature/my-change
   ```
3. Make changes and keep commits scoped and clear.
4. Run formatting, vetting, and tests locally:
   ```bash
   make fmt
   make vet
   make lint       # if golangci-lint is installed
   make test
   make test-e2e   # optional but recommended if you changed CLI behavior or TUIs
   ```
5. Push your branch and open a Pull Request against main:
   - Describe the change, rationale, and any trade-offs
   - Note any user-facing changes (CLI behavior, flags, output)
   - Include tests where practical (unit and/or E2E)
6. Address CI feedback and review comments.

### Coding guidelines

- Follow Go formatting and idioms (make fmt, make vet)
- Keep E2E scenarios deterministic; avoid network calls in tests unless explicitly marked or guarded
- When changing TUIs or interactive flows, prefer adding or updating E2E scenarios
- Keep linter warnings clean per .golangci.yml; adjust configuration only when justified

## Troubleshooting

- If you switch branches or encounter odd build/test behavior, run:
  ```bash
  make clean
  go mod tidy
  ```
- Ensure you are using Go 1.24.4.
- For E2E TUI tests, verify tmux is installed and operational.

## Questions and Support

If you have questions about the contribution process or the build/test workflow, open a discussion or issue with details about your environment, steps taken, and observed behavior.