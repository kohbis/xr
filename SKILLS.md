---
name: xr
description: >
  Multi-repo workspace CLI. Use this skill whenever the user mentions "xr", or when
  a task involves cross-repository search, diff, comparison, tree visualization,
  adding/removing/listing/syncing workspace repositories, or managing .gitignore
  for a multi-repo workspace. Also use when repos.yaml is referenced.
---

# xr — Agent Skills Reference

This document describes what AI agents can accomplish with the `xr` CLI across a multi-repository workspace. It is focused on capabilities and invocation — not installation or development of xr itself.

## Workspace Model

`xr` manages a set of repositories defined in `repos.yaml`. All repos are materialized under a single `workspace` directory (default: `./repos`). Three repo types exist:

| Type | How it works | When to use |
|------|-------------|-------------|
| `git` | added as a git submodule | remote repo, versioned reference |
| `clone` | plain `git clone` | remote repo, mutable local copy |
| `symlink` | symlink to a local path | local repo already on disk |

Type is auto-inferred: local paths (`/…` or `~…`) → `symlink`; remote URLs → `git`.

---

## Commands and What They Enable

### Workspace initialization

```sh
xr init [directory]
xr init -f path/to/repos.yaml [directory]
```

Sets up a workspace from `repos.yaml`: creates the directory, runs `git init`, adds submodules or clones for remote repos, and creates symlinks for local repos.

---

### Repository management (`xr repo`)

```sh
xr repo list                            # show all repos with type, branch, path, source
xr repo add <name> -s <source>          # add a repo (type inferred from source)
xr repo add <name> -s <url> -b main -t clone  # add as clone on specific branch
xr repo add <name> -s <source> -p sub/dir     # specify relative path in workspace
xr repo remove <name>                   # remove from config and workspace
xr repo remove <name> --force           # skip confirmation prompt
xr repo remove <name> --config-only     # remove from config only, keep files
xr repo update                          # sync all repos (submodule update)
xr repo update <name> [<name>...]       # sync specific repos
xr repo update --pull                   # sync + pull latest from remote
xr repo sync                            # switch to configured branches in repos.yaml
xr repo sync <name> [<name>...]         # sync specific repos only
xr repo sync --fetch --pull             # fetch, switch branch, and pull latest
xr repo sync --fetch --prune --pull     # fetch with prune, switch, and pull
xr repo sync --submodules               # also update submodules recursively
xr repo import                          # discover repos in workspace dir and add to repos.yaml
xr repo import --dry-run                # preview discovered repos without writing
```

**Agent use cases:**
- Enumerate the workspace before operating: `xr repo list`
- Add a newly created repo to the workspace: `xr repo add`
- Keep submodules in sync after upstream changes: `xr repo update --pull`
- Ensure all repos are on their configured branches: `xr repo sync`
- Bring all repos up to date with remote: `xr repo sync --fetch --pull`
- Switch symlink repos to their configured branch: `xr repo sync` (requires `branch` in config)
- Bootstrap a config from an existing workspace on disk: `xr repo import --dry-run`

---

### Cross-repository search (`xr search`)

```sh
xr search <pattern>
xr search -e "func\s+\w+"         # regex
xr search -i "error"               # case-insensitive
xr search -g "*.go" "TODO"         # filter by file glob
xr search -C 3 "panic"             # 3 lines of context
xr search -r project-a "main"      # limit to one repo
xr search -r a -r b "pattern"      # limit to multiple repos
```

**Agent use cases:**
- Find all usages of a symbol, pattern, or interface across repos
- Locate TODOs, FIXMEs, or deprecated calls workspace-wide
- Narrow scope with `-r` before making changes in a specific repo

---

### Cross-repository comparison (`xr diff`)

```sh
xr diff                        # git diff in each repo (pager disabled)
xr diff --pattern "version"        # show where pattern occurs per-repo (no diff output)
xr diff --file go.mod              # unified diff of a file across all repos
xr diff --history "fix:"           # search git commit messages across repos
```

**Agent use cases:**
- Compare dependency files (`go.mod`, `package.json`, `Cargo.toml`) to find version skew
- Find which repos have already applied a given change (`--pattern`)
- Audit recent fixes applied across the workspace (`--history`)

---

### Workspace structure (`xr tree`)

```sh
xr tree                            # all repos, depth 3
xr tree project-a                  # single repo
xr tree --depth 2                  # shallower view
xr tree --depth 0                  # unlimited depth
xr tree --deps                     # highlight dependency files
```

**Agent use cases:**
- Understand the layout of an unfamiliar repo before navigating it
- Identify dependency manifests (`go.mod`, `package.json`, `Cargo.toml`, etc.) with `--deps`
- Scope analysis before a cross-repo refactor

---

### .gitignore management (`xr gitignore`)

```sh
xr gitignore
```

Interactively adds the workspace directory to `.gitignore`. Useful after `xr init` to prevent committing the workspace directory from the parent repo.

---

### Config path override

All commands accept a global flag:

```sh
xr --config path/to/repos.yaml <command>
```

Useful when operating on multiple independent workspaces from the same working directory.
