# AGENTS.md

This file provides guidance for AI assistants **contributing to the `xr` codebase** (adding features, fixing bugs, reviewing code). It covers architecture, conventions, and CI requirements for development work.

For using the `xr` CLI as an agent tool across a multi-repository workspace, see @SKILL.md instead.

## Project Overview

`xr` is a Go CLI tool for managing multiple Git repositories as a single workspace. It uses git clones and symlinks to organize repos, and provides cross-repository search, comparison, and tree visualization.

## Environment Setup

Prerequisites for development:
- **Go 1.27+** — required to build and test
- **golangci-lint** — required for `make lint` and CI, and must itself be built with Go 1.27 or newer: an older build refuses the module rather than linting it
- **git** — required for clone operations and tests

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
- `internal/` packages are independent and do not import each other, except for the shared ones: `config`, `git` and `pathsafe` may be imported anywhere (git wraps the git binary, so nothing else shells out to it directly; `pathsafe` holds the one check that a derived path stays inside its directory), `output` is imported by the packages that render long-running progress (`runner`, `workspace`), and `parallel` by the ones that implement `--jobs` (`diff`, `runner`, `search`, `workspace`) — none of them as a general utility grab-bag.
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

### Worktrees

`internal/worktree` manages git worktrees. The user-facing model is `xr worktree --help`. In code:

- Do not add a task or group type. The unit is `(repository, branch)`; grouping is a branch-name filter, and `git worktree list --porcelain` is the source of truth. Nothing is written to repos.yaml.
- `Manager.PathFor` is the only place the layout `<cfg.Worktrees>/<repo.Path>/<branch>` is derived.
- `Manager.RepoDir` resolves symlink repos to their real location before git runs.
- Paths are validated to stay inside the worktree directory; empty parents are removed on cleanup.

### Config (repos.yaml)

The config is loaded via `internal/config.Load(path)` and saved via `config.Save(path, cfg)`. In `cmd/`, use `config.LoadCommand(cmd)` / `config.CommandPath(cmd)` rather than reading the `--config` flag by hand.

Without `--config`, `CommandPath` resolves to the nearest `repos.yaml` at or above the working directory (`config.FindPath`), so commands work from inside a repository of the workspace. The walk does not stop at a repository boundary, since the config sits above the repositories it manages. When no config exists anywhere above, it falls back to `repos.yaml` in the working directory — the path where `xr repo import` would create one.

A loaded config records its own path (`Config.Path`). `workspace` and `worktrees` are resolved relative to that file's directory through `Config.Root()`, `WorkspaceDir()` and `WorktreesDir()`; commands must go through these rather than `filepath.Abs(cfg.Workspace)`, so `xr --config other/repos.yaml ...` behaves the same from any working directory.

`normalize()` infers `symlink` for a source starting with `/` or `~` (otherwise `clone`), fills an empty `path` from `name`, rejects a repeated `name` or `path`, ignores unknown keys, and treats an empty `branch` as "do not check out". `workspace` defaults to `./repos`, `worktrees` to `./worktrees`.

### Output

Use `internal/output` for terminal formatting and machine-readable output: ANSI helpers, `SyncPrinter` (streams progress and records the same steps for JSON), `CommandResult` / `RepoResult`, and `PrintJSON` / `WriteJSONFile`. `--no-color` disables ANSI sequences.

`internal/` packages must not write to stdout or stderr on their own — a command with `--json` has to keep stdout clean, and tests should not have to capture pipes. Give the caller the content instead, in whichever of these three shapes fits:

- **return the rendered text**, for pure formatting (`structure.Render`);
- **return a result**, for per-repository work whose outcome the caller reports or serializes (`worktree.Result`, `workspace.ScanResult.Warnings`, `diff.GitDiffResult`, `diff.PatternResult.Error`, `search.Options.OnRepoError`);
- **take an injected writer or printer**, for long-running progress that must stream (`workspace.Workspace.Printer`, the `*output.SyncPrinter` passed through sync).

Failures belong in the returned result as an `Error` field rather than as a placeholder string inside the data (never `"(no git history available)"` in a results slice), so the `cmd/` layer can decide between a warning, a JSON `status: failed`, and the exit status.

### Non-interactive and automation flags

When adding/changing commands that prompt users, provide explicit non-interactive behavior:
- `--non-interactive` (global) disables TTY prompts
- `--yes` (global) opts into destructive or confirm-required actions
- in non-interactive mode, commands should return clear errors instead of waiting for input

Read the flags through `internal/interactive` (`ShouldPrompt` / `Yes`), not by inspecting the flag set in each command. Per-command prompt behavior is on that command's `--help`. Constraints that are easy to break while editing:

- `--jobs` above 1 cannot prompt: workers do not share stdin. Route concurrency through `internal/parallel` (below).
- `xr init` stays interactive. The unattended bootstrap is `xr repo sync --clone-missing`.
- `--force` does not mean the same thing everywhere: `xr repo remove --force` skips confirmation, `xr worktree remove --force` discards uncommitted changes.

### Exit status

A command that prints per-repository results must exit non-zero when any repository failed, so callers can gate on the exit status without parsing output. Return `exitcode.Failed(cmd)` after printing the summary: it silences cobra's error and usage output and exits with status 1. Repositories missing from the workspace are skipped, not failed. This applies to `repo sync`, `exec`, `worktree add` / `remove` / `prune`, `search` and `diff pattern`; `repo list --json` reports a missing repository as `status: missing` and a broken one as `failed`, but lists always exit 0.

### JSON/report output conventions

Prefer a consistent automation story across commands:
- `--json` for structured stdout output
- `--report <path>` for structured file output when the command produces aggregate results (for example, selected `xr diff` modes)
- include per-repository status and summary counts when applicable

Which commands accept `--json` or `--report` is on each command's `--help`. `xr repo sync --json` sets `SyncOptions.Quiet` and reads per-repository outcomes from `SyncResult.Repos`, which `output.SyncPrinter` records as it prints; it also disables prompts.

`internal/search` must return the same matches whichever engine runs. `listFiles` (built on `git.ListFiles`) is the single file set both engines search, the glob and the binary check are applied in Go rather than delegated to ripgrep, and results are sorted per repository because ripgrep answers a batch out of order. ripgrep is invoked with explicit paths, batched to stay inside the argument-size limit, and with `--field-match-separator` / `--field-context-separator` so its output parses unambiguously. When changing either engine, extend `TestSearchRepo_EnginesAgree` rather than only the engine you touched.

Warnings from `internal/` scans must not be printed from inside the package: `internal/search` reports per-repository errors through `Options.OnRepoError` and `internal/diff.SearchPattern` returns them in `PatternResult.Error`, so the `cmd/` layer can keep `--json` output clean. `internal/diff` scans only the files git knows about (`git.ListFiles`: tracked plus untracked, not ignored) and returns results in configuration order.

### Commit messages

Follow Conventional Commits format as seen in the git log:
```
type(scope): description
```
Common types: `feat`, `fix`, `refactor`, `test`, `docs`, `build`, `chore`.

Keep commit messages and pull request descriptions to the change itself. Do not
add tool or session links (for example a Claude Code session URL): they are
noise in the history, and a session link is an internal reference that does not
belong in a public repository. Authorship belongs in a `Co-Authored-By` trailer,
so a pull request description does not need to repeat it.

## Dependencies

Minimal by design. Three direct dependencies:
- `github.com/spf13/cobra` — CLI framework
- `gopkg.in/yaml.v3` — YAML parsing
- `github.com/manifoldco/promptui` — TTY select/input prompts, used only by `internal/interactive` (`xr init`, `xr repo remove`, `xr repo import`, `xr worktree add/remove`)

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

`xr` shells out to `git` and `diff` (required) and `rg` (optional; `xr search` falls back to a built-in engine). `internal/doctor` is where that list is checked (`xr doctor`). A tool added here gains a check there, marked required or optional to match: only a missing required tool or an unparsable config is a failure. A workspace that has not been materialized, and a missing optional tool, are warnings that still exit 0.

### Concurrency (`--jobs`)

`internal/parallel` is the single implementation of "run N items concurrently, keep the order":

- `parallel.Run(n, jobs, stdout, stderr, fn)` gives each item its own buffers and flushes them in index order, so a concurrent run produces byte-identical output to a sequential one.
- `parallel.Results(n, jobs, fn)` is the counterpart for work that returns a value rather than writing output (`internal/search`): the results come back in index order, and the `cmd/` layer reports them itself. Per-repository callbacks such as `search.Options.OnRepoError` must then be invoked from that ordered walk, not from inside a worker.
- Below two effective workers it passes the real streams through, so output still streams live.
- stdout and stderr are buffered separately; redirection must keep working. Subprocess output must be routed to the matching stream (see `output.SyncPrinter.Writer` / `ErrWriter`) rather than folded into one.

New commands that gain `--jobs` should use this package rather than growing their own worker pool.
