package worktree

import (
	"fmt"

	wt "github.com/kohbis/xr/internal/worktree"
	"github.com/spf13/cobra"
)

var (
	addRepos  []string
	addCreate bool
	addBase   string
	addDryRun bool
	addJSON   bool
)

var addCmd = &cobra.Command{
	Use:   "add <branch>",
	Short: "Create a worktree for a branch in the selected repositories",
	Long: `Create a worktree for <branch> in each selected repository.

The branch is resolved per repository:
  - existing local branch      → checked out in the new worktree
  - existing origin/<branch>   → local tracking branch is created
  - neither                    → error, unless --create is given

With --create the branch is created from --base, defaulting to the branch
configured for the repository in repos.yaml, then to HEAD.

Repositories are selected with --repo, which may be repeated. Without it you are
prompted to pick them, because a task rarely spans every repository in the
workspace; in non-interactive mode --repo is required.

Examples:
  # Same branch in two repositories
  xr worktree add feat-x -r api -r web

  # Second pull request for the same task in one repository
  xr worktree add feat-x-followup -r api --create --base main

  # Preview without creating anything
  xr worktree add feat-x -r api --dry-run`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		branch := args[0]

		m, err := newManager(cmd)
		if err != nil {
			return err
		}
		repos, err := resolveTargetRepos(cmd, m, addRepos)
		if err != nil {
			return err
		}
		if len(repos) == 0 {
			if addJSON {
				return reportResult("worktree add", &wt.Result{}, true)
			}
			fmt.Println("No repositories selected.")
			return nil
		}

		result, err := m.Add(branch, repos, wt.AddOptions{
			Create: addCreate,
			Base:   addBase,
			DryRun: addDryRun,
		})
		if err != nil {
			return err
		}
		return reportResult("worktree add", result, addJSON)
	},
}

func init() {
	addCmd.Flags().StringArrayVarP(&addRepos, "repo", "r", nil, "repository to create the worktree in (repeatable)")
	addCmd.Flags().BoolVar(&addCreate, "create", false, "create the branch when it exists neither locally nor on origin")
	addCmd.Flags().StringVar(&addBase, "base", "", "start point for a branch created by --create (default: repos.yaml branch, then HEAD)")
	addCmd.Flags().BoolVar(&addDryRun, "dry-run", false, "preview only, create no worktrees")
	addCmd.Flags().BoolVar(&addJSON, "json", false, "output in JSON format")
}
