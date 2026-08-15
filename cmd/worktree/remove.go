package worktree

import (
	"fmt"

	"github.com/kohbis/xr/internal/interactive"
	wt "github.com/kohbis/xr/internal/worktree"
	"github.com/spf13/cobra"
)

var (
	removeRepos  []string
	removeForce  bool
	removeDryRun bool
	removeJSON   bool
)

var removeCmd = &cobra.Command{
	Use:   "remove <branch>",
	Short: "Remove worktrees matching a branch pattern",
	Long: `Remove the worktrees whose branch matches <branch>, which may be a glob.

Without --repo, every repository in repos.yaml is searched, so a whole task can
be cleaned up in one call once its pull requests are merged.

git refuses to remove a worktree with uncommitted changes; --force discards them.
Confirmation is required before anything is removed: answer the prompt, or pass
--yes to skip it (in non-interactive mode --yes is required).

Examples:
  xr worktree remove feat-x
  xr worktree remove 'feat-x*'
  xr worktree remove feat-x -r api --force`,
	Args:              cobra.ExactArgs(1),
	ValidArgsFunction: completeBranches,
	RunE: func(cmd *cobra.Command, args []string) error {
		pattern := args[0]

		m, err := newManager(cmd)
		if err != nil {
			return err
		}
		repos, err := m.SelectRepos(removeRepos)
		if err != nil {
			return err
		}

		entries, err := m.List(repos, pattern)
		if err != nil {
			return err
		}
		if len(entries) == 0 {
			if removeJSON {
				return reportResult("worktree remove", &wt.Result{}, true)
			}
			fmt.Printf("No worktrees match %q.\n", pattern)
			return nil
		}

		if !removeJSON {
			fmt.Printf("The following worktree(s) will be removed:\n")
			for _, e := range entries {
				fmt.Printf("  - %-20s %-24s %s\n", e.Repo, e.Branch, displayPath(e.Path))
			}
		}

		if !removeDryRun {
			confirmed, err := confirmRemoval(cmd)
			if err != nil {
				return err
			}
			if !confirmed {
				if removeJSON {
					return reportResult("worktree remove", &wt.Result{}, true)
				}
				fmt.Println("Aborted.")
				return nil
			}
		}

		result, err := m.Remove(entries, removeForce, removeDryRun)
		if err != nil {
			return err
		}
		return reportResult("worktree remove", result, removeJSON)
	},
}

func confirmRemoval(cmd *cobra.Command) (bool, error) {
	if interactive.Yes(cmd) {
		return true, nil
	}
	shouldPrompt, err := interactive.ShouldPrompt(cmd)
	if err != nil {
		return false, err
	}
	if !shouldPrompt {
		return false, fmt.Errorf("non-interactive remove requires --yes")
	}
	return interactive.YesNo("Proceed", true)
}

func init() {
	removeCmd.Flags().StringArrayVarP(&removeRepos, "repo", "r", nil, "limit to a repository (repeatable)")
	removeCmd.Flags().BoolVarP(&removeForce, "force", "f", false, "remove even with uncommitted changes (discards them)")
	removeCmd.Flags().BoolVar(&removeDryRun, "dry-run", false, "preview only, remove no worktrees")
	removeCmd.Flags().BoolVar(&removeJSON, "json", false, "output in JSON format")
}
