# AGENTS.md

This file provides guidance for AI assistants **contributing to the `xr` codebase** (adding features, fixing bugs, reviewing code). It covers architecture, conventions, and CI requirements for development work.

For using the `xr` CLI as an agent tool across a multi-repository workspace, see @SKILL.md instead.

## Project Overview

`xr` is a Go CLI tool for managing multiple Git repositories as a single workspace. It uses git clones and symlinks to organize repos, and provides cross-repository search, comparison, and tree visualization.

## Repository Structure

```
xr/
├── main.go                  # Entry point, calls cmd.Execute()
├── cmd/                     # CLI commands (Cobra-based)
│   ├── repo/                # Repository management commands
│   └── worktree/            # Worktree management commands
├── internal/                # Internal packages (not exported)
│   ├── config/              # repos.yaml loading/saving and data types
│   ├── git/                 # Shared git command/query helpers
│   ├── workspace/           # Workspace initialization and git operations
│   ├── worktree/            # Worktree creation/listing/removal across repos
│   ├── search/              # Cross-repo search (ripgrep + fallback, one file set)
│   ├── runner/              # Cross-repo command execution (xr exec)
│   ├── parallel/            # Ordered concurrent execution shared by --jobs
│   ├── structure/           # Directory tree analysis and display
│   ├── output/              # Human/JSON output helpers and result models
│   ├── exitcode/            # Silent exit-status errors for self-reporting commands
│   ├── diff/                # File comparison and git history search
│   ├── doctor/              # Environment and workspace diagnosis (xr doctor)
│   └── shellcomp/           # Shared repository-name shell completion
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
- `internal/` packages are independent and do not import each other, except for the shared ones: `config` and `git` may be imported anywhere (git wraps the git binary, so nothing else shells out to it directly), and `output` / `parallel` are imported by the packages that render long-running progress (`runner`, `workspace`) — not as a general utility grab-bag.
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

`internal/worktree` manages git worktrees for the configured repositories.

- The unit is the pair `(repository, branch)` — git allows a branch in only one worktree.
  There is deliberately no "task" or "group" entity; grouping is a branch-name filter.
- Nothing is persisted in repos.yaml; `git worktree list --porcelain` is the source of truth.
- `Manager.PathFor` is the only place the layout `<cfg.Worktrees>/<repo.Path>/<branch>` is derived.
- `Manager.RepoDir` resolves symlink repos to their real location before git runs.
- Paths are validated to stay inside the worktree directory; empty parents are removed on cleanup.

### Config (repos.yaml)

The config is loaded via `internal/config.Load(path)` and saved via `config.Save(path, cfg)`. In `cmd/`, use `config.LoadCommand(cmd)` / `config.CommandPath(cmd)` rather than reading the `--config` flag by hand.

Without `--config`, `CommandPath` resolves to the nearest `repos.yaml` at or above the working directory (`config.FindPath`), so commands work from inside a repository of the workspace. The walk does not stop at a repository boundary, since the config sits above the repositories it manages. When no config exists anywhere above, it falls back to `repos.yaml` in the working directory — the path where `xr repo import` would create one.

A loaded config records its own path (`Config.Path`). `workspace` and `worktrees` are resolved relative to that file's directory through `Config.Root()`, `WorkspaceDir()` and `WorktreesDir()`; commands must go through these rather than `filepath.Abs(cfg.Workspace)`, so `xr --config other/repos.yaml ...` behaves the same from any working directory.

Top-level keys:
- `workspace` — directory holding the repositories (default `./repos`)
- `worktrees` — directory holding git worktrees (default `./worktrees`)

Repository types:
- `clone` — remote git repo cloned into the workspace directory (default)
- `symlink` — local path added as a symlink

Type inference in `normalize()`: local paths (starting with `/` or `~`) default to `symlink`; otherwise `clone`.

### Output

Use helpers from `internal/output` for consistent terminal formatting and machine-readable output. The package now provides:
- ANSI-colored output helpers for human-readable CLI output
- `SyncPrinter`, a writer-backed renderer for step-by-step progress (`Header`, `Action`, `OK`, `Skip`, `Fail`) that also records what it rendered, so the same steps can be reported as JSON
- shared result models (`CommandResult`, `RepoResult`) for JSON/report output
- JSON helpers (`PrintJSON`, `WriteJSONFile`) for command output and file reports

Global output controls:
- `--no-color` disables ANSI escape sequences for automation logs.

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

**Current behavior:**
- Global `--non-interactive` and `--yes` on the root command; `internal/interactive` helpers read them via `ShouldPrompt` / `Yes`.
- `xr repo remove`: repo name(s) and `--force` or `--yes` required when not prompting.
- `xr repo import`: `--yes` applies without prompt; `--non-interactive` without `--yes` returns an error; `--dry-run` previews.
- `xr repo sync`: no dirty/checkout prompts when `--non-interactive` or stdin is not a TTY; use `--allow-dirty` when appropriate. `--clone-missing` materializes repositories absent from the workspace (clone repos cloned, symlink repos linked), which is the unattended alternative to the interactive `xr init`. Sync exits non-zero when any repository fails, via `internal/exitcode` so the per-repo summary stays the only output. `--jobs`/`-j` syncs repositories concurrently via `internal/parallel`, which buffers each repository's output and flushes it in configuration order, so concurrency never reorders output. Values above 1 disable prompts, since workers cannot share stdin.
- `xr worktree add`: prompts for repositories when `--repo` is omitted; `--non-interactive` requires `--repo`.
- `xr worktree remove` / `prune --gone`: `--yes` confirms. Note `--force` here keeps git's meaning (discard uncommitted changes), unlike `xr repo remove --force`.
- `xr init`: interactive only; `--non-interactive` returns an error (use `xr repo sync --clone-missing`).
- `xr repo gitignore`: `--yes` adds the entry without prompting; without a prompt available it returns an error rather than leaving `.gitignore` untouched.
- `xr exec`: never prompts. It runs the command directly (no shell), skips repositories missing from the workspace, and exits non-zero via `internal/exitcode` when the command failed anywhere. `--jobs` uses `internal/parallel` to buffer per-repository output and flush it in configuration order.
- `xr search`: never prompts. `--jobs`/`-j` searches repositories concurrently through `parallel.Results`; matches and `OnRepoError` calls stay in configuration order, so `-j` changes only the speed.
- `xr doctor`: never prompts. It diagnoses the environment and exits non-zero only when a required tool is missing or a config exists but cannot be parsed.
- `xr diff`: never prompts. `--jobs`/`-j` applies to every mode (git diff, `file`, `pattern`, `history`); each one collects through `diff.scanRepos`, so results stay in configuration order.

### Exit status

A command that prints per-repository results must exit non-zero when any repository failed, so callers can gate on the exit status without parsing output. Return `exitcode.Failed(cmd)` after printing the summary: it silences cobra's error and usage output and exits with status 1. Repositories missing from the workspace are skipped, not failed. This applies to `repo sync`, `exec`, `worktree add` / `remove` / `prune`, `search` and `diff pattern`; `repo list --json` reports a missing repository as `status: missing` and a broken one as `failed`, but lists always exit 0.

### JSON/report output conventions

Prefer a consistent automation story across commands:
- `--json` for structured stdout output
- `--report <path>` for structured file output when the command produces aggregate results (for example, selected `xr diff` modes)
- include per-repository status and summary counts when applicable

**Current behavior:** `--json` is implemented on `xr repo list`, `xr repo sync`, `xr search`, `xr exec`, `xr worktree list` / `add` / `remove` / `prune`, `xr diff file` / `pattern` / `history`, and `xr doctor`. `--report` is implemented on `xr repo sync` and those `xr diff` subcommands. `xr repo sync --json` sets `SyncOptions.Quiet` and reads per-repository outcomes from `SyncResult.Repos`, which `output.SyncPrinter` records as it prints; it also disables prompts.

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

`xr` shells out to external tools at runtime:
- `git` — required for `xr init`, `xr repo sync`, `xr repo import`, `xr worktree`, `xr diff`, `xr diff history`
- `diff` — required for `xr diff file` (pre-installed on most systems)
- `rg` (ripgrep) — optional for `xr search`; falls back to a built-in implementation if absent

`internal/doctor` is where that list is checked at runtime (`xr doctor`). A tool added here should gain a check there, marked required or optional to match: only a missing required tool or an unparsable config is a failure, while a not-yet-materialized workspace and a missing optional tool are warnings that still exit 0.

### Concurrency (`--jobs`)

`internal/parallel` is the single implementation of "run N items concurrently, keep the order":

- `parallel.Run(n, jobs, stdout, stderr, fn)` gives each item its own buffers and flushes them in index order, so a concurrent run produces byte-identical output to a sequential one.
- `parallel.Results(n, jobs, fn)` is the counterpart for work that returns a value rather than writing output (`internal/search`): the results come back in index order, and the `cmd/` layer reports them itself. Per-repository callbacks such as `search.Options.OnRepoError` must then be invoked from that ordered walk, not from inside a worker.
- Below two effective workers it passes the real streams through, so output still streams live.
- stdout and stderr are buffered separately; redirection must keep working. Subprocess output must be routed to the matching stream (see `output.SyncPrinter.Writer` / `ErrWriter`) rather than folded into one.

New commands that gain `--jobs` should use this package rather than growing their own worker pool.
