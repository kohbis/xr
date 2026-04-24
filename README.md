# xr

Cross-repository search & management CLI.

`xr` manages multiple repositories as a single workspace using git submodules and symlinks, and provides tools to search, inspect, and compare across them.

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

Subcommands and flags are completed automatically. Repository names are completed for `xr tree`, `xr search --repo`, `xr diff --repo`, `xr repo sync`, and `xr repo remove`, using the same config as `xr --config` (default: `./repos.yaml`).

## Prerequisites

`xr` shells out to the following external commands at runtime:

| Command | Required | Used by | Purpose |
|---------|----------|---------|---------|
| `git` | **Yes** | `xr init`, `xr repo sync`, `xr repo import`, `xr diff`, `xr diff --history` | Repository initialization, branch switching, submodule management, `git log` / `git diff` in each repo |
| `diff` | Yes (pre-installed) | `xr diff --file` | Unified diff output between files across repositories |
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
workspace: ./repos  # directory where repos will be placed

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

## Commands

### `xr init [directory]`

Initialize a workspace. Creates the directory, runs `git init`, adds submodules for remote repos, and creates symlinks for local repos.

```sh
xr init              # initialize in current directory
xr init my-workspace # initialize in ./my-workspace
xr init -f path/to/repos.yaml my-workspace
```

If `repos.yaml` is not found, you will be prompted to either create one interactively or initialize without repos (creates a `README.md` only).

### `xr repo`

Manage repositories in the workspace.

#### `xr repo gitignore`

Interactively add the workspace directory to `.gitignore`.

```sh
xr repo gitignore
```

#### `xr repo list`

List all repositories defined in `repos.yaml`.

```sh
xr repo list
```

Example output:

```
NAME         TYPE      BRANCH  PATH         SOURCE
project-a    git       main    project-a    git@github.com:user/project-a.git
local-lib    symlink           local-lib    /Users/kohbis/workspace/local-lib
```

#### `xr repo sync [repo...]`

Synchronize repositories to match the configuration in `repos.yaml`. Switches branches to match the configured branch, and optionally fetches/pulls latest changes.

```sh
xr repo sync                          # switch to configured branches
xr repo sync --fetch --pull           # fetch, switch branch, and pull
xr repo sync project-a --pull         # sync specific repo with pull
xr repo sync --fetch --prune --pull   # fetch with prune, switch, and pull
xr repo sync --submodules             # also update submodules recursively
```

| Flag | Description |
|------|-------------|
| `--fetch` | Fetch from remote before switching branch |
| `--pull` | Pull latest changes after switching branch |
| `--prune` | Prune deleted remote branches during fetch (requires `--fetch`) |
| `--submodules` | Update submodules recursively after sync |

For symlink repos, branch switching / fetch / pull is performed only when the target is a git repository and `branch` is configured in `repos.yaml`. Symlinks without a configured branch are skipped.

#### `xr repo import`

Import repositories that already exist in the workspace directory into `repos.yaml`. Detects clones, submodules, and symlinks, shows a diff against the current config, and prompts before writing.

```sh
xr repo import            # interactive: preview then confirm
xr repo import --dry-run  # preview only, no writes
```

Example output:

```
Found 2 new repo(s):
  + my-lib               clone    https://github.com/foo/my-lib.git
  + local-tools          symlink  /Users/kohbis/workspace/local-tools

Add these to repos.yaml? [y/N]:
```

### `xr search <pattern>`

Search for a pattern across all repositories.

```sh
xr search "TODO"
xr search -i "error"           # case-insensitive
xr search -e "func\s+\w+"     # regex
xr search -g "*.go" "handler"  # filter by glob
xr search -C 3 "panic"         # show 3 lines of context
xr search -r project-a "main"  # limit to specific repo
```

### `xr diff`

By default runs `git diff` in each repository (pager disabled). Optional arguments after `--` are passed to `git diff`.
Other modes (`--pattern`, `--file`, `--history`) are mutually exclusive.

```sh
xr diff --pattern "version"              # regex: matches per line across repos
xr diff --file go.mod                    # unified diff via the system `diff` command
xr diff --history "fix:"                 # git log --grep in each repo
xr diff --history "fix:" -r project-a    # same, limited to one repo
xr diff                                  # git diff in each repo (pager disabled)
xr diff -r project-a                     # git diff only in listed repos
xr diff -- --stat                        # pass flags/args to git diff (use -- before them)
```

### `xr tree [repo]`

Display the directory structure of repositories.

```sh
xr tree                # list repos
xr tree project-a      # show tree for a specific repo
xr tree --depth 2      # limit depth
```

## Global Flags

| Flag | Description |
|------|-------------|
| `--config` | Path to config file (default: `repos.yaml` in current directory) |

## For AI Agents

`xr` is designed for use by AI agents managing multi-repository workspaces. [`SKILLS.md`](./SKILLS.md) documents all commands, flags, CI checks, and cross-repo workflow patterns from an agent's perspective. Placing it where your agent framework reads context makes `xr` immediately usable by the agent.
