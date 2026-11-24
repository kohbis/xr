---
name: xr
description: >
  Multi-repo workspace CLI. Use this skill whenever the user mentions "xr", or when
  a task involves cross-repository search, diff, comparison, tree visualization,
  running a command in every repository, git worktrees across repositories,
  the pull requests of a task that spans repositories (their state and checks),
  cleaning up worktrees whose work was merged or abandoned,
  adding/removing/listing/syncing workspace repositories, importing the
  repositories of a GitHub organization, checking the workspace setup, or
  managing .gitignore for a multi-repo workspace. Also use when repos.yaml is
  referenced.
---

# xr — Agent Skills Reference

What an agent can do with the `xr` CLI across a multi-repository workspace. Flags and examples are on `xr --help` and `xr <cmd> --help`; this file is which command to run, and the constraints that are easy to get wrong. `xr skill` prints this file.

## Workspace model

Repositories are declared in `repos.yaml` and materialized under one workspace directory (default `./repos`).

| Type | How it works | When to use |
|------|-------------|-------------|
| `clone` | `git clone` (default) | remote URL |
| `symlink` | symlink to a local path | repo already on disk |

A source starting with `/` or `~` is a `symlink` when `type` is omitted; any other source is a `clone`. `name` and `path` must be unique (`path` defaults to `name`). Unknown keys are ignored. Omit `branch` to leave that repository's checkout unchanged. A repository may set optional `tags`; `-t` on `exec`, `search`, `diff`, `repo list`, `repo sync`, and `worktree` keeps repositories that have any of the given tags, and together with `-r` or sync's repo names both filters must match. `worktree add` with `-t` selects the repositories without prompting.

`workspace` and `worktrees` are resolved relative to the config file. Without `--config`, xr uses the nearest `repos.yaml` at or above the working directory, including from inside a managed repository. `xr init` is the exception: it takes `-f` / `--file`, not `--config`.

## Which command

| Goal | Command |
|------|---------|
| Bootstrap unattended | `xr repo sync --clone-missing --update` |
| Match branches / fetch | `xr repo sync` |
| Import repos already on disk | `xr repo import` |
| Import a GitHub org's repos | `xr repo import --github OWNER -r NAME --yes` (`--dry-run` lists names; without `-r` it prompts), then `xr repo sync --clone-missing` |
| Add or remove one repo | `xr repo add -s org/repo` (ssh; `--protocol https`) / `xr repo remove` |
| List status | `xr repo list` |
| Run a command in each repo | `xr exec -- <command>` |
| Search file contents | `xr search PATTERN` |
| git diff / a file / commit messages | `xr diff` / `file` / `pattern` / `history` |
| Worktrees | `xr worktree` |
| Pull request of each worktree | `xr worktree list --pr` |
| Clean up finished worktrees | `xr repo sync --update --prune`, then `xr worktree prune --merged --closed --yes` |
| Run / search / diff in a task's worktrees | `xr exec --worktree GLOB -- <command>` (also `search`, `diff`) |
| Directory layout | `xr tree` |
| Ignore the workspace directory | `xr repo gitignore` |
| Check tools, config, and drift from repos.yaml | `xr doctor` |
| Another workspace | `xr --config PATH <command>` |

## A task across repositories

A task is a branch name shared by the repositories it touches; xr stores nothing else about it.

1. `xr repo sync --update` so origin is current, then `xr worktree add feat-x -r api -r web --create`. The branch starts from `origin/<branch>` of each repository's repos.yaml `branch`.
2. Work and test in the task's worktrees: `xr exec --worktree 'feat-x*' -- go test ./...`, `xr search --worktree 'feat-x*' PATTERN`, `xr diff --worktree 'feat-x*'`.
3. Open pull requests per repository (for example `xr exec --worktree 'feat-x*' -- gh pr create --fill`), then follow them with `xr worktree list -b 'feat-x*' --pr --json`.
4. Once they are merged or closed: `xr repo sync --update --prune`, then `xr worktree prune --merged --closed --yes`.

## Running unattended

- `xr init` prompts and refuses `--non-interactive`. Materialize a committed `repos.yaml` with `xr repo sync --clone-missing`.
- `--non-interactive` fails instead of prompting. `--yes` confirms writes and destructive actions (`repo import`, `repo remove`, `repo gitignore`, `worktree remove`, `worktree prune --merged` / `--closed`).
- `xr repo sync` prompts on a dirty checkout unless `--allow-dirty` or `--yes` is set. `--jobs` above 1 cannot prompt, so pass one of those or dirty repositories are skipped.
- Commands that report per-repository results (`repo sync`, `exec`, `worktree add` / `remove` / `prune`, `search`, `diff pattern`) exit non-zero when any repository failed. A repository missing from the workspace is skipped, not failed. `repo list` always exits 0.
- `xr doctor` exits non-zero only when a required tool is missing or `repos.yaml` cannot be parsed. A workspace that has not been cloned, a missing `rg`, a missing or logged-out `gh`, no config at all, or a repository that drifted from its `repos.yaml` entry (other branch, other `origin`, symlink elsewhere or dangling, not a git repository) are warnings and exit 0. Drift is listed per repository under the `repositories` check (`issues` in `--json`).

## Constraints

**`xr exec`** runs the command directly, with no shell. Pipelines need `xr exec -- bash -c '...'`. Each repository gets `XR_REPO_NAME` and `XR_REPO_PATH`. `--fail-fast` does not start the command in repositories that have not started after one fails; with `--jobs`, repositories already running finish.

**`xr search`** searches files git knows about (tracked, plus untracked that are not ignored) and skips binaries. A glob with no separator matches the file name at any depth (`*.go`); one with a separator matches the relative path (`cmd/*.go`). Results are the same whether or not `rg` is installed, and whatever `-j` is set to.

**Worktrees** are the pair `(repository, branch)`. Nothing is stored in `repos.yaml`; group a task with a branch-name glob. `add` without `--repo` prompts. The model, layout, and branch resolution are on `xr worktree --help`. `add --create` without `--base` starts from `origin/<branch>`, so fetch first (`xr repo sync`) or the branch starts from a stale commit. After merges: `xr repo sync --update --prune`, then `xr worktree prune --merged --yes`. `--merged` removes worktrees whose branch landed in origin's default branch (merge, squash or rebase) and keeps any with commits that did not land. Add `--closed` to also remove worktrees whose pull request was closed without merging.

`exec`, `search` and `diff` (every mode) take `--worktree GLOB` to run on the worktrees of the selected repositories whose branch matches, instead of the workspace checkouts: `xr exec --worktree 'feat-x*' -- go test ./...`. Each worktree is reported as `repo@branch` (also `XR_REPO_NAME` in `exec`); `-r` / `-t` still choose the repositories. A glob that matches nothing runs nothing and exits 0.

**GitHub flags** (`worktree list --pr`, `worktree prune --closed`, `repo import --github`) read GitHub through `gh` and need it installed and logged in. A repository whose origin is not on GitHub is reported, not failed. `XR_NO_GH=1` turns `gh` off: those flags fail and nothing else changes.

**`xr repo gitignore`** does not write `.gitignore` unless `--yes` is passed or the prompt is answered.

## Structured output

| Command | `--json` | `--report` |
|---------|----------|------------|
| `xr repo list` | yes | no |
| `xr repo sync` | yes | yes |
| `xr repo import` | yes (needs `--dry-run` or `--yes`) | no |
| `xr search`, `xr exec`, `xr doctor` | yes | no |
| `xr worktree list` / `add` / `remove` / `prune` | yes | no |
| `xr diff file` / `pattern` / `history` | yes | yes |
| `xr diff` (git diff) | no | no |

Prefer `--json` when the next step reads the result. `--no-color` keeps logs free of ANSI sequences. `xr repo sync --json` also disables prompts.
