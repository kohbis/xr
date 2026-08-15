package worktree

import "github.com/spf13/cobra"

var Cmd = &cobra.Command{
	Use:     "worktree",
	Aliases: []string{"wt"},
	Short:   "Manage git worktrees across repositories",
	GroupID: "workspace",
	Long: `Manage git worktrees for the repositories defined in repos.yaml.

A worktree is identified by the pair (repository, branch) — git refuses to check
out the same branch twice, so that pair is the smallest unit that can exist. One
task therefore maps to several worktrees, and a repository that needs two pull
requests for the same task simply gets two worktrees.

Nothing is recorded in repos.yaml: 'git worktree list' is the source of truth, so
worktrees created or deleted outside xr stay visible. Group worktrees belonging
to one task by naming their branches consistently and filtering by pattern:

  xr worktree add feat-x -r api -r web
  xr worktree add feat-x-followup -r api
  xr worktree list -b 'feat-x*'
  xr worktree remove 'feat-x*'

Worktrees live under the directory configured as 'worktrees' in repos.yaml
(default ./worktrees), at <worktrees>/<repo path>/<branch>. Branch names
containing slashes nest as directories.`,
}

func init() {
	Cmd.AddCommand(addCmd)
	Cmd.AddCommand(listCmd)
	Cmd.AddCommand(removeCmd)
	Cmd.AddCommand(pruneCmd)

	// Every subcommand declares --repo; a registration failure only means shell
	// completion is unavailable, which must not keep the command from running.
	for _, sub := range Cmd.Commands() {
		_ = sub.RegisterFlagCompletionFunc("repo", completeRepoFlag)
	}
}
