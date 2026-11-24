package worktree

import (
	"fmt"

	wt "github.com/kohbis/xr/internal/worktree"
	"github.com/spf13/cobra"
)

var (
	pruneRepos  []string
	pruneTags   []string
	pruneMerged bool
	pruneDryRun bool
	pruneJSON   bool
)

var pruneCmd = &cobra.Command{
	Use:   "prune",
	Short: "Drop stale worktree entries and merged worktrees",
	Long: `Run 'git worktree prune' in each repository, dropping the administrative
entries of worktrees whose directory has been deleted outside of git.

With --merged, worktrees whose branch has been merged into the repository's
default branch on origin are removed as well: origin/<branch> from repos.yaml,
or origin/HEAD when no branch is set. Merge commits and squash or rebase merges
are recognized. A branch with commits that did not land, or with no commits of
its own yet, is kept. A branch fast-forwarded into the default branch, or
squashed with further edits made while merging, is not recognized and is kept;
remove it with 'xr worktree remove'.

The check reflects the last fetch, so run 'xr repo sync --update --prune'
first. Removal requires confirmation or --yes, stops at worktrees with
uncommitted changes, and keeps the local branch.

Examples:
  xr worktree prune
  xr repo sync --update --prune && xr worktree prune --merged
  xr worktree prune --merged --dry-run`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		m, err := newManager(cmd)
		if err != nil {
			return err
		}
		repos, err := selectRepos(m, pruneRepos, pruneTags)
		if err != nil {
			return err
		}

		combined := &wt.Result{}

		if pruneMerged {
			merged, skipped, err := m.MergedEntries(repos)
			if err != nil {
				return err
			}
			combined.Outcomes = append(combined.Outcomes, skipped...)

			if len(merged) == 0 && !pruneJSON {
				fmt.Println("No merged worktrees.")
			}
			if len(merged) > 0 {
				if !pruneJSON {
					fmt.Printf("The following worktree(s) are merged into origin's default branch:\n")
					for _, e := range merged {
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
				result, err := m.Remove(merged, false, pruneDryRun)
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
	pruneCmd.Flags().StringArrayVarP(&pruneTags, "tag", "t", nil, "limit to repos with any of these tags (repeatable)")
	pruneCmd.Flags().BoolVar(&pruneMerged, "merged", false, "also remove worktrees whose branch is merged into origin's default branch")
	pruneCmd.Flags().BoolVar(&pruneDryRun, "dry-run", false, "preview only, remove nothing")
	pruneCmd.Flags().BoolVar(&pruneJSON, "json", false, "output in JSON format")
}
