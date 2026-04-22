# AGENTS.md

This file provides guidance for AI assistants **contributing to the `xr` codebase** (adding features, fixing bugs, reviewing code). It covers architecture, conventions, and CI requirements for development work.

For using the `xr` CLI as an agent tool across a multi-repository workspace, see @SKILLS.md instead.

## Project Overview

`xr` is a Go CLI tool for managing multiple Git repositories as a single workspace. It uses git submodules, clones, and symlinks to organize repos, and provides cross-repository search, comparison, and tree visualization.

## Repository Structure

```
xr/
├── main.go                  # Entry point, calls cmd.Execute()
├── cmd/                     # CLI commands (Cobra-based)
│   ├── root.go              # Root command, global --config flag
│   ├── search.go            # xr search
│   ├── init.go              # xr init
│   ├── tree.go              # xr tree
│   ├── diff.go              # xr diff
│   ├── helpers.go           # Shared CLI helpers
│   └── repo/                # xr repo subcommands
│       ├── cmd.go           # Parent repo command
│       ├── list.go          # xr repo list
│       ├── add.go           # xr repo add
│       ├── remove.go        # xr repo remove
│       ├── import.go        # xr repo import
│       ├── sync.go          # xr repo sync
│       ├── gitignore.go     # xr repo gitignore
│       └── helpers.go       # Shared repo helpers
├── internal/                # Internal packages (not exported)
│   ├── config/              # repos.yaml loading/saving and data types
│   ├── workspace/           # Workspace initialization and git operations
│   ├── search/              # Cross-repo search (ripgrep + fallback)
│   ├── structure/           # Directory tree analysis and display
│   ├── output/              # ANSI-colored terminal output helpers
│   └── diff/                # File comparison and git history search
├── go.mod                   # Module: github.com/kohbis/xr, Go 1.25.7
├── Makefile                 # Build, test, lint, release targets
├── .golangci.yml            # Linter configuration
├── .goreleaser.yaml         # Release automation (Homebrew + GitHub Releases)
├── .github/workflows/
│   ├── ci.yml               # CI: build, vet, test, lint on push/PR
│   └── release.yml          # Release: triggered by v* tags
└── repos.yaml.example       # Example workspace configuration
```

## Environment Setup

Prerequisites for development:
- **Go 1.25+** — required to build and test
- **golangci-lint** — required for `make lint` and CI
- **git** — required for submodule operations and tests

## Development Workflow

### Building

```sh
make build        # produces ./xr binary
go build ./...    # verify all packages compile
```

### Testing

```sh
make test         # runs go test ./...
go test ./...     # equivalent
```

All logic packages in `internal/` have corresponding `_test.go` files. Tests are table-driven using standard `testing` package. There is no external test framework.

### Linting

```sh
make lint         # runs golangci-lint (check only)
make lint-fix     # same, but apply auto-fixes (linters + formatters such as gofmt)
```

Enabled linters: `errcheck`, `govet`, `ineffassign`, `staticcheck`, `unused`. Enabled formatters: `gofmt`.

All errors must be checked — do not silently discard errors.

### CI

CI runs on every push to `main` and on all pull requests:
1. `go build ./...`
2. `go vet ./...`
3. `go test ./...`
4. `golangci-lint run`

All four must pass before merging.

## Key Conventions

### Error handling

- Always wrap errors with context using `fmt.Errorf("context: %w", err)`.
- Return errors up the call stack; print them at the CLI boundary (`cmd/` layer).
- Never use `panic` for expected error conditions.

### Package boundaries

- `cmd/` contains only CLI wiring (flags, args, output). Business logic belongs in `internal/`.
- `internal/` packages are independent and do not import each other, except `config` which is a shared dependency.
- New commands go in `cmd/`; new logic goes in `internal/`.

### Adding a new command

1. Create `cmd/<name>.go` (or `cmd/<parent>/<name>.go` for subcommands).
2. Define a `*cobra.Command` and register it in the parent command's `init()` or `AddCommand` call.
3. Keep the command file thin: parse flags, call `internal/` functions, handle output.
4. Add the command to the `root.go` (or parent `cmd.go`) `init()` function.

### Adding a new `xr repo` subcommand

1. Create `cmd/repo/<name>.go`.
2. Register the command in `cmd/repo/cmd.go`'s `init()`.

### Config (repos.yaml)

The config is loaded via `internal/config.Load(path)` and saved via `config.Save(path, cfg)`.

Repository types:
- `git` — remote git repo added as a submodule
- `clone` — remote git repo added as a plain clone
- `symlink` — local path added as a symlink

Type inference in `normalize()`: local paths (starting with `/` or `~`) default to `symlink`; otherwise `git`.

### Output

Use helpers from `internal/output` for consistent terminal formatting (colors, headers, warnings). Do not use `fmt.Println` directly for user-visible output in `internal/` packages — return strings or use the output helpers.

### Commit messages

Follow Conventional Commits format as seen in the git log:
```
type(scope): description
```
Common types: `feat`, `fix`, `refactor`, `test`, `docs`, `build`, `chore`.

## Dependencies

Minimal by design. Only two direct dependencies:
- `github.com/spf13/cobra` — CLI framework
- `gopkg.in/yaml.v3` — YAML parsing

Do not add new dependencies without strong justification. Prefer standard library.

## Release Process

Releases are fully automated via GoReleaser triggered by version tags:

```sh
# Create a tag (must be on main, in sync with origin/main)
make tag V=1.2.3

# Push tag to trigger release workflow
git push origin v1.2.3
# or use:
make release V=1.2.3  # tags and pushes in one step
```

The release workflow publishes:
- GitHub Release with archives and checksums
- Homebrew formula to `kohbis/homebrew-xr`

Changelog excludes commits with types `docs`, `test`, and `chore`.

## Scope & Boundaries

- Do not edit generated or vendored files: `dist/`, `go.sum`.
- Do not edit `.goreleaser.yaml` or `.github/workflows/release.yml` unless specifically asked — these affect the public release pipeline.
- `repos.yaml` is user-specific workspace config and should not be committed. Use `repos.yaml.example` for documentation purposes.

## External Runtime Dependencies

`xr` shells out to external tools at runtime:
- `git` — required for `xr init`, `xr repo sync`, `xr repo import`, `xr diff`, `xr diff --history`
- `diff` — required for `xr diff --file` (pre-installed on most systems)
- `rg` (ripgrep) — optional for `xr search`; falls back to built-in implementation if absent
