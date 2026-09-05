package worktree

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/kohbis/xr/internal/output"
	"github.com/kohbis/xr/internal/shellcomp"
	wt "github.com/kohbis/xr/internal/worktree"
	"github.com/spf13/cobra"
)

var (
	listRepos  []string
	listBranch string
	listJSON   bool
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List worktrees across repositories",
	Long: `List the worktrees of the repositories in repos.yaml.

The main checkout of each repository is not listed — that one is managed by
'xr repo sync'. Worktrees created outside xr are listed too, since the listing
comes from 'git worktree list' rather than from repos.yaml.

Use --branch to reconstruct the view of a single task from a branch naming
convention.

STATUS uses the same markers as 'xr repo list' (* modified, + staged, % untracked,
$ stash, < behind, > ahead), and ! for a worktree whose directory is missing.

Examples:
  xr worktree list
  xr worktree list -b 'feat-x*'
  xr worktree list -r api --json`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		m, err := newManager(cmd)
		if err != nil {
			return err
		}
		repos, err := m.SelectRepos(listRepos)
		if err != nil {
			return err
		}

		entries, err := m.List(repos, listBranch)
		if err != nil {
			return err
		}

		if listJSON {
			// Marshal an empty listing as [] rather than null.
			if entries == nil {
				entries = []wt.Entry{}
			}
			return output.PrintJSON(output.CommandResult{
				Command: "worktree list",
				Summary: map[string]int{"worktrees": len(entries)},
				Data:    map[string]any{"worktrees": entries},
			})
		}

		if len(entries) == 0 {
			fmt.Println("No worktrees found.")
			return nil
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		if _, err := fmt.Fprintln(w, "REPO\tBRANCH\tSTATUS\tPATH"); err != nil {
			return err
		}
		for _, e := range entries {
			branch := e.Branch
			if branch == "" {
				branch = "(detached)"
			}
			if _, err := fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", e.Repo, branch, e.Status, displayPath(e.Path)); err != nil {
				return err
			}
		}
		return w.Flush()
	},
}

func init() {
	listCmd.Flags().StringArrayVarP(&listRepos, "repo", "r", nil, "limit to a repository (repeatable)")
	listCmd.Flags().StringVarP(&listBranch, "branch", "b", "", "filter branches by glob pattern (e.g. 'feat-x*')")
	listCmd.Flags().BoolVar(&listJSON, "json", false, "output in JSON format")
	cobra.CheckErr(listCmd.RegisterFlagCompletionFunc("repo", shellcomp.CompleteRepoNames))
}
