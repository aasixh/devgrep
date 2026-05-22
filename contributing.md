# Contributing to devgrep

Thanks for your interest in contributing to devgrep! This project is built with Go and maintained using GitHub workflows, so we strive for clear issues, clean pull requests, and reliable tests.

## Get started

1. Fork the repository.
2. Clone your fork:
   ```sh
   git clone https://github.com/<your-username>/devgrep.git
   cd devgrep
   ```
3. Create a feature branch:
   ```sh
   git checkout -b feature/your-topic
   ```
4. Install dependencies and tools:
   ```sh
   go mod download
   ```

## Project conventions

- Go version: `go1.24`
- Build with: `make build`
- Test with: `make test`
- Format code with: `gofmt -w .`
- Vet code with: `go vet ./...`
- Optional linting with: `make lint`
- Release checks are run through GitHub Actions and `goreleaser check`.

## What to contribute

- Bug reports and reproduction steps
- Feature requests and enhancements
- Fixes for regressions, crashes, and edge cases
- Improvements to documentation, examples, and UX
- Tests for untested functionality

## Reporting issues

Before opening an issue:

- Search existing issues and pull requests to avoid duplicates.
- Verify the behavior with the latest `main` branch.

A good issue should include:

- A clear summary of the problem or request
- Expected behavior vs actual behavior
- Repro steps or commands
- The platform/OS and Go version
- Any relevant output, logs, or error messages

## Submitting pull requests

1. Fork and branch from `main`.
2. Make your changes and keep them focused.
3. Run formatting and tests locally:
   ```sh
   gofmt -w .
   go vet ./...
   go test -race ./...
   ```
4. If your change affects documentation, update `README.md`, `docs/`, or examples.
5. Push your branch and open a pull request against `main`.

### PR expectations

- Keep the PR title descriptive and scoped.
- Provide a summary of your change and why it is needed.
- Include test coverage for bug fixes and new features.
- Rebase or squash as needed to keep history clear.
- Link related issues when applicable.

## Testing and validation

The repository uses GitHub Actions in `.github/workflows/ci.yml` to verify contributions.

Key checks include:

- `gofmt -l .`
- `go vet ./...`
- `go test -race -coverprofile=coverage.out ./...`
- `make build`

For release-related changes, the `release-check` job also runs `goreleaser check`.

## Documentation updates

If your contribution changes behavior, configuration, or CLI usage, please update:

- `README.md`
- `docs/` files
- examples in `examples/`
- any relevant help text in the `cmd/` package

## Code style

- Follow Go idioms and naming conventions.
- Keep functions small and focused.
- Prefer clear variable names and readable control flow.
- Avoid committing generated or temporary files.

## Thank you

Contributions are what make devgrep better. Whether you send a bug fix, a doc improvement, or a new idea, your help is appreciated.

