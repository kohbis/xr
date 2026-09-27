---
name: xr
description: >
  Multi-repo workspace CLI. Use this skill whenever the user mentions "xr", or when
  a task involves cross-repository search, diff, comparison, tree visualization,
  adding/removing/listing/syncing workspace repositories, or managing .gitignore
  for a multi-repo workspace. Also use when repos.yaml is referenced.
---

# xr — Agent Skills Reference

What an agent can do with the `xr` CLI across a multi-repository workspace. Flags and examples are on `xr --help` and `xr <cmd> --help`; this file is which command to run, and the constraints that are easy to get wrong. `xr skill` prints this file.

## Workspace model

Repositories are declared in `repos.yaml` and materialized under one workspace directory (default `./repos`).

| Type | How it works | When to use |
|------|-------------|-------------|
| `clone` | `git clone` (default) | remote URL |
| `symlink` | symlink to a local path | repo already on disk |

A source starting with `/` or `~` is a `symlink` when `type` is omitted; any other source is a `clone`. `name` and `path` must be unique (`path` defaults to `name`). Unknown keys are ignored. Omit `branch` to leave that repository's checkout unchanged.

`workspace` and `worktrees` are resolved relative to the config file. Without `--config`, xr uses the nearest `repos.yaml` at or above the working directory, including from inside a managed repository. `xr init` is the exception: it takes `-f` / `--file`, not `--config`.

## Which command

| Goal | Command |
|------|---------|
| Bootstrap unattended | `xr repo sync --clone-missing --update` |
| Match branches / fetch | `xr repo sync` |
| Import repos already on disk | `xr repo import` |
| Add or remove one repo | `xr repo add` / `xr repo remove` |
| List status | `xr repo list` |
| Run a command in each repo | `xr exec -- <command>` |
| Search file contents | `xr search PATTERN` |
| git diff / a file / commit messages | `xr diff` / `file` / `pattern` / `history` |
| Worktrees | `xr worktree` |
| Directory layout | `xr tree` |
| Ignore the workspace directory | `xr repo gitignore` |
| Check tools and config | `xr doctor` |
| Another workspace | `xr --config PATH <command>` |

## Running unattended

- `xr init` prompts and refuses `--non-interactive`. Materialize a committed `repos.yaml` with `xr repo sync --clone-missing`.
- `--non-interactive` fails instead of prompting. `--yes` confirms writes and destructive actions (`repo import`, `repo remove`, `repo gitignore`, `worktree remove`, `worktree prune --gone`).
- `xr repo sync` prompts on a dirty checkout unless `--allow-dirty` or `--yes` is set. `--jobs` above 1 cannot prompt, so pass one of those or dirty repositories are skipped.
- Commands that report per-repository results (`repo sync`, `exec`, `worktree add` / `remove` / `prune`, `search`, `diff pattern`) exit non-zero when any repository failed. A repository missing from the workspace is skipped, not failed. `repo list` always exits 0.
- `xr doctor` exits non-zero only when a required tool is missing or `repos.yaml` cannot be parsed. A workspace that has not been cloned, a missing `rg`, or no config at all are warnings and exit 0.

## Constraints

**`xr exec`** runs the command directly, with no shell. Pipelines need `xr exec -- bash -c '...'`. Each repository gets `XR_REPO_NAME` and `XR_REPO_PATH`.

**`xr search`** searches files git knows about (tracked, plus untracked that are not ignored) and skips binaries. A glob with no separator matches the file name at any depth (`*.go`); one with a separator matches the relative path (`cmd/*.go`). Results are the same whether or not `rg` is installed, and whatever `-j` is set to.

**Worktrees** are the pair `(repository, branch)`. Nothing is stored in `repos.yaml`; group a task with a branch-name glob. `add` without `--repo` prompts. The model, layout, and branch resolution are on `xr worktree --help`. After merges: `xr repo sync --update --prune`, then `xr worktree prune --gone --yes`.

**`xr repo gitignore`** does not write `.gitignore` unless `--yes` is passed or the prompt is answered.

## Structured output

| Command | `--json` | `--report` |
|---------|----------|------------|
| `xr repo list` | yes | no |
| `xr repo sync` | yes | yes |
| `xr search`, `xr exec`, `xr doctor` | yes | no |
| `xr worktree list` / `add` / `remove` / `prune` | yes | no |
| `xr diff file` / `pattern` / `history` | yes | yes |
| `xr diff` (git diff) | no | no |

Prefer `--json` when the next step reads the result. `--no-color` keeps logs free of ANSI sequences. `xr repo sync --json` also disables prompts.
