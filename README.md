# xr

Cross-repository search & management CLI.

`xr` manages multiple repositories as a single workspace using git clones and symlinks, and provides tools to search, inspect, and compare across them.

## Installation

### Homebrew (macOS / Linux)

```sh
brew install kohbis/xr/xr
```

### go install

```sh
go install github.com/kohbis/xr@latest
```

### Shell completion

Cobra provides `xr completion` for **bash**, **zsh**, **fish**, and **powershell**. Typical setups:

```sh
# bash (install bash-completion if completions do not load)
source <(xr completion bash)

# zsh
source <(xr completion zsh)
```

Subcommands and flags are completed automatically. Repository names are completed for `xr tree`, `xr search --repo`, `xr exec --repo`, `xr diff` (and `xr diff file` / `pattern` / `history`), `xr repo sync`, and `xr repo remove`, using the same config as `xr --config` (default: `./repos.yaml`).

## Prerequisites

`xr` shells out to the following external commands at runtime:

| Command | Required | Used by | Purpose |
|---------|----------|---------|---------|
| `git` | **Yes** | `xr init`, `xr repo sync`, `xr repo import`, `xr diff`, `xr diff history` | Clones, branch switching, `git log` / `git diff` in each repo |
| `diff` | Yes (pre-installed) | `xr diff file` | Unified diff output between files across repositories |
| `rg` (ripgrep) | No | `xr search` | Fast search engine; falls back to a built-in implementation if not found |

Install missing tools as needed:

```sh
# git (usually pre-installed)
# macOS
brew install git

# ripgrep (optional but recommended for better search performance)
brew install ripgrep        # macOS
sudo apt install ripgrep    # Debian/Ubuntu
```

## Setup

Copy the example config and edit it:

```sh
cp repos.yaml.example repos.yaml
```

### repos.yaml

```yaml
workspace: ./repos      # directory where repos will be placed
worktrees: ./worktrees  # directory for git worktrees (optional, this is the default)

repositories:
  - name: project-a
    source: git@github.com:user/project-a.git
    branch: main
    path: project-a

  - name: local-lib
    source: /path/to/local-lib  # local path -> symlink
    type: symlink
    path: local-lib
```

## Usage (essentials)

`xr` is designed for multi-repo workflows. Below are a few common “recipes” that show how it can be used in practice.

If you want the full surface area, see `xr --help` and `xr <cmd> --help`.

### Quick reference

| Goal | Command |
|------|---------|
| Match branches | `xr repo sync` |
| Preview sync (no changes) | `xr repo sync --dry-run` |
| Fetch remote + match branches | `xr repo sync --update` |
| Fetch with prune stale refs | `xr repo sync --update --prune` |
| Fetch, pull | `xr repo sync --update` |
| Bootstrap without prompts | `xr repo sync --clone-missing --update` |
| Sync many repos in parallel | `xr repo sync --update -j 8` |
| Import discoveries without prompt | `xr repo import --yes` |
| Run a command in every repo | `xr exec -- go test ./...` |
| Search across repos | `xr search PATTERN` |
| Compare a file across repos | `xr diff file PATH` |
| Worktree in selected repos | `xr worktree add BRANCH -r NAME -r NAME` |
| Worktrees of one task | `xr worktree list -b 'feat-x*'` |
| Clean up merged worktrees | `xr worktree prune --gone` |
| Another workspace config | `xr --config PATH repo list` |

### Preview vs execute

Preview without side effects uses `--dry-run` on both commands:

- **`xr repo sync`**: runs by default; add `--dry-run` to preview git operations.
- **`xr repo import`**: prompts before writing; use `--yes` to apply unattended or `--dry-run` to scan without writing.

### Config path: `xr init` vs other commands

- Most commands: global `--config` (default: `./repos.yaml` in the current directory).
- **`xr init` only**: `-f` / `--file` selects the repos.yaml to create or read during setup. Prefer this flag for init rather than combining it with `--config`.

### Automation (CI / agents)

Global flags: `--non-interactive` (disable prompts; fail instead of blocking) and `--yes` (confirm writes or destructive actions).

| Command | Unattended-friendly approach |
|---------|------------------------------|
| `xr repo remove` | Pass repo name(s) and `--force` or `--yes` |
| `xr repo import` | `xr repo import --yes` to apply; `--dry-run` to inspect only |
| `xr repo sync` | Runs by default (often with `--update`); use `--allow-dirty` when dirty repos should proceed without prompts |
| Bootstrap a workspace | `xr repo sync --clone-missing --update` — materializes missing repos without the interactive `xr init` |
| `xr init` | Interactive only; `--non-interactive` returns an error |
| `xr worktree add` | `--repo` is required (it prompts otherwise); add `--create` when the branch is new |
| `xr worktree remove` / `prune --gone` | Pass `--yes`; `--force` additionally discards uncommitted changes |
| Machine-readable output | `--json` on `xr repo list`, `xr repo sync`, `xr search`, `xr exec`, `xr worktree list` / `add` / `remove` / `prune`, and `xr diff file` / `pattern` / `history`; `--report PATH` on `xr repo sync` and the `xr diff` subcommands; `--no-color` globally |

See [`SKILL.md`](./SKILL.md) for agent-oriented detail.

### 1) Bootstrap a workspace from `repos.yaml`

```sh
cp repos.yaml.example repos.yaml
${EDITOR:-vim} repos.yaml
xr init
```

`xr init` is interactive. For CI or agents, use `xr repo sync --clone-missing`
instead: it clones repositories missing from the workspace and recreates missing
symlinks, so a committed `repos.yaml` can be materialized unattended.

```sh
xr repo sync --clone-missing --update --non-interactive --allow-dirty
xr repo sync --clone-missing --dry-run    # preview what would be materialized
```

`xr repo sync` exits non-zero when any repository fails, so a failed clone or
fetch stops a CI pipeline. Skipped repositories are not failures. Add `--json`
to get per-repository status, skip reason or error, and the steps taken instead
of progress output, or `--report sync.json` to write that document to a file
while keeping the terminal output.

On a large workspace, `-j` / `--jobs` syncs several repositories at once. Output
stays grouped and ordered by repository, but each block appears only once that
repository finishes. Concurrent workers cannot share stdin, so `-j` above 1
disables prompts — pass `--allow-dirty` or `--yes` if dirty repositories should
proceed rather than be skipped.

### 2) Inspect repository status across the workspace

```sh
xr repo list
```

### 3) Run one command in every repository

```sh
xr exec -- go test ./...
xr exec -r api -r web -- make lint
xr exec -j 8 -- git fetch --prune
```

The command runs directly, without a shell, so its arguments pass through
unchanged; use `xr exec -- bash -c '...'` when you need a pipeline. Each
repository runs with `XR_REPO_NAME` and `XR_REPO_PATH` set. `xr exec` exits
non-zero if the command failed anywhere, and `--json` reports each repository's
exit code with its captured output.

### 4) Find a pattern across repositories

```sh
xr search \"TODO\"
```

### 5) Compare a file across repos / inspect drift

```sh
xr diff file go.mod
```

### 6) Work on one task across several repositories

A worktree is identified by the pair (repository, branch) — git refuses to check out
the same branch twice, so that pair is the smallest unit that can exist. One task
therefore maps to several worktrees, and a repository needing two pull requests for
the same task simply gets two of them:

```sh
xr worktree add feat-x -r api -r web        # same branch in two repositories
xr worktree add feat-x-followup -r api      # second PR from the same repository
```

Nothing is written to `repos.yaml`; `git worktree list` is the source of truth. Group
the worktrees of one task by naming their branches consistently and filtering:

```sh
xr worktree list -b 'feat-x*'
xr worktree remove 'feat-x*'
```

Worktrees are placed at `<worktrees>/<repo path>/<branch>` (branch names containing
slashes nest as directories). After the pull requests are merged:

```sh
xr repo sync --update --prune               # let git notice the deleted branches
xr worktree prune --gone                    # remove the worktrees left behind
```

### 7) Use `--config` when you manage multiple workspaces

```sh
xr --config /path/to/workspace-a/repos.yaml repo list
xr --config /path/to/workspace-b/repos.yaml repo sync --update
```

## Global Flags

| Flag | Description |
|------|-------------|
| `--config` | Path to config file (default: `repos.yaml` in current directory) |
| `--no-color` | Disable ANSI colors (useful for logs and parsers) |
| `--non-interactive` | Disable prompts; commands fail instead of waiting for input |
| `--yes` | Confirm destructive or write actions without prompting |

Per-command flags: `xr <cmd> --help`.

## For AI Agents

`xr` is designed for use by AI agents managing multi-repository workspaces.

- **Using `xr` as a tool**: see [`SKILL.md`](./SKILL.md) (agent-focused command/flag reference and workflows). If your agent framework supports loading context from stdout, you can also run `xr skill` to print it.
- **Contributing to `xr`**: see [`AGENTS.md`](./AGENTS.md) (architecture, conventions, and CI requirements).
