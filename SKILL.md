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

## Quick reference

| Goal | Command |
|------|---------|
| Match branches | `xr repo sync` |
| Preview sync (no changes) | `xr repo sync --dry-run` |
| Fetch remote + match branches | `xr repo sync --update` |
| Fetch with prune stale refs | `xr repo sync --update --prune` |
| Fetch, pull, and submodules | `xr repo sync --update --submodules` |
| Apply a work plan | `xr repo sync --work NAME` |
| Import discoveries without prompt | `xr repo import --yes` |
| Same via work alias | `xr work checkout NAME` |
| Search across repos | `xr search PATTERN` |
| Compare a file across repos | `xr diff file PATH` |
| Another workspace config | `xr --config PATH repo list` |

**Preview vs execute:** `xr repo sync` runs by default; use `--dry-run` to preview. `xr repo import` prompts before writing; use `--yes` to apply unattended or `--dry-run` to scan only.

**Config paths:** most commands use global `--config`. `xr init` uses `-f` / `--file` for the repos.yaml path during setup (not `--config`).

---

## Commands and What They Enable

### Workspace initialization

```sh
xr init [directory]
xr init -f path/to/repos.yaml [directory]
```

Sets up a workspace from `repos.yaml`: creates the directory, runs `git init`, adds submodules or clones for remote repos, and creates symlinks for local repos. **Interactive only** (multiple prompts; not suitable for unattended CI).

---

### Repository management (`xr repo`)

```sh
xr repo list                            # show all repos with type, branch, path, source
xr repo list --json                     # machine-readable repo status output
xr repo list --work <name>              # filter repos by work plan (.xr/work/<name>.yaml)
xr repo add <name> -s <source>          # add a repo (type inferred from source)
xr repo add <name> -s <url> -b main -t clone  # add as clone on specific branch
xr repo add <name> -s <source> -p sub/dir     # specify relative path in workspace
xr repo remove <name>                   # remove from config and workspace
xr repo remove <name> --force           # skip confirmation prompt
xr repo remove <name> --config-only     # remove from config only, keep files
xr repo sync                            # switch branches (repos.yaml or work plan)
xr repo sync --dry-run                  # preview without changes
xr repo sync <name> [<name>...]         # sync specific repos only
xr repo sync --update                   # fetch, switch branch, and pull latest
xr repo sync --update --submodules      # fetch, pull, and update submodules
xr repo sync --update --prune           # fetch with prune, switch, and pull
xr repo sync --work <name>              # scope sync to repos listed in .xr/work/<name>.yaml
xr repo sync --allow-dirty              # skip dirty-repo prompts (recommended with --non-interactive)
xr repo sync --create-branch-if-missing --update  # create local branch if missing (requires --update)
xr repo import                          # discover repos in workspace dir and add to repos.yaml
xr repo import --yes                    # apply discoveries without prompting
xr repo import --dry-run                # preview discovered repos without writing
```

**Agent use cases:**
- Enumerate the workspace before operating: `xr repo list`
- Add a newly created repo to the workspace: `xr repo add`
- Keep submodules in sync after upstream changes: `xr repo sync --update --submodules`
- Ensure all repos are on their configured branches: `xr repo sync`
- Bring all repos up to date with remote: `xr repo sync --update`
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
xr search --json "pattern"         # machine-readable match output
```

**Agent use cases:**
- Find all usages of a symbol, pattern, or interface across repos
- Locate TODOs, FIXMEs, or deprecated calls workspace-wide
- Narrow scope with `-r` before making changes in a specific repo

---

### Cross-repository comparison (`xr diff`)

```sh
xr diff                        # git diff in each repo (pager disabled)
xr diff -- --stat              # pass extra args to git
xr diff -r project-a           # limit git diff to one repo
xr diff file go.mod            # unified diff of a file across all repos
xr diff pattern "version"      # show where pattern occurs per-repo
xr diff history "fix:"         # search git commit messages across repos
xr diff file go.mod -r a -r b
xr diff history "fix:" --json
xr diff pattern "foo" --report diff-report.json
```

`--json` and `--report` work on `xr diff file`, `pattern`, and `history` only (not default git diff).

**Agent use cases:**
- Compare dependency files (`go.mod`, `package.json`, `Cargo.toml`) to find version skew (`xr diff file`)
- Find which repos have already applied a given change (`xr diff pattern`)
- Audit recent fixes applied across the workspace (`xr diff history`)

---

### Workspace structure (`xr tree`)

```sh
xr tree                            # all repos, depth 1
xr tree project-a                  # single repo
xr tree --depth 2                  # shallower view
xr tree --depth 0                  # unlimited depth
```

**Agent use cases:**
- Understand the layout of an unfamiliar repo before navigating it
- Scope analysis before a cross-repo refactor

---

### .gitignore management (`xr repo gitignore`)

```sh
xr repo gitignore
```

Interactively adds the workspace directory to `.gitignore`. Useful after `xr init` to prevent committing the workspace directory from the parent repo.

---

### Config path override

All commands accept global flags:

```sh
xr --config path/to/repos.yaml <command>
xr --no-color <command>   # disable ANSI colors for machine logs
```

Useful when operating on multiple independent workspaces from the same working directory.

---

### Work plans (`xr work`)

Work plans are YAML files stored under `.xr/work/<name>.yaml`. They scope multi-repo operations and can optionally override per-repo `branch` targets (used by `xr repo sync --work <name>`).

`xr work checkout <name>` is an alias for `xr repo sync --work <name>` and accepts the same sync flags (`--update`, `--submodules`, `--dry-run`, etc.).

```sh
xr work init <name>          # create a work plan from repos.yaml (repo names only)
xr work list                 # list available work plan names
xr work checkout <name>      # same as: xr repo sync --work <name>
xr work checkout <name> --update --submodules
xr work delete <name> --yes  # delete the work plan file
```

Work plan schema:

```yaml
name: example
repos:
  - name: repo-a
  - name: repo-b
    branch: example
```

---

## Structured output (`--json` / `--report`)

| Command | `--json` | `--report` |
|---------|----------|------------|
| `xr repo list` | yes | no |
| `xr search` | yes | no |
| `xr diff file` / `pattern` / `history` | yes | yes |
| `xr diff` (default git diff) | no | no |
| `xr repo sync` | not yet | not yet |

---

## Agent automation

Global flags:

- `--non-interactive` — disable prompts; commands return errors instead of blocking on stdin.
- `--yes` — confirm writes or destructive actions (for example `xr repo import --yes`, `xr repo remove NAME --yes`).

| Command | Unattended pattern |
|---------|-------------------|
| `xr repo remove` | `xr repo remove NAME --yes` (or `--force`) |
| `xr repo import` | `xr repo import --yes` to apply; `--dry-run` to inspect only |
| `xr repo sync` | Runs by default; add `--allow-dirty` when dirty repos should proceed without prompts |
| `xr init` | Interactive only; `--non-interactive` returns an error |

Tips:

- Prefer `--json` on `repo list`, `search`, and `diff file` / `pattern` / `history` when chaining output into other tools.
- Add `--no-color` for stable log parsing.
- For `repo sync`, use `--dry-run` to preview before running without it.
