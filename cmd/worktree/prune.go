package worktree

import (
	"fmt"

	wt "github.com/kohbis/xr/internal/worktree"
	"github.com/spf13/cobra"
)

var (
	pruneRepos  []string
	pruneGone   bool
	pruneDryRun bool
	pruneJSON   bool
)

var pruneCmd = &cobra.Command{
	Use:   "prune",
	Short: "Drop stale worktree entries",
	Long: `Run 'git worktree prune' in each repository, dropping the administrative
entries of worktrees whose directory has been deleted outside of git.

With --gone, worktrees whose branch tracked a remote branch that no longer
exists are removed as well — the usual leftovers of a merged and deleted pull
request. That check reflects the last fetch, so run
'xr repo sync --update --prune' first. Removal requires confirmation or --yes,
and stops at worktrees with uncommitted changes.

Examples:
  xr worktree prune
  xr repo sync --update --prune && xr worktree prune --gone
  xr worktree prune --gone --dry-run`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		m, err := newManager(cmd)
		if err != nil {
			return err
		}
		repos, err := m.SelectRepos(pruneRepos)
		if err != nil {
			return err
		}

		combined := &wt.Result{}

		if pruneGone {
			gone, err := m.GoneEntries(repos)
			if err != nil {
				return err
			}
			if len(gone) == 0 && !pruneJSON {
				fmt.Println("No worktrees with a gone upstream.")
			}
			if len(gone) > 0 {
				if !pruneJSON {
					fmt.Printf("The following worktree(s) track a branch that is gone from origin:\n")
					for _, e := range gone {
						fmt.Printf("  - %-20s %-24s %s\n", e.Repo, e.Branch, displayPath(e.Path))
					}
				}
				if !pruneDryRun {
					confirmed, err := confirmRemoval(cmd)
					if err != nil {
						return err
					}
					if !confirmed {
						if pruneJSON {
							return reportResult(cmd, "worktree prune", combined, true)
						}
						fmt.Println("Aborted.")
						return nil
					}
				}
				result, err := m.Remove(gone, false, pruneDryRun)
				if err != nil {
					return err
				}
				combined.Outcomes = append(combined.Outcomes, result.Outcomes...)
			}
		}

		result, err := m.Prune(repos, pruneDryRun)
		if err != nil {
			return err
		}
		combined.Outcomes = append(combined.Outcomes, result.Outcomes...)

		return reportResult(cmd, "worktree prune", combined, pruneJSON)
	},
}

func init() {
	pruneCmd.Flags().StringArrayVarP(&pruneRepos, "repo", "r", nil, "limit to a repository (repeatable)")
	pruneCmd.Flags().BoolVar(&pruneGone, "gone", false, "also remove worktrees whose upstream branch is gone from origin")
	pruneCmd.Flags().BoolVar(&pruneDryRun, "dry-run", false, "preview only, remove nothing")
	pruneCmd.Flags().BoolVar(&pruneJSON, "json", false, "output in JSON format")
}
