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

```sh
# bash (install bash-completion if completions do not load)
source <(xr completion bash)

# zsh
source <(xr completion zsh)
```

`xr completion --help` covers fish and powershell.

## Prerequisites

`xr` shells out to `git` and `diff` (both required) and, optionally, `rg`. Without ripgrep, `xr search` uses a built-in fallback and returns the same results. After install, `xr doctor` reports what is missing.

## Setup

```sh
cp repos.yaml.example repos.yaml
```

```yaml
workspace: ./repos       # directory where repos will be placed
worktrees: ./worktrees   # directory for git worktrees (optional, this is the default)

repositories:
  - name: project-a
    source: git@github.com:user/project-a.git
    branch: main

  - name: local-lib
    source: /path/to/local-lib  # local path -> symlink
```

Field rules are commented in [`repos.yaml.example`](./repos.yaml.example). Directories named in the file are resolved relative to the file, so `xr --config PATH ...` behaves the same from any working directory.

## Usage

Flags, examples, and per-command behavior are on `xr --help` and `xr <cmd> --help`.

| Goal | Command |
|------|---------|
| Create the workspace | `xr init` |
| Materialize without prompts | `xr repo sync --clone-missing --update` |
| Match branches | `xr repo sync` |
| Run a command in every repo | `xr exec -- go test ./...` |
| Search | `xr search PATTERN` |
| Compare a file | `xr diff file PATH` |
| Worktree in selected repos | `xr worktree add BRANCH -r NAME` |
| Another workspace | `xr --config PATH repo list` |

`xr init` is interactive and selects its config with `-f` / `--file`. Every other command uses the global `--config`. For CI or agents, bootstrap with `xr repo sync --clone-missing` instead of `init`.

Global flags (`--config`, `--no-color`, `--non-interactive`, `--yes`) are listed by `xr --help`.

## For AI Agents

- **Using `xr`**: see [`SKILL.md`](./SKILL.md). `xr skill` prints it.
- **Contributing to `xr`**: see [`AGENTS.md`](./AGENTS.md).
