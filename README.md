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

## Usage (essentials)

`xr` is designed for multi-repo workflows. Below are a few common “recipes” that show how it can be used in practice.

If you want the full surface area, see `xr --help` and `xr <cmd> --help`.

### 1) Bootstrap a workspace from `repos.yaml`

```sh
cp repos.yaml.example repos.yaml
${EDITOR:-vim} repos.yaml
xr init
```

### 2) Inspect repository status across the workspace

```sh
xr repo list
```

### 3) Keep a subset of repos in scope with a work plan

Work plans live at `.xr/work/<name>.yaml`. Start from “all repos”, then delete rows you don’t need, and optionally add `branch` to repos you want to pin.

```sh
xr work init example
${EDITOR:-vim} .xr/work/example.yaml

# preview by default
xr repo sync --work example

# apply when ready
xr repo sync --work example --apply
```

### 4) Find a pattern across repositories

```sh
xr search \"TODO\"
```

### 5) Compare a file across repos / inspect drift

```sh
xr diff --file go.mod
```

### 6) Use `--config` when you manage multiple workspaces

```sh
xr --config /path/to/workspace-a/repos.yaml repo list
xr --config /path/to/workspace-b/repos.yaml repo sync --work example
```

## Global Flags

| Flag | Description |
|------|-------------|
| `--config` | Path to config file (default: `repos.yaml` in current directory) |

## For AI Agents

`xr` is designed for use by AI agents managing multi-repository workspaces.

- **Using `xr` as a tool**: see [`SKILL.md`](./SKILL.md) (agent-focused command/flag reference and workflows). If your agent framework supports loading context from stdout, you can also run `xr skill` to print it.
- **Contributing to `xr`**: see [`AGENTS.md`](./AGENTS.md) (architecture, conventions, and CI requirements).
