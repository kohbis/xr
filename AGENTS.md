# AGENTS.md

This file provides guidance for AI assistants **contributing to the `xr` codebase** (adding features, fixing bugs, reviewing code). It covers architecture, conventions, and CI requirements for development work.

For using the `xr` CLI as an agent tool across a multi-repository workspace, see @SKILL.md instead.

## Project Overview

`xr` is a Go CLI tool for managing multiple Git repositories as a single workspace. It uses git submodules, clones, and symlinks to organize repos, and provides cross-repository search, comparison, and tree visualization.

## Repository Structure

```
xr/
├── main.go                  # Entry point, calls cmd.Execute()
├── cmd/                     # CLI commands (Cobra-based)
│   ├── repo/                # Repository management commands
│   └── work/                # Work plan commands (.xr/work)
├── internal/                # Internal packages (not exported)
│   ├── config/              # repos.yaml loading/saving and data types
│   ├── git/                 # Shared git command/query helpers
│   ├── work/                # Work plan file schema & path helpers (.xr/work)
│   ├── workspace/           # Workspace initialization and git operations
│   ├── search/              # Cross-repo search (ripgrep + fallback)
│   ├── structure/           # Directory tree analysis and display
│   ├── output/              # Human/JSON output helpers and result models
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
- For git interactions in internal packages, prefer `internal/git` helpers over direct `exec.Command("git", ...)`.

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

#### Sync settings

`Config.Sync` (top-level) and `Repository.Sync` (per-repo) hold a
`*SyncSettings` whose fields are `*bool` for `fetch` / `pull` / `prune` /
`submodules`. A nil pointer means "not configured at this layer".

`cmd/repo/sync.go` builds `workspace.SyncOptions` with `*bool` flags via the
`optBool(cmd, name, value)` helper, which only returns a non-nil pointer when
the CLI flag was explicitly set (`cmd.Flags().Changed(name)`).

`workspace.Sync()` resolves the effective flags **per repository** via
`resolveSyncFlags(repo, cfg.Sync, opts)` with the precedence
**CLI > Repository.Sync > Config.Sync > false**, then forwards a copy of
`SyncOptions` whose pointers are now concrete (`withResolved`) to
`syncSymlink` / `syncClone` / `syncSubmodule` / `syncGitRepo`. Downstream
helpers read flags through `opts.effective()` and treat them as plain bools.

When adding a new sync-related option that should be configurable from
`repos.yaml`, follow the same pattern: add a `*bool` field to `SyncSettings`,
wire it through `resolveSyncFlags` / `withResolved` / `effective`, and add a
matching CLI flag built with `optBool`.

### Output

Use helpers from `internal/output` for consistent terminal formatting and machine-readable output. The package now provides:
- ANSI-colored output helpers for human-readable CLI output
- shared result models (`CommandResult`, `RepoResult`) for JSON/report output
- JSON helpers (`PrintJSON`, `WriteJSONFile`) for command output and file reports

Global output controls:
- `--no-color` disables ANSI escape sequences for automation logs.

Do not use `fmt.Println` directly for user-visible output in `internal/` packages — return strings or use the output helpers.

### Non-interactive and automation flags

When adding/changing commands that prompt users, provide explicit non-interactive behavior:
- `--non-interactive` should disable TTY prompts
- `--yes` should explicitly opt into destructive or confirm-required actions
- in non-interactive mode, commands should return clear errors instead of waiting for input

Current commands with non-interactive support include `xr init`, `xr repo import`, `xr repo remove`, and `xr repo sync`.

### JSON/report output conventions

Prefer a consistent automation story across commands:
- `--json` for structured stdout output
- `--report <path>` for structured file output when the command produces aggregate results (for example, `xr repo sync` and selected `xr diff` modes)
- include per-repository status and summary counts when applicable

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
