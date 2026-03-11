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

### `xr update [repo...]`

Update repositories in the workspace. Without arguments, updates all repos.

```sh
xr update
xr update project-a
xr update --pull     # also pull latest changes from remote
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

Compare patterns or files across repositories.

```sh
xr diff --pattern "version"    # show where a pattern appears across repos
xr diff --file go.mod          # compare a specific file across repos
xr diff --history "fix:"       # search git commit history
```

### `xr tree [repo]`

Display the directory structure of repositories.

```sh
xr tree                # all repos
xr tree project-a      # specific repo
xr tree --depth 2      # limit depth
xr tree --deps         # highlight dependency files
```

## Global Flags

| Flag | Description |
|------|-------------|
| `--config` | Path to config file (default: `repos.yaml` in current directory) |
